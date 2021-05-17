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

package rcsserver

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/rcsserver/api"
	"github.com/finogeeks/ligase/rcsserver/processors"
	"github.com/finogeeks/ligase/rcsserver/rpc"
	rrpc "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/consul"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func SetupRCSServerComponent(
	base *basecomponent.BaseDendrite, rpcClient *common.RpcClient, rpcCli rrpc.RpcClient,
) {
	dbIface, err := common.GetDBInstance("rcsserver", base.Cfg)
	if err != nil {
		log.Panicw("Failed to connect to rcs server db", log.KeysAndValues{"error", err})
	}

	//monitor := mon.GetInstance()
	//counter := monitor.NewLabeledCounter("rcsserver_hit", []string{"target", "repo", "func"})

	db := dbIface.(model.RCSServerDatabase)
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	db.SetIDGenerator(idg)
	//db.SetMonitor(counter)
	/*
		repo := new(repos.RCSServerRepo)
		repo.SetPersist(db)
		repo.SetMonitor(counter)
	*/
	proc := processors.NewEventProcessor(base.Cfg, idg, db)
	if base.Cfg.Rpc.Driver == "nats" {
		consumer := rpc.NewEventConsumer(base.Cfg, rpcClient, proc)
		if err := consumer.Start(); err != nil {
			log.Panicw("Failed to start rcs event consumer", log.KeysAndValues{"error", err})
		}
	} else {
		grpcServer := rpc.NewServer(base.Cfg, proc)
		if err := grpcServer.Start(); err != nil {
			log.Panicf("failed to start rcs rpc server err:%v", err)
		}
	}

	if base.Cfg.Rpc.Driver == "grpc_with_consul" {
		if base.Cfg.Rpc.ConsulURL == "" {
			log.Panicf("grpc_with_consul consul url is null")
		}
		consulTag := base.Cfg.Rpc.Rcs.ConsulTagPrefix + "0"
		c := consul.NewConsul(base.Cfg.Rpc.ConsulURL, consulTag, base.Cfg.Rpc.Rcs.ServerName, base.Cfg.Rpc.Rcs.Port)
		c.Init()
	}

	apiConsumer := api.NewInternalMsgConsumer(*base.Cfg, rpcClient, rpcCli, idg, db)
	apiConsumer.Start()
}
