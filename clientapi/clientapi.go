// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package clientapi

import (
	"log"

	"github.com/finogeeks/ligase/clientapi/api"
	"github.com/finogeeks/ligase/clientapi/rpc"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	rrpc "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/consul"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
)

// SetupClientAPIComponent sets up and registers HTTP handlers for the ClientAPI
// component.
func SetupClientAPIComponent(
	base *basecomponent.BaseDendrite,
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	accountsDB model.AccountsDatabase,
	federation *fed.Federation,
	keyRing *gomatrixserverlib.KeyRing,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	presenceDB model.PresenceDatabase,
	roomDB model.RoomServerDatabase,
	rpcCli *common.RpcClient,
	rpcClient rrpc.RpcClient,
	tokenFilter *filter.Filter,
	idg *uid.UidGenerator,
	settings *common.Settings,
	fedDomians *common.FedDomains,
	complexCache *common.ComplexCache,
) {
	if base.Cfg.Rpc.Driver == "nats" {
		profileRpcConsumer := rpc.NewProfileRpcConsumer(rpcCli, base.Cfg, rsRpcCli, idg, accountsDB, presenceDB, cache, complexCache)
		profileRpcConsumer.Start()
	} else {
		grpcServer := rpc.NewServer(base.Cfg, idg, accountsDB, presenceDB, cache, complexCache, rsRpcCli)
		if err := grpcServer.Start(); err != nil {
			log.Panicf("failed to start front rpc server err:%v", err)
		}
	}

	if base.Cfg.Rpc.Driver == "grpc_with_consul" {
		if base.Cfg.Rpc.ConsulURL == "" {
			log.Panicf("grpc_with_consul consul url is null")
		}
		consulTag := base.Cfg.Rpc.Front.ConsulTagPrefix + "0"
		c := consul.NewConsul(base.Cfg.Rpc.ConsulURL, consulTag, base.Cfg.Rpc.Front.ServerName, base.Cfg.Rpc.Front.Port)
		c.Init()
	}

	apiConsumer := api.NewInternalMsgConsumer(
		base.APIMux, *base.Cfg,
		rsRpcCli, accountsDB, deviceDB,
		federation, *keyRing,
		cache, encryptDB, syncDB, presenceDB,
		roomDB, rpcCli, rpcClient, tokenFilter, settings, fedDomians, complexCache,
	)
	apiConsumer.Start()
}
