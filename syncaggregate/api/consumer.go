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

package api

import (
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/syncaggregate/sync"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer
	idg              *uid.UidGenerator
	sm               *sync.SyncMng
	userTimeLine     *repos.UserTimeLineRepo
	keyChangeRepo    *repos.KeyChangeStreamRepo
	stdEventTimeline *repos.STDEventStreamRepo
	db               model.SyncAPIDatabase
	cache            service.Cache
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	rpcClient rpc.RpcClient,
	idg *uid.UidGenerator,
	sm *sync.SyncMng,
	userTimeLine *repos.UserTimeLineRepo,
	keyChangeRepo *repos.KeyChangeStreamRepo,
	stdEventTimeline *repos.STDEventStreamRepo,
	db model.SyncAPIDatabase,
	cache service.Cache,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcClient = rpcClient
	c.idg = idg
	c.sm = sm
	c.userTimeLine = userTimeLine
	c.keyChangeRepo = keyChangeRepo
	c.stdEventTimeline = stdEventTimeline
	c.db = db
	c.cache = cache
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("synaggregatecapi", c, c.Cfg.Rpc.ProxySyncAggregateApiTopic, &c.Cfg.Rpc.SyncAggregateApi)
	//c.APIConsumer.InitGroup("synaggregatecapi", c, c.Cfg.Rpc.ProxySyncAggregateApiTopic,types.SYNC_AGGR_GROUP)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxySyncAggregateApiTopic
}
