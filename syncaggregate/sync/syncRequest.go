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
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/log"
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
	ctx       context.Context
	device    *authtypes.Device
	limit     int
	start     int64
	timeout   int64
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
	if (req.marks.utlRecv == 0 && req.lastPos != -1) || req.timeout == 0 || req.fullState {
		return true
	}
	return false
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
				if !strings.Contains(cacheFilter, "include_leave") {
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

	res.ctx = req.Ctx
	res.device = device

	//build timeout
	str := req.TimeOut
	if str == "" {
		res.timeout = 30000
	}

	res.start = now
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
	httpReq *types.HttpReq,
	device *authtypes.Device,
) (int, *syncapitypes.Response) {
	start := time.Now().UnixNano() / 1000000
	log.Debugf("SyncMng request start traceid:%s user:%s dev:%s start:%d", httpReq.TraceId, device.UserID, device.ID, start)
	lastPos := sm.onlineRepo.GetLastPos(device.UserID, device.ID)
	req := sm.buildRequest(httpReq, device, start, lastPos)
	sm.onlineRepo.Pet(device.UserID, device.ID, req.marks.utlRecv, req.timeout)
	sm.userDeviceActiveRepo.UpdateDevActiveTs(device.UserID, device.ID)
	sm.dispatch(device.UserID, req)

	for !req.ready || !req.remoteReady {
		time.Sleep(time.Millisecond * 100)

		now := time.Now().UnixNano() / 1000000
		if now > req.latest || (req.remoteFinished && !req.remoteReady && now-start > 1000) {
			return sm.BuildNotReadyResponse(req, now)
		}
	}

	//wait process ready
	if !req.checkNoWait() {
		curUtl, token, err := sm.userTimeLine.LoadToken(device.UserID, device.ID, req.marks.utlRecv)
		//load token from redis err
		if err != nil {
			log.Errorf("traceId:%s user:%s device:%s utl:%d load token err:%v", req.traceId, device.UserID, device.ID, req.marks.utlRecv, err)
			curUtl = req.marks.utlRecv
		}

		for !req.checkNoWait() {
			time.Sleep(time.Millisecond * 200)

			now := time.Now().UnixNano() / 1000000
			if now > req.latest { //超时
				log.Infof("SyncMng request ready timeout, break wait traceid:%s user:%s dev:%s now:%d latest:%d",
					req.traceId, req.device.UserID, req.device.ID, now, req.latest)
				break
			}

			if sm.CheckNewEvent(req, token, curUtl, start, 500) {
				break
			}
		}
	}

	if !req.ready || !req.remoteReady {
		log.Errorf("SyncMng not ready traceid:%s user:%s dev:%s", req.traceId, req.device.UserID, req.device.ID)
		return sm.BuildNotReadyResponse(req, time.Now().UnixNano()/1000000)
	}

	statusCode, res := sm.BuildResponse(req)

	now := time.Now().UnixNano() / 1000000
	if sm.isFullSync(req) {
		log.Infof("SyncMng full sync traceid:%s user:%s dev:%s presence:%s", req.traceId, req.device.UserID, req.device.ID, res.Presence)
		log.Infof("SyncMng full sync traceid:%s user:%s dev:%s account data:%s", req.traceId, req.device.UserID, req.device.ID, res.AccountData)
		spend := now - start
		if spend > types.CHECK_LOAD_EXCEED_TIME {
			log.Warnf("SyncMng full sync exceed %d ms traceid:%s user:%s dev:%s spend:%d ms",
				types.CHECK_LOAD_EXCEED_TIME, req.traceId, req.device.UserID, req.device.ID, spend)
		} else {
			log.Infof("SyncMng full sync succ traceid:%s user:%s dev:%s spend:%d ms",
				req.traceId, req.device.UserID, req.device.ID, spend)
		}
	} else {
		log.Infof("SyncMng Increment sync response succ traceid:%s user:%s dev:%s spend:%d ms events:%s",
			req.traceId, req.device.UserID, req.device.ID, now-start, res)
	}

	return statusCode, res
}

