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

package bgmng

import (
	"github.com/finogeeks/ligase/bgmng/api"
	"github.com/finogeeks/ligase/bgmng/devicemng"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func SetupBgMngComponent(
	base *basecomponent.BaseDendrite,
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	servernameDB model.ConfigDatabase,
	rpcClient rpc.RpcClient,
	tokenFilter *filter.Filter,
	scanUnActive int64,
	kickUnActive int64,
) {
	deviceMng := devicemng.NewDeviceMng(deviceDB, cache, encryptDB, syncDB, rpcClient, tokenFilter, scanUnActive, kickUnActive)
	log.Infof("scantime:%d,kicktime:%d", scanUnActive, kickUnActive)
	deviceMng.Start()
	apiConsumer := api.NewInternalMsgConsumer(
		*base.Cfg,
		rpcClient,
		servernameDB,
		cache,
	)
	apiConsumer.Start()
}
