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
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func SendInvite(
	ctx context.Context,
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) gomatrixserverlib.RespInvite {
	var event gomatrixserverlib.Event
	if err := json.Unmarshal(request.Extra, &event); err != nil {
		log.Errorf("federation Download unmarshal error: %v", err)
		return gomatrixserverlib.RespInvite{Code: 400}
	}

	fedResp, err := fedClient.SendInvite(ctx, destination, event)
	if err != nil {
		log.Errorf("federation SendInvite error %v", err)
	}
	return fedResp
}
