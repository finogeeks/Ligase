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

package pushapi

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/pushapi/api"
	"github.com/finogeeks/ligase/storage/model"
)

func SetupPushAPIComponent(
	base *basecomponent.BaseDendrite,
	redisCache service.Cache,
	rpcCli *common.RpcClient,
	pushDataRepo *repos.PushDataRepo,
) model.PushAPIDatabase {
	pushDB := base.CreatePushApiDB()
	apiConsumer := api.NewInternalMsgConsumer(
		*base.Cfg, pushDB, redisCache, rpcCli, pushDataRepo,
	)
	apiConsumer.Start()
	return pushDB
}
