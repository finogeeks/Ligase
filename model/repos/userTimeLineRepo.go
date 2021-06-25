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
	"strconv"
	"sync"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
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

	friendShip sync.Map //user friend ship
	// friendshipReverse sync.Map //user friend ship reverse mapping(for the users who is not in this syncaggregate instance)

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
	timespend := common.NewTimeSpend()
	if ev.Type == "m.room.member" {
		timespend := common.NewTimeSpend()
		userJoin, _ := tl.getJoinRooms(user)
		timespend.WarnIfExceedf(types.DB_EXCEED_TIME, "AddP2PEv user:%s eventID:%s eventOffset:%d get join room", user, ev.EventID, ev.EventOffset)

		timespend.Reset()
		userInvite, _ := tl.getInviteRooms(user)
		timespend.WarnIfExceedf(types.DB_EXCEED_TIME, "AddP2PEv user:%s eventID:%s eventOffset:%d get invite room", user, ev.EventID, ev.EventOffset)

		timespend.Reset()
		userLeave, _ := tl.getLeaveRooms(user)
		timespend.WarnIfExceedf(types.DB_EXCEED_TIME, "AddP2PEv user:%s eventID:%s eventOffset:%d get leave room", user, ev.EventID, ev.EventOffset)

		storeMap := func(m *sync.Map, key string, val int64) {
			if m != nil {
				m.Store(key, val)
			}
		}
		deleteMap := func(m *sync.Map, key string) {
			if m != nil {
				m.Delete(key)
			}
		}

		if user == *ev.StateKey {
			switch membership {
			case "join":
				storeMap(userJoin, ev.RoomID, ev.EventOffset)
				deleteMap(userInvite, ev.RoomID)
				deleteMap(userLeave, ev.RoomID)
			case "leave", "ban":
				deleteMap(userJoin, ev.RoomID)
				deleteMap(userInvite, ev.RoomID)
				storeMap(userLeave, ev.RoomID, ev.EventOffset)
			case "invite":
				deleteMap(userJoin, ev.RoomID)
				storeMap(userInvite, ev.RoomID, ev.EventOffset)
				deleteMap(userLeave, ev.RoomID)
			}
		}
	}
	timespend.Logf(types.DB_EXCEED_TIME, "UserTimeLineRepo.AddP2PEv update roomID:%s,eventNID:%d,user:%s,evoffset:%d,membership:%s", ev.RoomID, ev.EventNID, user, ev.EventOffset, membership)
}

func (tl *UserTimeLineRepo) LoadUserFriendShip(userID string) {
	if _, ok := tl.friendShip.Load(userID); ok {
		tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "LoadUserFriendShip").Add(1)
		return
	}
	joinRooms, err := tl.getJoinRooms(userID)
	if err == nil {
		var joined []string
		joinRooms.Range(func(key, value interface{}) bool {
			joined = append(joined, key.(string))
			return true
		})
		if len(joined) > 0 {
			timespend := common.NewTimeSpend()
			friends, err := tl.persist.GetFriendShip(context.TODO(), joined)
			if err != nil {
				log.Errorf("load db failed UserTimeLineRepo.LoadUserFriendShip user %s spend:%d ms err:%v", userID, timespend.MS(), err)
				return
			}
			timespend.Logf(types.DB_EXCEED_TIME, "load db succ UserTimeLineRepo.LoadUserFriendShip user:%s", userID)
			if friends != nil {
				for _, friend := range friends {
					tl.addFriendShip(userID, friend)
				}
			}
		} else {
			friendMap := new(sync.Map)
			tl.friendShip.LoadOrStore(userID, friendMap)
		}
	}

	tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "LoadUserFriendShip").Add(1)
}

