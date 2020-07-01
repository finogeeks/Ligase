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
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

type RoomServerUserMemberShip struct {
	uid    string `json:"user_id"`
	join   sync.Map
	leave  sync.Map
	invite sync.Map

	JoinExport   []string `json:"join_map"`
	LeaveExport  []string `json:"leave_map"`
	InviteExport []string `json:"invite_map"`
	dirt         int32
}

func (ums *RoomServerUserMemberShip) init(uid string) {
	ums.uid = uid
	ums.dirt = 0
}

func (rs *RoomServerUserMemberShip) serialize() ([]byte, error) {
	rs.JoinExport = []string{}
	rs.LeaveExport = []string{}
	rs.InviteExport = []string{}

	rs.join.Range(func(key, value interface{}) bool {
		rs.JoinExport = append(rs.JoinExport, key.(string))
		return true
	})
	rs.leave.Range(func(key, value interface{}) bool {
		rs.LeaveExport = append(rs.LeaveExport, key.(string))
		return true
	})
	rs.invite.Range(func(key, value interface{}) bool {
		rs.InviteExport = append(rs.InviteExport, key.(string))
		return true
	})

	return json.Marshal(rs)
}

func (rs *RoomServerUserMemberShip) onEvent(ev *gomatrixserverlib.Event) {
	if ev.Type() == "m.room.member" {
		member := external.MemberContent{}
		json.Unmarshal(ev.Content(), &member)
		pre := ""

		if _, ok := rs.join.Load(ev.RoomID()); ok {
			pre = "join"
		} else if _, ok := rs.leave.Load(ev.RoomID()); ok {
			pre = "leave"
		} else if _, ok := rs.invite.Load(ev.RoomID()); ok {
			pre = "invite"
		}

		log.Debugf("RoomServerUserMemberShip on event pre:%s, now:%s", pre, member.Membership)

		if member.Membership == "join" {
			rs.join.Store(ev.RoomID(), true)
		} else if member.Membership == "invite" {
			rs.invite.Store(ev.RoomID(), true)
		} else if member.Membership == "leave" || member.Membership == "ban" {
			rs.leave.Store(ev.RoomID(), true)
		}

		if member.Membership != pre {
			if pre == "join" {
				rs.join.Delete(ev.RoomID())
			} else if pre == "invite" {
				rs.invite.Delete(ev.RoomID())
			} else if pre == "leave" {
				rs.leave.Delete(ev.RoomID())
			}
			atomic.CompareAndSwapInt32(&rs.dirt, 0, 1)
		}
	}
}

func (rs *RoomServerUserMemberShip) GetJoinMap() *sync.Map {
	return &rs.join
}

func (rs *RoomServerUserMemberShip) GetLeaveMap() *sync.Map {
	return &rs.leave
}

func (rs *RoomServerUserMemberShip) GetInviteMap() *sync.Map {
	return &rs.invite
}

type RoomServerUserMembershipRepo struct {
	userMemberships sync.Map
	persist         model.RoomServerDatabase

	queryHitCounter mon.LabeledCounter
}

func (repo *RoomServerUserMembershipRepo) SetPersist(db model.RoomServerDatabase) {
	repo.persist = db
}

func (repo *RoomServerUserMembershipRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	repo.queryHitCounter = queryHitCounter
}

func (repo *RoomServerUserMembershipRepo) Start() {
	ticker := time.NewTimer(0)
	go func() {
		for {
			select {
			case <-ticker.C:
				ticker.Reset(time.Second * 15)
				repo.FlushUserMembership()
			}
		}
	}()
}

func (repo *RoomServerUserMembershipRepo) GetUserMemberShip(uid string) *RoomServerUserMemberShip {
	log.Debugf("GetUserMemberShip %s", uid)
	if val, ok := repo.userMemberships.Load(uid); ok {
		log.Debugf("GetUserMemberShip %s in cache, val :%v", uid, *(val.(*RoomServerUserMemberShip)))
		repo.queryHitCounter.WithLabelValues("cache", "RoomServerUserMembershipRepo", "GetUserMemberShip").Add(1)
		return val.(*RoomServerUserMemberShip)
	}

	repo.queryHitCounter.WithLabelValues("db", "RoomServerUserMembershipRepo", "GetUserMemberShip").Add(1)

	log.Debugf("GetUserMemberShip %s in persisit", uid)
	join, invite, leave, err := repo.persist.GetUserRooms(context.Background(), uid)
	if err != nil {
		log.Errorf("GetUserMemberShip from db uid: %s error: %v", uid, err)
		return nil
	}

	ums := new(RoomServerUserMemberShip)
	ums.init(uid)

	for _, id := range join {
		ums.join.Store(id, true)
	}
	for _, id := range invite {
		ums.invite.Store(id, true)
	}
	for _, id := range leave {
		ums.leave.Store(id, true)
	}
	val, _ := repo.userMemberships.LoadOrStore(uid, ums)
	return val.(*RoomServerUserMemberShip)
}

func (repo *RoomServerUserMembershipRepo) FlushUserMembership() {
	repo.userMemberships.Range(func(key, value interface{}) bool {
		ums := value.(*RoomServerUserMemberShip)
		dirt := atomic.LoadInt32(&ums.dirt)
		if dirt == 1 {
			bytes, err := ums.serialize()
			if err == nil {
				atomic.CompareAndSwapInt32(&ums.dirt, 1, 0)
				log.Infof("FlushUserMembership bytes: %s, dirt: %d", string(bytes), ums.dirt)
			}
		}

		return true
	})
}

func (repo *RoomServerUserMembershipRepo) OnEvent(ev *gomatrixserverlib.Event) {
	if ev.Type() == "m.room.member" {
		var ums *RoomServerUserMemberShip
		val, ok := repo.userMemberships.Load(*ev.StateKey())
		if ok {
			ums = val.(*RoomServerUserMemberShip)
		} else {
			ums = repo.GetUserMemberShip(*ev.StateKey())
			repo.userMemberships.Store(*ev.StateKey(), ums)
		}
		ums.onEvent(ev)
	}
}
