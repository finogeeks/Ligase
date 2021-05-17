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
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer
	idg   *uid.UidGenerator
	db    model.ConfigDatabase
	cache service.Cache
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	rpcCli *common.RpcClient,
	rpcClient rpc.RpcClient,
	db model.ConfigDatabase,
	cache service.Cache,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcCli = rpcCli
	c.RpcClient = rpcClient
	c.idg, _ = uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	c.db = db
	c.cache = cache
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("bgmgrproxy", c, c.Cfg.Rpc.ProxyBgmgrApiTopic, &c.Cfg.Rpc.FrontBgMngApi)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyBgmgrApiTopic
}