func (tl *UserTimeLineRepo) loadRoomLatest(user string, rooms []string) error {
	batchIdInt64, _ := tl.Idg.Next()
	batchId := strconv.FormatInt(batchIdInt64, 10)
	segmens := common.SplitStringArray(rooms, 500)
	for _, vals := range segmens {
		err := tl.loadRoomLatestByPage(batchId, user, vals)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tl *UserTimeLineRepo) loadRoomLatestByPage(batchId, user string, rooms []string) error {
	timespend := common.NewTimeSpend()
	roomMap, err := tl.persist.GetRoomLastOffsets(context.TODO(), rooms)
	if err != nil {
		log.Errorf("batchId:%s user:%s load db failed UserTimeLineRepo.loadRoomLatest len(rooms):%d spend:%d ms err:%v", batchId, user, len(rooms), timespend.MS(), err)
		return err
	}
	timespend.Logf(types.DB_EXCEED_TIME, "batchId:%s user:%s load db succ UserTimeLineRepo.loadRoomLatest len(rooms):%d", batchId, user, len(rooms))
	if roomMap != nil {
		for roomID, offset := range roomMap {
			tl.UpdateRoomOffset(roomID, offset)
		}
	}
	return nil
}

func (tl *UserTimeLineRepo) loadJoinRoomOffsets(user string, events []string, res *sync.Map) error {
	batchIdInt64, _ := tl.Idg.Next()
	batchId := strconv.FormatInt(batchIdInt64, 10)
	segmens := common.SplitStringArray(events, 500)
	for _, vals := range segmens {
		err := tl.loadJoinRoomOffsetsByPage(batchId, user, vals, res)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tl *UserTimeLineRepo) loadJoinRoomOffsetsByPage(batchId, user string, events []string, res *sync.Map) error {
	timespend := common.NewTimeSpend()
	offsets, _, roomIDs, err := tl.persist.GetJoinRoomOffsets(context.TODO(), events)
	if err != nil {
		log.Errorf("batchId:%s user:%s load db failed UserTimeLineRepo.loadRoomJoinOffsets len(events):%d spend:%d ms err:%v", batchId, user, len(events), timespend.MS(), err)
		return err
	}
	timespend.Logf(types.DB_EXCEED_TIME, "batchId:%s user:%s load db succ UserTimeLineRepo.loadRoomJoinOffsets len(events):%d", batchId, user, len(events))
	for idx, roomID := range roomIDs {
		res.Store(roomID, offsets[idx])
	}
	return nil
}

func (tl *UserTimeLineRepo) doLoad(loadMap, readyMap *sync.Map, key string, override bool, loader func() (interface{}, error)) error {
	if _, ok := readyMap.Load(key); !ok {
		res, err := loader()
		if err != nil {
			return err
		}

		if override {
			loadMap.Store(key, res)
		} else {
			loadMap.LoadOrStore(key, res)
		}
		readyMap.Store(key, true)
	}

	return nil
}

func (tl *UserTimeLineRepo) LoadJoinRooms(user string) error {
	err := tl.doLoad(&tl.join, &tl.joinReady, user, false, func() (interface{}, error) {
		timespend := common.NewTimeSpend()
		rooms, _, events, err := tl.persist.GetRidsForUser(context.TODO(), user)
		if err != nil {
			log.Errorf("load db failed UserTimeLineRepo.GetJoinRooms user %s spend:%d ms err:%v", user, timespend.MS(), err)
			return nil, err
		}
		timespend.Logf(types.DB_EXCEED_TIME, "load db succ UserTimeLineRepo.GetJoinRooms user:%s", user)
		loadrooms := []string{}
		loadEvents := []string{}
		res := new(sync.Map)
		for idx, id := range rooms {
			res.Store(id, int64(-1))
			if tl.GetRoomOffset(id, user, "join") == -1 {
				loadrooms = append(loadrooms, id)
			}
			loadEvents = append(loadEvents, events[idx])
		}
		if len(loadrooms) > 0 {
			err := tl.loadRoomLatest(user, loadrooms)
			if err != nil {
				return nil, err
			}
		}
		if len(loadEvents) > 0 {
			err := tl.loadJoinRoomOffsets(user, loadEvents, res)
			if err != nil {
				return nil, err
			}
		}

		tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "GetJoinRooms").Add(1)
		return res, nil
	})
	return err
}

func (tl *UserTimeLineRepo) getJoinRooms(user string) (*sync.Map, error) {
	err := tl.LoadJoinRooms(user)
	if err != nil {
		return nil, err
	}

	val, _ := tl.join.Load(user)
	tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetJoinRooms").Add(1)
	return val.(*sync.Map), nil
}

func (tl *UserTimeLineRepo) GetJoinRoomsMap(user string) (map[string]int64, error) {
	res, err := tl.getJoinRooms(user)
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	res.Range(func(k, v interface{}) bool {
		m[k.(string)] = v.(int64)
		return true
	})
	return m, nil
}

func (tl *UserTimeLineRepo) GetJoinRoomsArr(user string) ([]string, error) {
	res, err := tl.getJoinRooms(user)
	if err != nil {
		return nil, err
	}
	arr := []string{}
	res.Range(func(k, v interface{}) bool {
		arr = append(arr, k.(string))
		return true
	})
	return arr, nil
}

func (tl *UserTimeLineRepo) CheckIsJoinRoom(user, roomID string) (bool, error) {
	res, err := tl.getJoinRooms(user)
	if err != nil {
		return false, err
	}
	_, ok := res.Load(roomID)
	return ok, nil
}

func (tl *UserTimeLineRepo) GetUserRoomMembership(user, room string) string {
	joined, _ := tl.getJoinRooms(user)
	if joined != nil {
		if _, ok := joined.Load(room); ok {
			return "join"
		}
	}
	invited, _ := tl.getInviteRooms(user)
	if invited != nil {
		if _, ok := invited.Load(room); ok {
			return "invite"
		}
	}
	leaved, _ := tl.getLeaveRooms(user)
	if leaved != nil {
		if _, ok := leaved.Load(room); ok {
			return "leave"
		}
	}
	return "unknown"
}

func (tl *UserTimeLineRepo) LoadInviteRooms(user string) error {
	err := tl.doLoad(&tl.invite, &tl.inviteReady, user, false, func() (interface{}, error) {
		timespend := common.NewTimeSpend()
		rooms, offsets, _, err := tl.persist.GetInviteRidsForUser(context.TODO(), user)
		if err != nil {
			log.Errorf("load db failed UserTimeLineRepo.GetInviteRooms user:%s spend:%d ms err:%v", user, timespend.MS(), err)
			return nil, err
		}
		timespend.Logf(types.DB_EXCEED_TIME, "load db succ UserTimeLineRepo.GetInviteRooms user:%s", user)
		res := new(sync.Map)
		for idx, id := range rooms {
			res.Store(id, offsets[idx])
		}
		tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "GetInviteRooms").Add(1)
		return res, nil
	})
	return err
}

