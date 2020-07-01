// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package entry

import (
	"context"
	"encoding/json"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/pkg/errors"
)

func init() {
	Register(model.CMD_FED_ROOM_DIRECTORY, RoomDirectory)
	Register(model.CMD_FED_ROOM_STATE, RoomState)
}

func RoomDirectory(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	if msg == nil {
		return nil, errors.New("msg from connector is nil")
	}
	roomAlias := string(msg.Body)

	queryReq := roomserverapi.GetAliasRoomIDRequest{Alias: roomAlias}
	var queryRes roomserverapi.GetAliasRoomIDResponse
	if err := rpcCli.GetAliasRoomID(context.TODO(), &queryReq, &queryRes); err != nil {
		return nil, errors.New("")
	}
	stateReq := roomserverapi.QueryRoomStateRequest(queryRes)
	stateRes := roomserverapi.QueryRoomStateResponse{}
	if err := rpcCli.QueryRoomState(context.TODO(), &stateReq, &stateRes); err != nil {
		return nil, err
	}

	serversMap := map[string]struct{}{}
	for _, v := range stateRes.Join {
		if v.StateKey() != nil {
			if domain, err := common.DomainFromID(*v.StateKey()); err == nil {
				serversMap[domain] = struct{}{}
			}
		}
	}
	servers := []string{}
	for k := range serversMap {
		servers = append(servers, k)
	}
	body, _ := json.Marshal(&external.GetFedDirectoryResponse{
		RoomID:  queryRes.RoomID,
		Servers: servers,
	})
	retMsg := &model.GobMessage{
		Body: body,
	}
	log.Infof("RoomAliasToID, ret.body: %v", retMsg.Body)
	return retMsg, nil
}

func RoomState(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	if msg == nil {
		return nil, errors.New("msg from connector is nil")
	}
	roomID := string(msg.Body)

	queryReq := roomserverapi.QueryRoomStateRequest{RoomID: roomID}
	var queryRes roomserverapi.QueryRoomStateResponse

	if err := rpcCli.QueryRoomState(context.TODO(), &queryReq, &queryRes); err != nil {
		return nil, err
	}

	bytes, _ := json.Marshal(queryRes)
	retMsg := &model.GobMessage{
		Body: bytes,
	}
	log.Infof("RoomState, ret.body: %v", retMsg.Body)
	return retMsg, nil
}
