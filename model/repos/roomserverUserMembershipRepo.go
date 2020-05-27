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

	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

type RoomServerUserMembershipRepo struct {
	persist         model.RoomServerDatabase
	cache           service.Cache
	queryHitCounter mon.LabeledCounter
}

func NewRoomServerUserMembershipRepo(persist model.RoomServerDatabase, cache service.Cache, queryHitCounter mon.LabeledCounter) *RoomServerUserMembershipRepo {
	return &RoomServerUserMembershipRepo{
		persist:         persist,
		cache:           cache,
		queryHitCounter: queryHitCounter,
	}
}

func (repo *RoomServerUserMembershipRepo) getUserMemberShip(ctx context.Context, userID string) (map[string]int64, error) {
	ums, err := repo.getUserMemberShipFromCache(userID)
	if err != nil || ums == nil {
		if err != nil {
			log.Errorf("getUserMemberShipFromCache miss userID:%s,err:%v", userID, err)
		}
		return repo.getUserMemberShipFromDB(ctx, userID)
	}
	repo.queryHitCounter.WithLabelValues("cache", "RoomServerUserMembershipRepo", "GetUserMemberShip").Add(1)
	return ums, err
}

func (repo *RoomServerUserMembershipRepo) GetJoinMemberShip(ctx context.Context, userID string) (ms []string, err error) {
	ums, err := repo.getUserMemberShip(ctx, userID)
	if err != nil {
		return nil, err
	}
	for k, v := range ums {
		if v == int64(roomservertypes.MembershipStateJoin) {
			ms = append(ms, k)
		}
	}
	return ms, nil
}

func (repo *RoomServerUserMembershipRepo) GetInviteMemberShip(ctx context.Context, userID string) (map[string]int64, error) {
	ums, err := repo.getUserMemberShip(ctx, userID)
	if err != nil {
		return nil, err
	}
	rm := make(map[string]int64)
	for k, v := range ums {
		if v == int64(roomservertypes.MembershipStateInvite) {
			rm[k] = v
		}
	}
	return rm, nil
}

func (repo *RoomServerUserMembershipRepo) GetLeaveMemberShip(ctx context.Context, userID string) (map[string]int64, error) {
	ums, err := repo.getUserMemberShip(ctx, userID)
	if err != nil {
		return nil, err
	}
	rm := make(map[string]int64)
	for k, v := range ums {
		if v == int64(roomservertypes.MembershipStateLeaveOrBan) {
			rm[k] = v
		}
	}
	return rm, nil
}

func (repo *RoomServerUserMembershipRepo) getUserMemberShipFromCache(userID string) (map[string]int64, error) {
	repo.queryHitCounter.WithLabelValues("cache", "RoomServerUserMembershipRepo", "GetUserMemberShip").Add(1)
	return repo.cache.GetUserRoomMemberShip(userID)
}

func (repo *RoomServerUserMembershipRepo) getUserMemberShipFromDB(ctx context.Context, userID string) (map[string]int64, error) {
	repo.queryHitCounter.WithLabelValues("db", "RoomServerUserMembershipRepo", "GetUserMemberShip").Add(1)
	join, invite, leave, err := repo.persist.GetUserRooms(ctx, userID)
	if err != nil {
		log.Errorf("getUserMemberShipFromDB userID:%s,err:%v", userID, err)
		return nil, err
	}
	ums := make(map[string]int64)
	for _, id := range join {
		ums[id] = int64(roomservertypes.MembershipStateJoin)
	}
	for _, id := range invite {
		ums[id] = int64(roomservertypes.MembershipStateInvite)
	}
	for _, id := range leave {
		ums[id] = int64(roomservertypes.MembershipStateLeaveOrBan)
	}
	if ums != nil && len(ums) > 0 {
		err = repo.setMemberShipToCache(userID, ums)
		if err != nil {
			log.Warnf("getUserMemberShipFromDB userID:%s set ums from db to cache err:%v", userID, err)
		}
	}
	return ums, nil
}

func (repo *RoomServerUserMembershipRepo) setMemberShipToCache(userID string, ums map[string]int64) error {
	if ums == nil {
		return nil
	}
	return repo.cache.SetUserRoomMemberShipMulti(userID, ums)
}

func (repo *RoomServerUserMembershipRepo) initCacheIsNotExists(ctx context.Context, userID string) error {
	exists, err := repo.cache.CheckUserRoomMemberShipExists(userID)
	if err != nil {
		return err
	}
	if !exists {
		_, err = repo.getUserMemberShipFromDB(ctx, userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (repo *RoomServerUserMembershipRepo) OnEvent(ctx context.Context, ev, pre *gomatrixserverlib.Event) {
	if ev.Type() == "m.room.member" {
		old := ""
		if pre != nil {
			old, _ = pre.Membership()
		}
		new, _ := ev.Membership()
		if old != new {
			err := repo.initCacheIsNotExists(ctx, *ev.StateKey())
			if err != nil {
				log.Warnf("RoomServerUserMembershipRepo OnEvent user:%s initCacheIsNotExists err:%v", *ev.StateKey(), err)
				return
			}
			switch new {
			case "invite":
				err = repo.cache.SetUserRoomMemberShip(ev.RoomID(), *ev.StateKey(), int64(roomservertypes.MembershipStateInvite))
			case "join":
				err = repo.cache.SetUserRoomMemberShip(ev.RoomID(), *ev.StateKey(), int64(roomservertypes.MembershipStateJoin))
			default:
				err = repo.cache.SetUserRoomMemberShip(ev.RoomID(), *ev.StateKey(), int64(roomservertypes.MembershipStateLeaveOrBan))
			}
			if err != nil {
				log.Warn("RoomServerUserMembershipRepo OnEvent set user:%s room:%s membership:%s err:%s", *ev.StateKey(), ev.RoomID(), new, err.Error())
			}
		}
	}
}
