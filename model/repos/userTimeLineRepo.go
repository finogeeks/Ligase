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
	"fmt"
	"sync"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/skunkworks/util/cas"
	"github.com/finogeeks/ligase/storage/model"
)

type UserTimeLineRepo struct {
	persist model.SyncAPIDatabase
	cache   service.Cache

	userReady sync.Map //ready for loading

	join      sync.Map //user join rooms
	joinReady sync.Map //ready for user join rooms loading

	invite      sync.Map //user invite rooms
	inviteReady sync.Map //ready for user invite rooms loading

	leave      sync.Map //user leave rooms
	leaveReady sync.Map //ready for user leave rooms loading

	receiptLatest sync.Map //user latest receipt offset

	friendShip        sync.Map //user friend ship
	friendshipReverse sync.Map //user friend ship reverse mapping(for the users who is not in this syncaggregate instance)

	receiptMutex cas.Mutex

	Idg *uid.UidGenerator

	curRoom         sync.Map
	roomOffsets     sync.Map
	roomMutex       cas.Mutex
	queryHitCounter mon.LabeledCounter
}

func NewUserTimeLineRepo(
	idg *uid.UidGenerator,
) *UserTimeLineRepo {
	tls := new(UserTimeLineRepo)
	tls.Idg = idg
	return tls
}

func (tl *UserTimeLineRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *UserTimeLineRepo) SetCache(cache service.Cache) {
	tl.cache = cache
}

func (tl *UserTimeLineRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *UserTimeLineRepo) AddFriendShip(userID, friend string) (hasLoad, hasFriendship bool) {
	if _, ok := tl.friendShip.Load(userID); ok {
		hasFriendship = tl.addFriendShip(userID, friend) //如果没有加载，表示目标用户不在线，无需维护
		return true, hasFriendship
	} else {
		return false, false
	}
}

func (tl *UserTimeLineRepo) addFriendShip(userID, friend string) (hasFriendship bool) {
	friendMap, ok := tl.friendShip.Load(userID)
	if !ok {
		friendMap, _ = tl.friendShip.LoadOrStore(userID, new(sync.Map))
	}
	log.Infof("add friendship by %s %s", userID, friend)
	v, loaded := friendMap.(*sync.Map).LoadOrStore(friend, true)
	if !v.(bool) {
		friendMap.(*sync.Map).Store(friend, true)
	}
	friendReverseMap, ok := tl.friendshipReverse.Load(friend)
	if !ok {
		friendReverseMap, _ = tl.friendshipReverse.LoadOrStore(friend, new(sync.Map))
	}
	friendReverseMap.(*sync.Map).Store(userID, true)
	return v.(bool) && loaded
}

func (tl *UserTimeLineRepo) GetFriendShip(userID string, load bool) *sync.Map {
	if friendMap, ok := tl.friendShip.Load(userID); ok {
		return friendMap.(*sync.Map)
	}
	if load {
		tl.LoadUserFriendShip(userID)
		return tl.GetFriendShip(userID, false)
	}
	return nil
}

func (tl *UserTimeLineRepo) GetFriendshipReverse(userID string) *sync.Map {
	friendReverseMap, ok := tl.friendshipReverse.Load(userID)
	if ok {
		return friendReverseMap.(*sync.Map)
	}
	return nil
}

func (tl *UserTimeLineRepo) SetReceiptLatest(userID string, offset int64) {
	tl.receiptMutex.Lock()
	defer tl.receiptMutex.Unlock()
	val, ok := tl.receiptLatest.Load(userID)
	if ok {
		lastoffset := val.(int64)
		if lastoffset < offset {
			log.Debugf("update receipt lastoffset:%d,offset:%d", lastoffset, offset)
			tl.receiptLatest.Store(userID, offset)
		}
	} else {
		log.Debugf("update receipt first offset:%d", offset)
		tl.receiptLatest.Store(userID, offset)
	}
}

