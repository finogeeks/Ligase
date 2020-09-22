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
	"github.com/robfig/cron"
)

//ios not call report state, last_state 、cur_state = OFFLINE_STATE
//so ios will not notify, only android call report state and notify
const (
	OFFLINE_STATE = iota
	FORE_GROUND_STATE
	BACK_GROUND_STATE
)

//user state not differ foreground, background online
const (
	USER_OFFLINE_STATE = iota
	USER_ONLINE_STATE
)

type StateChangeHandler interface {
	OnStateChange(*types.NotifyDeviceState)
	OnUserStateChange(*types.NotifyUserState)
	OnBatchStateChange([]*types.NotifyDeviceState)
	OnUserOnlineDetail([]*types.NotifyOnlineDetail)
}

type DefaultHander struct {
}

func (d *DefaultHander) OnStateChange(*types.NotifyDeviceState) {
}

func (d *DefaultHander) OnUserStateChange(*types.NotifyUserState) {
}

func (d *DefaultHander) OnBatchStateChange([]*types.NotifyDeviceState) {
}

func (d *DefaultHander) OnUserOnlineDetail([]*types.NotifyOnlineDetail){
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
	lastActive int64
	lastCalc   int64
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
	spec 			string
	detailSpec      string
	cron  			*cron.Cron
	ttl				int64
	ttlReport  		int64
	onlineDetail    sync.Map
}

func NewOnlineUserRepo(ttl, ttlReport int64, spec, detailSpec string) *OnlineUserRepo {
	ol := new(OnlineUserRepo)
	ol.onlineUserCnt = 0
	ol.onlineDeviceCnt = 0
	ol.ttl = ttl
	ol.ttlReport = ttlReport
	ol.spec = spec
	ol.detailSpec = detailSpec
	ol.cron = cron.New()
	ol.cron.AddFunc(ol.spec, func() {
		go ol.job()
	})
	ol.cron.AddFunc(ol.detailSpec, func(){
		go ol.detailJob()
	})
	ol.cron.Start()
	ol.handler = new(DefaultHander)
	go ol.clean()
	go ol.onReport()
	return ol
}

func (ol *OnlineUserRepo) job(){
	batch := []*types.NotifyDeviceState{}
	ol.userMap.Range(func(key, value interface{}) bool {
		user := value.(*User)
		user.devMap.Range(func(k, v interface{}) bool {
			dev := v.(*Device)
			batch = append(batch, &types.NotifyDeviceState{
				UserID:    user.id,
				DeviceID:  dev.id,
				LastState: dev.lastState,
				CurState:  dev.curState,
			})
			return true
		})
		return true
	})
	log.Infof("cron job get online device len:%d", len(batch))
	if len(batch) > 0 {
		ol.handler.OnBatchStateChange(batch)
	}
}

func (ol *OnlineUserRepo) detailJob(){
	now := time.Now().UnixNano()/1000000
	ol.userMap.Range(func(key, value interface{}) bool {
		user := value.(*User)
		user.lastActive = now
		ol.calcOnlineDuration(user)
		return true
	})
	batch := []*types.NotifyOnlineDetail{}
	ol.onlineDetail.Range(func(k, v interface{}) bool {
		batch = append(batch, v.(*types.NotifyOnlineDetail))
		ol.onlineDetail.Delete(k)
		return true
	})
	log.Infof("cron detail job get online detail len:%d", len(batch))
	if len(batch) > 0 {
		ol.handler.OnUserOnlineDetail(batch)
	}
}

func (ol *OnlineUserRepo) Pet(uid, devId string, pos, ttl int64) {
	var user *User
	var dev *Device
	if val, ok := ol.userMap.Load(uid); ok {
		user = val.(*User)
		user.ts = time.Now().Unix()
		user.lastState = user.curState
		user.curState = USER_ONLINE_STATE
		user.lastActive = time.Now().UnixNano() / 1000000
	} else {
		user = new(User)
		user.id = uid
		user.ts = time.Now().Unix()
		user.lastState = user.curState
		user.curState = USER_ONLINE_STATE
		user.lastActive = time.Now().UnixNano() / 1000000
		ol.userMap.Store(uid, user)
		atomic.AddInt32(&ol.onlineUserCnt, 1)
	}
	ol.calcOnlineDuration(user)
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

func (ol *OnlineUserRepo) calcOnlineDuration(user *User){
	duration := int64(0)
	//cur online
	if user.curState == USER_ONLINE_STATE {
		//offline to online
		if user.lastState == USER_OFFLINE_STATE {
			duration = 0
		}else{
			//online remain online
			if user.lastCalc != 0 {
				duration = time.Now().UnixNano()/1000000 - user.lastCalc
			}else{
				duration = 0
			}
		}
	//cur offline
	}else{
		//online to offline
		if user.lastState == USER_ONLINE_STATE {
			if user.lastCalc != 0 {
				duration = time.Now().UnixNano()/1000000 - user.lastCalc
			}else{
				duration = 0
			}
		}else{
			//offline remain offline
			duration = 0
		}
	}
	user.lastCalc = time.Now().UnixNano()/1000000
	if v, ok := ol.onlineDetail.Load(user.id); ok {
		detail := v.(*types.NotifyOnlineDetail)
		detail.Duration += duration
		detail.LastActive = user.lastActive
	}else{
		ol.onlineDetail.Store(user.id, &types.NotifyOnlineDetail{
			UserID: user.id,
			Duration: duration,
			LastActive: user.lastActive,
		})
	}
}

func (ol *OnlineUserRepo) onReport(){
	t := time.NewTimer(time.Second * time.Duration(ol.ttlReport))
	for {
		select {
		case <-t.C:
			batch := []*types.NotifyOnlineDetail{}
			ol.onlineDetail.Range(func(k, v interface{}) bool {
				batch = append(batch, v.(*types.NotifyOnlineDetail))
				ol.onlineDetail.Delete(k)
				return true
			})
			if len(batch) > 0 {
				ol.handler.OnUserOnlineDetail(batch)
			}
		}
		t.Reset(time.Second * time.Duration(ol.ttlReport))
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

func (ol *OnlineUserRepo) clean() {
	t := time.NewTimer(time.Second * 5) //5s timer
	for {
		select {
		case <-t.C:
			now := time.Now().Unix()
			ol.userMap.Range(func(key, value interface{}) bool {
				user := value.(*User)
				remain := user.checkDeviceTtl(ol, now, ol.ttl)
				if remain == 0 {
					user.lastState = user.curState
					user.curState = USER_OFFLINE_STATE
					if user.lastState != user.curState {
						ol.UpdateUserState(user.id, user.lastState, user.curState)
					}
					ol.calcOnlineDuration(user)
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

func (ol *OnlineUserRepo) GetOnlineUserCount() int64 {
	return int64(ol.onlineUserCnt)
}

func (ol *OnlineUserRepo) GetOnlineUsers() []string {
	users := []string{}
	now := time.Now().Unix()
	ol.userMap.Range(func(key, value interface{}) bool {
		user := value.(*User)
		remain := user.checkDeviceTtl(ol, now, ol.ttl)
		if remain == 0 {
			user.lastState = user.curState
			user.curState = USER_OFFLINE_STATE
			if user.lastState != user.curState {
				ol.UpdateUserState(user.id, user.lastState, user.curState)
			}
			ol.calcOnlineDuration(user)
			ol.userMap.Delete(key)
			atomic.AddInt32(&ol.onlineUserCnt, -1)
		}else{
			users = append(users, user.id)
		}
		return true
	})
	return users
}