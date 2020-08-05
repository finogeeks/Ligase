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
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type offsetMarks struct {
	utlRecv  int64
	accRecv  int64
	recpRecv int64
	preRecv  int64
	kcRecv   int64
	stdRecv  int64

	utlProcess  int64
	accProcess  int64
	recpProcess int64
	preProcess  int64
	kcProcess   int64
	stdProcess  int64
}

func (oms *offsetMarks) reset(val int64) {
	oms.utlRecv = val
	oms.accRecv = val
	oms.recpRecv = val
	oms.preRecv = val
	oms.kcRecv = val
	oms.stdRecv = val

	oms.utlProcess = val
	oms.accProcess = val
	oms.recpProcess = val
	oms.preProcess = val
	oms.kcProcess = val
	oms.stdProcess = val
}

func (oms *offsetMarks) init(input string, sm *SyncMng) {
	offsets := strings.Split(input, "_")
	oms.reset(0)

	if len(offsets) == 1 {
		if input != "" {
			i, err := strconv.Atoi(input)
			if err == nil {
				oms.reset(int64(i))
			} else {
				oms.reset(0)
				log.Errorf("SyncMng offsetMarks init fail to convert since:%s setToLatest:%d", input, oms.utlRecv)
			}
		}
	} else {
		for _, val := range offsets {
			items := strings.Split(val, ":")
			switch items[0] {
			case "utl":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					oms.utlRecv = int64(i)
					oms.utlProcess = oms.utlRecv
				} else {
					oms.utlRecv = 0
					oms.utlProcess = oms.utlRecv
				}
			case "acc":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					oms.accRecv = int64(i)
					oms.accProcess = oms.accRecv
				} else {
					oms.accRecv = 0
					oms.accProcess = oms.accRecv
				}
			case "rec":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					oms.recpRecv = int64(i)
					oms.recpProcess = oms.recpRecv
				} else {
					oms.recpRecv = 0
					oms.recpProcess = oms.recpRecv
				}
			case "pre":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					oms.preRecv = int64(i)
					oms.preProcess = oms.preRecv
				} else {
					oms.preRecv = 0
					oms.preProcess = oms.preRecv
				}
			case "kc":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					oms.kcRecv = int64(i)
					oms.kcProcess = oms.kcRecv
				} else {
					oms.kcRecv = 0
					oms.kcProcess = oms.kcRecv
				}
			case "std":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					oms.stdRecv = int64(i)
					oms.stdProcess = oms.stdRecv
				} else {
					oms.stdRecv = 0
					oms.stdProcess = oms.stdRecv
				}
			}
		}
	}
}

func (oms *offsetMarks) build() string {
	return fmt.Sprintf("utl:%d_acc:%d_rec:%d_pre:%d_kc:%d_std:%d", oms.utlProcess, oms.accProcess, oms.recpProcess, oms.preProcess, oms.kcProcess, oms.stdProcess)
}

type request struct {
	ctx     context.Context
	device  *authtypes.Device
	limit   int
	timeout int64
	//since     int64
	fullState bool
	filter    *gomatrix.Filter
	presence  string
	token     string
	marks     *offsetMarks

	latest         int64
	lastPos        int64
	ready          bool
	remoteReady    bool
	remoteFinished bool
	maxEvOffset    int64
	isFullSync     bool
	reqRooms       sync.Map
	joinRooms      []string
	traceId        string
	slot           uint32
	hasNewEvent    bool
	MaxRoomOffset  map[string]int64
	offsets        map[string]int64
}

func (req *request) checkNoWait() bool {
	if (req.marks.utlRecv == 0 && req.lastPos != -1) || req.timeout == 0 || req.fullState == true {
		return true
	}
	return false
}

func (req *request) pushReqRoom(stream *syncapitypes.UserTimeLineStream, overwriteState bool) {
	var room *syncapitypes.SyncRoom
	if reqRoom, ok := req.reqRooms.Load(stream.RoomID); ok {
		room = reqRoom.(*syncapitypes.SyncRoom)
	} else {
		room = &syncapitypes.SyncRoom{
			RoomID:    stream.RoomID,
			Start:     stream.RoomOffset,
			RoomState: stream.RoomState,
		}
		req.reqRooms.Store(stream.RoomID, room)
	}
	log.Debugf("----------push room state", room.RoomID, room.RoomState, stream.RoomState)
	if overwriteState {
		room.RoomState = stream.RoomState
	}
	if room.Start > stream.RoomOffset {
		room.Start = stream.RoomOffset
	}
	if room.End < stream.RoomOffset {
		room.End = stream.RoomOffset
	}
}

