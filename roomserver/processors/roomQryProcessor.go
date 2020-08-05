// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package processors

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

type RoomQryProcessor struct {
	DB      model.RoomServerDatabase
	Repo    *repos.RoomServerCurStateRepo
	UmsRepo *repos.RoomServerUserMembershipRepo
	Cfg     *config.Dendrite
}

func (s *RoomQryProcessor) QueryRoomState(
	ctx context.Context,
	request *roomserverapi.QueryRoomStateRequest,
	response *roomserverapi.QueryRoomStateResponse,
) error {
	log.Debugf("RoomQryProcessor recv QueryRoomState room:%s", request.RoomID)
	response.RoomID = request.RoomID

	rs := s.Repo.GetRoomState(ctx, request.RoomID)
	if rs == nil {
		log.Debugf("RoomQryProcessor recv QueryRoomState can't find room:%s", request.RoomID)
		response.RoomExists = false
		return errors.New("room not exits")
	}

	log.Debugf("RoomQryProcessor recv QueryRoomState find room:%s", request.RoomID)
	response.RoomExists = true
	response.Creator = rs.Creator
	response.JoinRule = rs.JoinRule
	response.HistoryVisibility = rs.HistoryVisibility
	response.Visibility = rs.Visibility
	response.JoinRule = rs.JoinRule
	response.Name = rs.Name
	response.Topic = rs.Topic
	response.CanonicalAlias = rs.CanonicalAlias
	response.Power = rs.Power
	response.Alias = rs.Alias
	response.Avatar = rs.Avatar
	response.GuestAccess = rs.GuestAccess

	response.Join = make(map[string]*gomatrixserverlib.Event)
	response.Leave = make(map[string]*gomatrixserverlib.Event)
	response.Invite = make(map[string]*gomatrixserverlib.Event)
	response.ThirdInvite = make(map[string]*gomatrixserverlib.Event)

	rs.GetJoinMap().Range(func(key, value interface{}) bool {
		response.Join[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})
	rs.GetLeaveMap().Range(func(key, value interface{}) bool {
		response.Leave[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})
	rs.GetInviteMap().Range(func(key, value interface{}) bool {
		response.Invite[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})
	rs.GetThirdInviteMap().Range(func(key, value interface{}) bool {
		response.ThirdInvite[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})

	log.Debugf("RoomQryProcessor recv QueryRoomState set room:%s resp:%v", request.RoomID, response)
	return nil
}

func (s *RoomQryProcessor) QueryEventsByID(
	ctx context.Context,
	request *roomserverapi.QueryEventsByIDRequest,
	response *roomserverapi.QueryEventsByIDResponse,
) error {
	response.EventIDs = request.EventIDs

	log.Infof("QueryEventsByID %v", request.EventIDs)
	eventNIDMap, err := s.DB.EventNIDs(ctx, request.EventIDs)
	if err != nil {
		return err
	}

	var eventNIDs []int64
	for _, nid := range eventNIDMap {
		eventNIDs = append(eventNIDs, nid)
	}

	evs, _, err := s.DB.Events(ctx, eventNIDs)
	if err != nil {
		return err
	}
	if len(evs) != len(request.EventIDs) {
		log.Infof("QueryEventsByID ret len %d, reqLen %d", len(evs), len(request.EventIDs))
	}

	response.Events = evs
	return nil
}

func (s *RoomQryProcessor) QueryRoomEventByID(
	ctx context.Context,
	request *roomserverapi.QueryRoomEventByIDRequest,
	response *roomserverapi.QueryRoomEventByIDResponse,
) error {
	response.EventID = request.EventID
	response.RoomID = request.RoomID

	eventNID, err := s.DB.EventNID(ctx, request.EventID)
	if err != nil {
		return err
	}
	log.Infof("############## QueryRoomEventByID, EventNID: %d, eventID: %d, roomID: %d", eventNID, response.EventID, response.RoomID)

	evs, _, err := s.DB.Events(ctx, []int64{eventNID})
	if err != nil {
		return err
	}

	if evs != nil {
		response.Event = evs[0]
		log.Infof("############## QueryRoomEventByID, evs[0]: %#v", evs[0])
	}
	return nil
}

func (s *RoomQryProcessor) QueryJoinRooms(
	ctx context.Context,
	request *roomserverapi.QueryJoinRoomsRequest,
	response *roomserverapi.QueryJoinRoomsResponse,
) error {
	response.UserID = request.UserID
	ums, err := s.UmsRepo.GetJoinMemberShip(ctx, request.UserID)
	if err != nil {
		log.Errorf("QueryJoinRooms userID:%s, err:%v", request.UserID, err)
		return err
	}
	if ums == nil || len(ums) <= 0 {
		log.Errorf("QueryJoinRooms userID:%s ums empty err:%v", request.UserID, err)
		return errors.New(fmt.Sprintf("QueryJoinRooms userID:%s ums empty", request.UserID))
	}
	response.Rooms = append(response.Rooms, ums...)
	return nil
}

func (s *RoomQryProcessor) QueryBackFillEvents( //fed
	ctx context.Context,
	request *roomserverapi.QueryBackFillEventsRequest,
	response *roomserverapi.QueryBackFillEventsResponse,
) error {
	log.Infof("-------RoomQryProcessor QueryBackFillEvents start %#v", request)
	rs := s.Repo.GetRoomState(ctx, request.RoomID)
	if rs == nil {
		return errors.New("room not exits")
	}

	log.Infof("-------RoomQryProcessor QueryBackFillEvents rid:%s rnid:%d dir:%s", request.RoomID, rs.RoomNid, request.Dir)

	domain := request.Domain
	if domain == "" {
		domain = s.Cfg.Matrix.ServerName[0]
	}

	eventID := request.EventID

	var eventNIDs []int64
	var eventNid int64
	var checkFirstEv = false
	if eventID != "" {
		var err error
		eventNid, err = s.DB.EventNID(ctx, eventID)
		if err != nil {
			response.Error = err.Error()
			return err
		}
		if request.EventID == "" {
			// 因为这个eventID是查domain_offsets时自动赋值的，如果要和最后的domain_offset匹配，就要包含此消息，所以+1
			eventNid += 1
		}
	} else {
		if request.Dir == "b" {
			endEventNID, _ := s.DB.SelectEventNidForBackfill(ctx, rs.RoomNid, request.Origin)
			log.Infof("-------RoomQryProcessor QueryBackFillEvents rid:%s rnid:%d endEventNID: %d", request.RoomID, rs.RoomNid, endEventNID)
			if endEventNID == 0 {
				eventNid = math.MaxInt64
			} else {
				checkFirstEv = true
				eventNid = endEventNID
			}
		} else {
			eventNid = math.MinInt64
		}
	}

	// TODO: 验证第一条是否和请求的eventID一致
	var nids []int64
	var err error
	if checkFirstEv {
		nids, err = s.DB.BackFillNids(ctx, rs.RoomNid, domain, eventNid+1, request.Limit, request.Dir)
	} else {
		nids, err = s.DB.BackFillNids(ctx, rs.RoomNid, domain, eventNid, request.Limit, request.Dir)
	}
	log.Infof("-------RoomQryProcessor QueryBackFillEvents rid:%s domain:%s eventnid:%d backnids:%v", request.RoomID, domain, eventNid, nids)
	if err != nil {
		response.Error = err.Error()
		return err
	}
	eventNIDs = append(eventNIDs, nids...)

	log.Infof("-------RoomQryProcessor QueryBackFillEvents rid:%s backnids:%v", request.RoomID, eventNIDs)
	evs, _, err := s.DB.Events(ctx, eventNIDs)
	if err != nil {
		response.Error = err.Error()
		return err
	}

	if request.Dir == "b" && eventID == "" {
		lastNID := int64(0)
		if checkFirstEv {
			lastNID = eventNid
		} else if rs.GetLastMsgID() != "" {
			lastID := rs.GetLastMsgID()
			lastNID, err = s.DB.EventNID(ctx, lastID)
			if err != nil {
				log.Errorf("backfill event is not fully store lastID %s roomID: %s", lastID, request.RoomID)
				response.Error = "backfill event is not fully store"
				return errors.New("backfill event is not fully store")
			}
		}
		if lastNID != 0 {
			log.Infof("backfill event nid last %d", lastNID)
			found := false
			ignoreIdx := -1
			for idx, ev := range evs {
				log.Infof("backfill event nid ast %d", ev.EventNID())
				if ev.EventNID() >= lastNID {
					ignoreIdx = idx
					found = true
					break
				}
			}
			if checkFirstEv && ignoreIdx >= 0 {
				evs = append(evs[:ignoreIdx], evs[ignoreIdx+1:]...)
			}
			if !found {
				log.Errorf("backfill event is not fully store 2 lastNID %d roomID: %s", lastNID, request.RoomID)
				response.Error = "backfill event is not fully store"
				return errors.New("backfill event is not fully store")
			}
		} else {
			log.Errorf("backfill event lastID is empty, roomID: %s", request.RoomID)
		}
	}
	for _, ev := range evs {
		response.PDUs = append(response.PDUs, *ev)
	}

	return nil
}

func (s *RoomQryProcessor) QueryEventAuth(
	ctx context.Context,
	request *roomserverapi.QueryEventAuthRequest,
	response *roomserverapi.QueryEventAuthResponse,
) error {
	log.Infof("QueryEventAuth eventID: %s", request.EventID)
	snapshotNID, err := s.DB.SelectEventStateSnapshotNID(ctx, request.EventID)
	if err != nil {
		log.Errorf("QueryEventAuth select snapshotNID err: %v", err)
		return err
	}
	roomNID, stateBlockNIDs, err := s.DB.SelectState(ctx, snapshotNID)
	if err != nil {
		log.Errorf("QueryEventAuth select state err: %v", err)
		return err
	}

	log.Infof("QueryEventAuth select state snapshot result, roomNID: %d, stateBlockNIDs: %v", roomNID, stateBlockNIDs)

	maxEventNID := int64(math.MinInt64)
	stateEventNIDs := make([]int64, 0, len(stateBlockNIDs))
	for _, v := range stateBlockNIDs {
		if v != 0 {
			stateEventNIDs = append(stateEventNIDs, v)
			if maxEventNID < v {
				maxEventNID = v
			}
		}
	}
	if len(stateEventNIDs) == 0 {
		log.Errorf("QueryEventAuth stateBlockNIDs err %v", stateBlockNIDs)
		return fmt.Errorf("stateBlockNIDs %v", stateBlockNIDs)
	}

	// stateEvent, stateEventNIDs_, err := s.DB.Events(ctx, stateEventNIDs)
	// if err != nil {
	// 	log.Errorf("QueryEventAuth select stateBlock events error", err)
	// 	return err
	// }

	// if len(stateEventNIDs) != len(stateEvent) {
	// 	log.Errorf("QueryEventAuth state block event not found %v %v", stateEventNIDs, stateEventNIDs_)
	// 	return fmt.Errorf("state block event not found")
	// }

	eventNIDs, eventTypes, stateKeys, domains, err := s.DB.SelectRoomStateNIDByStateBlockNID(ctx, roomNID, maxEventNID)
	if err != nil {
		log.Errorf("QueryEventAuth select room state error %v", err)
		return err
	}

	domainMap := map[string]int64{}
	tmpStateEventNIDs := make([]int64, len(stateEventNIDs))
	copy(tmpStateEventNIDs, stateEventNIDs)
	for i := 0; i < len(eventNIDs); i++ {
		for j, v := range tmpStateEventNIDs {
			if eventNIDs[i] == v {
				domainMap[domains[i]] = v
				tmpStateEventNIDs = append(tmpStateEventNIDs[:j], tmpStateEventNIDs[j+1:]...)
				break
			}
		}
		if len(tmpStateEventNIDs) == 0 {
			break
		}
	}

	domainEventMap := map[string]map[string]int64{}
	stateList := []string{}
	for i := 0; i < len(eventNIDs); i++ {
		domain := domains[i]
		maxNID := domainMap[domain]
		if eventNIDs[i] > maxNID {
			continue
		}
		key := eventTypes[i] + stateKeys[i]
		stateList = append(stateList, key)
		tmpMap, ok := domainEventMap[domain]
		if !ok {
			tmpMap = map[string]int64{}
			domainEventMap[domain] = tmpMap
		}
		lastID := tmpMap[key]
		if eventNIDs[i] > lastID {
			tmpMap[key] = eventNIDs[i]
		}
	}
	authEventNIDs := []int64{}
	for _, key := range stateList {
		maxNID := int64(math.MinInt64)
		for _, v := range domainEventMap {
			if maxNID < v[key] {
				maxNID = v[key]
			}
		}
		authEventNIDs = append(authEventNIDs, maxNID)
	}

	authEvents, _, err := s.DB.Events(ctx, authEventNIDs)
	if err != nil {
		log.Errorf("QueryEventAuth select auth event error %v", err)
		return err
	}

	response.AuthEvents = authEvents

	return nil
}

func (s *RoomQryProcessor) QueryEventsByDomainOffset(
	ctx context.Context,
	request *roomserverapi.QueryEventsByDomainOffsetRequest,
	response *roomserverapi.QueryEventsByDomainOffsetResponse,
) error {
	log.Infof("-------RoomQryProcessor QueryEventsByDomainOffset start")
	rs := s.Repo.GetRoomState(ctx, request.RoomID)
	if rs == nil {
		return errors.New("room not exits")
	}

	domainOffset := request.DomainOffset
	if request.UseEventID {
		eventNID, err := s.DB.EventNID(ctx, request.EventID)
		if err != nil {
			return err
		}
		events, _, err := s.DB.Events(ctx, []int64{eventNID})
		if err != nil {
			return err
		}
		if len(events) < 1 {
			response.Error = "event not found " + request.EventID
			return errors.New("event not found " + request.EventID)
		}
		domainOffset = events[0].DomainOffset()
	}

	log.Infof("-------RoomQryProcessor QueryEventsByDomainOffset rid:%s rnid:%d domain:%s domainOffset:%d limit:%d", request.RoomID, rs.RoomNid, request.Domain, domainOffset, request.Limit)

	nids, err := s.DB.SelectRoomEventsByDomainOffset(ctx, rs.RoomNid, request.Domain, domainOffset, request.Limit)
	if err != nil {
		response.Error = err.Error()
		return err
	}

	evs, _, err := s.DB.Events(ctx, nids)
	log.Infof("-------RoomQryProcessor QueryEventsByDomainOffset rid:%s rnid:%d domain:%s nids:%v, event:%d", request.RoomID, rs.RoomNid, request.Domain, nids, len(evs))
	for _, v := range evs {
		response.PDUs = append(response.PDUs, *v)
	}

	return nil
}