func (tl *UserTimeLineRepo) getInviteRooms(user string) (*sync.Map, error) {
	err := tl.LoadInviteRooms(user)
	if err != nil {
		return nil, err
	}

	val, _ := tl.invite.Load(user)
	tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetInviteRooms").Add(1)
	return val.(*sync.Map), nil
}

func (tl *UserTimeLineRepo) GetInviteRoomsMap(user string) (map[string]int64, error) {
	res, err := tl.getInviteRooms(user)
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	res.Range(func(k, v interface{}) bool {
		m[k.(string)] = v.(int64)
		return true
	})
	return m, nil
}

func (tl *UserTimeLineRepo) LoadLeaveRooms(user string) error {
	err := tl.doLoad(&tl.leave, &tl.leaveReady, user, false, func() (interface{}, error) {
		timespend := common.NewTimeSpend()
		rooms, _, _, err := tl.persist.GetLeaveRidsForUser(context.TODO(), user)
		if err != nil {
			log.Errorf("load db failed UserTimeLineRepo.GetLeaveRooms user:%s spend:%d ms err:%v", user, timespend.MS(), err)
			return nil, err
		}
		timespend.Logf(types.DB_EXCEED_TIME, "load db succ UserTimeLineRepo.GetLeaveRooms user:%s", user)
		res := new(sync.Map)
		for _, id := range rooms {
			res.Store(id, int64(-1))
		}
		tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "GetLeaveRooms").Add(1)
		return res, nil
	})
	return err
}

func (tl *UserTimeLineRepo) getLeaveRooms(user string) (*sync.Map, error) {
	err := tl.LoadLeaveRooms(user)
	if err != nil {
		return nil, err
	}

	val, _ := tl.leave.Load(user)
	tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetLeaveRooms").Add(1)
	return val.(*sync.Map), nil
}

func (tl *UserTimeLineRepo) GetLeaveRoomsMap(user string) (map[string]int64, error) {
	res, err := tl.getLeaveRooms(user)
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	res.Range(func(k, v interface{}) bool {
		m[k.(string)] = v.(int64)
		return true
	})
	return m, nil
}

