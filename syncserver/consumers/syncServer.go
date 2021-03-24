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

package consumers

import (
	"fmt"
	"math"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/syncserver/extra"
)

type SyncServer struct {
	db             model.SyncAPIDatabase
	slot           uint32
	chanSize       int
	msgChan        []chan *syncapitypes.SyncServerRequest
	cfg            *config.Dendrite
	compressLength int64

	roomHistory           *repos.RoomHistoryTimeLineRepo
	rsTimeline            *repos.RoomStateTimeLineRepo
	rsCurState            *repos.RoomCurStateRepo
	receiptDataStreamRepo *repos.ReceiptDataStreamRepo
	userReceiptRepo       *repos.UserReceiptRepo
	readCountRepo         *repos.ReadCountRepo
	displayNameRepo       *repos.DisplayNameRepo
	cache                 service.Cache
	rpcClient             *common.RpcClient
	settings              *common.Settings
}

func NewSyncServer(
	db model.SyncAPIDatabase,
	slot uint32,
	chanSize int,
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
) *SyncServer {
	ss := new(SyncServer)
	ss.db = db
	ss.slot = slot
	ss.chanSize = chanSize
	ss.cfg = cfg
	ss.rpcClient = rpcClient

	if cfg.CompressLength != 0 {
		ss.compressLength = cfg.CompressLength
	} else {
		ss.compressLength = config.DefaultCompressLength
	}

	return ss
}

func (s *SyncServer) SetCache(cache service.Cache) *SyncServer {
	s.cache = cache
	return s
}

func (s *SyncServer) SetRoomHistory(roomHistory *repos.RoomHistoryTimeLineRepo) *SyncServer {
	s.roomHistory = roomHistory
	return s
}

func (s *SyncServer) SetRsTimeline(rsTimeline *repos.RoomStateTimeLineRepo) *SyncServer {
	s.rsTimeline = rsTimeline
	return s
}

func (s *SyncServer) SetRsCurState(rsCurState *repos.RoomCurStateRepo) *SyncServer {
	s.rsCurState = rsCurState
	return s
}

func (s *SyncServer) SetReceiptDataStreamRepo(receiptDataStreamRepo *repos.ReceiptDataStreamRepo) *SyncServer {
	s.receiptDataStreamRepo = receiptDataStreamRepo
	return s
}

func (s *SyncServer) SetUserReceiptDataRepo(userReceiptDataRepo *repos.UserReceiptRepo) *SyncServer {
	s.userReceiptRepo = userReceiptDataRepo
	return s
}

func (s *SyncServer) SetReadCountRepo(readCountRepo *repos.ReadCountRepo) *SyncServer {
	s.readCountRepo = readCountRepo
	return s
}

func (s *SyncServer) SetDisplayNameRepo(displayNameRepo *repos.DisplayNameRepo) *SyncServer {
	s.displayNameRepo = displayNameRepo
	return s
}

func (s *SyncServer) SetSettings(settings *common.Settings) {
	s.settings = settings
}

func (s *SyncServer) Start() {
	s.msgChan = make([]chan *syncapitypes.SyncServerRequest, s.slot)
	for i := uint32(0); i < s.slot; i++ {
		s.msgChan[i] = make(chan *syncapitypes.SyncServerRequest, s.chanSize)
		go s.startWorker(s.msgChan[i])
	}
}

func (s *SyncServer) startWorker(channel chan *syncapitypes.SyncServerRequest) {
	for msg := range channel {
		s.processSync(msg)
	}
}

func (s *SyncServer) OnSyncRequest(
	req *syncapitypes.SyncServerRequest,
) {
	hash := common.CalcStringHashCode(req.UserID)
	slot := hash % s.slot
	s.msgChan[slot] <- req
}

func (s *SyncServer) processSync(req *syncapitypes.SyncServerRequest) {
	defer func() {
		if e := recover(); e != nil {
			stack := common.PanicTrace(4)
			log.Panicf("%v\n%s\n", e, stack)
		}
	}()
	//bytes, _ := json.Marshal(*req)
	log.Infof("SyncServer.processSync received request traceid:%s slot:%d rslot:%d user %s device %s", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID)
	if req.IsFullSync {
		s.fullSyncLoading(req)
	} else {
		s.incrementSyncLoading(req)
	}

	if req.LoadReady {
		switch req.RequestType {
		case "load":
			s.responseLoad(req, true)
		case "sync":
			if req.IsFullSync { //full sync
				s.processFullSync(req)
			} else {
				s.processIncrementSync(req)
			}
		}

	} else {
		switch req.RequestType {
		case "load":
			s.responseLoad(req, false)
		}
	}
}

