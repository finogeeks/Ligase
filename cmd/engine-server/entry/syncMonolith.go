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
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/syncaggregate"
	"github.com/finogeeks/ligase/syncserver"
	"github.com/finogeeks/ligase/syncwriter"
)

func StartSyncMonolith(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	addProducer(transportMultiplexer, kafka.Producer.FedEduUpdate)
	addProducer(transportMultiplexer, kafka.Producer.DeviceStateUpdate)
	addProducer(transportMultiplexer, kafka.Producer.OutputProfileData)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncServer, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncWriter, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputClientData, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputProfileSyncServer, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputProfileSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdateSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdateSyncServer, base.Cfg.MultiInstance.Instance)

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()

	cache := base.PrepareCache()

	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(false)

	deviceDB := base.CreateDeviceDB()
	tokenFilter := filter.GetFilterMng().Register("device", deviceDB)
	tokenFilter.Load()

	accountDB := base.CreateAccountsDB()
	complexCache := common.NewComplexCache(accountDB, cache)
	complexCache.SetDefaultAvatarURL(base.Cfg.DefaultAvatar)

	syncwriter.SetupSyncWriterComponent(base)
	syncserver.SetupSyncServerComponent(base, accountDB, cache, rpcClient, idg)
	syncaggregate.SetupSyncAggregateComponent(base, cache, rpcClient, idg, complexCache)
}
