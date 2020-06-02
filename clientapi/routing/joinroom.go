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

package routing

import (
	"context"
	"net/http"
	"strings"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

// JoinRoomByIDOrAlias implements the "/join/{roomIDOrAlias}" API.
// https://matrix.org/docs/spec/client_server/r0.2.0.html#post-matrix-client-r0-join-roomidoralias
func JoinRoomByIDOrAlias(
	ctx context.Context,
	r *external.PostRoomsJoinByAliasRequest,
	userID string,
	roomIDOrAlias string,
	cfg config.Dendrite,
	federation *fed.Federation,
	rpcCli roomserverapi.RoomserverRPCAPI,
	keyRing gomatrixserverlib.KeyRing,
	cache service.Cache,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	content := make(map[string]interface{}) // must be a JSON object
	if err := json.Unmarshal(r.Content, &content); err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("The request body could not be decoded into valid JSON. " + err.Error())
	}

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)

	content["membership"] = "join"
	content["displayname"] = displayName
	content["avatar_url"] = avatarURL

	req := joinRoomReq{ctx, content, userID, cfg, federation, rpcCli, keyRing}

	if strings.HasPrefix(roomIDOrAlias, "!") {
		return req.joinRoomByID(ctx, roomIDOrAlias, idg)
	}
	if strings.HasPrefix(roomIDOrAlias, "#") {
		return req.joinRoomByAlias(ctx, roomIDOrAlias, idg)
	}
	// return http.StatusBadRequest,
	return http.StatusBadRequest, jsonerror.BadJSON("Invalid first character for room ID or alias")
}

type joinRoomReq struct {
	ctx        context.Context
	content    map[string]interface{}
	userID     string
	cfg        config.Dendrite
	federation *fed.Federation
	rpcCli     roomserverapi.RoomserverRPCAPI
	keyRing    gomatrixserverlib.KeyRing
}

// joinRoomByID joins a room by room ID
func (r joinRoomReq) joinRoomByID(
	ctx context.Context,
	roomID string,
	idg *uid.UidGenerator,
) (int, core.Coder) {

	// A client should only join a room by room ID when it has an invite
	// to the room. If the server is the host of the room then we can
	// lookup the invite and process the request as a normal state event.
	// If the server is not the host of the room the we will need to look up the
	// remote server the invite came from in order to request a join event
	// from that server.

	domainID, err1 := common.DomainFromID(r.userID)
	if err1 != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Room Id must be in the form '!localpart:domain'")
	}

	return r.joinRoom(ctx, roomID, domainID, idg)
}

// joinRoomByAlias joins a room using a room alias.
func (r joinRoomReq) joinRoomByAlias(
	ctx context.Context,
	roomAlias string,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	domainID, err1 := common.DomainFromID(r.userID)
	if err1 != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Room alias must be in the form '#localpart:domain'")
	}

	queryReq := roomserverapi.GetAliasRoomIDRequest{Alias: roomAlias}
	var queryRes roomserverapi.GetAliasRoomIDResponse
	if err := r.rpcCli.GetAliasRoomID(r.ctx, &queryReq, &queryRes); err != nil {
		return httputil.LogThenErrorCtx(r.ctx, err)
	}

	if len(queryRes.RoomID) > 0 {
		return r.joinRoom(ctx, queryRes.RoomID, domainID, idg)
	}
	// If the response doesn't contain a non-empty string, return an error
	return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")
}

func (r joinRoomReq) joinRoomByRemoteAlias(
	ctx context.Context,
	domain, roomAlias string,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	resp, err := r.federation.LookupRoomAlias(domain, roomAlias)
	if err != nil {
		switch x := err.(type) {
		case gomatrix.HTTPError:
			if x.Code == http.StatusNotFound {
				return http.StatusNotFound, jsonerror.NotFound("Room alias not found")
			}
		}
		return httputil.LogThenErrorCtx(r.ctx, err)
	}

	return r.joinRoomByID(ctx, resp.RoomID, idg)
}

func (r joinRoomReq) writeToBuilder(eb *gomatrixserverlib.EventBuilder, roomID string) error {
	eb.Type = "m.room.member"

	err := eb.SetContent(r.content)
	if err != nil {
		return err
	}

	err = eb.SetUnsigned(struct{}{})
	if err != nil {
		return err
	}

	eb.Sender = r.userID
	eb.StateKey = &r.userID
	eb.RoomID = roomID
	eb.Redacts = ""

	return nil
}

func (r joinRoomReq) joinRoom(
	ctx context.Context,
	roomID, domainID string,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	events := []gomatrixserverlib.Event{}

	err := r.rpcCli.QueryRoomState(r.ctx, &queryReq, &queryRes)
	if err != nil {
		domainID, _ := common.DomainFromID(roomID)
		if common.CheckValidDomain(domainID, r.cfg.Matrix.ServerName) == false {
			// resp, err := r.federation.LookupState(domainID, roomID)
			resp, err := r.federation.MakeJoin(domainID, roomID, r.userID, []string{"1"})
			if err != nil {
				return httputil.LogThenErrorCtx(r.ctx, err)
			}
			builder := resp.JoinEvent
			sendDomain, _ := common.DomainFromID(r.userID)
			ev, err := common.BuildEvent(&builder, sendDomain, r.cfg, idg)
			if err != nil {
				log.Errorf("make join build event err %v", err)
				return httputil.LogThenErrorCtx(r.ctx, err)
			}

			resp0, err := r.federation.SendJoin(domainID, roomID, ev.EventID(), *ev)
			if err != nil {
				log.Errorf("send join err %v", err)
				return httputil.LogThenErrorCtx(r.ctx, err)
			}

			events = resp0.StateEvents
			queryRes.InitFromEvents(events)
			log.Infof("joinRoom state :%v", resp)
		} else {
			return httputil.LogThenErrorCtx(r.ctx, err)
		}
	}

	if queryRes.RoomExists == false {
		return httputil.LogThenErrorCtx(r.ctx, err)
	}

	joinEv := queryRes.JoinRule
	rule := common.JoinRulesContent{}
	if err := json.Unmarshal(joinEv.Content(), &rule); err != nil {
		return httputil.LogThenErrorCtx(r.ctx, err)
	}

	if rule.JoinRule != joinRulePublic {
		if _, ok := queryRes.Invite[r.userID]; !ok {
			return http.StatusForbidden, jsonerror.Forbidden("You are not invited to this room.")
		}
	}

	var eb gomatrixserverlib.EventBuilder
	err = r.writeToBuilder(&eb, roomID)
	if err != nil {
		return httputil.LogThenErrorCtx(r.ctx, err)
	}

	event, err := common.BuildEvent(&eb, domainID, r.cfg, idg)
	if err != nil {
		return httputil.LogThenErrorCtx(r.ctx, err)
	}

	err = gomatrixserverlib.Allowed(*event, &queryRes)
	events = append(events, *event)

	if err == nil {
		rawEvent := roomserverapi.RawEvent{
			RoomID: roomID,
			Kind:   roomserverapi.KindNew,
			Trust:  true,
			BulkEvents: roomserverapi.BulkEvent{
				Events:  events,
				SvrName: domainID,
			},
			Query: []string{"join_room", ""},
		}

		_, err = r.rpcCli.InputRoomEvents(ctx, &rawEvent)
		if err != nil {
			return http.StatusForbidden, jsonerror.Forbidden("You are not invited to this room.")
		}

		return http.StatusOK, &external.PostRoomsJoinByAliasResponse{roomID}
	}

	return http.StatusForbidden, jsonerror.Forbidden(err.Error())
}
