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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
	//log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type AliasProcessor struct {
	DB            model.RoomServerDatabase
	Cfg           *config.Dendrite
	Filter        *filter.Filter
	Cache         service.LocalCache
	Repo          *repos.RoomServerCurStateRepo
	Idg           *uid.UidGenerator
	InputAPI      roomserverapi.RoomserverInputAPI
	aliasCacheKey int
	//QueryAPI roomserverapi.RoomserverQueryAPI
}

func (r *AliasProcessor) Init() {
	r.aliasCacheKey = r.Cache.Register("aliase")
}

func (r *AliasProcessor) AllocRoomAlias(
	ctx context.Context,
	request *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	if r.Filter.Lookup([]byte(request.Alias)) {
		response.AliasExists = true
		return nil
	}
	r.Filter.Insert([]byte(request.Alias))
	response.AliasExists = false

	// Save the new alias
	if err := r.DB.SetRoomAlias(ctx, request.Alias, request.RoomID); err != nil {
		return err
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

	r.Cache.Put(r.aliasCacheKey, request.Alias, request.RoomID)

	// Send a m.room.aliases event with the updated list of aliases for this room
	// At this point we've already committed the alias to the database so we
	// shouldn't cancel this request.
	// TODO: Ensure that we send unsent events when if server restarts.
	return r.sendUpdatedAliasesEvent(context.TODO(), request.UserID, request.RoomID)
}

// GetAliasRoomID implements roomserverapi.RoomserverAliasAPI
func (r *AliasProcessor) GetAliasRoomID(
	ctx context.Context,
	request *roomserverapi.GetAliasRoomIDRequest,
	response *roomserverapi.GetAliasRoomIDResponse,
) error {
	if r.Filter.Lookup([]byte(request.Alias)) == false {
		return nil
	}

	if val, ok := r.Cache.Get(r.aliasCacheKey, request.Alias); ok {
		response.RoomID = val.(string)
		return nil
	}

	// Look up the room ID in the database
	roomID, err := r.DB.GetRoomIDFromAlias(ctx, request.Alias)
	if err != nil {
		return err
	}

	response.RoomID = roomID
	r.Cache.Put(r.aliasCacheKey, request.Alias, roomID)

	return nil
}

// RemoveRoomAlias implements roomserverapi.RoomserverAliasAPI
func (r *AliasProcessor) RemoveRoomAlias(
	ctx context.Context,
	request *roomserverapi.RemoveRoomAliasRequest,
	response *roomserverapi.RemoveRoomAliasResponse,
) error {
	// Look up the room ID in the database
	var roomID string

	if val, ok := r.Cache.Get(r.aliasCacheKey, request.Alias); ok {
		roomID = val.(string)
	} else {
		id, err := r.DB.GetRoomIDFromAlias(ctx, request.Alias)
		if err != nil {
			return err
		}
		roomID = id
	}

	rs := r.Repo.GetRoomState(roomID)
	if rs == nil {
		return errors.New("can't find room state")
	}

	aliasEv := rs.Alias
	//log.Errorf("%v ", rs)
	if aliasEv.Sender() == "" || aliasEv.Sender() != request.UserID {
		return errors.New("not creator")
	}

	if err := r.DB.RemoveRoomAlias(ctx, request.Alias); err != nil {
		return err
	}
	r.Filter.Delete([]byte(request.Alias))

	// Send an updated m.room.aliases event
	// At this point we've already committed the alias to the database so we
	// shouldn't cancel this request.
	// TODO: Ensure that we send unsent events when if server restarts.
	return r.sendUpdatedAliasesEvent(context.TODO(), request.UserID, roomID)
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

	rs := r.Repo.GetRoomState(roomID)
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
