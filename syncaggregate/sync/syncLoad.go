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

package sync

import (
	"sync"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func (sm *SyncMng) buildIncreamSyncRequset(req *request) error {
	log.Debugf("traceid:%s begin buildIncreamSyncRequset", req.traceId)
	joinRooms, err := sm.userTimeLine.GetJoinRoomsMap(req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildIncreamSyncRequset GetJoinRooms err:%v", req.traceId, err)
		return err
	}
	for roomID := range joinRooms {
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "join")
		//joinoffset < 0 , invaild room, old backfill join event is < 0
		joinOffset := sm.userTimeLine.GetJoinMembershipOffset(req.device.UserID, roomID)
		req.joinRooms = append(req.joinRooms, roomID)
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset && joinOffset > 0 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, -1, roomID, "join", "build"))
			}
		} else {
			if joinOffset > 0 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, -1, roomID, "join", "build"))
			}
		}
	}
	inviteRooms, err := sm.userTimeLine.GetInviteRoomsMap(req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildIncreamSyncRequset GetInviteRooms err:%v", req.traceId, err)
		return err
	}
	for roomID := range inviteRooms {
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "invite")
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID, "invite", "build"))
			}
		} else {
			req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, latestOffset, roomID, "invite", "build"))
		}
	}
	leaveRooms, err := sm.userTimeLine.GetLeaveRoomsMap(req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildIncreamSyncRequset GetLeaveRooms err:%v", req.traceId, err)
		return err
	}
	for roomID := range leaveRooms {
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "leave")
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID, "leave", "build"))
			}
		} else {
			//token has not offset leave room can get only the leave room msg
			if latestOffset != -1 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, latestOffset-1, latestOffset, roomID, "leave", "build"))
			}
		}
	}
	return nil
}

func (sm *SyncMng) buildReqRoom(traceId string, start, end int64, roomID, roomState, source string) *syncapitypes.SyncRoom {
	log.Debugf("traceid:%s roomID:%s start:%d end:%d roomState:%s source:%s", traceId, roomID, start, end, roomState, source)
	return &syncapitypes.SyncRoom{
		RoomID:    roomID,
		RoomState: roomState,
		Start:     start,
		End:       end,
	}
}

func (sm *SyncMng) buildFullSyncRequest(req *request) error {
	log.Debugf("traceid:%s begin buildFullSyncRequest", req.traceId)
	req.limit = 10
	if !req.device.IsHuman {
		return nil
	}
	var err error
	req.joinRooms, err = sm.userTimeLine.GetJoinRoomsArr(req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildFullSyncRequest GetJoinRooms err:%v", req.traceId, err)
		return err
	}
	for _, roomID := range req.joinRooms {
		req.joinRooms = append(req.joinRooms, roomID)
		req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, -1, roomID, "join", "build"))
	}
	inviteRooms, err := sm.userTimeLine.GetInviteRoomsMap(req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildFullSyncRequest GetInviteRooms err:%v", req.traceId, err)
		return err
	}
	for roomID := range inviteRooms {
		req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "invite"), roomID, "invite", "build"))
	}
	return nil
}

func (sm *SyncMng) callSyncLoad(req *request) {
	if req.marks.utlRecv == 0 {
		req.isFullSync = true
	} else {
		curUtl, offsets, err := sm.userTimeLine.LoadToken(req.device.UserID, req.device.ID, req.marks.utlRecv)
		if err != nil {
			req.remoteReady = false
			req.remoteFinished = true
			log.Errorf("traceid:%s callSyncLoad.LoadToken err:%v", req.traceId, err)
			return
		}
		if req.marks.utlRecv != curUtl {
			log.Warnf("traceid:%s change old utl:%d to new utl:%d", req.traceId, req.marks.utlRecv, curUtl)
			req.marks.utlRecv = curUtl
		}
		if offsets == nil {
			log.Infof("traceid:%s before full sync load, token info userId:%s device:%s utl:%s", req.traceId, req.device.UserID, req.device.ID, req.marks.utlRecv)
			req.isFullSync = true
		} else {
			//token offsets is too large
			//log.Infof("traceid:%s before incream sync load, token info userId:%s device:%s utl:%s offsets:%+v", req.traceId, req.device.UserID, req.device.ID, req.marks.utlRecv, offsets)
			req.offsets = offsets
			err = sm.buildIncreamSyncRequset(req)
			if err != nil {
				log.Warnf("traceid:%s build incream sync request err:%v", req.traceId, err)
				return
			}
		}
	}

	if req.isFullSync == true {
		err := sm.buildFullSyncRequest(req)
		if err != nil {
			return
		}
	}
	sm.buildLoadRequest(req)
}

