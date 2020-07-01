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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
)

func init() {
	Register(model.CMD_FED_MAKEJOIN, MakeJoin)
	Register(model.CMD_FED_SENDJOIN, SendJoin)
}

func MakeJoin(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	retMsg := &model.GobMessage{Body: []byte{}}
	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}
	log.Debugf("MakeJoin reqParams: %s", msg.Body)

	reqParam := external.GetMakeJoinRequest{}
	if err := json.Unmarshal(msg.Body, &reqParam); err != nil {
		return retMsg, errors.New("MakeJoin unmarshal reqParam error: " + err.Error())
	}

	domain, err := common.DomainFromID(reqParam.UserID)
	if err != nil {
		return retMsg, errors.New("MakeJoin invalid UserID: " + reqParam.UserID)
	}

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = reqParam.RoomID
	err = rpcCli.QueryRoomState(context.TODO(), &queryReq, &queryRes)
	if err != nil {
		return retMsg, errors.New("MakeJoin query room state error: " + err.Error())
	}

	builder := gomatrixserverlib.EventBuilder{
		Sender:   reqParam.UserID,
		RoomID:   reqParam.RoomID,
		Type:     "m.room.member",
		StateKey: &reqParam.UserID,
	}

	err = builder.SetContent(map[string]interface{}{"membership": "join"})
	if err != nil {
		return retMsg, errors.New("MakeJoin builder.SetContent error: " + err.Error())
	}

	now := time.Now()
	nid, _ := idg.Next()
	ev, err := builder.Build(nid, now, gomatrixserverlib.ServerName(domain))
	if err != nil {
		return retMsg, errors.New("MakeJoin buildEvent error: " + err.Error())
	}

	if err = gomatrixserverlib.Allowed(ev, &queryRes); err != nil {
		return retMsg, errors.New("MakeJoin not allowed, error: " + err.Error())
	}

	retMsg.Body, err = json.Marshal(gomatrixserverlib.RespMakeJoin{
		JoinEvent: builder,
	})
	if err != nil {
		return retMsg, errors.New("MakeJoin marshal ret body error: " + err.Error())
	}

	return retMsg, nil
}

func SendJoin(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	retMsg := &model.GobMessage{Body: []byte{}}
	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}
	log.Debugf("SendJoin reqParams: %s", msg.Body)

	reqParam := external.PutSendJoinRequest{}
	if err := json.Unmarshal(msg.Body, &reqParam); err != nil {
		return retMsg, errors.New("SendJoin unmarshal reqParam error: " + err.Error())
	}

	// domain, err := common.DomainFromID(reqParam.EventID)
	// if err != nil {
	// 	return retMsg, errors.New("SendJoin invalid EventID: " + reqParam.EventID)
	// }

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = reqParam.RoomID
	err := rpcCli.QueryRoomState(context.TODO(), &queryReq, &queryRes)
	if err != nil {
		return retMsg, errors.New("SendJoin query room state error: " + err.Error())
	}

	if err = gomatrixserverlib.Allowed(reqParam.Event, &queryRes); err != nil {
		return retMsg, errors.New("SendJoin not allowed, error: " + err.Error())
	}

	var resp gomatrixserverlib.RespSendJoin
	resp.StateEvents = queryRes.GetAllState()
	resp.AuthEvents = queryRes.GetAllState()
	retMsg.Body, _ = json.Marshal(&resp)

	return retMsg, nil
}