func (tl *UserTimeLineRepo) CheckUserLoadingReady(user string) bool {
	_, ok := tl.userReady.Load(user)
	return ok
}

func (tl *UserTimeLineRepo) LoadHistory(user string, isHuman bool) {
	if tl.CheckUserLoadingReady(user) == false {
		if isHuman {
			if _, ok := tl.receiptLatest.Load(user); !ok {
				timespend := common.NewTimeSpend()
				maxPos, err := tl.persist.GetUserMaxReceiptOffset(context.TODO(), user)
				if err != nil {
					log.Errorf("load db failed UserTimeLineRepo.LoadReceiptHistory user %s spend:%d err %d", user, timespend.MS(), err)
					return
				}
				timespend.Logf(types.DB_EXCEED_TIME, "load db succ UserTimeLineRepo.LoadReceiptHistory user:%s", user)
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

func (tl *UserTimeLineRepo) GetJoinMembershipOffset(user, roomID string) (offset int64) {
	joins, err := tl.getJoinRooms(user)
	if err != nil || joins == nil {
		return -1
	}
	if offset, ok := joins.Load(roomID); ok {
		return offset.(int64)
	} else {
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
	invites, err := tl.getInviteRooms(user)
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
	leaves, err := tl.getLeaveRooms(user)
	if err != nil || leaves == nil {
		return -1
	}
	if offset, ok := leaves.Load(roomID); ok {
		return offset.(int64)
	} else {
		return -1
	}
}

func (tl *UserTimeLineRepo) ExistsUserEventUpdate(utl int64, token map[string]int64, user, device, traceId string) bool {
	//load token miss, cur handle is full sync
	if token == nil {
		log.Infof("traceId:%s user:%s device:%s utl:%d load token miss", traceId, user, device, utl)
		return true
	}
	//compare token room offset
	for roomID, offset := range token {
		membership := tl.GetUserRoomMembership(user, roomID)
		roomOffset := tl.GetRoomOffset(roomID, user, membership)
		if roomOffset != -1 && offset < roomOffset {
			if membership == "join" {
				if tl.GetJoinMembershipOffset(user, roomID) > 0 {
					log.Infof("traceId:%s user:%s device:%s utl:%d roomID:%s offset:%d roomOffset:%d membership:%s has event", traceId, user, device, utl, roomID, offset, roomOffset, membership)
					return true
				}
			} else {
				log.Infof("traceId:%s user:%s device:%s utl:%d roomID:%s offset:%d roomOffset:%d membership:%s has event", traceId, user, device, utl, roomID, offset, roomOffset, membership)
				return true
			}
		}
	}
	//compare related room offset
	//has new joined
	hasNewJoined := false
	joinedRooms, _ := tl.getJoinRooms(user)
	if joinedRooms != nil {
		joinedRooms.Range(func(key, value interface{}) bool {
			if _, ok := token[key.(string)]; ok {
				return true
			} else {
				if tl.GetJoinMembershipOffset(user, key.(string)) > 0 {
					hasNewJoined = true
					return false
				} else {
					return true
				}
			}
		})
	}
	if hasNewJoined {
		log.Infof("traceId:%s user:%s device:%s utl:%d join new room has event", traceId, user, device, utl)
		return true
	}
	//has new invite
	hasNewInvite := false
	InvitedRooms, _ := tl.getInviteRooms(user)
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
		return true
	}
	//has new leave
	hasNewLeave := false
	LeavedRooms, _ := tl.getLeaveRooms(user)
	if LeavedRooms != nil {
		LeavedRooms.Range(func(key, value interface{}) bool {
			if _, ok := token[key.(string)]; ok {
				return true
			} else {
				//leave room check self has new msg
				if value.(int64) != -1 {
					hasNewLeave = true
					return false
				} else {
					return true
				}
			}
		})
	}
	if hasNewLeave {
		log.Infof("traceId:%s user:%s device:%s utl:%d leave new room has event", traceId, user, device, utl)
		return true
	}
	return false
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
	timespend := common.NewTimeSpend()
	defer func(timespend *common.TimeSpend) {
		log.Infof("update user:%s device:%s token spend:%d ms", user, device, timespend.MS())
	}(timespend)
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
