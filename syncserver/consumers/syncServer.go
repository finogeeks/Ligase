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
	"context"
	"math"
	"sort"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/syncserver/extra"
)

type SyncServer struct {
	db       model.SyncAPIDatabase
	slot     uint32
	chanSize int
	//msgChan        []chan *syncapitypes.SyncServerRequest
	msgChan        []chan common.ContextMsg
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
	s.msgChan = make([]chan common.ContextMsg, s.slot)
	for i := uint32(0); i < s.slot; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, s.chanSize)
		go s.startWorker(s.msgChan[i])
	}
}

func (s *SyncServer) startWorker(channel chan common.ContextMsg) {
	for data := range channel {
		msg := data.Msg.(*syncapitypes.SyncServerRequest)
		s.processSync(data.Ctx, msg)
	}
}

func (s *SyncServer) OnSyncRequest(
	ctx context.Context,
	req *syncapitypes.SyncServerRequest,
) {
	hash := common.CalcStringHashCode(req.UserID)
	slot := hash % s.slot
	s.msgChan[slot] <- common.ContextMsg{Ctx: ctx, Msg: req}
}

func (s *SyncServer) processSync(ctx context.Context, req *syncapitypes.SyncServerRequest) {
	defer func() {
		if e := recover(); e != nil {
			stack := common.PanicTrace(4)
			log.Panicf("%v\n%s\n", e, stack)
		}
	}()
	//bytes, _ := json.Marshal(*req)
	log.Infof("SyncServer.processSync received request traceid:%s slot:%d rslot:%d user %s device %s", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID)
	if req.IsFullSync {
		s.fullSyncLoading(ctx, req)
	} else {
		s.incrementSyncLoading(ctx, req)
	}

	if req.LoadReady {
		switch req.RequestType {
		case "load":
			s.responseLoad(ctx, req, true)
		case "sync":
			if req.IsFullSync { //full sync
				s.processFullSync(ctx, req)
			} else {
				s.processIncrementSync(ctx, req)
			}
		}

	} else {
		switch req.RequestType {
		case "load":
			s.responseLoad(ctx, req, false)
		}
	}
}

