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
	"github.com/finogeeks/ligase/cachewriter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/dbupdates"
	"github.com/finogeeks/ligase/dbwriter"
)

func StartPersistServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addConsumer(transportMultiplexer, kafka.Consumer.CacheUpdates, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.DBUpdates, base.Cfg.MultiInstance.Instance)

	transportMultiplexer.PreStart()

	cachewriter.SetupCacheWriterComponent(base)
	dbwriter.SetupDBWriterComponent(base)

	dbupdates.SetupDBUpdateComponent(base.Cfg)
	dbupdates.SetupCacheUpdateComponent(base.Cfg)
}
