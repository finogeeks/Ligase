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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
)

func MakeLeave(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) gomatrixserverlib.RespMakeLeave {
	var req external.GetMakeLeaveRequest
	if err := json.Unmarshal(request.Extra, &req); err != nil {
		log.Errorf("federation make leave unmarshal error: %v", err)
		return gomatrixserverlib.RespMakeLeave{}
	}

	redResp, err := fedClient.MakeLeave(context.Background(), gomatrixserverlib.ServerName(destination), req.RoomID, req.UserID)
	if err != nil {
		log.Errorf("federation make leave error response: %v", err)
	}
	return redResp
}

func SendLeave(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
	proc backfilltypes.BackFillProcessor,
) gomatrixserverlib.RespSendLeave {
	var req external.PutSendLeaveRequest
	if err := json.Unmarshal(request.Extra, &req); err != nil {
		log.Errorf("federation send leave unmarshal error: %v", err)
		return gomatrixserverlib.RespSendLeave{}
	}

	redResp, err := fedClient.SendLeave(context.Background(), gomatrixserverlib.ServerName(destination), req.RoomID, req.EventID, req.Event)
	if err != nil {
		log.Errorf("federation send leave error response: %v", err)
	}
	return redResp
}
