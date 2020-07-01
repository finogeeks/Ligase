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
	"errors"

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
	Register(model.CMD_FED_EVENT_AUTH, EventAuth)
	Register(model.CMD_FED_QUERY_AUTH, QueryAuth)
}

func EventAuth(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	var reqParams external.GetEventAuthRequest
	_ = reqParams.Decode(msg.Body)

	ctx := context.TODO()
	qryEventAuthReq := roomserverapi.QueryEventAuthRequest{EventID: reqParams.EventID}
	qryEventAuthResp := roomserverapi.QueryEventAuthResponse{}
	err := rpcCli.QueryEventAuth(ctx, &qryEventAuthReq, &qryEventAuthResp)
	if err != nil {
		log.Errorf("handler auth query event auth err: %v", err)
		return &model.GobMessage{}, err
	}

	var resp external.GetEventAuthResponse
	resp.AuthChain = make([]gomatrixserverlib.Event, 0, len(qryEventAuthResp.AuthEvents))
	for i := 0; i < len(qryEventAuthResp.AuthEvents); i++ {
		resp.AuthChain = append(resp.AuthChain, *qryEventAuthResp.AuthEvents[i])
	}
	body, _ := resp.Encode()

	return &model.GobMessage{Body: body}, nil
}

func QueryAuth(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	var reqParams external.PostQueryAuthRequest
	_ = reqParams.Decode(msg.Body)

	ctx := context.TODO()

	eventIDs := make([]string, len(reqParams.Missing)+1)
	eventIDs[0] = reqParams.EventID
	if len(reqParams.Missing) > 0 {
		copy(eventIDs[1:], reqParams.Missing)
	}

	queryEventRequest := roomserverapi.QueryEventsByIDRequest{EventIDs: eventIDs}
	queryEventResponse := roomserverapi.QueryEventsByIDResponse{}
	if err := rpcCli.QueryEventsByID(ctx, &queryEventRequest, &queryEventResponse); err != nil {
		return &model.GobMessage{}, err
	}

	eventMap := make(map[string]*gomatrixserverlib.Event, len(queryEventResponse.Events))
	for i := 0; i < len(queryEventResponse.Events); i++ {
		eventMap[queryEventResponse.Events[i].EventID()] = queryEventResponse.Events[i]
	}

	event, ok := eventMap[reqParams.EventID]
	if !ok {
		return &model.GobMessage{}, errors.New("event not exists " + reqParams.EventID)
	}

	if event.RoomID() != reqParams.RoomID {
		return &model.GobMessage{}, errors.New("event " + reqParams.EventID + "'s owner room is " + event.RoomID() + " not " + reqParams.RoomID)
	}

	for _, v := range reqParams.Missing {
		ev, ok := eventMap[v]
		if !ok {
			// TODO: cjw get missing event
		}
		if ev.RoomID() != event.RoomID() {
			return &model.GobMessage{}, errors.New("missing event " + v + "'s owner room is " + ev.RoomID() + " not " + reqParams.RoomID)
		}
	}

	qryEventAuthReq := roomserverapi.QueryEventAuthRequest{EventID: reqParams.EventID}
	qryEventAuthResp := roomserverapi.QueryEventAuthResponse{}
	err := rpcCli.QueryEventAuth(ctx, &qryEventAuthReq, &qryEventAuthResp)
	if err != nil {
		log.Errorf("handler auth query event auth err: %v", err)
		return &model.GobMessage{}, err
	}
	authChain := make([]gomatrixserverlib.Event, len(qryEventAuthResp.AuthEvents))
	for i, v := range qryEventAuthResp.AuthEvents {
		authChain[i] = *v
	}

	remoteStateMap := make(map[string]*gomatrixserverlib.Event, len(reqParams.AuthChain))
	for i := 0; i < len(reqParams.AuthChain); i++ {
		remoteStateMap[reqParams.AuthChain[i].EventID()] = &reqParams.AuthChain[i]
	}

	remoteMissing := []string{}
	for i := 0; i < len(authChain); i++ {
		if _, ok := remoteStateMap[authChain[i].EventID()]; !ok {
			remoteMissing = append(remoteMissing, authChain[i].EventID())
		}
	}

	resp := external.PostQueryAuthResponse{}
	resp.AuthChain = authChain
	resp.Missing = remoteMissing

	body, _ := resp.Encode()
	return &model.GobMessage{Body: body}, nil
}