func (s *SyncServer) processFullSync(ctx context.Context, req *syncapitypes.SyncServerRequest) {
	response := syncapitypes.SyncServerResponse{}
	response.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
	response.Rooms.Invite = make(map[string]syncapitypes.InviteResponse)
	response.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
	response.MaxRoomOffset = make(map[string]int64)
	response.AllLoaded = true
	receiptMaxPos := req.MaxReceiptOffset

	for _, roomInfo := range req.JoinRooms {
		resp, pos, _ := s.buildRoomJoinResp(ctx, req, roomInfo.RoomID, roomInfo.Start, roomInfo.End)
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		response.Rooms.Join[roomInfo.RoomID] = *resp
		log.Infof("SyncServer.processFullSync buildRoomJoinResp traceid:%s slot:%d rslot:%d roomID:%s maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, pos)
	}

	for _, roomInfo := range req.InviteRooms {
		resp, pos := s.buildRoomInviteResp(ctx, req, roomInfo.RoomID, req.UserID)
		response.MaxRoomOffset[roomInfo.RoomID] = pos
		response.Rooms.Invite[roomInfo.RoomID] = *resp
		log.Infof("SyncServer.processFullSync buildRoomInviteResp traceid:%s slot:%d rslot:%d roomID:%s maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, pos)
	}

	if req.IsHuman {
		s.addReceipt(ctx, req, receiptMaxPos, &response)
		s.addUnreadCount(&response, req.UserID)
	}

	s.responseSync(req, &response)
}

func (s *SyncServer) processIncrementSync(ctx context.Context, req *syncapitypes.SyncServerRequest) {
	response := syncapitypes.SyncServerResponse{}
	response.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
	response.Rooms.Invite = make(map[string]syncapitypes.InviteResponse)
	response.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
	response.MaxRoomOffset = make(map[string]int64)
	receiptMaxPos := req.MaxReceiptOffset
	response.AllLoaded = true
	newUserMap := make(map[string]bool)

	for _, roomInfo := range req.JoinRooms {
		resp, pos, users := s.buildRoomJoinResp(ctx, req, roomInfo.RoomID, roomInfo.Start, roomInfo.End)
		log.Infof("SyncServer.processIncrementSync buildRoomJoinResp traceid:%s slot:%d rslot:%d roomID:%s reqStart:%d reqEnd:%d maxPos:%d", req.TraceID, req.Slot, req.RSlot, roomInfo.RoomID, roomInfo.Start, roomInfo.End, pos)
		if (pos > 0 && roomInfo.End > pos) && req.IsHuman == true {
			// event missing
			response.AllLoaded = false
			log.Warnf("SyncServer.processIncrementSync buildRoomJoinResp not all loaded, traceid:%s room:%s", req.TraceID,  roomInfo.RoomID)
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
		resp, pos := s.buildRoomInviteResp(ctx, req, roomInfo.RoomID, req.UserID)
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
		resp, pos := s.buildRoomLeaveResp(ctx, req, roomInfo.RoomID, req.UserID, roomInfo.Start)
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
		s.addReceipt(ctx, req, receiptMaxPos, &response)
		s.addUnreadCount(&response, req.UserID)
	}

	for user := range newUserMap {
		response.NewUsers = append(response.NewUsers, user)
	}

	s.responseSync(req, &response)
}

func (s *SyncServer) responseLoad(ctx context.Context, req *syncapitypes.SyncServerRequest, ready bool) {
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

func (s *SyncServer) fullSyncLoading(ctx context.Context, req *syncapitypes.SyncServerRequest) {
	start := time.Now().UnixNano()

	s.receiptDataStreamRepo.LoadRoomLatest(ctx, req.JoinedRooms)

	for _, roomInfo := range req.JoinRooms {
		s.rsTimeline.LoadStates(ctx, roomInfo.RoomID, false)
		s.rsTimeline.LoadStreamStates(ctx, roomInfo.RoomID, false)
		s.roomHistory.LoadHistory(ctx, roomInfo.RoomID, false)
		s.roomHistory.GetRoomMinStream(ctx, roomInfo.RoomID)

		s.receiptDataStreamRepo.LoadHistory(ctx, roomInfo.RoomID, false)
		s.userReceiptRepo.LoadHistory(ctx, req.UserID, roomInfo.RoomID, false)
	}

	for _, roomInfo := range req.InviteRooms {
		s.rsTimeline.LoadStates(ctx, roomInfo.RoomID, false)
	}

	loadStart := time.Now().Unix()
	for {
		loaded := true

		for _, roomInfo := range req.JoinRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.rsTimeline.CheckStreamLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.roomHistory.CheckLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}

			if ok := s.receiptDataStreamRepo.CheckLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.userReceiptRepo.CheckLoadReady(ctx, req.UserID, roomInfo.RoomID, false); !ok {
				loaded = false
			}
		}

		for _, roomInfo := range req.InviteRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(ctx, roomInfo.RoomID, false); !ok {
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

func (s *SyncServer) incrementSyncLoading(ctx context.Context, req *syncapitypes.SyncServerRequest) {
	start := time.Now().UnixNano()

	s.roomHistory.LoadRoomLatest(ctx, req.JoinRooms)
	s.roomHistory.LoadRoomLatest(ctx, req.InviteRooms)
	s.roomHistory.LoadRoomLatest(ctx, req.LeaveRooms)
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
		s.receiptDataStreamRepo.LoadRoomLatest(ctx, req.JoinedRooms)
	}
	for _, roomInfo := range req.JoinRooms {
		lastoffset := s.roomHistory.GetRoomLastOffset(roomInfo.RoomID)
		if lastoffset >= roomInfo.Start {
			s.rsTimeline.LoadStates(ctx, roomInfo.RoomID, false)
			s.rsTimeline.LoadStreamStates(ctx, roomInfo.RoomID, false)
			s.roomHistory.LoadHistory(ctx, roomInfo.RoomID, false)
			s.roomHistory.GetRoomMinStream(ctx, roomInfo.RoomID)
		} else {
			req.LoadReady = false
			log.Warnf("SyncServer.incrementSyncLoading join load not ready, traceid:%s slot:%d rslot:%d user:%s devID:%s roomID:%s lastoffset:%d start:%d", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, roomInfo.RoomID, lastoffset, roomInfo.Start)
			return
		}
	}

	if req.IsHuman {
		for _, roomID := range req.JoinedRooms {
			if s.receiptDataStreamRepo.GetRoomLastOffset(roomID) > req.ReceiptOffset {
				s.receiptDataStreamRepo.LoadHistory(ctx, roomID, false)
				s.userReceiptRepo.LoadHistory(ctx, req.UserID, roomID, false)
			}
		}
	}

	for _, roomInfo := range req.InviteRooms {
		lastoffset := s.roomHistory.GetRoomLastOffset(roomInfo.RoomID)
		if lastoffset >= roomInfo.Start {
			s.rsTimeline.LoadStates(ctx, roomInfo.RoomID, false)
		} else {
			req.LoadReady = false
			log.Warnf("SyncServer.incrementSyncLoading invite load not ready, traceid:%s slot:%d rslot:%d user:%s devID:%s roomID:%s lastoffset:%d start:%d", req.TraceID, req.Slot, req.RSlot, req.UserID, req.DeviceID, roomInfo.RoomID, lastoffset, roomInfo.Start)
			return
		}
	}

	for _, roomInfo := range req.LeaveRooms {
		lastoffset := s.roomHistory.GetRoomLastOffset(roomInfo.RoomID)
		if lastoffset >= roomInfo.Start {
			s.rsTimeline.LoadStates(ctx, roomInfo.RoomID, false)
			s.rsTimeline.LoadStreamStates(ctx, roomInfo.RoomID, false)
			s.roomHistory.LoadHistory(ctx, roomInfo.RoomID, false)
			s.roomHistory.GetRoomMinStream(ctx, roomInfo.RoomID)
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
				if ok := s.rsTimeline.CheckStateLoadReady(ctx, roomInfo.RoomID, false); !ok {
					loaded = false
				}
				if ok := s.rsTimeline.CheckStreamLoadReady(ctx, roomInfo.RoomID, false); !ok {
					loaded = false
				}
				if ok := s.roomHistory.CheckLoadReady(ctx, roomInfo.RoomID, false); !ok {
					loaded = false
				}
			}

			if req.IsHuman {
				if s.receiptDataStreamRepo.GetRoomLastOffset(roomInfo.RoomID) > req.ReceiptOffset {
					if ok := s.receiptDataStreamRepo.CheckLoadReady(ctx, roomInfo.RoomID, false); !ok {
						loaded = false
					} else {

					}
					if ok := s.userReceiptRepo.CheckLoadReady(ctx, req.UserID, roomInfo.RoomID, false); !ok {
						loaded = false
					}
				}
			}
		}

		for _, roomInfo := range req.InviteRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}
		}

		for _, roomInfo := range req.LeaveRooms {
			if ok := s.rsTimeline.CheckStateLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.rsTimeline.CheckStreamLoadReady(ctx, roomInfo.RoomID, false); !ok {
				loaded = false
			}
			if ok := s.roomHistory.CheckLoadReady(ctx, roomInfo.RoomID, false); !ok {
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

func (s *SyncServer) addReceipt(ctx context.Context, req *syncapitypes.SyncServerRequest,
	maxPos int64, response *syncapitypes.SyncServerResponse) {
	if !s.receiptDataStreamRepo.ExistsReceipt(req.ReceiptOffset, req.UserID) {
		response.MaxReceiptOffset = req.ReceiptOffset
		log.Infof("not ExistsReceipt traceid:%s MaxReceiptOffset:%d maxPos:%d", req.TraceID,response.MaxReceiptOffset, maxPos)
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
		rdsTimeLine := s.receiptDataStreamRepo.GetHistory(ctx, roomID)
		if rdsTimeLine != nil {
			_, feedUp := rdsTimeLine.GetFeedRange()
			if feedUp > req.ReceiptOffset {
				lastOffset := s.receiptDataStreamRepo.GetRoomLastOffset(roomID)

				// full sync请求，如果当前用户最新消息未读，应返回用户上次读到的offset
				if req.IsFullSync {
					uOffset := s.userReceiptRepo.GetLatestOffset(ctx, req.UserID, roomID)
					if uOffset < lastOffset && uOffset > req.ReceiptOffset {
						selfReceipt := s.userReceiptRepo.GetLatestReceipt(ctx, req.UserID, roomID)
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
	return
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
	return
}

func (s *SyncServer) buildRoomJoinResp(ctx context.Context, req *syncapitypes.SyncServerRequest, roomID string, reqStart, reqEnd int64) (*syncapitypes.JoinResponse, int64, []string) {
	log.Infof("SyncServer.buildRoomJoinResp traceid:%s user:%s device:%s roomID:%s reqStart:%d reqEnd:%d limit:%d isFull:%t", req.TraceID, req.UserID, req.DeviceID, roomID, reqStart, reqEnd, req.Limit, req.IsFullSync)
	//再加载一遍，避免数据遗漏
	s.rsTimeline.LoadStates(ctx, roomID, true)
	s.rsTimeline.LoadStreamStates(ctx, roomID, true)
	s.roomHistory.LoadHistory(ctx, roomID, true)
	s.receiptDataStreamRepo.LoadHistory(ctx, roomID, true)
	s.userReceiptRepo.LoadHistory(ctx, req.UserID, roomID, true)
	minStream := s.roomHistory.GetRoomMinStream(ctx, roomID)

	jr := syncapitypes.NewJoinResponse()
	jr.Timeline.Limited = false
	maxPos := int64(-1)

	rs := s.rsCurState.GetRoomState(roomID)
	if rs == nil {
		log.Warnf("SyncServer.buildRoomJoinResp rsCurState.GetRoomState nil traceid:%s roomID %s user %s device %s since %d roomLatest %d", req.TraceID, roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		jr.Timeline.Limited = true
		jr.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		jr.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return jr, maxPos, []string{}
	}

	history := s.roomHistory.GetHistory(ctx, roomID)
	if history == nil {
		log.Warnf("SyncServer.buildRoomJoinResp roomHistory.GetHistory nil traceid:%s roomID %s user %s device %s since %d roomLatest %d", req.TraceID, roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		jr.Timeline.Limited = true
		jr.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		jr.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return jr, maxPos, []string{}
	}

	stateEvt := rs.GetState("m.room.member", req.UserID)
	//old join members event's event_offset < 0
	if stateEvt == nil {
		log.Warnf("SyncServer.buildRoomJoinResp rs.GetState nil traceid:%s roomID %s user %s device %s since %d roomLatest %d", req.TraceID, roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		jr.Timeline.Limited = true
		jr.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		jr.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return jr, maxPos, []string{}
	}

	maxMementEvOffset := int64(0)
	rs.GetJoinMap().Range(func(k, v interface{}) bool {
		offset := v.(int64)
		if offset > maxMementEvOffset {
			maxMementEvOffset = offset
		}
		return true
	})
	addNewUser := maxMementEvOffset > reqStart

	feeds, start, end, low, up := history.GetAllFeedsReverse()

	log.Infof("SyncServer buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s load room-timeline start:%d end:%d low:%d up:%d", req.TraceID, roomID, req.UserID, req.DeviceID, start, end, low, up)

	firstTimeLine := int64(-1)
	firstTs := int64(-1)
	limit := req.Limit
	needState := false
	msgEvent := []gomatrixserverlib.ClientEvent{}

	if stateEvt.Offset > reqStart || req.IsFullSync {
		jr.Timeline.Limited = true
		needState = true
		log.Infof("SyncServer buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s update start:%d stateEvt.Offset:%d", req.TraceID, roomID, req.UserID, req.DeviceID, reqStart, stateEvt.Offset)
		reqStart = -1
	}

	visibilityTime := s.settings.GetMessageVisilibityTime()
	nowTs := time.Now().Unix()

	evRecords := make(map[string]int)
	latestValidEvIdx := -1
	latestEvOffset := int64(0)
	isDeleted := false
	if reqStart > 0 {
		for i := len(feeds) - 1; i >= 0; i-- {
			stream := feeds[i].(*feedstypes.StreamEvent)
			if stream != nil && stream.GetOffset() <= reqStart {
				continue
			}
			if stream != nil && stream.Ev.Type == "m.room.member" && *stream.Ev.StateKey == req.UserID {
				membership, _ := stream.GetEv().Membership()
				if membership != "join" {
					latestValidEvIdx = len(feeds) - i - 1
					latestEvOffset = stream.GetOffset()
					isDeleted = stream.IsDeleted
					break
				}
			}
		}
	}
	index := 0
	if latestValidEvIdx >= 0 {
		log.Infof("SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s truncate feed datas because data has been updated, latestValidEvIdx:%d, isDeleted:%t",
			req.TraceID, roomID, req.UserID, req.DeviceID, latestValidEvIdx, isDeleted)
		index = latestValidEvIdx + 1
	}
	for i := index; i < len(feeds); i++ {
		feed := feeds[i]
		if feed != nil {
			stream := feed.(*feedstypes.StreamEvent)
			if stream.GetOffset() <= reqStart {
				log.Infof("SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s break offset:%d", req.TraceID, roomID, req.UserID, req.DeviceID, stream.GetOffset())
				break
			}

			if stream.GetOffset() <= reqEnd || reqEnd == -1 {
				if firstTimeLine == -1 {
					firstTimeLine = stream.GetOffset()
					firstTs = int64(stream.Ev.OriginServerTS)
				}

				if firstTimeLine > stream.GetOffset() {
					firstTimeLine = stream.Offset
					firstTs = int64(stream.Ev.OriginServerTS)
				}

				if stream.Offset > maxPos {
					maxPos = stream.Offset
				}

				isStateEv := common.IsStateClientEv(stream.GetEv())
				if isStateEv || (!isStateEv && rs.CheckEventVisibility(req.UserID, int64(stream.Ev.OriginServerTS))) {
					skipEv := false
					if visibilityTime > 0 {
						ts := int64(stream.Ev.OriginServerTS) / 1000
						if ts+visibilityTime < nowTs && !isStateEv {
							log.Infof("traceid:%s buildJoinRoomResp skip event %s, ts: %d", req.TraceID, stream.Ev.EventID, stream.Ev.OriginServerTS)
							skipEv = true
						}
					}
					if !skipEv {
						// dereplication
						if idx, ok := evRecords[stream.GetEv().EventID]; !ok {
							msgEvent = append(msgEvent, *stream.GetEv())
							limit = limit - 1
							evRecords[stream.GetEv().EventID] = len(msgEvent) - 1
						} else {
							log.Warnf("SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s found replicate events, eventID:%s, eventNID: %d, index: %d",
								req.TraceID, roomID, req.UserID, req.DeviceID, stream.GetEv().EventID, stream.GetEv().EventNID, idx)
						}
					}
				}
			}

			if limit == 0 || stream.GetEv().Type == "m.room.create" || stream.Offset <= minStream {
				if stream.GetEv().Type != "m.room.create" {
					jr.Timeline.Limited = true
				} else {
					jr.Timeline.Limited = false
				}
				log.Infof("SyncServer.buildRoomJoinResp traceid:%s break roomID:%s user:%s device:%s limit:%d eventType:%s stream.Offset:%d minStream:%d", req.TraceID, roomID, req.UserID, req.DeviceID, limit, stream.GetEv().Type, stream.Offset, minStream)
				break
			}

		} else {
			log.Errorf("SyncServer.buildRoomJoinResp traceid:%s roomID:%s user:%s device:%s get feed nil offset %d", req.TraceID, roomID, req.UserID, req.DeviceID, end-i)
			history.Console()
		}
	}

	for i := 0; i < len(msgEvent)/2; i++ {
		tmp := msgEvent[i]
		msgEvent[i] = msgEvent[len(msgEvent)-i-1]
		msgEvent[len(msgEvent)-i-1] = tmp
	}
	log.Infof("SyncServer.buildRoomJoinResp traceid:%s buildJoinRoomResp user:%s room:%s firsttimeLine:%d get msgevent len:%d", req.TraceID, req.UserID, roomID, firstTimeLine, len(msgEvent))
	var stateEvent []gomatrixserverlib.ClientEvent
	stateEvent = []gomatrixserverlib.ClientEvent{}
	states := s.rsTimeline.GetStates(ctx, roomID)

	if states != nil {
		realEnd := reqEnd
		if reqEnd != -1 && latestEvOffset != 0 && latestEvOffset-1 < reqEnd {
			realEnd = latestEvOffset - 1
		}
		statesMap := make(map[string]StateEvWithPrio)
		feeds, start, end, low, up := states.GetAllFeeds()
		log.Infof("SyncServer buildRoomJoinResp traceid:%s user:%s device:%s load state-timeline roomID:%s start:%d end:%d low:%d up:%d, realEnd: %d", req.TraceID, req.UserID, req.DeviceID, roomID, start, end, low, up, realEnd)
		for i, feed := range feeds { //填充state
			if feed == nil {
				continue
			}

			stream := feed.(*feedstypes.StreamEvent)
			// if stream.IsDeleted {
			// 	continue
			// }

			if stream.GetOffset() <= realEnd || realEnd == -1 {
				ev := stream.GetEv()
				if stream.GetOffset() >= firstTimeLine && firstTimeLine > 0 {
					break
				}

				if stream.Offset > maxPos {
					maxPos = stream.Offset
				}

				if needState {
					pushStateIntoMap(statesMap, ev, i)
				} else {
					if stream.GetOffset() > reqStart {
						pushStateIntoMap(statesMap, ev, i)
					}
				}
			}
		}
		stateEvent = s.aggregateStates(statesMap)
	} else {
		log.Errorf("SyncServer.buildRoomJoinResp rsTimeline.GetStates nil traceid:%s roomID %s user %s device %s since %d roomLatest %d", req.TraceID, roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		jr.Timeline.Limited = true
		jr.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		jr.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return jr, maxPos, []string{}
	}
	jr.State.Events = stateEvent

	var users []string
	if addNewUser || needState {
		if req.IsHuman {
			joinMap := s.rsCurState.GetRoomState(roomID).GetJoinMap()
			if joinMap != nil {
				joinMap.Range(func(key, value interface{}) bool {
					users = append(users, key.(string))
					return true
				})
			}
		}
	}

	if firstTimeLine == -1 {
		firstTimeLine = math.MaxInt64
		firstTs = math.MaxInt64
		jr.Timeline.Limited = true
		msgEvent = []gomatrixserverlib.ClientEvent{}
	} else {
		if firstTimeLine > reqStart && reqStart > 0 && len(msgEvent) >= req.Limit {
			jr.Timeline.Limited = true
		}

		// when we call get_messages with param "from"=prev_batch, it will start from prev_batch,
		// but not from (prev_batch-1), so we set (firstTimeLine-1) to avoid getting repeat message
		firstTimeLine = firstTimeLine - 1
		if firstTimeLine < 0 {
			firstTimeLine = 0
		}
	}
	jr.Timeline.PrevBatch = common.BuildPreBatch(firstTimeLine, firstTs)
	jr.Timeline.Events = msgEvent
	return jr, maxPos, users
}

func (s *SyncServer) buildRoomInviteResp(ctx context.Context,
	req *syncapitypes.SyncServerRequest, roomID, user string) (*syncapitypes.InviteResponse, int64) {
	ir := syncapitypes.NewInviteResponse()
	maxPos := int64(-1)
	s.rsTimeline.LoadStates(ctx, roomID, true)

	states := s.rsTimeline.GetStates(ctx, roomID)
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
func (s *SyncServer) buildRoomLeaveResp(ctx context.Context, req *syncapitypes.SyncServerRequest, roomID, user string, reqStart int64) (*syncapitypes.LeaveResponse, int64) {
	lv := syncapitypes.NewLeaveResponse()
	maxPos := int64(-1)
	s.rsTimeline.LoadStates(ctx, roomID, true)
	s.rsTimeline.LoadStreamStates(ctx, roomID, true)
	s.roomHistory.LoadHistory(ctx, roomID, true)

	rs := s.rsCurState.GetRoomState(roomID)
	if rs == nil {
		log.Errorf("SyncServer.buildRoomLeaveResp rsCurState.GetRoomState nil roomID %s user %s device %s since %d roomLatest %d", roomID, req.UserID, req.DeviceID, reqStart, s.roomHistory.GetRoomLastOffset(roomID))
		lv.Timeline.Limited = true
		lv.Timeline.PrevBatch = common.BuildPreBatch(math.MaxInt64, math.MaxInt64)
		lv.Timeline.Events = []gomatrixserverlib.ClientEvent{}
		return lv, maxPos
	}

	history := s.roomHistory.GetHistory(ctx, roomID)
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
	minStream := s.roomHistory.GetRoomMinStream(ctx, roomID)

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
					log.Infof("SyncServer.buildRoomLeaveResp traceid:%s break roomID:%s user:%s device:%s limit:%s eventType:%s stream.Offset:%d minStream:%d", req.TraceID, roomID, req.UserID, req.DeviceID, stream.Offset, minStream)
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

func pushStateIntoMap(states map[string]StateEvWithPrio, ev *gomatrixserverlib.ClientEvent, prio int) {
	key := ev.Type
	if ev.Type == "m.room.member" {
		key = ev.Type + *ev.StateKey
	}
	states[key] = StateEvWithPrio{prio, ev}
}

type StateEvWithPrio struct {
	prio int
	ev   *gomatrixserverlib.ClientEvent
}

type StateEvWithPrioSorter []StateEvWithPrio

func (s StateEvWithPrioSorter) Len() int           { return len(s) }
func (s StateEvWithPrioSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s StateEvWithPrioSorter) Less(i, j int) bool { return s[i].prio < s[j].prio }

func (s *SyncServer) aggregateStates(states map[string]StateEvWithPrio) []gomatrixserverlib.ClientEvent {
	sorter := make(StateEvWithPrioSorter, 0, len(states))
	for _, v := range states {
		sorter = append(sorter, v)
	}
	if len(sorter) == 0 {
		return []gomatrixserverlib.ClientEvent{}
	}

	sort.Sort(sorter)
	ret := make([]gomatrixserverlib.ClientEvent, 0, len(sorter))
	for _, v := range sorter {
		ret = append(ret, *v.ev)
	}

	return ret
}