func (sm *SyncMng) buildFilter(req *types.HttpReq, userID string) *gomatrix.Filter {
	str := req.Filter
	if str != "" {
		if strings.HasPrefix(str, "{") {
			filter := new(gomatrix.Filter)
			if err := json.Unmarshal([]byte(str), filter); err != nil {
				log.Errorf("SyncMng buildRequest traceid:%s user:%s fail to unmarshal fiter, str:%s err:%v", req.TraceId, userID, str, err)
				return nil
			} else {
				return filter
			}
		} else {
			cacheFilter, ok := sm.cache.GetAccountFilterById(userID, str)
			if !ok || cacheFilter == "" {
				return nil
			}
			filter := new(gomatrix.Filter)
			err := json.Unmarshal([]byte(cacheFilter), filter)
			if err == nil {
				if strings.Contains(cacheFilter, "include_leave") == false {
					filter.Room.IncludeLeave = true
				}
				return filter
			} else {
				return nil
			}
		}
	}
	return nil
}

func (sm *SyncMng) buildRequest(
	req *types.HttpReq,
	device *authtypes.Device,
	now int64,
	lastPos int64,
) *request {
	res := new(request)

	res.ctx = context.TODO()
	res.device = device

	//build timeout
	str := req.TimeOut
	if str == "" {
		res.timeout = 30000
	}

	val, err := strconv.Atoi(str)
	if err == nil {
		res.timeout = int64(val)
	} else {
		res.timeout = 30000
	}

	//build full_state
	str = req.FullState
	if str != "" && str != "false" {
		res.fullState = true
	} else {
		res.fullState = false
	}

	//build presence
	res.presence = req.SetPresence

	//build since
	str = req.From
	if str == "" {
		str = req.Since
	}
	res.marks = new(offsetMarks)
	res.token = str
	res.marks.init(str, sm)

	//build limit
	res.limit = 20
	if res.marks.utlRecv == 0 {
		res.limit = 10
	}

	res.filter = sm.buildFilter(req, device.UserID)

	if res.filter != nil {
		if res.marks.utlRecv == 0 {
			if res.filter.Room.State.Limit != nil {
				if *res.filter.Room.State.Limit > 0 {
					res.limit = *res.filter.Room.State.Limit
				}
			}
		} else {
			if res.filter.Room.Timeline.Limit != nil {
				if *res.filter.Room.Timeline.Limit > 0 {
					res.limit = *res.filter.Room.Timeline.Limit
				}
			}
		}
	}

	//build latest
	if res.timeout == 0 {
		res.latest = now + 30000 //30秒延
	} else {
		res.latest = now + res.timeout
		if res.timeout < 3000 {
			res.latest = now + 3000
		}
	}

	//build initialRequest
	res.lastPos = lastPos

	res.ready = false
	res.remoteReady = false
	res.remoteFinished = false
	res.traceId = req.TraceId
	res.hasNewEvent = false
	res.MaxRoomOffset = make(map[string]int64)
	res.offsets = make(map[string]int64)
	log.Infof("SyncMng buildRequest traceid:%s now:%d diff:%d user:%s device:%s utlRecv:%d request:%v", res.traceId, now, res.latest-now, device.UserID, device.ID, res.marks.utlRecv, res)
	return res
}

func (sm *SyncMng) isFullSync(req *request) bool {
	return req.isFullSync || req.marks.utlRecv == 0
}

