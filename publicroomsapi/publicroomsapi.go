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

package publicroomsapi

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/publicroomsapi/api"
	"github.com/finogeeks/ligase/publicroomsapi/consumers"
	"github.com/finogeeks/ligase/publicroomsapi/rpc"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

// SetupPublicRoomsAPIComponent sets up and registers HTTP handlers for the PublicRoomsAPI
// component.
func SetupPublicRoomsAPIComponent(
	base *basecomponent.BaseDendrite,
	rpcCli *common.RpcClient,
	rpcClient rpcService.RpcClient,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	publicRoomsDB model.PublicRoomAPIDatabase,
) {
	roomConsumer := consumers.NewOutputRoomEventConsumer(
		base.Cfg, publicRoomsDB, rsRpcCli,
	)
	if err := roomConsumer.Start(); err != nil {
		log.Panicw("failed to start room server consumer", log.KeysAndValues{"error", err})
	}

	apiConsumer := api.NewInternalMsgConsumer(*base.Cfg, publicRoomsDB, rpcCli, rpcClient)
	apiConsumer.Start()

	if base.Cfg.Rpc.Driver == "nats" {
		prQryConsumer := rpc.NewPublicRoomsRpcConsumer(base.Cfg, rpcCli, publicRoomsDB)
		prQryConsumer.Start()
	} else {
		grpcServer := rpc.NewServer(base.Cfg, publicRoomsDB)
		if err := grpcServer.Start(); err != nil {
			log.Panicf("failed to start publicRoom rpc server err:%v", err)
		}
	}
}
