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

package rpc

import (
	"context"
	"encoding/json"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
)

type FedPublicRoomsRpcClient struct {
	cfg       *config.Fed
	rpcClient *common.RpcClient
}

func NewFedPublicRoomsRpcClient(
	cfg *config.Fed,
	rpcClient *common.RpcClient,
) *FedPublicRoomsRpcClient {
	fed := &FedPublicRoomsRpcClient{
		cfg:       cfg,
		rpcClient: rpcClient,
	}

	return fed
}

func (fed *FedPublicRoomsRpcClient) Start() {
}

func (fed *FedPublicRoomsRpcClient) QueryPublicRooms(ctx context.Context, request *publicroomsapi.QueryPublicRoomsRequest, response *publicroomsapi.QueryPublicRoomsResponse) error {
	content := publicroomsapi.PublicRoomsRpcRequest{
		QueryPublicRooms: request,
	}
	bytes, _ := json.Marshal(content)
	data, err := fed.rpcClient.Request(fed.cfg.Rpc.PrQryTopic, bytes, 30000)
	if err == nil {
		err = json.Unmarshal(data, response)
		if err != nil {
			return err
		}
	}

	return err
}
