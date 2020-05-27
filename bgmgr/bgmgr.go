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

package bgmgr

import (
	"github.com/finogeeks/ligase/bgmgr/devicemgr"
	"github.com/finogeeks/ligase/bgmgr/txnmgr"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func SetupBgMgrComponent(
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	rpcCli *common.RpcClient,
	tokenFilter *filter.Filter,
	scanUnActive int64,
	kickUnActive int64,
) {
	deviceMgr := devicemgr.NewDeviceMgr(deviceDB, cache, encryptDB, syncDB, rpcCli, tokenFilter, scanUnActive, kickUnActive)
	log.Infof("scantime:%d,kicktime:%d", scanUnActive, kickUnActive)
	deviceMgr.Start()
	txnMgr := txnmgr.NewTxnMgr(cache)
	txnMgr.Start()
}
