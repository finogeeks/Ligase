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

package fedsync

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/fedsync/syncconsumer"
	"github.com/finogeeks/ligase/federation/model/backfilltypes"
)

type FederationSync struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	consumer  *syncconsumer.SyncConsumer
	// roomAliasRpcCli *syncconsumer.RoomAliasRpcConsumer
	// profileRpcCli *syncconsumer.ProfileRpcConsumer
}

func NewFederationSync(cfg *config.Dendrite, fedClient *client.FedClientWrap, feddomains *common.FedDomains) *FederationSync {
	rpcClient := common.NewRpcClient(cfg.Nats.Uri)

	consumer := syncconsumer.NewSyncConsumer(cfg, fedClient, rpcClient, feddomains)
	// roomAliasRpcConsumer := syncconsumer.NewRoomAliasRpcConsumer(cfg, fedClient, rpcClient)
	// profileRpcConsumer := syncconsumer.NewProfileRpcConsumer(cfg, fedClient, rpcClient)

	fedSync := &FederationSync{
		cfg:       cfg,
		rpcClient: rpcClient,
		consumer:  consumer,
		// roomAliasRpcCli: roomAliasRpcConsumer,
		// profileRpcCli: profileRpcConsumer,
	}
	return fedSync
}

func (fedSync *FederationSync) Setup(backfill backfilltypes.BackFillProcessor) {
	fedSync.consumer.SetBackfill(backfill)
	fedSync.rpcClient.Start(true)
	fedSync.consumer.Start()
	// fedSync.roomAliasRpcCli.Start()
	// fedSync.profileRpcCli.Start()
}
