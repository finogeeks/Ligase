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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/syncserver/consumers"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer
	idg             *uid.UidGenerator
	db              model.SyncAPIDatabase
	rsCurState      *repos.RoomCurStateRepo
	rsTimeline      *repos.RoomStateTimeLineRepo
	rmHsTimeline    *repos.RoomHistoryTimeLineRepo
	displayNameRepo *repos.DisplayNameRepo
	receiptConsumer *consumers.ReceiptConsumer
	settings        *common.Settings
	cache           service.Cache
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	rpcCli *common.RpcClient,
	idg *uid.UidGenerator,
	db model.SyncAPIDatabase,
	rsCurState *repos.RoomCurStateRepo,
	rsTimeline *repos.RoomStateTimeLineRepo,
	rmHsTimeline *repos.RoomHistoryTimeLineRepo,
	displayNameRepo *repos.DisplayNameRepo,
	receiptConsumer *consumers.ReceiptConsumer,
	settings *common.Settings,
	cache service.Cache,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcCli = rpcCli
	c.idg = idg
	c.db = db
	c.rsCurState = rsCurState
	c.rsTimeline = rsTimeline
	c.rmHsTimeline = rmHsTimeline
	c.displayNameRepo = displayNameRepo
	c.receiptConsumer = receiptConsumer
	c.settings = settings
	c.cache = cache
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("syncapi", c, c.Cfg.Rpc.ProxySyncApiTopic)
	//c.APIConsumer.InitGroup("syncapi",c,c.Cfg.Rpc.ProxySyncApiTopic,types.SYNC_API_GROUP)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxySyncApiTopic
}
