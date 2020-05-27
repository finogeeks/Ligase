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
	"errors"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(model.CMD_FED_MAKELEAVE, MakeLeave)
	Register(model.CMD_FED_SENDLEAVE, SendLeave)
}

func MakeLeave(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	retMsg := &model.GobMessage{Body: []byte{}}
	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}
	log.Debugf("MakeLeave reqParams: %s", msg.Body)

	reqParam := external.GetMakeLeaveRequest{}
	if err := json.Unmarshal(msg.Body, &reqParam); err != nil {
		return retMsg, errors.New("MakeLeave unmarshal reqParam error: " + err.Error())
	}

	domain, err := common.DomainFromID(reqParam.UserID)
	if err != nil {
		return retMsg, errors.New("MakeLeave invalid UserID: " + reqParam.UserID)
	}

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = reqParam.RoomID
	err = rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		return retMsg, errors.New("MakeLeave query room state error: " + err.Error())
	}

	builder := gomatrixserverlib.EventBuilder{
		Sender:   reqParam.UserID,
		RoomID:   reqParam.RoomID,
		Type:     "m.room.member",
		StateKey: &reqParam.UserID,
	}

	err = builder.SetContent(map[string]interface{}{"membership": "leave"})
	if err != nil {
		return retMsg, errors.New("MakeLeave builder.SetContent error: " + err.Error())
	}

	now := time.Now()
	nid, _ := idg.Next()
	ev, err := builder.Build(nid, now, gomatrixserverlib.ServerName(domain))
	if err != nil {
		return retMsg, errors.New("MakeLeave buildEvent error: " + err.Error())
	}

	if err = gomatrixserverlib.Allowed(ev, &queryRes); err != nil {
		return retMsg, errors.New("MakeLeave not allowed, error: " + err.Error())
	}

	retMsg.Body, err = json.Marshal(gomatrixserverlib.RespMakeLeave{
		Event: builder,
	})
	if err != nil {
		return retMsg, errors.New("MakeLeave marshal ret body error: " + err.Error())
	}

	return retMsg, nil
}

func SendLeave(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	retMsg := &model.GobMessage{Body: []byte{}}
	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}
	log.Debugf("SendLeave reqParams: %s", msg.Body)

	reqParam := external.PutSendLeaveRequest{}
	if err := json.Unmarshal(msg.Body, &reqParam); err != nil {
		return retMsg, errors.New("SendLeave unmarshal reqParam error: " + err.Error())
	}
	if reqParam.Event.StateKey() == nil {
		return retMsg, errors.New("SendLeave stateKey is nil")
	}

	// domain, err := common.DomainFromID(reqParam.EventID)
	// if err != nil {
	// 	return retMsg, errors.New("SendLeave invalid EventID: " + reqParam.EventID)
	// }

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = reqParam.RoomID
	err := rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		return retMsg, errors.New("SendLeave query room state error: " + err.Error())
	}

	if _, ok := queryRes.Invite[*reqParam.Event.StateKey()]; ok {
	} else if _, ok := queryRes.Join[*reqParam.Event.StateKey()]; ok {
	} else {
		return retMsg, errors.New("User is not in the room.")
	}

	if err = gomatrixserverlib.Allowed(reqParam.Event, &queryRes); err != nil {
		return retMsg, errors.New("SendLeave not allowed, error: " + err.Error())
	}

	var resp gomatrixserverlib.RespSendLeave
	resp.Code = 200
	retMsg.Body, _ = json.Marshal(&resp)

	return retMsg, nil
}
