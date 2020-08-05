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

	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(model.CMD_FED_BACKFILL, Backfill)
}

func Backfill(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	log.Infof("Enter Backfill......")

	retMsg := &model.GobMessage{
		Body: []byte{},
	}

	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}

	request := &external.GetFedBackFillRequest{}
	request.Decode(msg.Body)

	//request := msg.Body.(*external.GetFedBackFillRequest)

	var req roomserverapi.QueryBackFillEventsRequest
	var resp roomserverapi.QueryBackFillEventsResponse
	req.EventID = request.BackFillIds
	req.Limit = request.Limit
	req.RoomID = request.RoomID
	req.Dir = request.Dir
	req.Domain = request.Domain
	req.Origin = request.Origin
	bytes, _ := json.Marshal(*request)
	log.Infof("Backfill request: %v", string(bytes))
	err := rpcCli.QueryBackFillEvents(ctx, &req, &resp)

	if err != nil {
		log.Errorf("Backfill err %v", err)
		return retMsg, err
	}

	bytes, _ = json.Marshal(resp)
	log.Infof("Backfill response: %v", string(bytes))

	retMsg.Body = bytes
	return retMsg, nil
}
