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
	"context"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"sync"
	"time"
)

func (sm *SyncMng) buildIncreamSyncRequset(req *request) error {
	ctx := context.TODO()
	joinRooms, err := sm.userTimeLine.GetJoinRooms(ctx, req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildIncreamSyncRequset GetJoinRooms err:%v", req.traceId, err)
		return err
	}
	joinRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "join")
		//joinoffset < 0 , invaild room, old backfill join event is < 0
		joinOffset := sm.userTimeLine.GetJoinMembershipOffset(req.device.UserID, roomID)
		req.joinRooms = append(req.joinRooms, roomID)
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset && joinOffset > 0 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID,"join","build"))
			}
		}else{
			if joinOffset > 0 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId,-1, latestOffset, roomID,"join","build"))
			}
		}
		return true
	})
	inviteRooms, err := sm.userTimeLine.GetInviteRooms(ctx, req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildIncreamSyncRequset GetInviteRooms err:%v", req.traceId, err)
		return err
	}
	inviteRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "invite")
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID,"invite","build"))
			}
		}else{
			req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, latestOffset, roomID,"invite","build"))
		}
		return true
	})
	leaveRooms, err := sm.userTimeLine.GetLeaveRooms(ctx, req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildIncreamSyncRequset GetLeaveRooms err:%v", req.traceId, err)
		return err
	}
	leaveRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "leave")
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID,"leave","build"))
			}
		}else{
			//token has not offset leave room can get only the leave room msg
			if latestOffset != -1 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, latestOffset - 1, latestOffset, roomID,"leave", "build"))
			}
		}
		return true
	})
	return nil
}

func (sm *SyncMng) buildReqRoom(traceId string, start, end int64, roomID, roomState, source string) *syncapitypes.SyncRoom {
	log.Infof("traceid:%s roomID:%s start:%d end:%d roomState:%s source:%s", traceId, roomID, start, end, roomState, source)
	return &syncapitypes.SyncRoom{
		RoomID:    roomID,
		RoomState: roomState,
		Start:     start,
		End:       end,
	}
}

func (sm *SyncMng) buildFullSyncRequest(req *request) error {
	ctx := context.TODO()
	req.limit = 10
	if !req.device.IsHuman {
		return nil
	}
	joinRooms, err := sm.userTimeLine.GetJoinRooms(ctx, req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildFullSyncRequest GetJoinRooms err:%v", req.traceId, err)
		return err
	}
	joinRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		joinOffset := sm.userTimeLine.GetJoinMembershipOffset(req.device.UserID, roomID)
		req.joinRooms = append(req.joinRooms, key.(string))
		if joinOffset > 0 {
			req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId,-1, sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "join"), roomID,"join", "build"))
		}
		return true
	})
	inviteRooms, err := sm.userTimeLine.GetInviteRooms(ctx, req.device.UserID)
	if err != nil {
		req.remoteReady = false
		req.remoteFinished = true
		log.Errorf("traceid:%s buildFullSyncRequest GetInviteRooms err:%v", req.traceId, err)
		return err
	}
	inviteRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId,-1, sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "invite"), roomID,"invite", "build"))
		return true
	})
	return nil
}

//sync load async for syncserver repare db data
func (sm *SyncMng) callSyncLoad(req *request) {
	if req.marks.utlRecv == 0 {
		req.isFullSync = true
	} else {
		curUtl, offsets,err := sm.userTimeLine.LoadToken(req.device.UserID, req.device.ID, req.marks.utlRecv)
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
		}else{
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

func (sm *SyncMng) processSyncLoad(ctx context.Context, req *request) {
	user := req.device.UserID
	sm.userTimeLine.LoadHistory(ctx, user, req.device.IsHuman)
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
			sm.clientDataStreamRepo.LoadHistory(ctx, user, false)
			sm.stdEventStreamRepo.LoadHistory(ctx, user, req.device.ID, false)
			sm.presenceStreamRepo.LoadHistory(ctx, user, false)
			sm.keyChangeRepo.LoadHistory(ctx, user, false)
			loadStart := time.Now().Unix()
			for {
				loaded := true
				if ok := sm.clientDataStreamRepo.CheckLoadReady(ctx, user, false); !ok {
					loaded = false
				}
				if ok := sm.stdEventStreamRepo.CheckLoadReady(ctx, user, req.device.ID, false); !ok {
					loaded = false
				}
				if ok := sm.presenceStreamRepo.CheckLoadReady(ctx, user, false); !ok {
					loaded = false
				}
				if ok := sm.keyChangeRepo.CheckLoadReady(ctx, user, false); !ok {
					loaded = false
				}

				if loaded {
					req.ready = true
					spend := (time.Now().UnixNano() - start) / 1000000
					if spend > types.CHECK_LOAD_EXCEED_TIME {
						log.Warnf("SyncMng processRequest load exceed %d ms traceid:%s slot:%d user:%s device:%s spend:%d ms", types.DB_EXCEED_TIME, req.traceId, req.slot, user, req.device.ID, spend)
					} else {
						log.Infof("SyncMng processRequest load succ traceid:%s slot:%d user:%s device:%s spend:%d ms", req.traceId, req.slot, user, req.device.ID, spend)
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
		log.Infof("SyncMng processRequest not load ready traceid:%s slot:%d user:%s device:%s", req.traceId, req.slot, user, req.device.ID)
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
	log.Infof("SyncMng.callSyncLoad remote load request start traceid:%s slot:%d user:%s device:%s utl:%d fullstate:%t joins:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv, req.isFullSync, len(req.joinRooms))
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
				log.Infof("SyncMng.callSyncLoad load traceid:%s slot:%d user %s device %s instance:%d",req.traceId,req.slot, req.device.UserID, req.device.ID, instance)
				data, err := sm.rpcClient.Request(types.SyncServerTopicDef, bytes, 35000)

				spend := time.Now().UnixNano()/1000000 - bs

				if err == nil {
					var result syncapitypes.SyncServerResponse
					err := json.Unmarshal(data, &result)
					if err != nil {
						log.Errorf("SyncMng.callSyncLoad Unmarshal error traceid:%s slot:%d spend:%d ms user %s device %s instance %d err %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance, err)
						req.remoteReady = false
						syncReq.LoadReady = false
					} else if result.Ready == true {
						log.Infof("SyncMng.callSyncLoad traceid:%s slot:%d spend:%d ms user %s device %s instance %d response true", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance)
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
	for _, syncReq := range requestMap {
		if syncReq.LoadReady == false {
			loaded = false
			log.Warnf("SyncMng.callSyncLoad traceid:%s instance:%d loaded false" , req.traceId, syncReq.SyncInstance)
		}
	}
	if loaded == false {
		req.remoteReady = false
	}
	es := time.Now().UnixNano() / 1000000
	log.Infof("SyncMng.callSyncLoad remote load request end traceid:%s slot:%d user:%s device:%s spend:%d ms remoteReady:%t remoteFinished:%t len(requestMap):%d", req.traceId, req.slot, req.device.UserID, req.device.ID, es-bs, req.remoteReady, req.remoteFinished, len(requestMap))
}