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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/proxy"
)

func StartApiGateWay(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.FedBridgeOut)
	addProducer(transportMultiplexer, kafka.Producer.FedBridgeOutHs)

	addConsumer(transportMultiplexer, kafka.Consumer.FedBridgeOutRes, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SetttngUpdateProxy, base.Cfg.MultiInstance.Instance)

	transportMultiplexer.PreStart()
	cache := base.PrepareCache()

	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)
	rsRpcCli := base.CreateRsRPCCli(rpcClient)

	serverConfDB := base.CreateServerConfDB()
	domain.GetDomainMngInstance(cache, serverConfDB, base.Cfg.Matrix.ServerName, base.Cfg.Matrix.ServerFromDB, idg)
	base.CheckDomainCfg()

	deviceDB := base.CreateDeviceDB()

	tokenFilter := filter.NewSimpleFilter(deviceDB)
	tokenFilter.Load()

	proxy.SetupProxy(base, cache, rpcClient, rsRpcCli, tokenFilter)
}
