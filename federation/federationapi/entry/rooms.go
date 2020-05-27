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

	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(model.CMD_FED_GET_PUBLIC_ROOMS, GetPublicRooms)
	Register(model.CMD_FED_POST_PUBLIC_ROOMS, PostPublicRooms)
}

func GetPublicRooms(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	reqParams := external.GetFedPublicRoomsRequest{}
	reqParams.Decode(msg.Body)

	log.Infof("fed GetPublicRooms params, limit: %d, since: %s, include_all_networks: %t, third_party_instance_id: %s", reqParams.Limit, reqParams.Since, reqParams.IncludeAllNetworks, reqParams.ThirdPartyInstanceID)
	if reqParams.Limit == 0 || reqParams.Limit > 30 {
		reqParams.Limit = 30
	}
	qryRequest := publicroomsapi.QueryPublicRoomsRequest{
		Limit: reqParams.Limit,
		Since: reqParams.Since,
	}
	qryResponse := publicroomsapi.QueryPublicRoomsResponse{}
	err := publicroomsAPI.QueryPublicRooms(ctx, &qryRequest, &qryResponse)
	if err != nil {
		log.Errorf("QueryPublicRooms err %v", err)
		return &model.GobMessage{}, err
	}

	response := external.GetFedPublicRoomsResponse{
		Chunk:     qryResponse.Chunk,
		NextBatch: qryResponse.NextBatch,
		PrevBatch: qryResponse.PrevBatch,
		Estimate:  qryResponse.Estimate,
	}

	body, _ := response.Encode()
	return &model.GobMessage{Body: body}, nil
}

func PostPublicRooms(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	reqParams := external.PostFedPublicRoomsRequest{}
	reqParams.Decode(msg.Body)

	log.Infof("fed PostPublicRooms params, limit: %d, since: %s, filter: %s, include_all_networks: %t, third_party_instance_id: %s", reqParams.Limit, reqParams.Since, reqParams.Filter, reqParams.IncludeAllNetworks, reqParams.ThirdPartyInstanceID)
	if reqParams.Limit == 0 || reqParams.Limit > 30 {
		reqParams.Limit = 30
	}
	qryRequest := publicroomsapi.QueryPublicRoomsRequest{
		Limit:  reqParams.Limit,
		Since:  reqParams.Since,
		Filter: reqParams.Filter,
	}
	qryResponse := publicroomsapi.QueryPublicRoomsResponse{}
	err := publicroomsAPI.QueryPublicRooms(ctx, &qryRequest, &qryResponse)
	if err != nil {
		log.Errorf("QueryPublicRooms err %v", err)
		return &model.GobMessage{}, err
	}

	response := external.PostFedPublicRoomsResponse{
		Chunk:     qryResponse.Chunk,
		NextBatch: qryResponse.NextBatch,
		PrevBatch: qryResponse.PrevBatch,
		Estimate:  qryResponse.Estimate,
	}

	body, _ := response.Encode()
	return &model.GobMessage{Body: body}, nil
}
