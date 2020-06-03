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

package devicemgr

import (
	"context"
	"github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"time"
)

type DeviceMgr struct {
	deviceDB     model.DeviceDatabase
	cache        service.Cache
	encryptDB    model.EncryptorAPIDatabase
	syncDB       model.SyncAPIDatabase
	rpcClient    *common.RpcClient
	tokenFilter  *filter.Filter
	scanUnActive int64
	kickUnActive int64
}

func NewDeviceMgr(
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	rpcClient *common.RpcClient,
	tokenFilter *filter.Filter,
	scanUnActive int64,
	kickUnActive int64,
) *DeviceMgr {
	dm := new(DeviceMgr)
	dm.deviceDB = deviceDB
	dm.cache = cache
	dm.encryptDB = encryptDB
	dm.syncDB = syncDB
	dm.rpcClient = rpcClient
	dm.tokenFilter = tokenFilter
	dm.scanUnActive = scanUnActive
	dm.kickUnActive = kickUnActive
	return dm
}

func (dm *DeviceMgr) Start() {
	go func() {
		t := time.NewTicker(time.Millisecond * time.Duration(dm.scanUnActive))
		for {
			select {
			case <-t.C:
				func() {
					span, ctx := common.StartSobSomSpan(context.Background(), "DeviceMgr.Start")
					defer span.Finish()
					dm.scanUnActionDevice(ctx)
				}()
			}
		}
	}()
}

func (dm *DeviceMgr) scanUnActionDevice(ctx context.Context) {
	offset := 0
	finish := false
	limit := 500
	//s
	kickUnActive, _ := dm.cache.GetSetting("im.setting.autoLogoutTime")
	//ms
	kickUnActive = kickUnActive * 1000
	if kickUnActive <= 0 {
		kickUnActive = dm.kickUnActive
	}
	lastActiveTs := time.Now().UnixNano()/1000000 - kickUnActive

	for {
		if finish {
			return
		}

		log.Infof("load unactive device timestamp:%d,limit:%d @offset:%d", lastActiveTs, limit, offset)
		devids, dids, uids, total, err := dm.deviceDB.SelectUnActiveDevice(ctx, lastActiveTs, limit, offset)
		if err != nil {
			log.Errorf("load unactive device with err %v", err)
			return
		}
		for idx := range dids {
			log.Infof("kick out userId:%s,deviceId:%s", uids[idx], devids[idx])
			go routing.LogoutDevice(ctx, uids[idx], devids[idx], dm.deviceDB, dm.cache, dm.encryptDB, dm.syncDB, dm.tokenFilter, dm.rpcClient)
		}

		if total < limit {
			finish = true
		} else {
			offset = offset + limit
		}
	}
}