func (sm *SyncMng) processSyncLoad(req *request) {
	user := req.device.UserID
	log.Debugf("traceid:%s SyncMng processRequest begin", req.traceId)
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000000 - bs
		if spend > 1000 {
			log.Warnf("trace:%s SyncMng processRequest spend:%d", req.traceId, spend)
		} else {
			log.Debugf("trace:%s SyncMng processRequest spend:%d", req.traceId, spend)
		}
	}(bs)
	sm.userTimeLine.LoadHistory(user, req.device.IsHuman)
	if sm.userTimeLine.CheckUserLoadingReady(req.device.UserID) {
		if req.marks.utlRecv == 0 && req.device.IsHuman == false {
			req.ready = true
			req.remoteReady = true
			req.remoteFinished = true
			req.isFullSync = true
			log.Infof("SyncMng processRequest return not load traceid:%s slot:%d user:%s device:%s", req.traceId, req.slot, user, req.device.ID)
			return
		}
		go sm.callSyncLoad(req)
		if req.device.IsHuman == true {
			start := time.Now().UnixNano()
			sm.clientDataStreamRepo.LoadHistory(user, false)
			sm.stdEventStreamRepo.LoadHistory(user, req.device.ID, false)
			sm.presenceStreamRepo.LoadHistory(user, false)
			sm.keyChangeRepo.LoadHistory(user, false)
			loadStart := time.Now().Unix()
			for {
				loaded := true
				if ok := sm.clientDataStreamRepo.CheckLoadReady(user, false); !ok {
					loaded = false
				}
				if ok := sm.stdEventStreamRepo.CheckLoadReady(user, req.device.ID, false); !ok {
					loaded = false
				}
				if ok := sm.presenceStreamRepo.CheckLoadReady(user, false); !ok {
					loaded = false
				}
				if ok := sm.keyChangeRepo.CheckLoadReady(user, false); !ok {
					loaded = false
				}

				if loaded {
					req.ready = true
					spend := (time.Now().UnixNano() - start) / 1000000
					if spend > types.DB_EXCEED_TIME {
						log.Warnf("SyncMng processRequest load exceed %d ms traceid:%s slot:%d user:%s device:%s spend:%d ms", types.DB_EXCEED_TIME, req.traceId, req.slot, user, req.device.ID, spend)
					} else {
						log.Debugf("SyncMng processRequest load succ traceid:%s slot:%d user:%s device:%s spend:%d ms", req.traceId, req.slot, user, req.device.ID, spend)
					}
					break
				}
				now := time.Now().Unix()
				if now-loadStart > 35 {
					req.ready = false
					log.Errorf("SyncMng processRequest load failed traceid:%s slot:%d user:%s device:%s spend:%d s", req.traceId, req.slot, user, req.device.ID, now-loadStart)
					break
				}
				time.Sleep(time.Millisecond * 50)
			}
		} else {
			req.ready = true
		}
	} else {
		log.Warnf("SyncMng processRequest not load ready traceid:%s slot:%d user:%s device:%s", req.traceId, req.slot, user, req.device.ID)
		req.ready = false
	}
}

func (sm *SyncMng) buildLoadRequest(req *request) {
	requestMap := make(map[uint32]*syncapitypes.SyncServerRequest)
	req.reqRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		reqRoom := value.(*syncapitypes.SyncRoom)
		instance := common.GetSyncInstance(roomID, sm.cfg.MultiInstance.SyncServerTotal)
		var request *syncapitypes.SyncServerRequest
		if data, ok := requestMap[instance]; ok {
			request = data
		} else {
			request = &syncapitypes.SyncServerRequest{}
			requestMap[instance] = request
		}
		switch reqRoom.RoomState {
		case "invite":
			request.InviteRooms = append(request.InviteRooms, *reqRoom)
		case "join":
			request.JoinRooms = append(request.JoinRooms, *reqRoom)
		case "leave":
			request.LeaveRooms = append(request.LeaveRooms, *reqRoom)
		}
		return true
	})
	for _, roomID := range req.joinRooms {
		instance := common.GetSyncInstance(roomID, sm.cfg.MultiInstance.SyncServerTotal)
		var request *syncapitypes.SyncServerRequest
		if data, ok := requestMap[instance]; ok {
			request = data
		} else {
			request = &syncapitypes.SyncServerRequest{}
			requestMap[instance] = request
		}
		request.JoinedRooms = append(request.JoinedRooms, roomID)
	}
	log.Debugf("SyncMng.callSyncLoad remote load request start traceid:%s slot:%d user:%s device:%s utl:%d fullstate:%t joins:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv, req.isFullSync, len(req.joinRooms))
	sm.sendSyncLoadReqAndHandle(req, requestMap)
}

