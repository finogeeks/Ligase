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

package syncconsumer

import (
	"context"

	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/model/backfilltypes"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func GetAliasRoomID(
	ctx context.Context,
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) external.GetDirectoryRoomAliasResponse {
	var aliasReq external.GetDirectoryRoomAliasRequest
	if err := json.Unmarshal(request.Extra, &aliasReq); err != nil {
		log.Errorf("federation GetAliasRoomIDRequest unmarshal error: %v", err)
		return external.GetDirectoryRoomAliasResponse{}
	}

	log.Infof("extra: %s, aliasreq: %v", string(request.Extra), aliasReq)

	// destination := fmt.Sprintf("%s:%s", request.Destination, s.cfg.GetConnectorPort())
	fedResp, err := fedClient.LookupRoomAlias(ctx, destination, aliasReq.RoomAlias)
	if err != nil {
		log.Errorf("federation LookupRoomAlias error %v", err)
		return external.GetDirectoryRoomAliasResponse{}
	}

	log.Infof("LookupRoomAlias return :%v", fedResp)
	response := external.GetDirectoryRoomAliasResponse{
		RoomID: fedResp.RoomID,
	}
	return response
	// s.rpcClient.PubObj(reply, response)
}

func GetRoomState(
	ctx context.Context,
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
	proc backfilltypes.BackFillProcessor,
) gomatrixserverlib.RespState {
	var stateReq external.GetFedRoomStateRequest
	if err := json.Unmarshal(request.Extra, &stateReq); err != nil {
		log.Errorf("federation GetRoomState unmarshal error: %v", err)
		return gomatrixserverlib.RespState{}
	}

	log.Infof("extra: %s, statereq: %v", string(request.Extra), stateReq)

	// destination := fmt.Sprintf("%s:%s", request.Destination, s.cfg.GetConnectorPort())
	fedResp, err := fedClient.LookupState(ctx, destination, stateReq.RoomID, stateReq.EventID)
	if err != nil {
		log.Errorf("federation LookupRoomState error %v", err)
		return gomatrixserverlib.RespState{}
	}

	proc.AddRequest(ctx, fedResp.StateEvents, false) // TODO: false是因为自动邀请时有可能需要历史消息，这是临时解决方案，看以后有没有更好的处理
	log.Infof("LookupState return :%v", fedResp)
	return fedResp
	// s.rpcClient.PubObj(reply, response)
}
