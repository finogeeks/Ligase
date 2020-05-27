// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package processors

import (
	"context"

	"errors"
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/gomodule/redigo/redis"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	//log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type AliasProcessor struct {
	DB            model.RoomServerDatabase
	Cfg           *config.Dendrite
	Filter        *filter.Filter
	Cache         service.Cache
	Repo          *repos.RoomServerCurStateRepo
	Idg           *uid.UidGenerator
	InputAPI      roomserverapi.RoomserverInputAPI
	aliasCacheKey int
	//QueryAPI roomserverapi.RoomserverQueryAPI
}

func (r *AliasProcessor) AllocRoomAlias(
	ctx context.Context,
	request *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	response.AliasExists = false
	exsit, err := r.Cache.AliasExists(request.Alias)
	if err != nil {
		return err
	}
	if exsit {
		response.AliasExists = true
		return nil
	}
	roomID, err := r.DB.GetRoomIDFromAlias(ctx, request.Alias)
	if err != nil {
		return err
	}
	if roomID == "" {
		if err := r.DB.SetRoomAlias(ctx, request.Alias, request.RoomID); err != nil {
			return err
		} else {
			err := r.Cache.SetAlias(request.Alias, request.RoomID, 0)
			if err != nil {
				log.Warnf("AllocRoomAlias set alias:%s room_id:%s to cache err:%v", request.Alias, request.RoomID, err)
			}
		}
	} else {
		response.AliasExists = true
		err := r.Cache.SetAlias(request.Alias, request.RoomID, 0)
		if err != nil {
			log.Warnf("AllocRoomAlias set db alias:%s room_id:%s to cache err:%v", request.Alias, request.RoomID, err)
		}
	}
	return nil
}

func (r *AliasProcessor) SetRoomAlias(
	ctx context.Context,
	request *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	err := r.AllocRoomAlias(ctx, request, response)
	if err != nil {
		return err
	}
	// TODO: Ensure that we send unsent events when if server restarts.
	return r.sendUpdatedAliasesEvent(ctx, request.UserID, request.RoomID)
}

// GetAliasRoomID implements roomserverapi.RoomserverAliasAPI
func (r *AliasProcessor) GetAliasRoomID(
	ctx context.Context,
	request *roomserverapi.GetAliasRoomIDRequest,
	response *roomserverapi.GetAliasRoomIDResponse,
) error {
	roomID, err := r.Cache.GetAlias(request.Alias)
	if err != nil {
		if err != redis.ErrNil {
			return err
		}
	} else {
		if roomID != "" {
			response.RoomID = roomID
			return nil
		}
	}
	// redis not exsit Look up the room ID in the database
	roomID, err = r.DB.GetRoomIDFromAlias(ctx, request.Alias)
	if err != nil {
		return err
	}
	response.RoomID = roomID
	if roomID != "" {
		err = r.Cache.SetAlias(request.Alias, roomID, 0)
		if err != nil {
			log.Warnf("GetAliasRoomID set db alias:%s room_id:%s to cache err:%v", request.Alias, roomID, err)
		}
	}
	return nil
}

// RemoveRoomAlias implements roomserverapi.RoomserverAliasAPI
func (r *AliasProcessor) RemoveRoomAlias(
	ctx context.Context,
	request *roomserverapi.RemoveRoomAliasRequest,
	response *roomserverapi.RemoveRoomAliasResponse,
) error {
	// Look up the room ID in the database
	roomID, err := r.Cache.GetAlias(request.Alias)
	if err != nil {
		if err != redis.ErrNil {
			return err
		}
	}
	if roomID == "" {
		roomID, err = r.DB.GetRoomIDFromAlias(ctx, request.Alias)
		if err != nil {
			return err
		}
	}
	if roomID == "" {
		return errors.New(fmt.Sprintf("not room alias is %s"))
	}
	rs := r.Repo.GetRoomState(ctx, roomID)
	if rs == nil {
		return errors.New(fmt.Sprintf("can't find room:%s state", roomID))
	}
	aliasEv := rs.Alias
	if aliasEv.Sender() == "" || aliasEv.Sender() != request.UserID {
		return errors.New(fmt.Sprintf("user:%s if not room:%s creator", request.UserID, roomID))
	}

	err = r.Cache.DelAlias(request.Alias)
	if err != nil {
		return err
	}

	if err := r.DB.RemoveRoomAlias(ctx, request.Alias); err != nil {
		return err
	}
	// Send an updated m.room.aliases event
	// At this point we've already committed the alias to the database so we
	// shouldn't cancel this request.
	// TODO: Ensure that we send unsent events when if server restarts.
	return r.sendUpdatedAliasesEvent(ctx, request.UserID, roomID)
}

type roomAliasesContent struct {
	Aliases []string `json:"aliases"`
}

// Build the updated m.room.aliases event to send to the room after addition or
// removal of an alias
func (r *AliasProcessor) sendUpdatedAliasesEvent(
	ctx context.Context, userID string, roomID string,
) error {
	domainID, _ := common.DomainFromID(userID)

	builder := gomatrixserverlib.EventBuilder{
		Sender:   userID,
		RoomID:   roomID,
		Type:     "m.room.aliases",
		StateKey: &domainID,
	}

	// Retrieve the updated list of aliases, marhal it and set it as the
	// event's content
	aliases, err := r.DB.GetAliasesFromRoomID(ctx, roomID)
	if err != nil {
		return err
	}
	content := roomAliasesContent{Aliases: aliases}
	err = builder.SetContent(content)
	if err != nil {
		return err
	}

	rs := r.Repo.GetRoomState(ctx, roomID)
	if rs == nil {
		return errors.New("can't find room state")
	}

	// Build the event
	event, err := common.BuildEvent(&builder, domainID, *r.Cfg, r.Idg)
	if err != nil {
		return err
	}

	err = gomatrixserverlib.Allowed(*event, rs)
	if err != nil {
		return err
	}

	rawEvent := &roomserverapi.RawEvent{
		RoomID: roomID,
		Kind:   roomserverapi.KindNew,
		Trust:  true,
		BulkEvents: roomserverapi.BulkEvent{
			Events:  []gomatrixserverlib.Event{*event},
			SvrName: domainID,
		},
	}

	// Send the request
	_, err = r.InputAPI.InputRoomEvents(ctx, rawEvent)
	return err
}