func (sm *SyncMng) sendSyncLoadReqAndHandle(req *request, requestMap map[uint32]*syncapitypes.SyncServerRequest) {
	bs := time.Now().UnixNano() / 1000000
	var wg sync.WaitGroup
	for instance, syncReq := range requestMap {
		wg.Add(1)
		go func(
			instance uint32,
			syncReq *syncapitypes.SyncServerRequest,
			req *request,
		) {
			defer wg.Done()
			syncReq.RequestType = "load"
			syncReq.UserID = req.device.UserID
			syncReq.DeviceID = req.device.ID
			syncReq.IsHuman = req.device.IsHuman
			syncReq.Limit = req.limit
			syncReq.ReceiptOffset = req.marks.recpRecv
			syncReq.SyncInstance = instance
			syncReq.IsFullSync = req.isFullSync
			syncReq.TraceID = req.traceId
			syncReq.Slot = req.slot
			bytes, err := json.Marshal(*syncReq)
			if err == nil {
				timeout := 0
				if req.isFullSync {
					timeout = int(sm.cfg.Sync.FullSyncTimeout)
				} else {
					timeout = int(sm.cfg.Sync.RpcTimeout)
				}
				log.Debugf("SyncMng.callSyncLoad load traceid:%s slot:%d user %s device %s instance:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, instance)
				data, err := sm.rpcClient.Request(types.SyncServerTopicDef, bytes, timeout)

				spend := time.Now().UnixNano()/1000000 - bs
				//only for debug
				if adapter.GetDebugLevel() == adapter.DEBUG_LEVEL_DEBUG {
					delay := utils.GetRandomSleepSecondsForDebug()
					log.Debugf("SyncMng.callSyncLoad random sleep %fs", delay)
					time.Sleep(time.Duration(delay*1000) * time.Millisecond)
				}
				if err == nil {
					var result syncapitypes.SyncServerResponse
					err := json.Unmarshal(data, &result)
					if err != nil {
						log.Errorf("SyncMng.callSyncLoad Unmarshal error traceid:%s slot:%d spend:%d ms user %s device %s instance %d err %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance, err)
						req.remoteReady = false
						syncReq.LoadReady = false
					} else if result.Ready == true {
						log.Debugf("SyncMng.callSyncLoad traceid:%s slot:%d spend:%d ms user %s device %s instance %d response true", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance)
						syncReq.LoadReady = true
					} else {
						log.Warnf("SyncMng.callSyncLoad traceid:%s slot:%d spend:%d ms user %s device %s instance %d response false", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance)
						syncReq.LoadReady = false
					}
				} else {
					log.Errorf("call rpc for syncServer load traceid:%s slot:%d spend:%d ms user %s device %s topic:%s error %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, types.SyncServerTopicDef, err)
					req.remoteReady = false
					syncReq.LoadReady = false
				}
			} else {
				log.Errorf("marshal callSyncLoad content error traceid:%s slot:%d user %s device %s error %v", req.traceId, req.slot, req.device.UserID, req.device.ID, err)
				req.remoteReady = false
				syncReq.LoadReady = false
			}
		}(instance, syncReq, req)
	}
	wg.Wait()
	loaded := true
	req.remoteReady = true
	req.remoteFinished = true
	if req.isFullSync {
		for _, syncReq := range requestMap {
			if syncReq.LoadReady == false {
				loaded = false
				log.Warnf("SyncMng.callSyncLoad traceid:%s instance:%d loaded false", req.traceId, syncReq.SyncInstance)
			}
		}
	} else {
		loaded = len(requestMap) == 0
		for _, syncReq := range requestMap {
			if syncReq.LoadReady {
				loaded = true
			} else {
				log.Warnf("SyncMng.callSyncLoad traceid:%s instance:%d loaded false", req.traceId, syncReq.SyncInstance)
			}
		}
	}
	if loaded == false {
		req.remoteReady = false
	}
	es := time.Now().UnixNano() / 1000000
	log.Infof("SyncLoad.sendSyncLoadReqAndHandle traceid:%s slot:%d user:%s device:%s spend:%d ms remoteReady:%t remoteFinished:%t", req.traceId, req.slot, req.device.UserID, req.device.ID, es-bs, req.remoteReady, req.remoteFinished)
}
