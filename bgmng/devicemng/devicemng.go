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

package devicemng

import (
	"context"
	"github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/storage/model"
	"time"
)

type DeviceMng struct {
	deviceDB     model.DeviceDatabase
	cache        service.Cache
	encryptDB    model.EncryptorAPIDatabase
	syncDB       model.SyncAPIDatabase
	rpcClient    *common.RpcClient
	tokenFilter  *filter.Filter
	scanUnActive int64
	kickUnActive int64
}

func NewDeviceMng(
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	rpcClient *common.RpcClient,
	tokenFilter *filter.Filter,
	scanUnActive int64,
	kickUnActive int64,
) *DeviceMng {
	dm := new(DeviceMng)
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

func (dm *DeviceMng) Start() {
	go func() {
		t := time.NewTimer(time.Millisecond * time.Duration(dm.scanUnActive))
		for {
			select {
			case <-t.C:
				dm.scanUnActionDevice()
				t.Reset(time.Millisecond * time.Duration(dm.scanUnActive))
			}
		}
	}()
}

func (dm *DeviceMng) scanUnActionDevice() {
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
	ctx := context.Background()

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
			go routing.LogoutDevice(uids[idx], devids[idx], dm.deviceDB, dm.cache, dm.encryptDB, dm.syncDB, dm.tokenFilter, dm.rpcClient)
		}

		if total < limit {
			finish = true
		} else {
			offset = offset + limit
		}
	}
}