func (tl *UserTimeLineRepo) AddP2PEv(ev *gomatrixserverlib.ClientEvent, user string) {
	membership := "join"
	if ev.Type == "m.room.member" {
		if user == *ev.StateKey {
			member := external.MemberContent{}
			json.Unmarshal(ev.Content, &member)
			membership = member.Membership
			switch membership {
			case "leave", "ban":
				membership = "leave"
			case "invite":
				membership = "invite"
			}
		}
	}
	start := time.Now().UnixNano() / 1000000
	if ev.Type == "m.room.member" {
		var userJoin *sync.Map
		var userInvite *sync.Map
		var userLeave *sync.Map
		bs := time.Now().UnixNano() / 1000000
		userJoin, _ = tl.GetJoinRooms(user)
		spend := time.Now().UnixNano()/1000000 - bs
		if spend > types.DB_EXCEED_TIME {
			log.Infof("AddP2PEv user:%s eventID:%s eventOffset:%d get join room spend:%d", user, ev.EventID, ev.EventOffset, spend)
		}
		if userJoin == nil {
			userJoin = new(sync.Map)
			tl.join.Store(user, userJoin)
		}
		bs = time.Now().UnixNano() / 1000000
		userInvite, _ = tl.GetInviteRooms(user)
		spend = time.Now().UnixNano()/1000000 - bs
		if spend > types.DB_EXCEED_TIME {
			log.Infof("AddP2PEv user:%s eventID:%s eventOffset:%d get invite room spend:%d", user, ev.EventID, ev.EventOffset, spend)
		}
		if userInvite == nil {
			userInvite = new(sync.Map)
			tl.invite.Store(user, userInvite)
		}
		bs = time.Now().UnixNano() / 1000000
		userLeave, _ = tl.GetLeaveRooms(user)
		spend = time.Now().UnixNano()/1000000 - bs
		if spend > types.DB_EXCEED_TIME {
			log.Infof("AddP2PEv user:%s eventID:%s eventOffset:%d get leave room spend:%d", user, ev.EventID, ev.EventOffset, spend)
		}
		if userLeave == nil {
			userLeave = new(sync.Map)
			tl.leave.Store(user, userLeave)
		}
		if user == *ev.StateKey {
			switch membership {
			case "join":
				userJoin.Store(ev.RoomID, ev.EventOffset)
				userInvite.Delete(ev.RoomID)
				userLeave.Delete(ev.RoomID)
			case "leave", "ban":
				userLeave.Store(ev.RoomID, ev.EventOffset)
				userJoin.Delete(ev.RoomID)
				userInvite.Delete(ev.RoomID)
			case "invite":
				userJoin.Delete(ev.RoomID)
				userLeave.Delete(ev.RoomID)
				userInvite.Store(ev.RoomID, ev.EventOffset)
			}
		}
	}
	spend := time.Now().UnixNano()/1000000 - start
	log.Infof("UserTimeLineRepo.AddP2PEv update roomID:%s,eventNID:%d,user:%s,evoffset:%d,membership:%s spend:%d", ev.RoomID, ev.EventNID, user, ev.EventOffset, membership, spend)
}

func (tl *UserTimeLineRepo) LoadUserFriendShip(userID string) {
	if _, ok := tl.friendShip.Load(userID); ok {
		tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "LoadUserFriendShip").Add(1)
		return
	}
	joinRooms, err := tl.GetJoinRooms(userID)
	if err == nil {
		var joined []string
		joinRooms.Range(func(key, value interface{}) bool {
			joined = append(joined, key.(string))
			return true
		})
		if len(joined) > 0 {
			bs := time.Now().UnixNano() / 1000000
			friends, err := tl.persist.GetFriendShip(context.TODO(), joined)
			spend := time.Now().UnixNano()/1000000 - bs
			if err != nil {
				log.Errorf("load db failed UserTimeLineRepo.LoadUserFriendShip user %s spend:%d ms err:%v", userID, spend, err)
				return
			}
			if spend > types.DB_EXCEED_TIME {
				log.Warnf("load db exceed %d ms UserTimeLineRepo.LoadUserFriendShip user:%s spend:%d ms", types.DB_EXCEED_TIME, userID, spend)
			} else {
				log.Infof("load db succ UserTimeLineRepo.LoadUserFriendShip user:%s spend:%d ms", userID, spend)
			}
			if friends != nil {
				for _, friend := range friends {
					tl.addFriendShip(userID, friend)
				}
			}
		} else {
			friendMap := new(sync.Map)
			tl.friendShip.Store(userID, friendMap)
		}
	}

	tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "LoadUserFriendShip").Add(1)
}

