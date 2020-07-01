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
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	fed "github.com/finogeeks/ligase/federation/fedreq"
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

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(userID)

	content["membership"] = "join"
	content["displayname"] = displayName
	content["avatar_url"] = avatarURL

	req := joinRoomReq{ctx, content, userID, cfg, federation, rpcCli, keyRing}

	if strings.HasPrefix(roomIDOrAlias, "!") {
		return req.joinRoomByID(roomIDOrAlias, idg)
	}
	if strings.HasPrefix(roomIDOrAlias, "#") {
		return req.joinRoomByAlias(roomIDOrAlias, idg)
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

	return r.joinRoom(roomID, domainID, idg)

	/*
		domainID, err1 := common.DomainFromID(r.userID)
		roomDomain, err2 := common.DomainFromID(roomID)
		if err1 != nil || err2 != nil {
			return http.StatusBadRequest, jsonerror.BadJSON("Room Id must be in the form '!localpart:domain'")
		}

		if common.CheckValidDomain(roomDomain, r.cfg.Matrix.ServerName) {
			return r.joinRoom(roomID, domainID, idg)
		}

		return r.joinRemoteRoom(roomID, roomDomain, idg)
	*/
}

// joinRoomByAlias joins a room using a room alias.
func (r joinRoomReq) joinRoomByAlias(
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
		return r.joinRoom(queryRes.RoomID, domainID, idg)
	}
	// If the response doesn't contain a non-empty string, return an error
	return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")

	/*
		domainID, err1 := common.DomainFromID(r.userID)
		roomDomain, err2 := common.DomainFromID(roomAlias)
		if err1 != nil || err2 != nil {
			return http.StatusBadRequest, jsonerror.BadJSON("Room alias must be in the form '#localpart:domain'")
		}
		if common.CheckValidDomain(roomDomain, r.cfg.Matrix.ServerName) {
			queryReq := roomserverapi.GetAliasRoomIDRequest{Alias: roomAlias}
			var queryRes roomserverapi.GetAliasRoomIDResponse
		if err := r.rpcCli.GetAliasRoomID(r.ctx, &queryReq, &queryRes); err != nil {
			return httputil.LogThenErrorCtx(r.ctx, err)
			}

			if len(queryRes.RoomID) > 0 {
				return r.joinRoom(queryRes.RoomID, domainID, idg)
			}
			// If the response doesn't contain a non-empty string, return an error
			return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")
		}
		// If the room isn't local, use federation to join
		return r.joinRoomByRemoteAlias(roomDomain, roomAlias, idg)
	*/
}

func (r joinRoomReq) joinRoomByRemoteAlias(
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

	return r.joinRoomByID(resp.RoomID, idg)
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

/*
func (r joinRoomReq) joinRemoteRoom(
	roomID, roomserver string, idg *uid.UidGenerator,
) (int, core.Coder) {
	//when join room ,servers actually is no use , only send request to the server created room
	code, coder, lastErr := r.joinRoomUsingFederationServer(roomID, roomserver, idg)
	if lastErr != nil {
		// There was a problem talking to one of the servers.
		fields := util.GetLogFields(r.ctx)
		fields = append(fields, log.KeysAndValues{"error", lastErr, "server", roomserver}...)
		log.Warnw("Failed to join room using server", fields)

		return httputil.LogThenErrorCtx(r.ctx, lastErr)
	}

	return code, coder
}
*/

func (r joinRoomReq) joinRoom(
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

		_, err = r.rpcCli.InputRoomEvents(context.Background(), &rawEvent)
		if err != nil {
			return http.StatusForbidden, jsonerror.Forbidden("You are not invited to this room.")
		}

		return http.StatusOK, &external.PostRoomsJoinByAliasResponse{roomID}
	}

	return http.StatusForbidden, jsonerror.Forbidden(err.Error())
}

// joinRoomUsingFederationServer tries to join a remote room using a given matrix server.
// If there was a failure communicating with the server or the response from the
// server was invalid this returns an error.
// Otherwise this returns a JSONResponse.
/*
func (r joinRoomReq) joinRoomUsingFederationServer(
	roomID, server string, idg *uid.UidGenerator,
) (int, core.Coder, error) {
	respMakeJoin, err := r.federation.MakeJoin(r.ctx, gomatrixserverlib.ServerName(server), roomID, r.userID)
	if err != nil {
		// TODO: Check if the user was not allowed to join the room.
		return http.StatusInternalServerError, nil, err
	}

	// Set all the fields to be what they should be, this should be a no-op
	// but it's possible that the remote server returned us something "odd"
	err = r.writeToBuilder(&respMakeJoin.JoinEvent, roomID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	event, err := common.BuildEvent(&respMakeJoin.JoinEvent, server, r.cfg, idg)
	if err != nil {
		code, matrixErr := httputil.LogThenErrorCtx(r.ctx, err)
		return code, matrixErr, nil
	}

	respSendJoin, err := r.federation.SendJoin(r.ctx, gomatrixserverlib.ServerName(server), *event)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if err = respSendJoin.Check(r.ctx, r.keyRing, *event); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	// TODO: Put the response struct somewhere common.
	return http.StatusOK, &external.PostRoomsJoinByAliasResponse{roomID}, nil
}
*/
