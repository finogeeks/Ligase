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

	"github.com/finogeeks/ligase/common"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

// DirectoryRoom looks up a room alias
func DirectoryRoom(
	ctx context.Context,
	roomAlias string,
	federation *fed.Federation,
	cfg *config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI,
	// rpcClient *common.RpcClient,
) (int, core.Coder) {
	/*
		domain, err := common.DomainFromID(roomAlias)
		if err != nil {
			return http.StatusBadRequest, jsonerror.BadJSON("Room alias must be in the form '#localpart:domain'")
		}
	*/

	queryReq := roomserverapi.GetAliasRoomIDRequest{Alias: roomAlias}
	var queryRes roomserverapi.GetAliasRoomIDResponse
	if err := rpcCli.GetAliasRoomID(ctx, &queryReq, &queryRes); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if queryRes.RoomID == "" {
		return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")
	}

	queryJoinedMemberReq := roomserverapi.QueryRoomStateRequest{RoomID: queryRes.RoomID}
	var queryJoinedMemberRes roomserverapi.QueryRoomStateResponse
	if err := rpcCli.QueryRoomState(ctx, &queryJoinedMemberReq, &queryJoinedMemberRes); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	var userServers []string

	for userId := range queryJoinedMemberRes.Join {
		_, userDomain, _ := gomatrixserverlib.SplitID('@', userId)

		insert := true
		for _, domain := range userServers {
			if domain == string(userDomain) {
				insert = false
				break
			}
		}

		if insert {
			userServers = append(userServers, string(userDomain))
		}
	}

	if len(queryRes.RoomID) > 0 {
		// TODO: List servers that are aware of this room alias
		return http.StatusOK, &external.GetDirectoryRoomAliasResponse{
			RoomID:  queryRes.RoomID,
			Servers: userServers,
		}
	} else {
		// If the response doesn't contain a non-empty string, return an error
		return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")
	}
	/*
		if common.CheckValidDomain(domain, cfg.Matrix.ServerName) {
			queryReq := roomserverapi.GetAliasRoomIDRequest{Alias: roomAlias}
			var queryRes roomserverapi.GetAliasRoomIDResponse
			if err := rpcCli.GetAliasRoomID(ctx, &queryReq, &queryRes); err != nil {
				return httputil.LogThenErrorCtx(ctx, err)
			}

			queryJoinedMemberReq := roomserverapi.QueryRoomStateRequest{RoomID: queryRes.RoomID}
			var queryJoinedMemberRes roomserverapi.QueryRoomStateResponse
			if err := rpcCli.QueryRoomState(ctx, &queryJoinedMemberReq, &queryJoinedMemberRes); err != nil {
				return httputil.LogThenErrorCtx(ctx, err)
			}

			var userServers []string

			for userId := range queryJoinedMemberRes.Join {
				_, userDomain, _ := gomatrixserverlib.SplitID('@', userId)

				insert := true
				for _, domain := range userServers {
					if domain == string(userDomain) {
						insert = false
						break
					}
				}

				if insert {
					userServers = append(userServers, string(userDomain))
				}
			}

			if len(queryRes.RoomID) > 0 {
				// TODO: List servers that are aware of this room alias
				return http.StatusOK, &external.GetDirectoryRoomAliasResponse{
					RoomID:  queryRes.RoomID,
					Servers: userServers,
				}
			} else {
				// If the response doesn't contain a non-empty string, return an error
				return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")
			}
		} else {
			resp, err := federation.LookupRoomAlias(domain, roomAlias)
			if err != nil {
				switch x := err.(type) {
				case gomatrix.HTTPError:
					if x.Code == http.StatusNotFound {
						return http.StatusNotFound, jsonerror.NotFound("Room alias not found")
					}
				}
				// TODO: Return 502 if the remote server errored.
				// TODO: Return 504 if the remote server timed out.
				return httputil.LogThenErrorCtx(ctx, err)
			}

			//queryReq := external.GetDirectoryRoomAliasRequest{RoomAlias: roomAlias}
			//var queryRes external.GetDirectoryRoomAliasResponse
			//err = rpc.GetAliasRoomID(cfg, rpcClient, domain, &queryReq, &queryRes)

			if len(resp.RoomID) > 0 {
				// TODO: List servers that are aware of this room alias
				resp.Servers = nil //userServers
			} else {
				// If the response doesn't contain a non-empty string, return an error
				return http.StatusNotFound, jsonerror.NotFound("Room alias " + roomAlias + " not found.")
			}

			return http.StatusOK, &resp
		}
	*/
}

// SetLocalAlias implements PUT /directory/room/{roomAlias}
// TODO: Check if the user has the power level to set an alias
func SetLocalAlias(
	ctx context.Context,
	req *external.PutDirectoryRoomAliasRequest,
	userID string,
	alias string,
	cfg *config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI,
) (int, core.Coder) {
	domain, err := common.DomainFromID(alias)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Room alias must be in the form '#localpart:domain'")
	}

	if common.CheckValidDomain(domain, cfg.Matrix.ServerName) == false {
		return http.StatusForbidden, jsonerror.Forbidden("Alias must be on local homeserver")
	}

	var queryRsRes roomserverapi.QueryRoomStateResponse
	var queryRsReq roomserverapi.QueryRoomStateRequest
	queryRsReq.RoomID = req.RoomID

	err = rpcCli.QueryRoomState(ctx, &queryRsReq, &queryRsRes)
	if err != nil || queryRsRes.RoomExists == false {
		return http.StatusBadRequest, nil
	}
	_, ok1 := queryRsRes.Join[userID]
	_, ok2 := queryRsRes.Invite[userID]
	_, ok3 := queryRsRes.Leave[userID]

	if !ok1 && !ok2 && !ok3 {
		return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
	} else {
		if !ok1 {
			return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
		}
	}

	queryReq := roomserverapi.SetRoomAliasRequest{
		UserID: userID,
		RoomID: req.RoomID,
		Alias:  alias,
	}
	var queryRes roomserverapi.SetRoomAliasResponse
	if err := rpcCli.SetRoomAlias(ctx, &queryReq, &queryRes); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if queryRes.AliasExists {
		return http.StatusConflict, jsonerror.Unknown("The alias " + alias + " already exists.")
	}

	return http.StatusOK, nil
}

// RemoveLocalAlias implements DELETE /directory/room/{roomAlias}
// TODO: Check if the user has the power level to remove an alias
func RemoveLocalAlias(
	ctx context.Context,
	userID string,
	alias string,
	rpcCli roomserverapi.RoomserverRPCAPI,
) (int, core.Coder) {
	queryReq := roomserverapi.RemoveRoomAliasRequest{
		Alias:  alias,
		UserID: userID,
	}
	var queryRes roomserverapi.RemoveRoomAliasResponse
	if err := rpcCli.RemoveRoomAlias(ctx, &queryReq, &queryRes); err != nil {
		log.Errorf("RemoveLocalAlias err: %v", err)
		return http.StatusForbidden, jsonerror.Forbidden(err.Error())
	}

	return http.StatusOK, nil
}

// GetVisibility implements GET /directory/list/room/{roomID}
func GetVisibility(
	ctx context.Context,
	roomID string,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
) (int, core.Coder) {
	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	err := rsRpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	v := &external.GetDirectoryRoomResponse{}
	vsEv := queryRes.Visibility
	if vsEv == nil {
		v.Visibility = "private"
	} else {
		vi := common.VisibilityContent{}
		err = json.Unmarshal(vsEv.Content(), &vi)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}

		v.Visibility = vi.Visibility
	}

	return http.StatusOK, v
}