func (tl *UserTimeLineRepo) GetJoinRooms(user string) (*sync.Map, error) {
	res := new(sync.Map)
	if _, ok := tl.joinReady.Load(user); !ok {
		bs := time.Now().UnixNano() / 1000000
		rooms, offsets, err := tl.persist.GetRidsForUser(context.TODO(), user)
		spend := time.Now().UnixNano()/1000000 - bs
		if err != nil {
			log.Errorf("load db failed UserTimeLineRepo.GetJoinRooms user %s spend:%d ms err:%v", user, spend, err)
			return res, err
		}
		if spend > types.DB_EXCEED_TIME {
			log.Warnf("load db exceed %d ms UserTimeLineRepo.GetJoinRooms user:%s spend:%d ms", types.DB_EXCEED_TIME, user, spend)
		} else {
			log.Infof("load db succ UserTimeLineRepo.GetJoinRooms user:%s spend:%d ms", user, spend)
		}
		for idx, id := range rooms {
			res.Store(id, offsets[idx])
		}

		tl.join.Store(user, res)
		tl.joinReady.Store(user, true)

		tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "GetJoinRooms").Add(1)
	}

	val, ok := tl.join.Load(user)
	if ok == true {
		tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetJoinRooms").Add(1)
		return val.(*sync.Map), nil
	}

	return res, nil
}

func (tl *UserTimeLineRepo) CheckIsJoinRoom(user, room string) (isJoin bool) {
	joined, _ := tl.GetJoinRooms(user)
	isJoin = false
	if joined != nil {
		if _, ok := joined.Load(room); ok {
			isJoin = true
		}
	}
	return
}

func (tl *UserTimeLineRepo) GetUserRoomMembership(user, room string) string {
	joined, _ := tl.GetJoinRooms(user)
	if joined != nil {
		if _, ok := joined.Load(room); ok {
			return "join"
		}
	}
	invited, _ := tl.GetInviteRooms(user)
	if invited != nil {
		if _, ok := invited.Load(room); ok {
			return "invite"
		}
	}
	leaved, _ := tl.GetLeaveRooms(user)
	if leaved != nil {
		if _, ok := leaved.Load(room); ok {
			return "leave"
		}
	}
	return "unknown"
}

func (tl *UserTimeLineRepo) GetInviteRooms(user string) (*sync.Map, error) {
	res := new(sync.Map)

	if _, ok := tl.inviteReady.Load(user); !ok {
		bs := time.Now().UnixNano() / 1000000
		rooms, offsets, err := tl.persist.GetInviteRidsForUser(context.TODO(), user)
		spend := time.Now().UnixNano()/1000000 - bs
		if err != nil {
			log.Errorf("load db failed UserTimeLineRepo.GetInviteRooms user:%s spend:%d ms err:%v", user, spend, err)
			return nil, err
		}
		if spend > types.DB_EXCEED_TIME {
			log.Warnf("load db exceed %d ms UserTimeLineRepo.GetInviteRooms user:%s spend:%d ms", types.DB_EXCEED_TIME, user, spend)
		} else {
			log.Infof("load db succ UserTimeLineRepo.GetInviteRooms user:%s spend:%d ms", user, spend)
		}
		for idx, id := range rooms {
			res.Store(id, offsets[idx])
		}

		tl.invite.Store(user, res)
		tl.inviteReady.Store(user, true)

		tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "GetInviteRooms").Add(1)
	}

	val, ok := tl.invite.Load(user)
	if ok == true {
		tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetInviteRooms").Add(1)
		return val.(*sync.Map), nil
	}

	return res, nil
}

func (tl *UserTimeLineRepo) GetLeaveRooms(user string) (*sync.Map, error) {
	res := new(sync.Map)

	if _, ok := tl.leaveReady.Load(user); !ok {
		bs := time.Now().UnixNano() / 1000000
		rooms, offsets, err := tl.persist.GetLeaveRidsForUser(context.TODO(), user)
		spend := time.Now().UnixNano()/1000000 - bs
		if err != nil {
			log.Errorf("load db failed UserTimeLineRepo.GetLeaveRooms user:%s spend:%d ms err:%v", user, spend, err)
			return nil, err
		}
		if spend > types.DB_EXCEED_TIME {
			log.Warnf("load db exceed %d ms UserTimeLineRepo.GetLeaveRooms user:%s spend:%d ms", types.DB_EXCEED_TIME, user, spend)
		} else {
			log.Infof("load db succ UserTimeLineRepo.GetLeaveRooms user:%s spend:%d ms", user, spend)
		}
		for idx, id := range rooms {
			res.Store(id, offsets[idx])
		}

		tl.leave.Store(user, res)
		tl.leaveReady.Store(user, true)

		tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "GetLeaveRooms").Add(1)
	}

	val, ok := tl.leave.Load(user)
	if ok == true {
		tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetLeaveRooms").Add(1)
		return val.(*sync.Map), nil
	}

	return res, nil
}

