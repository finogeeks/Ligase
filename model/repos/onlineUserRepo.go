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
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

//ios not call report state, last_state 、cur_state = OFFLINE_STATE
//so ios will not notify, only android call report state and notify
const (
	OFFLINE_STATE = iota
	FORE_GROUND_STATE
	BACK_GROUND_STATE
)

//user state differ foreground, background online
const (
	USER_OFFLINE_STATE = iota
	USER_ONLINE_STATE
)

type StateChangeHandler interface {
	OnStateChange(*types.NotifyDeviceState)
	OnUserStateChange(*types.NotifyUserState)
}

type DefaultHander struct {
}

func (d *DefaultHander) OnStateChange(*types.NotifyDeviceState) {
}

func (d *DefaultHander) OnUserStateChange(*types.NotifyUserState) {

}

type Device struct {
	ts        int64
	id        string
	pos       int64
	lastState int
	curState  int
}

type User struct {
	id     string
	ts 	   int64
	lastState int
	curState  int
	devMap sync.Map
}

func (u *User) isEmpty() bool {
	cnt := 0
	u.devMap.Range(func(key, value interface{}) bool {
		cnt = 1
		return false
	})

	return cnt == 0
}

func (u *User) checkDeviceTtl(ol *OnlineUserRepo, now, ttl int64) int {
	cnt := 0
	hasDelete := false
	if u.ts+ttl < now {
		u.lastState = u.curState
		u.curState = OFFLINE_STATE
		if u.lastState != u.curState {
			ol.UpdateUserState(u.id, u.lastState, u.curState)
		}
	}
	u.devMap.Range(func(key, value interface{}) bool {
		dev := value.(*Device)
		if dev.ts+ttl < now {
			hasDelete = true
			u.devMap.Delete(key)
			atomic.AddInt32(&ol.onlineDeviceCnt, -1)
			dev.lastState = dev.curState
			dev.curState = OFFLINE_STATE
			if dev.lastState != dev.curState {
				ol.Notify(u.id, key.(string), dev.lastState, dev.curState)
			}
		} else {
			cnt = cnt + 1
		}

		return true
	})
	if hasDelete {
		log.Infof("device offline uid:%s remain:%d", u.id, cnt)
	}

	return cnt
}

//只维护在线
type OnlineUserRepo struct {
	onlineUserCnt   int32
	onlineDeviceCnt int32
	userMap         sync.Map
	timer           time.Timer
	handler         StateChangeHandler
}

func NewOnlineUserRepo(ttl, ttlIOS int64) *OnlineUserRepo {
	ol := new(OnlineUserRepo)
	ol.onlineUserCnt = 0
	ol.onlineDeviceCnt = 0
	ol.handler = new(DefaultHander)
	go ol.clean(ttl, ttlIOS)
	return ol
}

func (ol *OnlineUserRepo) Pet(uid, devId string, pos, ttl int64) {
	var user *User
	var dev *Device
	if val, ok := ol.userMap.Load(uid); ok {
		user = val.(*User)
		user.ts = time.Now().Unix()
		user.lastState = user.curState
		user.curState = USER_ONLINE_STATE
	} else {
		user = new(User)
		user.id = uid
		user.ts = time.Now().Unix()
		user.lastState = user.curState
		user.curState = USER_ONLINE_STATE
		ol.userMap.Store(uid, user)
		atomic.AddInt32(&ol.onlineUserCnt, 1)
	}
	if user.lastState != user.curState {
		ol.UpdateUserState(user.id, user.lastState, user.curState)
	}
	if val, ok := user.devMap.Load(devId); ok {
		dev = val.(*Device)
		dev.ts = time.Now().Unix()
		dev.pos = pos

	} else {
		dev = new(Device)
		dev.id = devId
		dev.ts = time.Now().Unix()
		dev.pos = pos

		user.devMap.Store(devId, dev)
		atomic.AddInt32(&ol.onlineDeviceCnt, 1)
	}
	if dev != nil {
		if dev.curState == OFFLINE_STATE {
			ol.UpdateState(uid, devId, FORE_GROUND_STATE)
		}
	}
}

func (ol *OnlineUserRepo) UpdateUserState(uid string, lastState,curState int){
	ol.handler.OnUserStateChange(&types.NotifyUserState{
		UserID:    uid,
		LastState: lastState,
		CurState:  curState,
	})
}

func (ol *OnlineUserRepo) UpdateState(uid, devId string, state int) {
	var user *User
	var dev *Device
	if val, ok := ol.userMap.Load(uid); ok {
		user = val.(*User)
	} else {
		user = new(User)
		user.id = uid
		ol.userMap.Store(uid, user)
		atomic.AddInt32(&ol.onlineUserCnt, 1)
	}
	log.Infof("online repo presence user:%s device:%s state:%d", uid, devId, state)
	if val, ok := user.devMap.Load(devId); ok {
		dev = val.(*Device)
		dev.ts = time.Now().Unix()
		dev.lastState = dev.curState
		dev.curState = state
		if dev.lastState != dev.curState {
			ol.Notify(uid, devId, dev.lastState, dev.curState)
		}
	} else {
		dev = new(Device)
		dev.id = devId
		dev.ts = time.Now().Unix()
		dev.lastState = OFFLINE_STATE
		dev.curState = state
		user.devMap.Store(devId, dev)
		atomic.AddInt32(&ol.onlineDeviceCnt, 1)
		if dev.lastState != dev.curState {
			ol.Notify(uid, devId, dev.lastState, dev.curState)
		}
	}
}

func (ol *OnlineUserRepo) GetLastPos(uid, devId string) int64 {
	var user *User
	var dev *Device
	if val, ok := ol.userMap.Load(uid); ok {
		user = val.(*User)
	} else {
		return -1
	}

	if val, ok := user.devMap.Load(devId); ok {
		dev = val.(*Device)
		return dev.pos
	}

	return -1
}

func (ol *OnlineUserRepo) Release(uid, devId string) {
	var user *User
	if val, ok := ol.userMap.Load(uid); ok {
		user = val.(*User)
		if _, ok := user.devMap.Load(devId); ok {
			user.devMap.Delete(devId)
			atomic.AddInt32(&ol.onlineDeviceCnt, -1)
		}
	}
}

func (ol *OnlineUserRepo) clean(ttl, ttlIOS int64) {
	t := time.NewTimer(time.Second * 5) //5s timer
	for {
		select {
		case <-t.C:
			now := time.Now().Unix()
			ol.userMap.Range(func(key, value interface{}) bool {
				user := value.(*User)
				remain := user.checkDeviceTtl(ol, now, ttl)
				if remain == 0 {
					ol.userMap.Delete(key)
					atomic.AddInt32(&ol.onlineUserCnt, -1)
				}
				return true
			})
			t.Reset(time.Second * 5)
			if ol.onlineUserCnt > 0 || ol.onlineDeviceCnt > 0 {
				log.Infof("OnlineUserRepo online user %d dev: %d", atomic.LoadInt32(&ol.onlineUserCnt), atomic.LoadInt32(&ol.onlineDeviceCnt))
			}
		}
	}

}

func (ol *OnlineUserRepo) SetHandler(handler StateChangeHandler) {
	ol.handler = handler
}

func (ol *OnlineUserRepo) Notify(userID, devID string, lastState, curState int) {
	ol.handler.OnStateChange(&types.NotifyDeviceState{
		UserID:    userID,
		DeviceID:  devID,
		LastState: lastState,
		CurState:  curState,
	})
}