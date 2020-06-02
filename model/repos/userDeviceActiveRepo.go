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

package repos

import (
	"context"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"sync"
	"time"
)

type DeviceInfo struct {
	ts  int64
	did string
}

type UserDev struct {
	uid    string
	devMap sync.Map
}

type UserDeviceActiveRepo struct {
	persist    model.DeviceDatabase
	userDevMap sync.Map
	timer      time.Timer
	flushDB    bool
	delay      int64
}

func NewUserDeviceActiveRepo(
	delay int64,
	flushDB bool,
) *UserDeviceActiveRepo {
	uda := new(UserDeviceActiveRepo)
	uda.flushDB = flushDB
	uda.delay = delay
	if flushDB {
		uda.startFlush()
	}
	return uda
}

func (uda *UserDeviceActiveRepo) SetPersist(db model.DeviceDatabase) {
	uda.persist = db
}

func (uda *UserDeviceActiveRepo) startFlush() error {
	go func() {
		t := time.NewTimer(time.Millisecond * time.Duration(uda.delay))
		for {
			select {
			case <-t.C:
				func() {
					span, ctx := common.StartSobSomSpan(context.Background(), "UserDeviceActiveRepo.startFlush")
					defer span.Finish()
					uda.flush(ctx)
				}()
				t.Reset(time.Millisecond * time.Duration(uda.delay))
			}
		}
	}()

	return nil
}

func (uda *UserDeviceActiveRepo) UpdateDevActiveTs(uid, devId string) {
	var userDev *UserDev
	var devInfo *DeviceInfo
	if val, ok := uda.userDevMap.Load(uid); ok {
		userDev = val.(*UserDev)
	} else {
		userDev = new(UserDev)
		userDev.uid = uid
		uda.userDevMap.Store(uid, userDev)
	}

	if val, ok := userDev.devMap.Load(devId); ok {
		devInfo = val.(*DeviceInfo)
		devInfo.ts = time.Now().UnixNano() / 1000000
	} else {
		devInfo = new(DeviceInfo)
		devInfo.did = devId
		devInfo.ts = time.Now().UnixNano() / 1000000
		userDev.devMap.Store(devId, devInfo)
	}
}

func (uda *UserDeviceActiveRepo) flush(ctx context.Context) {
	log.Infof("UserDeviceActiveRepo start flush")
	uda.userDevMap.Range(func(key, value interface{}) bool {
		userDev := value.(*UserDev)
		uda.flushToDB(ctx, userDev)
		uda.userDevMap.Delete(key)
		return true
	})
	log.Infof("UserDeviceActiveRepo finished flush")
}

func (uda *UserDeviceActiveRepo) flushToDB(ctx context.Context, userDev *UserDev) {
	if userDev == nil {
		return
	}
	userDev.devMap.Range(func(key, value interface{}) bool {
		devInfo := value.(*DeviceInfo)
		err := uda.persist.UpdateDeviceActiveTs(ctx, devInfo.did, userDev.uid, devInfo.ts)
		if err != nil {
			log.Errorw("UserDeviceActiveRepo flushToDB could not update device last_active_ts", log.KeysAndValues{
				"deviceID", devInfo.did, "userID", userDev.uid, "error", err,
			})
		}
		userDev.devMap.Delete(key)
		return true
	})
}
