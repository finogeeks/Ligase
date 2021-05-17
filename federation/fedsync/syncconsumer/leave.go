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
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func MakeLeave(
	fedClient *client.FedClientWrap,
	req *external.GetMakeLeaveRequest,
	destination string,
) gomatrixserverlib.RespMakeLeave {
	redResp, err := fedClient.MakeLeave(context.Background(), gomatrixserverlib.ServerName(destination), req.RoomID, req.UserID)
	if err != nil {
		log.Errorf("federation make leave error response: %v", err)
	}
	return redResp
}

func SendLeave(
	fedClient *client.FedClientWrap,
	req *external.PutSendLeaveRequest,
	destination string,
	proc backfilltypes.BackFillProcessor,
) gomatrixserverlib.RespSendLeave {
	redResp, err := fedClient.SendLeave(context.Background(), gomatrixserverlib.ServerName(destination), req.RoomID, req.EventID, req.Event)
	if err != nil {
		log.Errorf("federation send leave error response: %v", err)
	}
	return redResp
}
