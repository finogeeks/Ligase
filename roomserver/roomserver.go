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

package roomserver

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/roomserver/consumers"
	"github.com/finogeeks/ligase/roomserver/processors"
	"github.com/finogeeks/ligase/roomserver/rpc"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/skunkworks/log"
)

// SetupRoomServerComponent sets up and registers HTTP handlers for the
// RoomServer component. Returns instances of the various roomserver APIs,
// allowing other components running in the same process to hit the query the
// APIs directly instead of having to use HTTP.
func SetupRoomServerComponent(
	base *basecomponent.BaseDendrite,
	processEvent bool,
	rpcClient *common.RpcClient,
	repoCache service.Cache,
	federation *fed.Federation,
) (roomserverapi.RoomserverInputAPI, roomserverapi.RoomserverRPCAPI, model.RoomServerDatabase) {
	rdb, err := common.GetDBInstance("roomserver", base.Cfg)
	if err != nil {
		log.Panicw("failed to connect to room server db", log.KeysAndValues{"error", err})
	}

	monitor := mon.GetInstance()
	queryHitCounter := monitor.NewLabeledCounter("roomserver_query_hit", []string{"target", "repo", "func"})

	roomserverDB := rdb.(model.RoomServerDatabase)
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	roomserverDB.SetIDGenerator(idg)
	repo := repos.NewRoomServerCurStateRepo(roomserverDB, repoCache, queryHitCounter)
	umsRepo := repos.NewRoomServerUserMembershipRepo(roomserverDB, repoCache, queryHitCounter)

	inputAPI := processors.EventsProcessor{
		DB:         roomserverDB,
		Repo:       repo,
		UmsRepo:    umsRepo,
		Cfg:        base.Cfg,
		Idg:        idg,
		RpcClient:  rpcClient,
		Federation: federation,
	}

	queryAPI := processors.RoomQryProcessor{
		DB:      roomserverDB,
		Repo:    repo,
		UmsRepo: umsRepo,
		Cfg:     base.Cfg,
	}

	aliasFilter := filter.GetFilterMng().Register("alias", roomserverDB)
	aliasFilter.Load()

	aliasAPI := processors.AliasProcessor{
		DB:       roomserverDB,
		Cfg:      base.Cfg,
		Filter:   aliasFilter,
		Cache:    repoCache,
		Repo:     repo,
		Idg:      idg,
		InputAPI: &inputAPI,
		//QueryAPI: &queryAPI,
	}
	//aliasAPI.Init()

	fedProcessor := processors.FedProcessor{
		Alias: aliasAPI,
	}

	inputAPI.SetFed(&fedProcessor)
	inputAPI.NewMonitor()
	inputAPI.Start()
	if processEvent {
		consumer := consumers.NewInputRoomEventConsumer(
			base.Cfg, &inputAPI, rpcClient,
		)
		if err := consumer.Start(); err != nil {
			log.Panicw("failed to start api server consumer", log.KeysAndValues{"error", err})
		}
	}

	rsRpcCli := rpc.NewRoomserverRpcClient(base.Cfg, rpcClient, &aliasAPI, &queryAPI, &inputAPI)

	rpcConsumer := consumers.NewQueryConsumer(base.Cfg, roomserverDB, repo, umsRepo, rpcClient, &aliasAPI, &queryAPI)
	rpcConsumer.Start()

	return &inputAPI, rsRpcCli, roomserverDB
}

func FixCorruptRooms(
	base *basecomponent.BaseDendrite,
) {
	rdb, err := common.GetDBInstance("roomserver", base.Cfg)
	if err != nil {
		log.Panicw("failed to connect to room server db", log.KeysAndValues{"error", err})
	}

	roomserverDB := rdb.(model.RoomServerDatabase)
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	roomserverDB.SetIDGenerator(idg)
	log.Errorf("%v %v", roomserverDB, err)
	roomserverDB.FixCorruptRooms()
}
