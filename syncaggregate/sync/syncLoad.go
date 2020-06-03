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
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"math"
	"sync"
	"time"
)

type LoadState struct {
	startPos int64
	maxPos   int64
	fullSync bool
}

type UserRoomOffset struct {
	UserId     string
	CreateTime int64
	Offsets    map[string]map[int64]int64 //roomId, roomoffset, offset
}

func (sm *SyncMng) callSyncLoad(ctx context.Context, req *request) {
	loadState := &LoadState{
		fullSync: false,
		maxPos:   int64(-1),
		startPos: int64(math.MaxInt64),
	}
	if req.marks.utlRecv == 0 {
		loadState.fullSync = true
	} else {
		low, up := sm.userTimeLine.GetUserRange(ctx, req.device.UserID)
		log.Infof("SyncMng.callSyncLoad traceid:%s slot:%d user:%s device:%s utl:%d low:%d up:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv, low, up)
		if low > req.marks.utlRecv {
			err := sm.loadSyncLag(req, loadState)
			if err != nil {
				log.Warnf("sm.loadSyncLag traceid:%s slot:%d user:%s device:%s error %v", req.traceId, req.slot, req.device.UserID, req.device.ID, err)
				return
			}
		} else {
			if up > req.marks.utlRecv {
				sm.loadSyncNormal(ctx, req, loadState)
			} else {
				//no update
				req.maxEvOffset = req.marks.utlRecv
			}
		}
		joinRooms, err := sm.userTimeLine.GetJoinRooms(ctx, req.device.UserID)
		if err != nil {
			req.remoteReady = false
			req.remoteFinished = true
			return
		}
		joinRooms.Range(func(key, value interface{}) bool {
			req.joinRooms = append(req.joinRooms, key.(string))
			return true
		})
	}

	if loadState.fullSync {
		sm.loadFullSync(ctx, req, loadState)
	}
	req.isFullSync = loadState.fullSync
	sm.buildLoadRequest(req, loadState)
}

func (sm *SyncMng) processSyncLoad(ctx context.Context, req *request) {
	user := req.device.UserID
	sm.userTimeLine.LoadHistory(ctx, user, req.device.IsHuman)
	if sm.userTimeLine.CheckUserLoadingReady(req.device.UserID) {
		if req.marks.utlRecv == 0 && req.device.IsHuman == false {
			req.ready = true
			req.remoteReady = true
			req.remoteFinished = true
			log.Infof("SyncMng processRequest return not load traceid:%s slot:%d user:%s device:%s", req.traceId, req.slot, user, req.device.ID)
			return
		}
		go sm.callSyncLoad(ctx, req)
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

func (sm *SyncMng) loadSyncLag(req *request, loadState *LoadState) (err error) {
	minPos := sm.userTimeLine.GetUserMinPos(req.device.UserID)
	log.Infof("SyncMng.callSyncLoad traceid:%s slot:%d user:%s device:%s utl:%d minPos:%d low>req.marks.utlRecv", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv, minPos)
	if minPos > req.marks.utlRecv && req.marks.utlRecv > 1 {
		loadState.fullSync = true
		return nil
	} else {
		return sm.loadTimelineFromDb(req, loadState)
	}
}

func (sm *SyncMng) updateOffsetPair(req *request, roomId string, roomOffset, offset int64) {
	if val, ok := sm.syncOffset.Load(req.traceId); ok {
		userRoomOffset := val.(*UserRoomOffset)
		if _, ok := userRoomOffset.Offsets[roomId]; ok {
			userRoomOffset.Offsets[roomId][roomOffset] = offset
		} else {
			userRoomOffset.Offsets[roomId] = make(map[int64]int64)
			userRoomOffset.Offsets[roomId][roomOffset] = offset
		}
	} else {
		userRoomOffset := &UserRoomOffset{
			UserId:     req.device.UserID,
			CreateTime: time.Now().Unix(),
			Offsets:    make(map[string]map[int64]int64),
		}
		userRoomOffset.Offsets[roomId] = make(map[int64]int64)
		userRoomOffset.Offsets[roomId][roomOffset] = offset
		sm.syncOffset.Store(req.traceId, userRoomOffset)
	}
}

func (sm *SyncMng) loadTimelineFromDb(req *request, loadState *LoadState) (err error) {
	bs := time.Now().UnixNano() / 1000000
	streams, err := sm.db.SelectUserTimeLineEvents(context.TODO(), req.device.UserID, req.marks.utlRecv, 1000)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed SyncMng SelectUserTimeLineEvents traceid:%s slot:%d user:%s device:%s spend:%d ms error %v", req.traceId, req.slot, req.device.UserID, req.device.ID, spend, err)
		req.remoteReady = false
		req.remoteFinished = true
		return err
	} else {
		if spend > types.DB_EXCEED_TIME {
			log.Warnf("load db exceed %d ms SyncMng SelectUserTimeLineEvents traceid:%s slot:%d user:%s dev:%s spend:%d ms", types.DB_EXCEED_TIME, req.traceId, req.slot,
				req.device.UserID, req.device.ID, spend)
		} else {
			log.Infof("load db succ SyncMng SelectUserTimeLineEvents traceid:%s slot:%d user:%s dev:%s spend:%d ms", req.traceId, req.slot,
				req.device.UserID, req.device.ID, spend)
		}
	}
	for _, stream := range streams {
		req.pushReqRoom(&stream, true)
		if stream.Offset > loadState.maxPos {
			loadState.maxPos = stream.Offset
		}
		if stream.Offset < loadState.startPos {
			loadState.startPos = stream.Offset
		}
		sm.updateOffsetPair(req, stream.RoomID, stream.RoomOffset, stream.Offset)
	}
	if loadState.maxPos == -1 {
		loadState.maxPos = req.marks.utlRecv
	}
	req.maxEvOffset = loadState.maxPos
	return nil
}

func (sm *SyncMng) loadSyncNormal(ctx context.Context, req *request, loadState *LoadState) {
	timeLine := sm.userTimeLine.GetHistory(ctx, req.device.UserID)
	newMsgRoom := make(map[string]bool)
	if timeLine != nil {
		feeds, start, end, low, up := timeLine.GetAllFeedsReverse()
		log.Infof("SyncMng.userTimeLine.GetRange traceid:%s slot:%d user:%s device:%s start:%d end:%d low:%d up:%d up>req.marks.utlRecv feeds:%+v", req.traceId, req.slot, req.device.UserID, req.device.ID, start, end, low, up, feeds)
		for _, feed := range feeds {
			if feed != nil {
				stream := feed.(*feedstypes.TimeLineEvent)
				if stream.GetOffset() <= req.marks.utlRecv {
					break
				}
				newMsgRoom[stream.Ev.RoomID] = true
				req.pushReqRoom(stream.Ev, false)
				if stream.Offset > loadState.maxPos {
					loadState.maxPos = stream.Offset
				}
				if stream.Offset < loadState.startPos {
					loadState.startPos = stream.Offset
				}
				sm.updateOffsetPair(req, stream.GetEv().RoomID, stream.GetEv().RoomOffset, stream.GetEv().Offset)
			} else {
				log.Errorf("SyncMng.callSyncLoad get feeds nil traceid:%s slot:%d user:%s device:%s now:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv)
				loadState.fullSync = true
				break
			}
		}
	} else {
		log.Errorf("SyncMng.callSyncLoad userTimeLine is nil traceid:%s slot:%d user:%s device:%s now:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv)
		loadState.fullSync = true
	}
	if loadState.maxPos == -1 {
		loadState.maxPos = req.marks.utlRecv
	}
	req.maxEvOffset = loadState.maxPos
	log.Infof("SyncMng.userTimeLine.loadnormal traceid:%s slot:%d user:%s device:%s maxEvOffset:%d utlRecv:%d startPos:%d maxPos:%d newMsgRoom:%+v", req.traceId, req.slot, req.device.UserID, req.device.ID, req.maxEvOffset, req.marks.utlRecv, loadState.startPos, loadState.maxPos, newMsgRoom)

}

func (sm *SyncMng) getRoomLatest(offsetPair *repos.UserRoomOffset) int64 {
	if offsetPair == nil {
		return -1
	} else {
		return offsetPair.GetRoomOffset()
	}
}

func (sm *SyncMng) loadFullSync(ctx context.Context, req *request, loadState *LoadState) {
	loadState.maxPos = sm.userTimeLine.GetUserLatestOffset(ctx, req.device.UserID, req.device.IsHuman)
	if loadState.maxPos == -1 {
		loadState.maxPos = req.marks.utlRecv
	}
	offsetPair := sm.userTimeLine.GetUserRoomLatestOffset(ctx, req.device.UserID, req.device.IsHuman)
	if offsetPair != nil {
		sm.updateOffsetPair(req, offsetPair.GetRoomId(), offsetPair.GetRoomOffset(), offsetPair.GetUserOffset())
	}
	req.maxEvOffset = loadState.maxPos
	if req.device.IsHuman {
		joinRooms, err := sm.userTimeLine.GetJoinRooms(ctx, req.device.UserID)
		if err != nil {
			req.remoteReady = false
			req.remoteFinished = true
			return
		}
		joinRooms.Range(func(key, value interface{}) bool {
			room := &syncapitypes.SyncRoom{
				RoomID:    key.(string),
				RoomState: "join",
				Start:     -1,
				End:       sm.getRoomLatest(offsetPair),
			}
			req.reqRooms.Store(key.(string), room)
			req.joinRooms = append(req.joinRooms, key.(string))
			return true
		})

		inviteRooms, err := sm.userTimeLine.GetInviteRooms(ctx, req.device.UserID)
		if err != nil {
			req.remoteReady = false
			req.remoteFinished = true
			return
		}
		inviteRooms.Range(func(key, value interface{}) bool {
			room := &syncapitypes.SyncRoom{
				RoomID:    key.(string),
				RoomState: "invite",
				Start:     -1,
				End:       sm.getRoomLatest(offsetPair),
			}
			req.reqRooms.Store(key.(string), room)
			return true
		})
	}
}

func (sm *SyncMng) buildLoadRequest(req *request, loadState *LoadState) {
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
	log.Infof("SyncMng.callSyncLoad remote load request start traceid:%s slot:%d user:%s device:%s utl:%d startPos:%d maxPos:%d fullstate:%t joins:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv, loadState.startPos, loadState.maxPos, loadState.fullSync, len(req.joinRooms))
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
				//log.Infof("SyncMng.callSyncLoad load traceid:%s slot:%d user %s device %s request %s",req.traceId,req.slot, req.device.UserID, req.device.ID, string(bytes))
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
						log.Infof("SyncMng.callSyncLoad traceid:%s slot:%d spend:%d ms user %s device %s instance %d response false", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance)
						syncReq.LoadReady = false
						sm.userTimeLine.SetUserLastFail(req.device.UserID, req.marks.utlRecv)
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
	es := time.Now().UnixNano() / 1000000
	log.Infof("SyncMng.callSyncLoad remote load request end traceid:%s slot:%d user:%s device:%s spend:%d ms", req.traceId, req.slot, req.device.UserID, req.device.ID, es-bs)
	loaded := true
	req.remoteReady = true
	req.remoteFinished = true
	for _, syncReq := range requestMap {
		if syncReq.LoadReady == false {
			loaded = false
		}
	}
	if loaded == false {
		req.remoteReady = false
	}
}
