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
	"github.com/finogeeks/ligase/common/uid"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncaggregate"
)

func StartSyncAggregate(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	addProducer(transportMultiplexer, kafka.Producer.FedEduUpdate)
	addProducer(transportMultiplexer, kafka.Producer.DeviceStateUpdate)
	addProducer(transportMultiplexer, kafka.Producer.OutputProfileData)
	addProducer(transportMultiplexer, kafka.Producer.OutputStatic)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputClientData, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputProfileSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdateSyncAggregate, base.Cfg.MultiInstance.Instance)

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()
	serverConfDB := base.CreateServerConfDB()
	cache := base.PrepareCache()
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	domain.GetDomainMngInstance(cache, serverConfDB, base.Cfg.Matrix.ServerName, base.Cfg.Matrix.ServerFromDB, idg)
	base.CheckDomainCfg()

	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}

	accountDB := base.CreateAccountsDB()
	complexCache := common.NewComplexCache(accountDB, cache)
	complexCache.SetDefaultAvatarURL(base.Cfg.DefaultAvatar)

	syncaggregate.SetupSyncAggregateComponent(base, cache, rpcCli, idg, complexCache)
}
