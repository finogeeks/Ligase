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
	"fmt"

	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
)

func init() {
	Register(model.CMD_FED_EVENT, QueryEvent)
	Register(model.CMD_FED_STATE_IDS, QueryStateIDs)
}

func QueryEvent(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	ctx := context.TODO()
	var reqParam external.GetEventRequest
	reqParam.Decode(msg.Body)
	qryEventReq := roomserverapi.QueryEventsByIDRequest{EventIDs: []string{reqParam.EventID}}
	qryEventResp := roomserverapi.QueryEventsByIDResponse{}
	err := rpcCli.QueryEventsByID(ctx, &qryEventReq, &qryEventResp)
	if err != nil {
		log.Errorf("handle query event request err %v", err)
		return &model.GobMessage{}, err
	}

	events := qryEventResp.Events
	if len(events) != 1 {
		log.Errorf("handle query event response %d", len(events))
		return &model.GobMessage{}, fmt.Errorf("handle query event response nil")
	}

	resp := external.GetEventResponse{
		Origin:         string(events[0].Origin()),
		OriginServerTS: int64(events[0].OriginServerTS()),
	}
	for i := 0; i < len(events); i++ {
		resp.PDUs = append(resp.PDUs, *events[i])
	}
	body, _ := resp.Encode()
	return &model.GobMessage{Body: body}, nil
}

func QueryStateIDs(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	ctx := context.TODO()
	var reqParam external.GetStateIDsRequest
	reqParam.Decode(msg.Body)

	resp := external.GetStateIDsResponse{}
	if reqParam.EventID == "" {
		qryStateReq := roomserverapi.QueryRoomStateRequest{RoomID: reqParam.RoomID}
		qryStateResp := roomserverapi.QueryRoomStateResponse{}
		err := rpcCli.QueryRoomState(ctx, &qryStateReq, &qryStateResp)
		if err != nil {
			log.Errorf("handle query stateIDs request err %v", err)
			return &model.GobMessage{}, err
		}

		events := qryStateResp.GetAllState()
		if len(events) == 0 {
			log.Errorf("handle query stateIDs response nil")
			return &model.GobMessage{}, fmt.Errorf("handle query stateIDs response nil")
		}
		for i := 0; i < len(events); i++ {
			resp.PduIds = append(resp.PduIds, events[i].EventID())
		}
	} else {
		qryAuthReq := roomserverapi.QueryEventAuthRequest{EventID: reqParam.EventID}
		qryAuthResp := roomserverapi.QueryEventAuthResponse{}
		err := rpcCli.QueryEventAuth(ctx, &qryAuthReq, &qryAuthResp)
		if err != nil {
			log.Errorf("handle query stateIDs request auth err %v", err)
			return &model.GobMessage{}, err
		}
		events := qryAuthResp.AuthEvents
		if len(events) == 0 {
			log.Errorf("handle query stateIDs auth response nil")
			return &model.GobMessage{}, fmt.Errorf("handle query stateIDs auth response nil")
		}
		for i := 0; i < len(events); i++ {
			resp.PduIds = append(resp.PduIds, events[i].EventID())
		}

		authChain := map[string]bool{}
		for i := 0; i < len(resp.PduIds); i++ {
			authChain[resp.PduIds[i]] = false
		}
		for {
			found := false
			for k, v := range authChain {
				if v {
					continue
				}
				qryAuthReq := roomserverapi.QueryEventAuthRequest{EventID: k}
				qryAuthResp := roomserverapi.QueryEventAuthResponse{}
				err := rpcCli.QueryEventAuth(ctx, &qryAuthReq, &qryAuthResp)
				if err != nil {
					log.Errorf("handle query stateIDs request auth chain err %v", err)
					return &model.GobMessage{}, err
				}
				events := qryAuthResp.AuthEvents
				for i := 0; i < len(events); i++ {
					if _, ok := authChain[events[i].EventID()]; ok {
						authChain[events[i].EventID()] = true
					} else {
						authChain[events[i].EventID()] = false
					}
				}
				found = true
				break
			}
			if !found {
				break
			}
		}
		for k := range authChain {
			resp.AuthChainIDs = append(resp.AuthChainIDs, k)
		}
	}

	body, _ := resp.Encode()
	return &model.GobMessage{Body: body}, nil
}
