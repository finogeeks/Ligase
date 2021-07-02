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

package tokenrewrite

import (
	"log"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/rpc/consul"
	"github.com/finogeeks/ligase/tokenrewrite/rpc"
)

func SetupTokenRewrite(
	cfg *config.Dendrite,
) {
	grpcServer := rpc.NewServer(cfg)
	if err := grpcServer.Start(); err != nil {
		log.Panicf("failed to start tokenwriter rpc server err:%v", err)
	}
	if cfg.Rpc.Driver == "grpc_with_consul" {
		if cfg.Rpc.ConsulURL == "" {
			log.Panicf("grpc_with_consul consul url is null")
		}
		consulTag := cfg.Rpc.TokenWriter.ConsulTagPrefix + "0"
		c := consul.NewConsul(cfg.Rpc.ConsulURL, consulTag, cfg.Rpc.TokenWriter.ServerName, cfg.Rpc.TokenWriter.Port)
		c.Init()
	}
}