func (sm *SyncMng) OnSyncRequest(
	ctx context.Context,
	req *types.HttpReq,
	device *authtypes.Device,
) (int, *syncapitypes.Response) {
	log.Infof("SyncMng request start traceid:%s user:%s dev:%s", req.TraceId, device.UserID, device.ID)
	start := time.Now().UnixNano() / 1000000
	lastPos := sm.onlineRepo.GetLastPos(device.UserID, device.ID)
	request := sm.buildRequest(req, device, start, lastPos)
	sm.onlineRepo.Pet(device.UserID, device.ID, request.marks.utlRecv, request.timeout)
	sm.userDeviceActiveRepo.UpdateDevActiveTs(device.UserID, device.ID)
	sm.dispatch(ctx, device.UserID, request)

	for request.ready == false || request.remoteReady == false {
		time.Sleep(time.Millisecond * 100)

		now := time.Now().UnixNano() / 1000000
		if now > request.latest || (request.remoteFinished == true && request.remoteReady == false && now-start > 1000) {
			res := syncapitypes.NewResponse(0)
			res.NextBatch = request.token

			if sm.isFullSync(request) && request.device.IsHuman == true {
				log.Errorf("SyncMng request not ready failed traceid:%s user:%s dev:%s latest:%d spend:%d ms errcode:%d", request.traceId, request.device.UserID, request.device.ID, request.latest, now-start, http.StatusServiceUnavailable)
				return http.StatusServiceUnavailable, res
			} else {
				log.Infof("SyncMng request not ready succ traceid:%s user:%s dev:%s latest:%d spend:%d ms", request.traceId, request.device.UserID, request.device.ID, request.latest, now-start)
				return http.StatusOK, res
			}
		}
	}

	//wait process ready
	for request.checkNoWait() == false {
		time.Sleep(time.Millisecond * 200)

		now := time.Now().UnixNano() / 1000000
		if now > request.latest { //超时
			log.Infof("SyncMng request ready timeout, break wait traceid:%s user:%s dev:%s now:%d latest:%d", request.traceId, request.device.UserID, request.device.ID, now, request.latest)
			break
		}
		hasEventUpdate, curUtl := sm.userTimeLine.ExistsUserEventUpdate(request.marks.utlRecv, device.UserID, device.ID, req.TraceId)
		if curUtl != request.marks.utlRecv {
			log.Warnf("SyncMng ExistsUserEventUpdate update oldUtl:%d to newUtl:%d", request.marks.utlRecv, curUtl)
			request.marks.utlRecv = curUtl
		}
		if hasEventUpdate && now-start > 500 {
			log.Infof("SyncMng break has user event traceid:%s user:%s dev:%s now:%d latest:%d utlRecv:%d", request.traceId, request.device.UserID, request.device.ID, now, start, request.marks.utlRecv)
			break
		}

		if sm.clientDataStreamRepo.ExistsAccountDataUpdate(ctx, request.marks.accRecv, device.UserID) && now-start > 500 && request.device.IsHuman == true {
			log.Infof("SyncMng break has account data traceid:%s user:%s dev:%s now:%d latest:%d", request.traceId, request.device.UserID, request.device.ID, now, start)
			break
		}

		if sm.typingConsumer.ExistsTyping(device.UserID, device.ID, sm.userTimeLine.GetUserCurRoom(device.UserID, device.ID)) && now-start > 500 && request.device.IsHuman == true {
			log.Infof("SyncMng break has typing traceid:%s user:%s dev:%s now:%d latest:%d ", request.traceId, request.device.UserID, request.device.ID, now, start)
			break
		}

		hasReceiptUpdate, lastReceipt := sm.userTimeLine.ExistsUserReceiptUpdate(request.marks.recpRecv, device.UserID)
		if hasReceiptUpdate && now-start > sm.cfg.CheckReceipt && request.device.IsHuman == true {
			log.Infof("SyncMng break has receipt traceid:%s user:%s dev:%s now:%d latest:%d recpRecv:%d lastReceipt:%d", request.traceId, request.device.UserID, request.device.ID, now, start, request.marks.recpRecv, lastReceipt)
			break
		}

		if sm.cfg.UseEncrypt {
			if common.IsActualDevice(device.DeviceType) && sm.keyChangeRepo.ExistsKeyChange(request.marks.kcRecv, device.UserID) && now-start > 500 && request.device.IsHuman == true {
				log.Infof("SyncMng break has key change traceid:%s user:%s dev:%s now:%d latest:%d ", request.traceId, request.device.UserID, request.device.ID, now, start)
				break
			}
		}

		if common.IsActualDevice(device.DeviceType) && sm.stdEventStreamRepo.ExistsSTDEventUpdate(ctx, request.marks.stdRecv, device.UserID, device.ID) && now-start > 500 && request.device.IsHuman == true {
			log.Infof("SyncMng break has send to device messages traceid:%s user:%s dev:%s now:%d latest:%d ", request.traceId, request.device.UserID, request.device.ID, now, start)
			break
		}

		if sm.cfg.SendMemberEvent == false {
			if sm.presenceStreamRepo.ExistsPresence(request.device.UserID, request.marks.preRecv) && now-start > 500 && request.device.IsHuman == true {
				log.Infof("SyncMng break has presence messages traceid:%s user:%s dev:%s now:%d latest:%d ", request.traceId, request.device.UserID, request.device.ID, now, start)
				break
			}
		}
	}

	var res *syncapitypes.Response
	if request.ready == false || request.remoteReady == false {
		res = syncapitypes.NewResponse(0)
		res.NextBatch = request.token
		now := time.Now().UnixNano() / 1000000
		if sm.isFullSync(request) && request.device.IsHuman == true {
			log.Errorf("SyncMng OnSyncRequest still not ready failed traceid:%s user:%s dev:%s last:%d spend:%d ms errcode:%d", request.traceId, request.device.UserID, device.ID, lastPos, now-start, http.StatusServiceUnavailable)
			return http.StatusServiceUnavailable, res
		} else {
			log.Infof("SyncMng OnSyncRequest still not ready succ traceid:%s user:%s dev:%s last:%d spend:%d ms", request.traceId, request.device.UserID, device.ID, lastPos, now-start)
			return http.StatusOK, res
		}
	} else {
		res = syncapitypes.NewResponse(0)
		bs := time.Now().UnixNano()/1000000
		ok := sm.buildSyncData(ctx, request, res)
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("traceid:%s buildSyncData spend:%d",request.traceId, spend)
		if !ok {
			res = syncapitypes.NewResponse(0)
			res.NextBatch = request.token
			now := time.Now().UnixNano() / 1000000
			if sm.isFullSync(request) && request.device.IsHuman == true {
				log.Errorf("SyncMng OnSyncRequest failed ready traceid:%s user:%s dev:%s spend:%d ms errcode:%d", request.traceId, request.device.UserID, request.device.ID, now-start, http.StatusServiceUnavailable)
				return http.StatusServiceUnavailable, res
			} else {
				log.Warnf("SyncMng OnSyncRequest succ ready traceid:%s user:%s dev:%s spend:%d ms", request.traceId, request.device.UserID, request.device.ID, now-start)
				return http.StatusOK, res
			}
		}

		if request.device.IsHuman == true {
			if sm.cfg.SendMemberEvent == false {
				sm.addPresence(ctx, request, res)
			}

			res = sm.addAccountData(ctx, request, res)

			if sm.cfg.UseEncrypt {
				if common.IsActualDevice(device.DeviceType) {
					sm.addKeyChangeInfo(ctx, request, res)
					sm.addSendToDevice(ctx, request, res)
					sm.addOneTimeKeyCountInfo(ctx, request, res)
				} else {
					res.SignNum = common.DefaultKeyCount()
				}
			} else {
				if common.IsActualDevice(device.DeviceType) {
					sm.addSendToDevice(ctx, request, res)
				}
				res.SignNum = common.DefaultKeyCount()
			}

			sm.addTyping(ctx, request, res, sm.userTimeLine.GetUserCurRoom(device.UserID, device.ID))
		}

		if sm.cfg.UseMessageFilter {
			res = sm.filterSyncData(request, res)
		}

		res.NextBatch = request.marks.build()
	}
	sm.FillSortEventOffset(res, request)
	now := time.Now().UnixNano() / 1000000
	if sm.isFullSync(request) {
		bytes, _ := json.Marshal(res.Presence)
		log.Infof("SyncMng full sync traceid:%s user:%s dev:%s presence:%s", request.traceId, request.device.UserID, request.device.ID, string(bytes))
		bytes, _ = json.Marshal(res.AccountData)
		log.Infof("SyncMng full sync traceid:%s user:%s dev:%s account data:%s", request.traceId, request.device.UserID, request.device.ID, string(bytes))
		spend := now - start
		if spend > types.CHECK_LOAD_EXCEED_TIME {
			log.Warnf("SyncMng full sync exceed %d ms traceid:%s user:%s dev:%s spend:%d ms", types.CHECK_LOAD_EXCEED_TIME, request.traceId, request.device.UserID, request.device.ID, spend)
		} else {
			log.Infof("SyncMng full sync succ traceid:%s user:%s dev:%s spend:%d ms", request.traceId, request.device.UserID, request.device.ID, spend)
		}

	} else {
		bytes, _ := json.Marshal(res)
		log.Infof("SyncMng Increment sync response succ traceid:%s user:%s dev:%s spend:%d ms events:%s", request.traceId, request.device.UserID, request.device.ID, now-start, string(bytes))
	}

	return http.StatusOK, res
}
