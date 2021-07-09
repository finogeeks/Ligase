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

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/rpc"
)

type FedPublicRoomsRpcClient struct {
	cfg    *config.Dendrite
	rpcCli rpc.RpcClient
}

func NewFedPublicRoomsRpcClient(
	cfg *config.Dendrite,
	rpcCli rpc.RpcClient,
) *FedPublicRoomsRpcClient {
	fed := &FedPublicRoomsRpcClient{
		cfg:    cfg,
		rpcCli: rpcCli,
	}

	return fed
}

func (fed *FedPublicRoomsRpcClient) Start() {
}

func (fed *FedPublicRoomsRpcClient) QueryPublicRooms(ctx context.Context, request *publicroomsapi.QueryPublicRoomsRequest, response *publicroomsapi.QueryPublicRoomsResponse) error {
	data, err := fed.rpcCli.QueryPublicRoomState(ctx, request)
	if err != nil {
		return err
	}
	*response = *data
	return err
}