func (tl *UserTimeLineRepo) CheckUserLoadingReady(user string) bool {
	_, ok := tl.userReady.Load(user)
	return ok
}

func (tl *UserTimeLineRepo) LoadHistory(user string, isHuman bool) {
	if tl.CheckUserLoadingReady(user) == false {
		if isHuman {
			if _, ok := tl.receiptLatest.Load(user); !ok {
				bs := time.Now().UnixNano() / 1000000
				maxPos, err := tl.persist.GetUserMaxReceiptOffset(context.TODO(), user)
				spend := time.Now().UnixNano()/1000000 - bs
				if err != nil {
					log.Errorf("load db failed UserTimeLineRepo.LoadReceiptHistory user %s spend:%d err %d", user, spend, err)
					return
				} else {
					if spend > types.DB_EXCEED_TIME {
						log.Warnf("load db exceed %d ms UserTimeLineRepo.LoadReceiptHistory user:%s spend:%d ms", types.DB_EXCEED_TIME, user, spend)
					} else {
						log.Infof("load db succ UserTimeLineRepo.LoadReceiptHistory user:%s spend:%d ms", user, spend)
					}
				}
				log.Warnf("load history update user:%s receipt:%d", user, maxPos)

				tl.SetReceiptLatest(user, maxPos)
			}
			if _, ok := tl.friendShip.Load(user); !ok {
				tl.LoadUserFriendShip(user)
			}
		}
		tl.userReady.Store(user, true)
	}
}

func (tl *UserTimeLineRepo) UpdateRoomOffset(roomID string, offset int64) {
	tl.roomMutex.Lock()
	defer tl.roomMutex.Unlock()
	val, ok := tl.roomOffsets.Load(roomID)
	if ok {
		lastoffset := val.(int64)
		if lastoffset < offset {
			log.Infof("update roomID:%s lastoffset:%d,offset:%d", roomID, lastoffset, offset)
			tl.roomOffsets.Store(roomID, offset)
		}
	} else {
		log.Infof("update roomID:%s first offset:%d ", roomID, offset)
		tl.roomOffsets.Store(roomID, offset)
	}
}

func (tl *UserTimeLineRepo) GetRoomOffset(roomID, user, membership string) int64 {
	switch membership {
	case "invite", "leave":
		return tl.GetNotJoinRoomLatestOffset(roomID, user, membership)
	case "join":
		return tl.GetJoinRoomOffset(roomID)
	default:
		return -1
	}
}

func (tl *UserTimeLineRepo) GetNotJoinRoomLatestOffset(roomID, user, membership string) int64 {
	switch membership {
	case "invite":
		return tl.GetInviteRoomOffset(roomID, user)
	case "leave":
		return tl.GetLeaveRoomOffset(roomID, user)
	default:
		return -1
	}
}

func (tl *UserTimeLineRepo) GetJoinRoomOffset(roomID string) int64 {
	val, ok := tl.roomOffsets.Load(roomID)
	if ok {
		return val.(int64)
	} else {
		return -1
	}
}

func (tl *UserTimeLineRepo) GetInviteRoomOffset(roomID, user string) int64 {
	invites, err := tl.GetInviteRooms(user)
	if err != nil || invites == nil {
		return -1
	}
	if offset, ok := invites.Load(roomID); ok {
		return offset.(int64)
	} else {
		return -1
	}
}

func (tl *UserTimeLineRepo) GetLeaveRoomOffset(roomID, user string) int64 {
	leaves, err := tl.GetLeaveRooms(user)
	if err != nil || leaves == nil {
		return -1
	}
	if offset, ok := leaves.Load(roomID); ok {
		return offset.(int64)
	} else {
		return -1
	}
}

