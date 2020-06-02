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

	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/skunkworks/util/cas"
	"github.com/finogeeks/ligase/storage/model"
)

type UserRoomOffset struct {
	userOffset int64
	roomOffset int64
	roomId     string
}

func (u UserRoomOffset) GetRoomId() string {
	return u.roomId
}

func (u UserRoomOffset) GetUserOffset() int64 {
	return u.userOffset
}

func (u UserRoomOffset) GetRoomOffset() int64 {
	return u.roomOffset
}

type UserTimeLineRepo struct {
	persist model.SyncAPIDatabase
	repo    *TimeLineRepo
	loading sync.Map
	ready   sync.Map

	userReady sync.Map //ready for loading

	join      sync.Map //user join rooms
	joinReady sync.Map //ready for user join rooms loading

	invite      sync.Map //user invite rooms
	inviteReady sync.Map //ready for user invite rooms loading

	userLatest     sync.Map //user latest offset
	userRoomLatest sync.Map // user room latest offset

	receiptLatest sync.Map //user latest receipt offset
	lastFail      sync.Map //user last fail request offset

	friendShip        sync.Map //user friend ship
	friendshipReverse sync.Map //user friend ship reverse mapping(for the users who is not in this syncaggregate instance)

	userMinPos sync.Map //user min pos in persist

	userMutex     cas.Mutex
	userRoomMutex cas.Mutex
	receiptMutex  cas.Mutex

	Idg *uid.UidGenerator

	curRoom sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewUserTimeLineRepo(
	bukSize,
	maxEntries,
	gcPerNum int,
	idg *uid.UidGenerator,
) *UserTimeLineRepo {
	tls := new(UserTimeLineRepo)
	tls.repo = NewTimeLineRepo(bukSize, 500, true, maxEntries, gcPerNum)
	tls.Idg = idg
	return tls
}

func (tl *UserTimeLineRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
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

func (tl *UserTimeLineRepo) GetFriendShip(ctx context.Context, userID string, load bool) *sync.Map {
	if friendMap, ok := tl.friendShip.Load(userID); ok {
		return friendMap.(*sync.Map)
	}
	if load {
		tl.LoadUserFriendShip(ctx, userID)
		return tl.GetFriendShip(ctx, userID, false)
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

func (tl *UserTimeLineRepo) SetUserLatest(userID string, offset int64) {
	tl.userMutex.Lock()
	defer tl.userMutex.Unlock()

	val, ok := tl.userLatest.Load(userID)
	if ok {
		lastoffset := val.(int64)
		if lastoffset < offset {
			log.Debugf("update userId:%s,lastoffset:%d,offset:%d", userID, lastoffset, offset)
			tl.userLatest.Store(userID, offset)
		}
	} else {
		log.Debugf("update userId:%s first offset:%d ", userID, offset)
		tl.userLatest.Store(userID, offset)
	}
}

func (tl *UserTimeLineRepo) SetUserRoomLatset(userID, roomId string, offset int64, roomOffset int64) {
	tl.userRoomMutex.Lock()
	defer tl.userRoomMutex.Unlock()
	val, ok := tl.userRoomLatest.Load(userID)
	if ok {
		userRoomOffset := val.(*UserRoomOffset)
		if userRoomOffset.roomOffset <= roomOffset && userRoomOffset.userOffset <= offset {
			userRoomOffset.roomOffset = roomOffset
			userRoomOffset.userOffset = offset
		} else if userRoomOffset.roomOffset > roomOffset && userRoomOffset.userOffset > offset {
			return
		} else {
			log.Warnf("user:%s roomId:%s offset:%d roomOffset:%d lastOffset:%d lastRoomOffset:%d not consistent",
				userID, roomId, offset, roomOffset, userRoomOffset.userOffset, userRoomOffset.roomOffset)
			if userRoomOffset.roomOffset <= roomOffset {
				userRoomOffset.roomOffset = roomOffset
			}
			if userRoomOffset.userOffset <= offset {
				userRoomOffset.userOffset = offset
			}
		}
	} else {
		userRoomOffset := &UserRoomOffset{
			userOffset: offset,
			roomOffset: roomOffset,
			roomId:     roomId,
		}
		tl.userRoomLatest.Store(userID, userRoomOffset)
	}
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

func (tl *UserTimeLineRepo) AddP2PEv(ctx context.Context, ev *gomatrixserverlib.ClientEvent, offset int64, user string) {
	tl.loadUserHistory(ctx, user)

	stream := syncapitypes.UserTimeLineStream{
		Offset:     offset,
		UserID:     user,
		RoomID:     ev.RoomID,
		RoomOffset: ev.EventOffset,
	}

	stream.RoomState = "join"
	var membership string
	if ev.Type == "m.room.member" {
		if user == *ev.StateKey {
			member := external.MemberContent{}
			json.Unmarshal(ev.Content, &member)
			membership = member.Membership
			switch membership {
			case "leave", "ban":
				stream.RoomState = "leave"
			case "invite":
				stream.RoomState = "invite"
			}
		}
	}

	err := tl.persist.InsertUserTimeLine(ctx, offset, ev.RoomID, ev.EventNID, user, stream.RoomState, time.Now().UnixNano()/1000000, ev.EventOffset)
	if err != nil {
		log.Errorf("UserTimeLineRepo InsertUserTimeLine user %s room %s event %d err %v", user, ev.RoomID, ev.EventNID, err)
	}
	log.Infof("UserTimeLineRepo.AddP2PEv insert db offset:%d,roomID:%s,eventNID:%d,user:%s,evoffset:%d,roomstate:%s", offset, ev.RoomID, ev.EventNID, user, ev.EventOffset, stream.RoomState)

	if ev.Type == "m.room.member" {
		var userJoin *sync.Map
		var userInvite *sync.Map

		userJoin, _ = tl.GetJoinRooms(ctx, user)
		userInvite, _ = tl.GetInviteRooms(ctx, user)

		if user == *ev.StateKey {
			switch membership {
			case "join":
				if _, ok := userJoin.Load(ev.RoomID); !ok {
					userJoin.Store(ev.RoomID, true)
				}
				userInvite.Delete(ev.RoomID)
			case "leave", "ban":
				userJoin.Delete(ev.RoomID)
				userInvite.Delete(ev.RoomID)
			case "invite":
				if _, ok := userInvite.Load(ev.RoomID); !ok {
					userInvite.Store(ev.RoomID, true)
				}
				userJoin.Delete(ev.RoomID)
			}
		}
	}

	sev := new(feedstypes.TimeLineEvent)
	sev.Ev = &stream
	sev.Offset = sev.Ev.Offset
	tl.repo.add(user, sev)
	log.Infof("update userTimeLineRepo offset user:%s,room:%s,offset:%d,roomoffset:%d,roomstate:%s", user, sev.Ev.RoomID, sev.Ev.Offset, sev.Ev.RoomOffset, sev.Ev.RoomState)
	tl.SetUserLatest(user, sev.Offset)
	tl.SetUserRoomLatset(user, sev.Ev.RoomID, sev.Ev.Offset, sev.Ev.RoomOffset)
}

func (tl *UserTimeLineRepo) LoadUserFriendShip(ctx context.Context, userID string) {
	if _, ok := tl.friendShip.Load(userID); ok {
		tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "LoadUserFriendShip").Add(1)
		return
	}
	joinRooms, err := tl.GetJoinRooms(ctx, userID)
	if err == nil {
		var joined []string
		joinRooms.Range(func(key, value interface{}) bool {
			joined = append(joined, key.(string))
			return true
		})
		if len(joined) > 0 {
			bs := time.Now().UnixNano() / 1000000
			friends, err := tl.persist.GetFriendShip(ctx, joined)
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

func (tl *UserTimeLineRepo) GetJoinRooms(ctx context.Context, user string) (*sync.Map, error) {
	res := new(sync.Map)

	if _, ok := tl.joinReady.Load(user); !ok {
		bs := time.Now().UnixNano() / 1000000
		rooms, _, err := tl.persist.GetRidsForUser(ctx, user)
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
		for _, id := range rooms {
			res.Store(id, true)
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

func (tl *UserTimeLineRepo) CheckIsJoinRoom(ctx context.Context, user, room string) (isJoin bool) {
	joined, _ := tl.GetJoinRooms(ctx, user)
	isJoin = false
	if joined != nil {
		if _, ok := joined.Load(room); ok {
			isJoin = true
		}
	}
	return
}

func (tl *UserTimeLineRepo) GetInviteRooms(ctx context.Context, user string) (*sync.Map, error) {
	res := new(sync.Map)

	if _, ok := tl.inviteReady.Load(user); !ok {
		bs := time.Now().UnixNano() / 1000000
		rooms, _, err := tl.persist.GetInviteRidsForUser(ctx, user)
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
		for _, id := range rooms {
			res.Store(id, true)
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

func (tl *UserTimeLineRepo) CheckUserLoadingReady(user string) bool {
	_, ok := tl.userReady.Load(user)
	return ok
}

func (tl *UserTimeLineRepo) LoadHistory(ctx context.Context, user string, isHuman bool) {
	if tl.CheckUserLoadingReady(user) == false {
		if _, ok := tl.userMinPos.Load(user); !ok {
			bs := time.Now().UnixNano() / 1000000
			minPos, err := tl.persist.SelectUserTimeLineMinPos(ctx, user)
			spend := time.Now().UnixNano()/1000000 - bs
			if err != nil {
				log.Errorf("load db failed UserTimeLineRepo.SelectUserTimeLineMinPos user %s spend:%d ms err %d", user, spend, err)
			} else {
				if spend > types.DB_EXCEED_TIME {
					log.Warnf("load db exceed %d ms UserTimeLineRepo.SelectUserTimeLineMinPos user:%s spend:%d ms", types.DB_EXCEED_TIME, user, spend)
				} else {
					tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "SelectUserTimeLineMinPos").Add(1)
					log.Infof("load db succ UserTimeLineRepo.SelectUserTimeLineMinPos user:%s spend:%d ms", user, spend)
				}
				tl.userMinPos.Store(user, minPos)
			}
			tl.loadUserHistory(ctx, user)
		} else {
			tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "SelectUserTimeLineMinPos").Add(1)
		}

		if isHuman {
			if _, ok := tl.receiptLatest.Load(user); !ok {
				bs := time.Now().UnixNano() / 1000000
				maxPos, err := tl.persist.GetUserMaxReceiptOffset(ctx, user)
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
				tl.LoadUserFriendShip(ctx, user)
			}
		}
		tl.userReady.Store(user, true)
	}
}

func (tl *UserTimeLineRepo) loadUserHistory(ctx context.Context, user string) {
	for {
		if _, ok := tl.ready.Load(user); !ok {
			tl.loadHistory(ctx, user)
			if _, ok := tl.ready.Load(user); !ok {
				time.Sleep(time.Millisecond * 3)
			} else {
				tl.queryHitCounter.WithLabelValues("db", "UserTimeLineRepo", "loadUserHistory").Add(1)
			}
		} else {
			tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "loadUserHistory").Add(1)
			break
		}
	}
}

func (tl *UserTimeLineRepo) loadHistory(ctx context.Context, user string) {
	_, loaded := tl.loading.LoadOrStore(user, true)
	if loaded {
		return
	}
	defer tl.loading.Delete(user)
	bs := time.Now().UnixNano() / 1000000
	streams, err := tl.persist.SelectUserTimeLineHistory(ctx, user, 100)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed UserTimeLineRepo.SelectUserTimeLineHistory user %s spend:%d ms err %d", user, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms UserTimeLineRepo.SelectUserTimeLineHistory user:%s spend:%d ms", types.DB_EXCEED_TIME, user, spend)
	} else {
		log.Infof("load db succ UserTimeLineRepo.SelectUserTimeLineHistory user:%s spend:%d ms", user, spend)
	}

	length := len(streams)
	for i := 0; i < length/2; i++ {
		ev := streams[i]
		streams[i] = streams[length-1-i]
		streams[length-1-i] = ev
	}

	empty := true
	for _, stream := range streams {
		empty = false
		sev := new(feedstypes.TimeLineEvent)
		ev := stream
		sev.Ev = &ev
		sev.Offset = stream.Offset
		tl.repo.add(user, sev)
		tl.SetUserLatest(user, sev.Offset)
		tl.SetUserRoomLatset(user, sev.Ev.RoomID, sev.Offset, sev.Ev.RoomOffset)
	}

	if empty {
		tl.repo.setDefault(user)
	}

	tl.ready.Store(user, true)
}

func (tl *UserTimeLineRepo) ExistsUserEventUpdate(pos int64, user string) (bool, int64) {
	val, ok := tl.userLatest.Load(user)
	if ok {
		u := val.(int64)
		if u > pos {
			return true, u
		}
	}

	return false, 0
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

func (tl *UserTimeLineRepo) SetUserLastFail(user string, offset int64) {
	tl.lastFail.Store(user, offset)
}

func (tl *UserTimeLineRepo) GetUserLastFail(user string) int64 {
	if val, ok := tl.lastFail.Load(user); ok {
		return val.(int64)
	}
	return int64(-1)
}

func (tl *UserTimeLineRepo) GetUserLatestOffset(ctx context.Context, user string, isHuman bool) int64 {
	if tl.CheckUserLoadingReady(user) == false {
		tl.LoadHistory(ctx, user, isHuman)
	}

	if val, ok := tl.userLatest.Load(user); ok {
		return val.(int64)
	}

	return -1
}

func (tl *UserTimeLineRepo) GetUserRoomLatestOffset(ctx context.Context, user string, isHuman bool) *UserRoomOffset {
	if tl.CheckUserLoadingReady(user) == false {
		tl.LoadHistory(ctx, user, isHuman)
	}
	val, ok := tl.userRoomLatest.Load(user)
	if ok {
		return val.(*UserRoomOffset)
	} else {
		return nil
	}
}

func (tl *UserTimeLineRepo) GetUserLatestReceiptOffset(ctx context.Context, user string, isHuman bool) int64 {
	if tl.CheckUserLoadingReady(user) == false {
		tl.LoadHistory(ctx, user, isHuman)
	}

	val, ok := tl.receiptLatest.Load(user)
	if ok {
		return val.(int64)
	}

	return -1
}

func (tl *UserTimeLineRepo) GetHistory(ctx context.Context, user string) *feedstypes.TimeLines {
	tl.loadUserHistory(ctx, user)
	if !tl.repo.exits(user) && tl.CheckUserLoadingReady(user) {
		tl.userReady.Store(user, false)
		return nil //tl已被兑换出去，等待下次加载
	}

	// tl.queryHitCounter.WithLabelValues("cache", "UserTimeLineRepo", "GetHistory").Add(1)
	return tl.repo.getTimeLine(user)
}

func (tl *UserTimeLineRepo) GetUserRange(ctx context.Context, user string) (int64, int64) {
	timeLine := tl.GetHistory(ctx, user)
	if timeLine != nil {
		return timeLine.GetFeedRange()
	}
	return -1, -1
}

func (tl *UserTimeLineRepo) GetUserMinPos(user string) int64 {
	if val, ok := tl.userMinPos.Load(user); ok {
		return val.(int64)
	}
	return -1
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