func (s *SyncServer) processFullSync(req *syncapitypes.SyncServerRequest) {
	response := syncapitypes.SyncServerResponse{}
	response.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
	response.Rooms.Invite = make(map[string]syncapitypes.InviteResponse)
	response.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
	response.MaxRoomOffset = make(map[string]int64)
	response.AllLoaded = true
	receiptMaxPos := req.MaxReceiptOffset

	for _, roomInfo := range req.JoinRooms {
		resp, pos, _ := s.buildRoomJoinResp(req, roomInfo.RoomID, roomInfo.Start, roomInfo.End)
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		response.Rooms.Join[roomInfo.RoomID] = *resp
		log.Infof("SyncServer.processFullSync buildRoomJoinResp traceid:%s slot:%d rslot:%d roomID:%s maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, pos)
	}

	for _, roomInfo := range req.InviteRooms {
		resp, pos := s.buildRoomInviteResp(req, roomInfo.RoomID, req.UserID)
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		response.Rooms.Invite[roomInfo.RoomID] = *resp
		log.Infof("SyncServer.processFullSync buildRoomInviteResp traceid:%s slot:%d rslot:%d roomID:%s maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, pos)
	}

	if req.IsHuman {
		s.addReceipt(req, receiptMaxPos, &response)
		s.addUnreadCount(&response, req.UserID)
	}

	s.responseSync(req, &response)
}

func (s *SyncServer) processIncrementSync(req *syncapitypes.SyncServerRequest) {
	response := syncapitypes.SyncServerResponse{}
	response.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
	response.Rooms.Invite = make(map[string]syncapitypes.InviteResponse)
	response.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
	response.MaxRoomOffset = make(map[string]int64)
	receiptMaxPos := req.MaxReceiptOffset
	response.AllLoaded = true
	newUserMap := make(map[string]bool)

	for _, roomInfo := range req.JoinRooms {
		resp, pos, users := s.buildRoomJoinResp(req, roomInfo.RoomID, roomInfo.Start, roomInfo.End)
		log.Infof("SyncServer.processIncrementSync buildRoomJoinResp traceid:%s slot:%d rslot:%d roomID:%s reqStart:%d reqEnd:%d maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, roomInfo.Start, roomInfo.End, pos)
		if (pos > 0 && roomInfo.End > pos) && req.IsHuman == true {
			// event missing
			response.AllLoaded = false
			log.Warnf("SyncServer.processIncrementSync buildRoomJoinResp not all loaded, traceid:%s room:%s", req.TraceID, roomInfo.RoomID)
		}
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		if users != nil {
			for _, user := range users {
				newUserMap[user] = true
			}
		}
		response.Rooms.Join[roomInfo.RoomID] = *resp
	}

	for _, roomInfo := range req.InviteRooms {
		resp, pos := s.buildRoomInviteResp(req, roomInfo.RoomID, req.UserID)
		log.Infof("SyncServer.processIncrementSync buildRoomInviteResp traceid:%s slot:%d rslot:%d roomID:%s reqStart:%d reqEnd:%d maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, roomInfo.Start, roomInfo.End, pos)
		if roomInfo.End > pos && req.IsHuman == true {
			// event missing
			response.AllLoaded = false
			log.Warnf("SyncServer.processIncrementSync buildRoomInviteResp not all loaded, traceid:%s room:%s", req.TraceID, roomInfo.RoomID)
		}
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		response.Rooms.Invite[roomInfo.RoomID] = *resp
	}

	for _, roomInfo := range req.LeaveRooms {
		resp, pos := s.buildRoomLeaveResp(req, roomInfo.RoomID, req.UserID, roomInfo.Start)
		log.Infof("SyncServer.processIncrementSync buildRoomLeaveResp traceid:%s slot:%d rslot:%d roomID:%s reqStart:%d reqEnd:%d maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, roomInfo.Start, roomInfo.End, pos)
		if roomInfo.End > pos && req.IsHuman == true {
			// event missing
			response.AllLoaded = false
			log.Warnf("SyncServer.processIncrementSync buildRoomLeaveResp not all loaded, traceid:%s room:%s", req.TraceID, roomInfo.RoomID)
		}
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		response.Rooms.Leave[roomInfo.RoomID] = *resp
	}

	if req.IsHuman {
		s.addReceipt(req, receiptMaxPos, &response)
		s.addUnreadCount(&response, req.UserID)
	}

	for user := range newUserMap {
		response.NewUsers = append(response.NewUsers, user)
	}

	s.responseSync(req, &response)
}

func (s *SyncServer) responseLoad(req *syncapitypes.SyncServerRequest, ready bool) {
	res := syncapitypes.SyncServerResponse{
		Ready: ready,
	}
	log.Infof("SyncServer.responseLoad traceid:%s slot:%d rslot:%d user %s device %s", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID)
	s.rpcClient.PubObj(req.Reply, res)
}

func (s *SyncServer) responseSync(req *syncapitypes.SyncServerRequest, resp *syncapitypes.SyncServerResponse) {
	device := authtypes.Device{
		UserID:  req.UserID,
		IsHuman: req.IsHuman,
	}
	extra.ExpandSyncData(s.rsCurState, &device, s.displayNameRepo, resp)
	contentBytes, _ := json.Marshal(*resp)
	log.Infof("SyncServer.responseSync traceid:%s slot:%d rslot:%d user %s device %s AllLoaded:%t maxroomoffset:%v", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, resp.AllLoaded, resp.MaxRoomOffset)
	msgSize := int64(len(contentBytes))

	result := types.CompressContent{
		Compressed: false,
	}
	if msgSize > s.compressLength {
		contentBytes = common.DoCompress(contentBytes)
		result.Compressed = true
		log.Infof("nats pub message with content traceid:%s slot:%d rslot:%d, before compress %d after compress %d", req.TraceID, req.Slot, req.RSlot, msgSize, len(contentBytes))
	}
	result.Content = contentBytes
	s.rpcClient.PubObj(req.Reply, result)
}

func (s *SyncServer) fullSyncLoading(req *syncapitypes.SyncServerRequest) {
	start := time.Now().UnixNano()

	s.receiptDataStreamRepo.LoadRoomLatest(req.JoinedRooms)

	for _, roomInfo := range req.JoinRooms {
		s.rsTimeline.LoadStates(roomInfo.RoomID, false)
		s.rsTimeline.LoadStreamStates(roomInfo.RoomID, false)
		s.roomHistory.LoadHistory(roomInfo.RoomID, false)
		s.roomHistory.GetRoomMinStream(roomInfo.RoomID)

		s.receiptDataStreamRepo.LoadHistory(roomInfo.RoomID, false)
		s.userReceiptRepo.LoadHistory(req.UserID, roomInfo.RoomID, false)
	}

	for _, roomInfo := range req.InviteRooms {
		s.rsTimeline.LoadStates(roomInfo.RoomID, false)
	}

	loadStart := time.Now().Unix()
	for {
		loaded := true

		for _, roomInfo := range req.JoinRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.rsTimeline.CheckStreamLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.roomHistory.CheckLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}

			if ok := s.receiptDataStreamRepo.CheckLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.userReceiptRepo.CheckLoadReady(req.UserID, roomInfo.RoomID, false); !ok {
				loaded = false
			}
		}

		for _, roomInfo := range req.InviteRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
		}

		if loaded {
			req.LoadReady = true
			spend := (time.Now().UnixNano() - start) / 1000000
			if spend > types.CHECK_LOAD_EXCEED_TIME {
				log.Warnf("SyncServer fullSyncLoading exceed %d ms traceid:%s slot:%d rslot:%d user:%s device:%s spend:%d ms", types.CHECK_LOAD_EXCEED_TIME, req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, spend)
			} else {
				log.Infof("SyncServer fullSyncLoading succ traceid:%s slot:%d rslot:%d user:%s device:%s spend:%d ms", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, spend)
			}
			break
		}

		now := time.Now().Unix()
		if now-loadStart > 35 {
			req.LoadReady = false
			log.Errorf("SyncServer fullSyncLoading failed traceid:%s slot:%d rslot:%d user:%s device:%s spend:%d s", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, now-loadStart)
			break
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func (s *SyncServer) incrementSyncLoading(req *syncapitypes.SyncServerRequest) {
	start := time.Now().UnixNano()

	s.roomHistory.LoadRoomLatest(req.JoinRooms)
	s.roomHistory.LoadRoomLatest(req.InviteRooms)
	s.roomHistory.LoadRoomLatest(req.LeaveRooms)
	ms := "joins:"
	for _, room := range req.JoinRooms {
		ms += room.RoomID + ","
	}
	ms += "invite:"
	for _, room := range req.InviteRooms {
		ms += room.RoomID + ","
	}
	ms += "leave:"
	for _, room := range req.LeaveRooms {
		ms += room.RoomID + ","
	}
	log.Infof("SyncServer.incrementSyncLoading membership traceid:%s slot:%d rslot:%d user:%s devID:%s ms:%s", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, ms)
	if req.IsHuman {
		s.receiptDataStreamRepo.LoadRoomLatest(req.JoinedRooms)
	}
	for _, roomInfo := range req.JoinRooms {
		lastoffset := s.roomHistory.GetRoomLastOffset(roomInfo.RoomID)
		if lastoffset >= roomInfo.Start {
			s.rsTimeline.LoadStates(roomInfo.RoomID, false)
			s.rsTimeline.LoadStreamStates(roomInfo.RoomID, false)
			s.roomHistory.LoadHistory(roomInfo.RoomID, false)
			s.roomHistory.GetRoomMinStream(roomInfo.RoomID)
		} else {
			req.LoadReady = false
			log.Warnf("SyncServer.incrementSyncLoading join load not ready, traceid:%s slot:%d rslot:%d user:%s devID:%s roomID:%s lastoffset:%d start:%d", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, roomInfo.RoomID, lastoffset, roomInfo.Start)
			return
		}
	}

	if req.IsHuman {
		for _, roomID := range req.JoinedRooms {
			if s.receiptDataStreamRepo.GetRoomLastOffset(roomID) > req.ReceiptOffset {
				s.receiptDataStreamRepo.LoadHistory(roomID, false)
				s.userReceiptRepo.LoadHistory(req.UserID, roomID, false)
			}
		}
	}

	for _, roomInfo := range req.InviteRooms {
		lastoffset := s.roomHistory.GetRoomLastOffset(roomInfo.RoomID)
		if lastoffset >= roomInfo.Start {
			s.rsTimeline.LoadStates(roomInfo.RoomID, false)
		} else {
			req.LoadReady = false
			log.Warnf("SyncServer.incrementSyncLoading invite load not ready, traceid:%s slot:%d rslot:%d user:%s devID:%s roomID:%s lastoffset:%d start:%d", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, roomInfo.RoomID, lastoffset, roomInfo.Start)
			return
		}
	}

	for _, roomInfo := range req.LeaveRooms {
		lastoffset := s.roomHistory.GetRoomLastOffset(roomInfo.RoomID)
		if lastoffset >= roomInfo.Start {
			s.rsTimeline.LoadStates(roomInfo.RoomID, false)
			s.rsTimeline.LoadStreamStates(roomInfo.RoomID, false)
			s.roomHistory.LoadHistory(roomInfo.RoomID, false)
			s.roomHistory.GetRoomMinStream(roomInfo.RoomID)
		} else {
			req.LoadReady = false
			log.Warnf("SyncServer.incrementSyncLoading invite load not ready, traceid:%s slot:%d rslot:%d user:%s devID:%s roomID:%s lastoffset:%d start:%d", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, roomInfo.RoomID, lastoffset, roomInfo.Start)
			return
		}
	}

	loadStart := time.Now().Unix()

	for {
		loaded := true
		for _, roomInfo := range req.JoinRooms {
			if s.roomHistory.GetRoomLastOffset(roomInfo.RoomID) > roomInfo.Start {
				if ok := s.rsTimeline.CheckStateLoadReady(roomInfo.RoomID, false); !ok {
					loaded = false
				}
				if ok := s.rsTimeline.CheckStreamLoadReady(roomInfo.RoomID, false); !ok {
					loaded = false
				}
				if ok := s.roomHistory.CheckLoadReady(roomInfo.RoomID, false); !ok {
					loaded = false
				}
			}

			if req.IsHuman {
				if s.receiptDataStreamRepo.GetRoomLastOffset(roomInfo.RoomID) > req.ReceiptOffset {
					if ok := s.receiptDataStreamRepo.CheckLoadReady(roomInfo.RoomID, false); !ok {
						loaded = false
					}
					if ok := s.userReceiptRepo.CheckLoadReady(req.UserID, roomInfo.RoomID, false); !ok {
						loaded = false
					}
				}
			}
		}

		for _, roomInfo := range req.InviteRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
		}

		for _, roomInfo := range req.LeaveRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.rsTimeline.CheckStreamLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.roomHistory.CheckLoadReady(roomInfo.RoomID, false); !ok {
				loaded = false
			}
		}

		if loaded {
			req.LoadReady = true
			spend := (time.Now().UnixNano() - start) / 1000000
			if spend > types.CHECK_LOAD_EXCEED_TIME {
				log.Warnf("SyncServer incrementSyncLoading exceed %d ms traceid:%s slot:%d rslot:%d user:%s device:%s use:%d ms", types.CHECK_LOAD_EXCEED_TIME, req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, spend)
			} else {
				log.Infof("SyncServer incrementSyncLoading succ traceid:%s slot:%d rslot:%d user:%s device:%s use:%d ms", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, spend)
			}
			break
		}

		now := time.Now().Unix()
		if now-loadStart > 35 {
			req.LoadReady = false
			log.Errorf("SyncServer incrementSyncLoading failed traceid:%s slot:%d rslot:%d user:%s device:%s spend:%d s", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, now-loadStart)
			break
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func (s *SyncServer) addReceipt(req *syncapitypes.SyncServerRequest, maxPos int64, response *syncapitypes.SyncServerResponse) {
	if !s.receiptDataStreamRepo.ExistsReceipt(req.ReceiptOffset, req.UserID) {
		response.MaxReceiptOffset = req.ReceiptOffset
		log.Infof("not ExistsReceipt traceid:%s MaxReceiptOffset:%d maxPos:%d", req.TraceID, response.MaxReceiptOffset, maxPos)
		return
	}

	var jr *syncapitypes.JoinResponse
	var receiptEvent gomatrixserverlib.ClientEvent
	var selfReceiptEvent gomatrixserverlib.ClientEvent
	maxRes := int64(-1)

	for _, roomID := range req.JoinedRooms {
		if s.receiptDataStreamRepo.GetRoomLastOffset(roomID) <= req.ReceiptOffset {
			continue
		}
		rdsTimeLine := s.receiptDataStreamRepo.GetHistory(roomID)
		if rdsTimeLine != nil {
			_, feedUp := rdsTimeLine.GetFeedRange()
			if feedUp > req.ReceiptOffset {
				lastOffset := s.receiptDataStreamRepo.GetRoomLastOffset(roomID)

				// full sync请求，如果当前用户最新消息未读，应返回用户上次读到的offset
				if req.IsFullSync {
					uOffset := s.userReceiptRepo.GetLatestOffset(req.UserID, roomID)
					if uOffset < lastOffset && uOffset > req.ReceiptOffset {
						selfReceipt := s.userReceiptRepo.GetLatestReceipt(req.UserID, roomID)
						if selfReceipt != nil {
							err := json.Unmarshal(selfReceipt, &selfReceiptEvent)
							if err != nil {
								log.Errorw("addReceipt: Unmarshal json error for receipt", log.KeysAndValues{"roomID", roomID, "eventJson", string(selfReceipt), "error", err})
								continue
							}

							if joinResponse, ok := response.Rooms.Join[roomID]; ok {
								jr = &joinResponse
							} else {
								jr = syncapitypes.NewJoinResponse()
							}
							jr.Ephemeral.Events = append(jr.Ephemeral.Events, selfReceiptEvent)
							response.Rooms.Join[roomID] = *jr
						}
					}
				}

				var feeds []feedstypes.Feed
				rdsTimeLine.ForRange(func(offset int, feed feedstypes.Feed) bool {
					if feed == nil {
						log.Errorf("SyncMng.addReceipt user:%s device:%s get feed nil offset %d", req.UserID, req.DeviceID, offset)
						rdsTimeLine.Console()
					} else {
						feeds = append(feeds, feed)
					}
					return true
				})
				for _, feed := range feeds {
					if feed != nil {
						stream := feed.(*feedstypes.ReceiptDataStream)

						if stream.GetOffset() > req.ReceiptOffset && stream.GetOffset() <= maxPos {
							if joinResponse, ok := response.Rooms.Join[roomID]; ok {
								jr = &joinResponse
							} else {
								jr = syncapitypes.NewJoinResponse()
							}

							err := json.Unmarshal(stream.DataStream.Content, &receiptEvent)
							if err != nil {
								log.Errorw("addReceipt: Unmarshal json error for receipt", log.KeysAndValues{
									"roomID", stream.DataStream.RoomID, "eventJson", string(stream.DataStream.Content), "error", err,
								})
								continue
							}

							jr.Ephemeral.Events = append(jr.Ephemeral.Events, receiptEvent)
							response.Rooms.Join[roomID] = *jr

							if stream.GetOffset() > maxRes {
								maxRes = stream.GetOffset()
							}
						}
					}
				}
			}
		}
	}

	if maxRes == -1 {
		response.MaxReceiptOffset = req.ReceiptOffset
		log.Infof("traceid:%s MaxReceiptOffset:%d maxRes is -1", req.TraceID, response.MaxReceiptOffset)
	} else {
		response.MaxReceiptOffset = maxRes
		log.Infof("traceid:%s MaxReceiptOffset:%d maxRes not -1", req.TraceID, response.MaxReceiptOffset)
	}
}

func (s *SyncServer) addUnreadCount(response *syncapitypes.SyncServerResponse, userID string) {
	for rid, joinRooms := range response.Rooms.Join {
		ntfCount, hlCount := s.readCountRepo.GetRoomReadCount(rid, userID)
		joinRooms.Unread = &syncapitypes.UnreadNotifications{
			NotificationCount: ntfCount,
			HighLightCount:    hlCount,
		}
		response.Rooms.Join[rid] = joinRooms

	}
}

type BuildRoomRespOpt struct {
	TraceID      string
	UserID       string
	DeviceID     string
	roomID       string
	Left         int
	Limit        int
	reqStart     int64
	reqEnd       int64
	realEnd      int64
	isFullSync   bool
	addNewUser   bool
	needOldState bool

	// maxPos histroytimeline max offset
	maxPos    int64
	minOffset int64
	// firstTimeLine histroy timeline min offset
	firstTimeLine int64
	firstTs       int64

	meetCreate bool
}

func newBuildRoomRespOpt(req *syncapitypes.SyncServerRequest, roomID string, reqStart, reqEnd int64) *BuildRoomRespOpt {
	return &BuildRoomRespOpt{
		TraceID:       req.TraceID,
		UserID:        req.UserID,
		DeviceID:      req.DeviceID,
		roomID:        roomID,
		Left:          req.Limit,
		Limit:         req.Limit,
		reqStart:      reqStart,
		reqEnd:        reqEnd,
		realEnd:       reqEnd,
		isFullSync:    req.IsFullSync,
		maxPos:        -1,
		minOffset:     -1,
		firstTimeLine: -1,
		firstTs:       -1,
	}
}

func (opt *BuildRoomRespOpt) String() string {
	return fmt.Sprintf("traceid:%s user:%s device:%s roomID:%s maxPos:%d minOffset:%d addNewUser:%t isFullSync:%t needOldState:%t reqStart:%d reqEnd:%d realEnd:%d limit:%d left:%d firstTimeLine:%d firstTs:%d meetCreate:%t",
		opt.TraceID,
		opt.UserID,
		opt.DeviceID,
		opt.roomID,
		opt.maxPos,
		opt.minOffset,
		opt.addNewUser,
		opt.isFullSync,
		opt.needOldState,
		opt.reqStart,
		opt.reqEnd,
		opt.realEnd,
		opt.Limit,
		opt.Left,
		opt.firstTimeLine,
		opt.firstTs,
		opt.meetCreate,
	)
}

func (opt *BuildRoomRespOpt) lteStart(offset int64) bool {
	return offset <= opt.reqStart
}

func (opt *BuildRoomRespOpt) lteEnd(offset int64) bool {
	return offset <= opt.reqEnd || opt.reqEnd == -1
}

func (opt *BuildRoomRespOpt) lteRealEnd(offset int64) bool {
	return offset <= opt.realEnd || opt.realEnd == -1
}

func (opt *BuildRoomRespOpt) ltFirst(offset int64) bool {
	return offset < opt.firstTimeLine
}

func (opt *BuildRoomRespOpt) isJoinedRoomLimited(msgEvent, states []gomatrixserverlib.ClientEvent) bool {
	if len(states) > 0 {
		log.Infof("SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s set limit true because len(state event) %d > 0", opt.TraceID, opt.roomID, opt.UserID, len(states))
		return true
	}
	limited := false
	if opt.addNewUser || opt.isFullSync {
		limited = true
	}
	if opt.Left == 0 || opt.meetCreate {
		if opt.meetCreate {
			limited = false
		} else {
			limited = true // FIXME: 优化：这里如果凑巧limit=0时，刚好遇到边界，就不应该是true了
		}
	}
	if opt.minOffset > opt.reqStart && len(msgEvent) >= 128 { // FIXME: 128是timlineRepo的大小
		limited = true
	}

	if opt.firstTimeLine == -1 {
		limited = true // 没有交集，应该是true
	} else {
		if opt.firstTimeLine > opt.reqStart && opt.reqStart > 0 && len(msgEvent) >= opt.Limit {
			limited = true
		}
	}
	return limited
}

func (opt *BuildRoomRespOpt) calcRoomPrevBatch() string {
	prevBatch := ""
	//firsttimeline == -1, syncserver historytime not has event, bat syncaggserver judge has event
	if opt.firstTimeLine == -1 {
		prevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		log.Infof("SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s firstTimeLine:-1 ", opt.TraceID, opt.roomID, opt.UserID)
	} else {
		// when we call get_messages with param "from"=prev_batch, it will start from prev_batch,
		// but not from (prev_batch-1), so we set (firstTimeLine-1) to avoid getting repeat message
		streamPos := opt.firstTimeLine - 1
		if streamPos < 0 {
			streamPos = 0
		}
		prevBatch = common.BuildPreBatch(streamPos, opt.firstTs)
	}
	return prevBatch
}

//roomtimeline (]
func (s *SyncServer) buildRoomJoinResp(req *syncapitypes.SyncServerRequest, roomID string, reqStart, reqEnd int64) (*syncapitypes.JoinResponse, int64, []string) {
	opt := newBuildRoomRespOpt(req, roomID, reqStart, reqEnd)
	log.Infof("SyncServer.buildRoomJoinResp start %s", opt)
	//再加载一遍，避免数据遗漏
	s.rsTimeline.LoadStates(roomID, true)
	s.rsTimeline.LoadStreamStates(roomID, true)
	s.roomHistory.LoadHistory(roomID, true)
	s.receiptDataStreamRepo.LoadHistory(roomID, true)
	s.userReceiptRepo.LoadHistory(opt.UserID, roomID, true)
	minStream := s.roomHistory.GetRoomMinStream(roomID)

	jr := syncapitypes.NewJoinResponse()
	jr.Timeline.Limited = true
	jr.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
	jr.Timeline.Events = []gomatrixserverlib.ClientEvent{}

	rs := s.rsCurState.GetRoomState(roomID)
	if rs == nil {
		log.Warnf("SyncServer.buildRoomJoinResp rsCurState.GetRoomState nil roomLatest %d %s", s.roomHistory.GetRoomLastOffset(roomID), opt)
		return jr, opt.maxPos, []string{}
	}

	history := s.roomHistory.GetHistory(roomID)
	if history == nil {
		log.Warnf("SyncServer.buildRoomJoinResp roomHistory.GetHistory nil roomLatest %d %s", s.roomHistory.GetRoomLastOffset(roomID), opt)
		return jr, opt.maxPos, []string{}
	}

	stateEvt := rs.GetState("m.room.member", opt.UserID)
	if stateEvt == nil {
		log.Warnf("SyncServer.buildRoomJoinResp rs.GetState nil roomLatest %d %s", s.roomHistory.GetRoomLastOffset(roomID), opt)
		//old join members event's has event_offset < 0 event
		return jr, opt.maxPos, []string{}
	}

	states := s.rsTimeline.GetStates(roomID)
	if states == nil {
		log.Errorf("SyncServer.buildRoomJoinResp rsTimeline.GetStates nil roomLatest %d %s", s.roomHistory.GetRoomLastOffset(roomID), opt)
		return jr, opt.maxPos, []string{}
	}

	jr.Timeline.Limited = false

	if stateEvt.GetOffset() > opt.reqStart {
		joinContent := external.MemberContent{}
		json.Unmarshal(stateEvt.GetEv().Content, &joinContent)
		if joinContent.Reason != "BuildMembershipAndFireEvents" {
			opt.addNewUser = true
		}
	}

	if opt.addNewUser || opt.isFullSync {
		opt.needOldState = true
		log.Infof("SyncServer buildRoomJoinResp update stateEvt.Offset:%d %s", stateEvt.Offset, opt)
		opt.reqStart = -1
	}

	feeds, start, end, low, up := history.GetAllFeedsReverse()
	log.Infof("SyncServer buildRoomJoinResp load room-timeline start:%d end:%d low:%d up:%d len(feeds):%d %s", start, end, low, up, len(feeds), opt)

	index, latestEvOffset := s.getLatestValidEvIdx(*opt, feeds)
	if latestEvOffset != 0 && !opt.lteEnd(latestEvOffset-1) {
		opt.realEnd = latestEvOffset - 1
	}

	msgEvent := s.getJoinRoomEvents(opt, feeds, rs, index, minStream)

	var users []string
	if (opt.addNewUser || opt.needOldState) && req.IsHuman {
		users = s.getJoinUsers(opt.roomID)
	}

	statesList := s.getJoinRoomStates(*opt, states, latestEvOffset)

	jr.Timeline.Limited = opt.isJoinedRoomLimited(msgEvent, statesList)
	jr.Timeline.PrevBatch = opt.calcRoomPrevBatch()
	jr.Timeline.Events = msgEvent
	jr.State.Events = statesList
	log.Infof("SyncServer.buildRoomJoinResp limited:%t prevBatch:%s len(msgEvent):%d len(stateEvent):%d %s", jr.Timeline.Limited, jr.Timeline.PrevBatch, len(jr.Timeline.Events), len(jr.State.Events), opt)
	return jr, opt.maxPos, users
}

func (s *SyncServer) reverseMsgEvent(msgEvent []gomatrixserverlib.ClientEvent) {
	for i := 0; i < len(msgEvent)/2; i++ {
		tmp := msgEvent[i]
		msgEvent[i] = msgEvent[len(msgEvent)-i-1]
		msgEvent[len(msgEvent)-i-1] = tmp
	}
}

// getLatestValidEvIdx get the latest event index of the reversed feeds.
// Sometime the reversed feeds contains more then one membership event of the
// spec user. If this user has already exists this room, then the sync can't
// response to the client. The client will sync the left events at next time.
// FIXME: syncaggregate改成度扩散后，可能没注意到这一点
func (s *SyncServer) getLatestValidEvIdx(opt BuildRoomRespOpt, reversed []feedstypes.Feed) (int, int64) {
	latestValidEvIdx := -1
	latestEvOffset := int64(0)
	isDeleted := false
	if opt.reqStart > 0 {
		for i := len(reversed) - 1; i >= 0; i-- {
			stream := reversed[i].(*feedstypes.StreamEvent)
			if stream == nil || opt.lteStart(stream.GetOffset()) {
				continue
			}
			if stream.Ev.Type == "m.room.member" && *stream.Ev.StateKey == opt.UserID {
				membership, _ := stream.GetEv().Membership()
				if membership != "join" {
					latestValidEvIdx = len(reversed) - i - 1
					latestEvOffset = stream.GetOffset()
					log.Infof("SyncServer.buildRoomJoinResp latestEvOffset:%d %s", latestEvOffset, &opt)
					isDeleted = stream.IsDeleted
					break
				}
			}
		}
	}
	index := 0
	if latestValidEvIdx >= 0 {
		log.Infof("SyncServer.buildRoomJoinResp truncate feed datas because data has been updated, latestValidEvIdx:%d, latestEvOffset:%d, isDeleted:%t %s",
			latestValidEvIdx, latestEvOffset, isDeleted, &opt)
		index = latestValidEvIdx + 1
	}
	return index, latestEvOffset
}

func (s *SyncServer) getJoinRoomEvents(opt *BuildRoomRespOpt, feeds []feedstypes.Feed, rs *repos.RoomState, index int, minStream int64) []gomatrixserverlib.ClientEvent {
	nowTs := time.Now().Unix()
	evRecords := make(map[string]int)
	msgEvent := []gomatrixserverlib.ClientEvent{}
	firstTimeLine := opt.firstTimeLine
	firstTs := opt.firstTs
	maxPos := opt.maxPos
	left := opt.Limit
	minOffset := opt.minOffset
	meetCreate := opt.meetCreate
	for i := index; i < len(feeds); i++ {
		feed := feeds[i]
		if feed == nil {
			log.Errorf("SyncServer.buildRoomJoinResp get state feed nil %s", opt)
			continue
		}
		stream := feed.(*feedstypes.StreamEvent)
		streamOffset := stream.GetOffset()
		//log.Infof("only for test SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s, offset:%d reqStart:%d", req.TraceID, roomID, req.UserID, req.DeviceID, stream.GetOffset(), reqStart)
		if opt.lteStart(streamOffset) {
			log.Infof("SyncServer.buildRoomJoinResp break state offset:%d %s", stream.GetOffset(), opt)
			break
		}

		ev := stream.GetEv()
		if !opt.lteEnd(streamOffset) {
			continue
		}
		if firstTimeLine == -1 || firstTimeLine > streamOffset {
			firstTimeLine = streamOffset
			firstTs = int64(stream.Ev.OriginServerTS)
		}
		if streamOffset > maxPos {
			maxPos = streamOffset
		}

		isStateEv := common.IsStateClientEv(ev)
		if isStateEv || (!isStateEv && rs.CheckEventVisibility(opt.UserID, int64(stream.Ev.OriginServerTS))) {
			if !s.isSkipEv(opt, stream.Ev, isStateEv, nowTs) {
				if idx, ok := evRecords[ev.EventID]; !ok {
					msgEvent = append(msgEvent, *ev)
					left--
					evRecords[ev.EventID] = i
					if streamOffset < minOffset {
						minOffset = streamOffset
					}
				} else {
					log.Warnf("SyncServer.buildRoomJoinResp found replicate events, eventID:%s eventNID:%d index:%d thisIndex:%d %s",
						stream.GetEv().EventID, stream.GetEv().EventNID, idx, i, opt)
				}
			} else {
				log.Infof("SyncServer.buildRoomJoinResp skip event for setting eventID:%s %s", stream.Ev.EventID, opt)
			}
		} else {
			log.Infof("SyncServer.buildRoomJoinResp skip event for visibity eventID:%s %s", stream.Ev.EventID, opt)
		}

		if left == 0 || ev.Type == "m.room.create" {
			meetCreate = ev.Type == "m.room.create"
			log.Infof("SyncServer.buildRoomJoinResp break eventType:%s stream.Offset:%d minStream:%d %s", stream.GetEv().Type, stream.Offset, minStream, opt)
			break
		}
	}

	opt.firstTimeLine = firstTimeLine
	opt.firstTs = firstTs
	opt.maxPos = maxPos
	opt.Left = left
	opt.minOffset = minOffset
	opt.meetCreate = meetCreate

	s.reverseMsgEvent(msgEvent)
	log.Infof("SyncServer.buildRoomJoinResp after sort get msgevent len:%d %s", len(msgEvent), opt)

	return msgEvent
}

func (s *SyncServer) getJoinRoomStates(opt BuildRoomRespOpt, states *feedstypes.TimeLines, latestEvOffset int64) []gomatrixserverlib.ClientEvent {
	stateEvent := []gomatrixserverlib.ClientEvent{}
	feeds, start, end, low, up := states.GetAllFeeds()
	log.Infof("SyncServer buildRoomJoinResp load state-timeline start:%d end:%d low:%d up:%d len(feeds):%d %s", start, end, low, up, opt.realEnd, &opt)
	//state get from since to historytimeline min offset
	for _, feed := range feeds { //填充state
		if feed == nil {
			continue
		}

		stream := feed.(*feedstypes.StreamEvent)
		// if stream.IsDeleted {
		// 	continue
		// }

		streamOffset := stream.GetOffset()
		if !opt.lteRealEnd(streamOffset) {
			continue
		}
		// offset >= history min offset, has get new event from historytimeline, not need get event from rstimeline again
		if !opt.ltFirst(streamOffset) {
			log.Infof("SyncServer.buildRoomJoinResp rsTimeline.GetStates traceid:%s break because of offfset:%d >= historytimeline min offset %d", opt.TraceID, streamOffset, opt.firstTimeLine)
			break
		}

		if opt.needOldState || !opt.lteStart(streamOffset) {
			stateEvent = append(stateEvent, *stream.GetEv())
		}
	}
	return stateEvent
}

func (s *SyncServer) getJoinUsers(roomID string) []string {
	var users []string
	joinMap := s.rsCurState.GetRoomState(roomID).GetJoinMap()
	if joinMap != nil {
		joinMap.Range(func(key, value interface{}) bool {
			users = append(users, key.(string))
			return true
		})
	}
	return users
}

func (s *SyncServer) isSkipEv(opt *BuildRoomRespOpt, ev *gomatrixserverlib.ClientEvent, isStateEv bool, checkTs int64) bool {
	if isStateEv {
		return false
	}
	skipEv := false
	visibilityTime := s.settings.GetMessageVisilibityTime()
	if visibilityTime > 0 {
		if int64(ev.OriginServerTS)/1000+visibilityTime < checkTs {
			log.Infof("SyncServer.buildRoomJoinResp skip traceid:%s event:%s, ts:%d", opt.TraceID, ev.EventID, ev.OriginServerTS)
			skipEv = true
		}
	}
	return skipEv
}

func (s *SyncServer) buildRoomInviteResp(req *syncapitypes.SyncServerRequest, roomID, user string) (*syncapitypes.InviteResponse, int64) {
	ir := syncapitypes.NewInviteResponse()
	maxPos := int64(-1)
	s.rsTimeline.LoadStates(roomID, true)

	states := s.rsTimeline.GetStates(roomID)
	if states != nil {
		feeds, _, _, _, _ := states.GetAllFeeds()
		endIdx := len(feeds) - 1
		latestInviteEvIdx := endIdx

		// get latest join event index
		for i := endIdx; i >= 0; i-- {
			stream := feeds[i].(*feedstypes.StreamEvent)
			if stream.Ev.Type == "m.room.member" && *stream.Ev.StateKey == user {
				membership, _ := stream.GetEv().Membership()
				if membership == "invite" {
					latestInviteEvIdx = i
					break
				}
			}
		}

		// we may get join or leave event as the latest m.room.member event,
		// because the "buildInviteResp" process may be too long so a new join or
		// leave event may be produced and added to state timeline, then we will
		// find out the invite event has been mark with "deleted".
		// "auto-join" a room will often reproduce this situation
		for i := 0; i <= latestInviteEvIdx; i++ {
			if feeds[i] != nil {
				stream := feeds[i].(*feedstypes.StreamEvent)
				if stream.IsDeleted {
					continue
				}

				if stream.Offset > maxPos {
					maxPos = stream.Offset
				}
				ir.InviteState.Events = append(ir.InviteState.Events, *stream.GetEv())
				/*if stream.Ev.Type == "m.room.member" {
					if *stream.Ev.StateKey == user {
						break
					}
				}*/
			}
		}
	} else {
		log.Errorf("SyncServer.buildRoomInviteResp rsTimeline.GetStates nil traceid:%s roomID %s user %s", req.TraceID, roomID, user)
	}

	return ir, maxPos
}

//对于leave，理论上只要返回leave事件即可，其余消息属于多余
func (s *SyncServer) buildRoomLeaveResp(req *syncapitypes.SyncServerRequest, roomID, user string, reqStart int64) (*syncapitypes.LeaveResponse, int64) {
	lv := syncapitypes.NewLeaveResponse()
	maxPos := int64(-1)
	s.rsTimeline.LoadStates(roomID, true)
	s.rsTimeline.LoadStreamStates(roomID, true)
	s.roomHistory.LoadHistory(roomID, true)

	rs := s.rsCurState.GetRoomState(roomID)
	if rs == nil {
		log.Errorf("SyncServer.buildRoomLeaveResp rsCurState.GetRoomState nil roomID %s user %s device %s since %d roomLatest %d", roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		lv.Timeline.Limited = true
		lv.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		lv.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return lv, maxPos
	}

	history := s.roomHistory.GetHistory(roomID)
	if history == nil {
		log.Errorf("SyncServer.buildRoomLeaveResp roomHistory.GetHistory nil roomID %s user %s device %s since %d roomLatest %d", roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		lv.Timeline.Limited = true
		lv.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		lv.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return lv, maxPos
	}

	stateEvt := rs.GetState("m.room.member", req.UserID)
	if stateEvt == nil {
		log.Errorf("SyncServer.buildRoomLeaveResp rs.GetState nil roomID %s user %s device %s since %d roomLatest %d", roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		lv.Timeline.Limited = true
		lv.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		lv.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return lv, maxPos
	}

	maxPos = stateEvt.Offset

	// low, up := history.GetFeedRange()
	// start, end := history.GetRange()
	firstTimeLine := int64(-1)
	firstTs := int64(-1)

	feeds, start, end, low, up := history.GetAllFeedsReverse()

	log.Infof("SyncServer buildRoomJoinResp roomID:%s user:%s device:%s load room-timeline start:%d end:%d low:%d up:%d", roomID, req.UserID, req.DeviceID, start, end, low, up)

	limit := req.Limit
	var msgEvent []gomatrixserverlib.ClientEvent
	msgEvent = []gomatrixserverlib.ClientEvent{}
	minStream := s.roomHistory.GetRoomMinStream(roomID)

	visibilityTime := s.settings.GetMessageVisilibityTime()
	nowTs := time.Now().Unix()

	for i, feed := range feeds {
		if feed != nil {
			stream := feed.(*feedstypes.StreamEvent)
			if stream.GetOffset() <= reqStart {
				break
			}

			if stream.GetOffset() < maxPos {
				if firstTimeLine == -1 {
					firstTimeLine = stream.GetOffset()
					firstTs = int64(stream.Ev.OriginServerTS)
				}

				if firstTimeLine > stream.GetOffset() {
					firstTimeLine = stream.Offset
					firstTs = int64(stream.Ev.OriginServerTS)
				}

				if common.IsStateClientEv(stream.GetEv()) == true || rs.CheckEventVisibility(req.UserID, int64(stream.Ev.OriginServerTS)) {
					skipEv := false
					if visibilityTime > 0 {
						ts := int64(stream.Ev.OriginServerTS) / 1000
						if ts+visibilityTime < nowTs && !common.IsStateClientEv(stream.Ev) {
							log.Infof("buildLeaveRoomResp skip event %s, ts: %d", stream.Ev.EventID, stream.Ev.OriginServerTS)
							skipEv = true
						}
					}

					if !skipEv {
						msgEvent = append(msgEvent, *stream.GetEv())
						limit = limit - 1
					}
				}

				if limit == 0 || stream.GetEv().Type == "m.room.create" || stream.Offset <= minStream {
					if stream.GetEv().Type != "m.room.create" {
						lv.Timeline.Limited = true
					} else {
						lv.Timeline.Limited = false
					}
					log.Infof("SyncServer.buildRoomLeaveResp traceid:%s break roomID:%s user:%s device:%s limit:%d eventType:%s stream.Offset:%d minStream:%d", req.TraceID, roomID, req.UserID, req.DeviceID, req.Limit, stream.GetEv().Type, stream.Offset, minStream)
					break
				}
			}
		} else {
			log.Errorf("SyncServer.buildRoomLeaveResp roomID:%s user:%s device:%s get feed nil offset %d, lower %d, upper %d", roomID, req.UserID, req.DeviceID, end-1-i, low, up)
			history.Console()
		}

	}

	for i := 0; i < len(msgEvent)/2; i++ {
		tmp := msgEvent[i]
		msgEvent[i] = msgEvent[len(msgEvent)-i-1]
		msgEvent[len(msgEvent)-i-1] = tmp
	}

	if firstTimeLine == -1 {
		firstTimeLine = math.MaxInt64
		firstTs = math.MaxInt64
		lv.Timeline.Limited = true
	} else {
		if firstTimeLine > reqStart && reqStart > 0 && len(msgEvent) >= req.Limit {
			lv.Timeline.Limited = true
		}
		firstTimeLine = firstTimeLine - 1
		if firstTimeLine < 0 {
			firstTimeLine = 0
		}
	}
	lv.Timeline.PrevBatch = common.BuildPreBatch(firstTimeLine, firstTs)
	msgEvent = append(msgEvent, *stateEvt.Ev)
	lv.Timeline.Events = msgEvent

	return lv, maxPos
}
