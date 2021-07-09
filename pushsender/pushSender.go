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

package pushsender

import (
	"log"

	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/pushsender/rpc"
	"github.com/finogeeks/ligase/rpc/consul"
	"github.com/finogeeks/ligase/storage/model"
)

func SetupPushSenderComponent(
	base *basecomponent.BaseDendrite,
) model.PushAPIDatabase {
	pushDB := base.CreatePushApiDB()

	grpcServer := rpc.NewServer(base.Cfg, pushDB)
	if err := grpcServer.Start(); err != nil {
		log.Panicf("failed to start push rpc server err:%v", err)
	}
	if base.Cfg.Rpc.Driver == "grpc_with_consul" {
		if base.Cfg.Rpc.ConsulURL == "" {
			log.Panicf("grpc_with_consul consul url is null")
		}
		consulTag := base.Cfg.Rpc.Push.ConsulTagPrefix + "0"
		c := consul.NewConsul(base.Cfg.Rpc.ConsulURL, consulTag, base.Cfg.Rpc.Push.ServerName, base.Cfg.Rpc.Push.Port)
		c.Init()
	}

	return pushDB
}