func (tl *UserTimeLineRepo) ExistsUserEventUpdate(utl int64, user, device, traceId string) (bool, int64) {
	curUtl, token, err := tl.LoadToken(user, device, utl)
	//load token from redis err
	if err != nil {
		log.Errorf("traceId:%s user:%s device:%s utl:%d load token err:%v", traceId, user, device, utl, err)
		return false, utl
	}
	//load token miss, cur handle is full sync
	if token == nil {
		log.Infof("traceId:%s user:%s device:%s utl:%d load token miss", traceId, user, device, utl)
		return true, curUtl
	}
	//compare token room offset
	for roomID, offset := range token {
		roomOffset := tl.GetRoomOffset(roomID, user, tl.GetUserRoomMembership(user, roomID))
		if roomOffset != -1 && offset < roomOffset {
			log.Infof("traceId:%s user:%s device:%s utl:%d roomID:%s offset:%d roomOffset:%d has event", traceId, user, device, utl, roomID, offset, roomOffset)
			return true, curUtl
		}
	}
	//compare related room offset
	//has new joined
	hasNewJoined := false
	joinedRooms, _ := tl.GetJoinRooms(user)
	if joinedRooms != nil {
		joinedRooms.Range(func(key, value interface{}) bool {
			if _, ok := token[key.(string)]; ok {
				return true
			} else {
				hasNewJoined = true
				return false
			}
		})
	}
	if hasNewJoined {
		log.Infof("traceId:%s user:%s device:%s utl:%d join new room has event", traceId, user, device, utl)
		return true, curUtl
	}
	//has new invite
	hasNewInvite := false
	InvitedRooms, _ := tl.GetJoinRooms(user)
	if InvitedRooms != nil {
		InvitedRooms.Range(func(key, value interface{}) bool {
			if _, ok := token[key.(string)]; ok {
				return true
			} else {
				hasNewInvite = true
				return false
			}
		})
	}
	if hasNewInvite {
		log.Infof("traceId:%s user:%s device:%s utl:%d invite new room has event", traceId, user, device, utl)
		return true, curUtl
	}
	//has new leave
	hasNewLeave := false
	LeavedRooms, _ := tl.GetLeaveRooms(user)
	if LeavedRooms != nil {
		LeavedRooms.Range(func(key, value interface{}) bool {
			if _, ok := token[key.(string)]; ok {
				return true
			} else {
				hasNewLeave = true
				return false
			}
		})
	}
	if hasNewLeave {
		log.Infof("traceId:%s user:%s device:%s utl:%d leave new room has event", traceId, user, device, utl)
		return true, curUtl
	}
	return false, curUtl
}

func (tl *UserTimeLineRepo) ExistsUserReceiptUpdate(pos int64, user string) (bool, int64) {
	if val, ok := tl.receiptLatest.Load(user); ok {
		u := val.(int64)
		if u > pos {
			return true, u
		}
	}
	return false, 0
}

func (tl *UserTimeLineRepo) GetUserLatestReceiptOffset(user string, isHuman bool) int64 {
	if tl.CheckUserLoadingReady(user) == false {
		tl.LoadHistory(user, isHuman)
	}

	val, ok := tl.receiptLatest.Load(user)
	if ok {
		return val.(int64)
	}
	return -1
}

func (tl *UserTimeLineRepo) LoadToken(user, device string, utl int64) (int64, map[string]int64, error) {
	token, err := tl.cache.GetToken(user, device, utl)
	//get token err, return err
	if err != nil {
		return 0, nil, err
	}
	//has token, return token
	if token != nil {
		return utl, token, nil
	} else {
		//not has token
		//full sync, return
		if utl == 0 {
			return 0, nil, nil
		} else {
			//get latest token
			return tl.cache.GetLastValidToken(user, device)
		}
	}
}

func (tl *UserTimeLineRepo) UpdateToken(user, device string, utl int64, roomOffsets map[string]int64) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("update user:%s device:%s token spend:%d ms", user, device, spend)
	}(bs)
	err := tl.cache.SetToken(user, device, utl, roomOffsets)
	if err != nil {
		log.Errorf("update user:%s device:%s utl:%d err:%v", user, device, utl, err)
		return err
	}
	err = tl.cache.AddTokenUtl(user, device, utl)
	if err != nil {
		log.Warnf("add token utl user:%s device:%s utl:%d err:%v", user, device, utl, err)
		return nil
	}
	utls, err := tl.cache.GetTokenUtls(user, device)
	if err != nil {
		log.Warnf("scan user:%s device:%s token err:%v", user, device, err)
		return nil
	}
	if len(utls) > adapter.GetLatestToken() {
		tl.cache.DelTokens(user, device, utls[adapter.GetLatestToken():])
	}
	return nil
}

func (tl *UserTimeLineRepo) SetUserCurRoom(user, device, room string) {
	key := fmt.Sprintf("%s:%s", user, device)
	tl.curRoom.Store(key, room)
}

func (tl *UserTimeLineRepo) GetUserCurRoom(user, device string) (room string) {
	room = ""
	key := fmt.Sprintf("%s:%s", user, device)
	if val, ok := tl.curRoom.Load(key); ok {
		room = val.(string)
	}
	return
}
