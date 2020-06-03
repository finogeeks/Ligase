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
)

func StartCacheLoader(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	//cachewriter.SetupCacheWriterComponent(base)
	//loader 本身不再去读写，只加载到Kafka，由persist服务正常处理
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()
	transportMultiplexer.Start()

	roomDB := base.CreateRoomDB()
	accountDB := base.CreateAccountsDB()
	deviceDB := base.CreateDeviceDB()
	pushDB := base.CreatePushApiDB()
	e2eDB := base.CreateEncryptApiDB()
	presenceDB := base.CreatePresenceDB()

	roomDB.RecoverCache()
	accountDB.RecoverCache()
	deviceDB.RecoverCache()
	pushDB.RecoverCache()
	e2eDB.RecoverCache()
	presenceDB.RecoverCache()
}