func (sm *SyncMng) CheckNewEvent(req *request, token map[string]int64, curUtl, start, afterMS int64) bool {
	device := req.device
	now := time.Now().UnixNano() / 1000000
	receiptAfterMS := int64(0)
	if afterMS != 0 {
		receiptAfterMS = sm.cfg.CheckReceipt
	}
	hasEventUpdate := sm.userTimeLine.ExistsUserEventUpdate(req.marks.utlRecv, token, device.UserID, device.ID, req.traceId)
	if curUtl != req.marks.utlRecv {
		log.Warnf("SyncMng ExistsUserEventUpdate update oldUtl:%d to newUtl:%d", req.marks.utlRecv, curUtl)
		req.marks.utlRecv = curUtl
	}
	if hasEventUpdate && now-start > afterMS {
		log.Infof("SyncMng break has user event traceid:%s user:%s dev:%s now:%d latest:%d utlRecv:%d",
			req.traceId, req.device.UserID, req.device.ID, now, start, req.marks.utlRecv)
		return true
	}

	if sm.clientDataStreamRepo.ExistsAccountDataUpdate(req.marks.accRecv, device.UserID) && now-start > afterMS && req.device.IsHuman {
		log.Infof("SyncMng break has account data traceid:%s user:%s dev:%s now:%d latest:%d", req.traceId, req.device.UserID, req.device.ID, now, start)
		return true
	}

	if sm.typingConsumer.ExistsTyping(device.UserID, device.ID, sm.userTimeLine.GetUserCurRoom(device.UserID, device.ID)) && now-start > afterMS && req.device.IsHuman {
		log.Infof("SyncMng break has typing traceid:%s user:%s dev:%s now:%d latest:%d ", req.traceId, req.device.UserID, req.device.ID, now, start)
		return true
	}

	hasReceiptUpdate, lastReceipt := sm.userTimeLine.ExistsUserReceiptUpdate(req.marks.recpRecv, device.UserID)
	if hasReceiptUpdate && now-start > receiptAfterMS && req.device.IsHuman {
		log.Infof("SyncMng break has receipt traceid:%s user:%s dev:%s now:%d latest:%d recpRecv:%d lastReceipt:%d",
			req.traceId, req.device.UserID, req.device.ID, now, start, req.marks.recpRecv, lastReceipt)
		return true
	}

	if sm.cfg.UseEncrypt {
		if common.IsActualDevice(device.DeviceType) && sm.keyChangeRepo.ExistsKeyChange(req.marks.kcRecv, device.UserID) && now-start > afterMS && req.device.IsHuman {
			log.Infof("SyncMng break has key change traceid:%s user:%s dev:%s now:%d latest:%d ", req.traceId, req.device.UserID, req.device.ID, now, start)
			return true
		}
	}

	if common.IsActualDevice(device.DeviceType) && sm.stdEventStreamRepo.ExistsSTDEventUpdate(req.marks.stdRecv, device.UserID, device.ID) && now-start > afterMS && req.device.IsHuman {
		log.Infof("SyncMng break has send to device messages traceid:%s user:%s dev:%s now:%d latest:%d ", req.traceId, req.device.UserID, req.device.ID, now, start)
		return true
	}

	if sm.presenceStreamRepo.ExistsPresence(req.device.UserID, req.marks.preRecv) && now-start > afterMS && req.device.IsHuman {
		log.Infof("SyncMng break has presence messages traceid:%s user:%s dev:%s now:%d latest:%d ", req.traceId, req.device.UserID, req.device.ID, now, start)
		return true
	}
	return false
}

func (sm *SyncMng) BuildNotReadyResponse(req *request, now int64) (int, *syncapitypes.Response) {
	res := syncapitypes.NewResponse(0)
	res.NextBatch = req.token
	if sm.isFullSync(req) && req.device.IsHuman {
		log.Errorf("SyncMng request not ready failed traceid:%s user:%s dev:%s latest:%d spend:%d ms errcode:%d remoteFinished:%t remoteReady:%t",
			req.traceId, req.device.UserID, req.device.ID, req.latest, now-req.start, http.StatusServiceUnavailable, req.remoteFinished, req.remoteReady)
		return http.StatusServiceUnavailable, res
	}
	log.Infof("SyncMng request not ready succ traceid:%s user:%s dev:%s latest:%d spend:%d ms remoteFinished:%t remoteReady:%t",
		req.traceId, req.device.UserID, req.device.ID, req.latest, now-req.start, req.remoteFinished, req.remoteReady)
	return http.StatusOK, res
}

func (sm *SyncMng) BuildResponse(req *request) (int, *syncapitypes.Response) {
	res := syncapitypes.NewResponse(0)
	bs := time.Now().UnixNano() / 1000000
	ok := sm.buildSyncData(req, res)
	spend := time.Now().UnixNano()/1000000 - bs
	log.Debugf("traceid:%s buildSyncData spend:%d", req.traceId, spend)
	if !ok {
		log.Errorf("SyncMng buildSyncData not ok traceid:%s user:%s dev:%s", req.traceId, req.device.UserID, req.device.ID)
		return sm.BuildNotReadyResponse(req, time.Now().UnixNano()/1000000)
	}

	if req.device.IsHuman {
		sm.addPresence(req, res)

		res = sm.addAccountData(req, res)

		if sm.cfg.UseEncrypt {
			if common.IsActualDevice(req.device.DeviceType) {
				sm.addKeyChangeInfo(req, res)
				sm.addSendToDevice(req, res)
				sm.addOneTimeKeyCountInfo(req, res)
			} else {
				res.SignNum = common.DefaultKeyCount()
			}
		} else {
			if common.IsActualDevice(req.device.DeviceType) {
				sm.addSendToDevice(req, res)
			}
			res.SignNum = common.DefaultKeyCount()
		}

		sm.addTyping(req, res, sm.userTimeLine.GetUserCurRoom(req.device.UserID, req.device.ID))
	}

	if sm.cfg.UseMessageFilter {
		res = sm.filterSyncData(req, res)
	}

	res.NextBatch = req.marks.build()

	sm.FillSortEventOffset(res, req)

	return http.StatusOK, res
}
