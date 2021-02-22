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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/feedstypes"
	push "github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/pushapi/routing"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/syncaggregate/consumers"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type SyncMng struct {
	db           model.SyncAPIDatabase
	slot         uint32
	chanSize     int
	msgChan      []chan *request
	cfg          *config.Dendrite
	rpcClient    *common.RpcClient
	cache        service.Cache
	complexCache *common.ComplexCache
	syncOffset   sync.Map //traceId, UserRoomOffset
	//repos
	onlineRepo           *repos.OnlineUserRepo
	userTimeLine         *repos.UserTimeLineRepo
	typingConsumer       *consumers.TypingConsumer
	clientDataStreamRepo *repos.ClientDataStreamRepo
	keyChangeRepo        *repos.KeyChangeStreamRepo
	stdEventStreamRepo   *repos.STDEventStreamRepo
	presenceStreamRepo   *repos.PresenceDataStreamRepo
	userDeviceActiveRepo *repos.UserDeviceActiveRepo
}

func NewSyncMng(
	db model.SyncAPIDatabase,
	slot uint32,
	chanSize int,
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
) *SyncMng {
	mng := new(SyncMng)
	mng.db = db
	mng.slot = slot
	mng.chanSize = chanSize
	mng.cfg = cfg
	mng.rpcClient = rpcClient
	return mng
}

func (sm *SyncMng) SetCache(cache service.Cache) *SyncMng {
	sm.cache = cache
	return sm
}

func (sm *SyncMng) SetComplexCache(complexCache *common.ComplexCache) *SyncMng {
	sm.complexCache = complexCache
	return sm
}

func (sm *SyncMng) SetOnlineRepo(onlineRepo *repos.OnlineUserRepo) *SyncMng {
	sm.onlineRepo = onlineRepo
	sm.onlineRepo.SetHandler(sm)
	return sm
}

func (sm *SyncMng) GetOnlineRepo() *repos.OnlineUserRepo {
	return sm.onlineRepo
}

func (sm *SyncMng) OnStateChange(state *types.NotifyDeviceState) {
	if !sm.cfg.StateMgr.StateNotify {
		log.Warnln("not open state notify cfg")
		return
	}
	state.DeviceID = common.GetDeviceMac(state.DeviceID)
	state.Pushkeys = sm.GetPushkeyByUserDeviceID(state.UserID, state.DeviceID)
	sm.sendStateChange(state)
}

func (sm *SyncMng) sendStateChange(state *types.NotifyDeviceState) {
	err := common.GetTransportMultiplexer().SendWithRetry(
		sm.cfg.Kafka.Producer.DeviceStateUpdate.Underlying,
		sm.cfg.Kafka.Producer.DeviceStateUpdate.Name,
		&core.TransportPubMsg{
			Keys: []byte(state.UserID),
			Obj:  *state,
		})
	if err != nil {
		log.Errorf("OnStateChange publish kafka topic:%s err:%v userID:%s,deviceID:%s", sm.cfg.Kafka.Producer.DeviceStateUpdate.Topic, err, state.UserID, state.DeviceID)
	} else {
		log.Infof("OnStateChange publish kafka topic:%s succ userID:%s,deviceID:%s,laststate:%d,curstate:%d,pushkeys:%v", sm.cfg.Kafka.Producer.DeviceStateUpdate.Topic, state.UserID, state.DeviceID, state.LastState, state.CurState, state.Pushkeys)
	}
}

// other also use this topic, maintain notify data send to kafka same to single notify
func (sm *SyncMng) OnBatchStateChange(batch []*types.NotifyDeviceState) {
	if !sm.cfg.StateMgr.StateNotify {
		log.Warnln("not open state notify cfg")
		return
	}
	log.Infof("cron notify online device len:%d", len(batch))
	go func(batch []*types.NotifyDeviceState) {
		for _, state := range batch {
			state.DeviceID = common.GetDeviceMac(state.DeviceID)
			sm.sendStateChange(state)
			time.Sleep(time.Duration(20) * time.Millisecond)
		}
	}(batch)
}

func (sm *SyncMng) OnUserOnlineDetail(batch []*types.NotifyOnlineDetail) {
	log.Infof("cron notify online detail len:%d", len(batch))
	data := new(types.StaticItem)
	data.Type = types.STATIC_ONLINE
	onlineInfo := &types.StaticOnlineItem{
		Detail: batch,
	}
	data.Online = onlineInfo
	common.GetTransportMultiplexer().SendWithRetry(
		sm.cfg.Kafka.Producer.OutputStatic.Underlying,
		sm.cfg.Kafka.Producer.OutputStatic.Name,
		&core.TransportPubMsg{
			Keys: []byte{},
			Obj:  data,
		})
}

func (sm *SyncMng) OnUserStateChange(state *types.NotifyUserState) {
	sm.stateChangePresent(state)
}

func (sm *SyncMng) stateChangePresent(state *types.NotifyUserState) {
	if state.LastState == state.CurState {
		//not go here
		log.Warnf("stateChangePresent not change userID:%s,laststate:%d,curstate:%d", state.UserID, state.LastState, state.CurState)
		return
	}
	presence := ""
	presenceContent := types.PresenceShowJSON{}
	presencCache, ok := sm.cache.GetPresences(state.UserID)
	feed := sm.presenceStreamRepo.GetHistoryByUserID(state.UserID)
	if ok && presencCache.UserID == state.UserID {
		presenceContent.UserID = state.UserID
		presenceContent.Presence = presencCache.Status
		presenceContent.StatusMsg = presencCache.StatusMsg
		presenceContent.ExtStatusMsg = presencCache.ExtStatusMsg
	} else {
		if feed != nil && feed.GetDataStream().UserID == state.UserID {
			var presenceEvent gomatrixserverlib.ClientEvent
			err := json.Unmarshal(feed.DataStream.Content, &presenceEvent)
			if err == nil {
				content := types.PresenceShowJSON{}
				err = json.Unmarshal(presenceEvent.Content, &content)
				if err == nil {
					presenceContent = content
				}
			}
		}
	}
	if state.CurState == repos.USER_OFFLINE_STATE {
		presence = "offline"
	} else {
		presence = "online"
		if feed != nil && presenceContent.Presence != "offline" {
			presence = presenceContent.Presence
		}
	}
	if presencCache != nil {
		log.Infof("stateChangePresent succ userID:%s,laststate:%d,curstate:%d, cache: userID:%s presence:%s statusMsg:%s extStatusMsg:%s, feed: userID:%s presence:%s statusMsg:%s extStatusMsg:%s", state.UserID, state.LastState, state.CurState, presencCache.UserID, presencCache.Status, presencCache.StatusMsg, presencCache.ExtStatusMsg, presenceContent.UserID, presenceContent.Presence, presenceContent.StatusMsg, presenceContent.ExtStatusMsg)
	} else {
		log.Infof("stateChangePresent succ userID:%s,laststate:%d,curstate:%d, feed: userID:%s presence:%s statusMsg:%s extStatusMsg:%s", state.UserID, state.LastState, state.CurState, presenceContent.UserID, presenceContent.Presence, presenceContent.StatusMsg, presenceContent.ExtStatusMsg)
	}
	statusMsg := presenceContent.StatusMsg
	extStatusMsg := presenceContent.ExtStatusMsg
	sm.cache.SetPresences(state.UserID, presence, statusMsg, extStatusMsg)
	sm.cache.SetPresencesServerStatus(state.UserID, presence)
	displayName, avatarURL, _ := sm.complexCache.GetProfileByUserID(state.UserID)
	user_info := sm.cache.GetUserInfoByUserID(state.UserID)
	currentlyActive := false
	if presence == "online" {
		currentlyActive = true
	}
	content := types.PresenceJSON{
		Presence:        presence,
		StatusMsg:       statusMsg,
		ExtStatusMsg:    extStatusMsg,
		CurrentlyActive: currentlyActive,
		UserID:          state.UserID,
		LastActiveAgo:   0,
	}

	content.AvatarURL = avatarURL
	content.DisplayName = displayName
	if user_info != nil {
		content.UserName = user_info.UserName
		content.JobNumber = user_info.JobNumber
		content.Mobile = user_info.Mobile
		content.Landline = user_info.Landline
		content.Email = user_info.Email
		content.State = user_info.State
	}
	data := new(types.ProfileStreamUpdate)
	data.UserID = state.UserID
	data.Presence = content
	log.Infof("state change presence user:%s ", state.UserID)
	common.GetTransportMultiplexer().SendWithRetry(
		sm.cfg.Kafka.Producer.OutputProfileData.Underlying,
		sm.cfg.Kafka.Producer.OutputProfileData.Name,
		&core.TransportPubMsg{
			Keys: []byte(state.UserID),
			Obj:  data,
		})
}

func (sm *SyncMng) GetPushkeyByUserDeviceID(userID, deviceID string) []types.PushKeyContent {
	pushkeys := []types.PushKeyContent{}
	resp := sm.rpcGetPushData(userID, deviceID, types.GET_PUSHER_BY_DEVICE)
	if resp.Error != "" {
		log.Error("GetPushkeyByUserDeviceID user:%s device:%s error:%s", userID, deviceID, resp.Error)
		return pushkeys
	}
	pushers := push.Pushers{}
	err := json.Unmarshal(resp.Payload, &pushers)
	if err != nil {
		log.Error("GetPushkeyByUserDeviceID user:%s device:%s json.Unmarshal error:%v", userID, deviceID, err)
		return pushkeys
	}
	for _, pusher := range pushers.Pushers {
		if pusher.DeviceID != deviceID {
			continue
		}
		// ios and not set push_channel not notify
		if v, ok := (pusher.Data).(map[string]interface{}); ok {
			var data map[string]interface{}
			data = v
			if v, ok := data["push_channel"]; ok {
				if v.(string) == "ios" {
					continue
				}
			} else {
				continue
			}
		} else {
			continue
		}
		pushkeys = append(pushkeys, types.PushKeyContent{
			PushKey: pusher.PushKey,
			AppID:   pusher.AppId,
		})
	}
	return pushkeys
}

func (sm *SyncMng) SetUserTimeLine(userTimeLine *repos.UserTimeLineRepo) *SyncMng {
	sm.userTimeLine = userTimeLine
	return sm
}

func (sm *SyncMng) SetTypingConsumer(typingConsumer *consumers.TypingConsumer) *SyncMng {
	sm.typingConsumer = typingConsumer
	return sm
}

func (sm *SyncMng) SetClientDataStreamRepo(clientDataStreamRepo *repos.ClientDataStreamRepo) *SyncMng {
	sm.clientDataStreamRepo = clientDataStreamRepo
	return sm
}

func (sm *SyncMng) SetKeyChangeRepo(kChangeRepo *repos.KeyChangeStreamRepo) *SyncMng {
	sm.keyChangeRepo = kChangeRepo
	return sm
}

func (sm *SyncMng) SetStdEventStreamRepo(stdEventRepo *repos.STDEventStreamRepo) *SyncMng {
	sm.stdEventStreamRepo = stdEventRepo
	return sm
}

func (sm *SyncMng) SetPresenceStreamRepo(presenceStreamRepo *repos.PresenceDataStreamRepo) *SyncMng {
	sm.presenceStreamRepo = presenceStreamRepo
	return sm
}

func (sm *SyncMng) SetUserDeviceActiveTsRepo(userDeviceActiveTsRepo *repos.UserDeviceActiveRepo) *SyncMng {
	sm.userDeviceActiveRepo = userDeviceActiveTsRepo
	return sm
}

func (sm *SyncMng) Start() {
	sm.msgChan = make([]chan *request, sm.slot)
	for i := uint32(0); i < sm.slot; i++ {
		sm.msgChan[i] = make(chan *request, sm.chanSize)
		go sm.startWorker(sm.msgChan[i])
	}
}

func (sm *SyncMng) startWorker(channel chan *request) {
	for msg := range channel {
		sm.processSyncLoad(msg)
	}
}

func (sm *SyncMng) dispatch(uid string, req *request) {
	hash := common.CalcStringHashCode(uid)
	req.slot = hash % sm.slot
	log.Infof("traceid:%s dispatch slot:%d len:%d", req.traceId, req.slot, len(sm.msgChan[req.slot]))
	sm.msgChan[req.slot] <- req
}

func (sm *SyncMng) reBuildIncreamSyncReqRoom(req *request) {
	log.Infof("traceid:%s begin reBuildIncreamSyncReqRoom", req.traceId)
	joinRooms, err := sm.userTimeLine.GetJoinRooms(req.device.UserID)
	if err != nil {
		log.Warnf("traceid:%s reBuildIncreamSyncReqRoom.GetJoinRooms err:%v", req.traceId, err)
		return
	}
	inviteRooms, err := sm.userTimeLine.GetInviteRooms(req.device.UserID)
	if err != nil {
		log.Warnf("traceid:%s reBuildIncreamSyncReqRoom.GetInviteRooms err:%v", req.traceId, err)
		return
	}
	leaveRooms, err := sm.userTimeLine.GetLeaveRooms(req.device.UserID)
	if err != nil {
		log.Warnf("traceid:%s reBuildIncreamSyncReqRoom.GetLeaveRooms err:%v", req.traceId, err)
		return
	}
	//rebuild
	req.reqRooms = sync.Map{}
	req.joinRooms = []string{}
	joinRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "join")
		joinOffset := sm.userTimeLine.GetJoinMembershipOffset(req.device.UserID, roomID)
		req.joinRooms = append(req.joinRooms, roomID)
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset && joinOffset > 0 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID, "join", "rebuild"))
			}
		} else {
			if joinOffset > 0 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, latestOffset, roomID, "join", "rebuild"))
			}
		}
		return true
	})
	inviteRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "invite")
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID, "invite", "rebuild"))
			}
		} else {
			req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, -1, latestOffset, roomID, "invite", "rebuild"))
		}
		return true
	})
	leaveRooms.Range(func(key, value interface{}) bool {
		roomID := key.(string)
		latestOffset := sm.userTimeLine.GetRoomOffset(roomID, req.device.UserID, "leave")
		if offset, ok := req.offsets[roomID]; ok {
			if offset < latestOffset {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, offset, latestOffset, roomID, "leave", "rebuild"))
			}
		} else {
			//token has not offset leave room can get only the leave room msg
			if latestOffset != -1 {
				req.reqRooms.Store(roomID, sm.buildReqRoom(req.traceId, latestOffset-1, latestOffset, roomID, "leave", "rebuild"))
			}
		}
		return true
	})
}

func (sm *SyncMng) buildSyncData(req *request, res *syncapitypes.Response) bool {
	//bot not build full sync data
	if sm.isFullSync(req) && req.device.IsHuman == false {
		_, err := sm.userTimeLine.GetJoinRooms(req.device.UserID)
		if err != nil {
			return false
		}
		_, err = sm.userTimeLine.GetInviteRooms(req.device.UserID)
		if err != nil {
			return false
		}
		sm.updateFullSyncNotData(req)
		return true
	}
	if !sm.isFullSync(req) {
		//rebuild reqroom for reduce empty incr sync
		sm.reBuildIncreamSyncReqRoom(req)
	}
	requestMap := make(map[uint32]*syncapitypes.SyncServerRequest)
	maxReceiptOffset := sm.userTimeLine.GetUserLatestReceiptOffset(req.device.UserID, req.device.IsHuman)
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
	bs := time.Now().UnixNano() / 1000000
	log.Infof("SyncMng.buildSyncData remote sync request start traceid:%s slot:%d user:%s device:%s utl:%d joins:%d maxReceiptOffset:%d", req.traceId, req.slot, req.device.UserID, req.device.ID, req.marks.utlRecv, len(req.joinRooms), maxReceiptOffset)
	var wg sync.WaitGroup
	for instance, syncReq := range requestMap {
		wg.Add(1)
		go func(
			instance uint32,
			syncReq *syncapitypes.SyncServerRequest,
			req *request,
			maxReceiptOffset int64,
			res *syncapitypes.Response,
		) {
			defer wg.Done()
			syncReq.RequestType = "sync"
			syncReq.UserID = req.device.UserID
			syncReq.DeviceID = req.device.ID
			syncReq.IsHuman = req.device.IsHuman
			syncReq.Limit = req.limit
			syncReq.ReceiptOffset = req.marks.recpRecv
			syncReq.MaxReceiptOffset = maxReceiptOffset
			syncReq.SyncInstance = instance
			syncReq.IsFullSync = req.isFullSync
			syncReq.TraceID = req.traceId
			syncReq.Slot = req.slot
			bytes, err := json.Marshal(*syncReq)
			if err == nil {
				//log.Infof("SyncMng.buildSyncData sync traceid:%s slot:%d user %s device %s request %s", req.traceId,req.slot, req.device.UserID, req.device.ID, string(bytes))
				data, err := sm.rpcClient.Request(types.SyncServerTopicDef, bytes, 35000)

				spend := time.Now().UnixNano()/1000000 - bs
				if err == nil {
					var result types.CompressContent
					err = json.Unmarshal(data, &result)
					if err != nil {
						log.Errorf("SyncMng.buildSyncData response traceid:%s slot:%d spend:%d ms user:%s, device:%s, Unmarshal error %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, err)
						syncReq.SyncReady = false
					} else {
						if result.Compressed {
							result.Content = common.DoUnCompress(result.Content)
						}
						var response syncapitypes.SyncServerResponse
						err = json.Unmarshal(result.Content, &response)
						if err != nil {
							log.Errorf("SyncMng.buildSyncData SyncServerResponse response traceid:%s slot:%d spend:%d ms user:%s, device:%s Unmarshal error %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, err)
							syncReq.SyncReady = false
						} else {
							log.Infof("SyncMng.buildSyncData traceid:%s slot:%d spend:%d ms user %s device %s instance %d MaxReceiptOffset:%d response %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, instance, maxReceiptOffset, response.AllLoaded)
							if response.AllLoaded {
								syncReq.SyncReady = true
								sm.addSyncData(req, res, &response)
							} else {
								syncReq.SyncReady = false
							}
						}
					}
				} else {
					log.Errorf("SyncMng.buildSyncData call rpc for syncServer sync traceid:%s slot:%d spend:%d ms user %s device %s error %v", req.traceId, req.slot, spend, req.device.UserID, req.device.ID, err)
					syncReq.SyncReady = false
				}
			} else {
				log.Errorf("SyncMng.buildSyncData marshal callSyncLoad content error,traceid:%s slot:%d spend:%d ms device %s user %s error %v", req.traceId, req.slot, req.device.ID, req.device.UserID, err)
				syncReq.SyncReady = false
			}
		}(instance, syncReq, req, maxReceiptOffset, res)
	}
	wg.Wait()
	es := time.Now().UnixNano() / 1000000
	log.Infof("SyncMng.buildSyncData remote sync request end traceid:%s slot:%d user:%s device:%s spend:%d ms", req.traceId, req.slot, req.device.UserID, req.device.ID, es-bs)
	finished := true
	for _, syncReq := range requestMap {
		if syncReq.SyncReady == false {
			finished = false
		}
	}
	if finished {
		if res.Rooms.Join == nil {
			res.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
		}

		if res.Rooms.Invite == nil {
			res.Rooms.Invite = make(map[string]syncapitypes.InviteResponse)
		}

		if res.Rooms.Leave == nil {
			res.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
		}
		if req.marks.preProcess == 0 {
			req.marks.preProcess = 1
		}
		if req.marks.kcProcess == 0 {
			req.marks.kcProcess = 1
		}
		sm.freshToken(req, res)
		log.Infof("SyncMng.buildSyncData update utl request end traceid:%s slot:%d user:%s device:%s finish:%t", req.traceId, req.slot, req.device.UserID, req.device.ID, finished)
		return true
	} else {
		log.Warnf("SyncMng.buildSyncData update utl request end traceid:%s slot:%d user:%s device:%s finish:%t", req.traceId, req.slot, req.device.UserID, req.device.ID, finished)
		return false
	}
}

func (sm *SyncMng) addSendToDevice(req *request, response *syncapitypes.Response) {
	stdTimeLine := sm.stdEventStreamRepo.GetHistory(req.device.UserID, req.device.ID)
	if stdTimeLine == nil {
		return
	}

	_, feedUp := stdTimeLine.GetFeedRange()
	maxPos := int64(-1)

	if req.marks.stdRecv == 0 {
		//对于full sync，意味着device已重新生成密钥信息，原来的std信息已不能解密
		if feedUp > 0 && feedUp > maxPos {
			maxPos = feedUp
		}

		err := sm.db.DeleteDeviceStdMessage(req.ctx, req.device.UserID, req.device.ID)
		if err != nil {
			log.Errorf("addSendToDevice: delete all history std message error traceid:%s user:%s dev:%s err:%v", req.traceId, req.device.UserID, req.device.ID, err)
			return
		}

		if maxPos == -1 {
			maxPos = 1
		}
	} else {
		if feedUp <= req.marks.stdRecv {
			response.ToDevice.StdEvent = []types.StdEvent{}
			return
		}

		err := sm.db.DeleteStdMessage(req.ctx, req.marks.stdRecv, req.device.UserID, req.device.ID)
		if err != nil {
			log.Errorf("addSendToDevice: delete history std message error traceid:%s user:%s dev:%s err:%v", req.traceId, req.device.UserID, req.device.ID, err)
		}

		var feeds []feedstypes.Feed
		stdTimeLine.ForRange(func(offset int, feed feedstypes.Feed) bool {
			if feed == nil {
				log.Errorf("SyncMng.addSendToDevice traceid:%s user:%s dev:%s get feed nil offset %d", req.traceId, req.device.UserID, req.device.ID, offset)
				stdTimeLine.Console()
			} else {
				feeds = append(feeds, feed)
			}
			return true
		})
		for _, feed := range feeds {
			if feed != nil {
				stream := feed.(*feedstypes.STDEventStream)
				if stream.GetOffset() > req.marks.stdRecv {
					if stream.Read == false {
						response.ToDevice.StdEvent = append(response.ToDevice.StdEvent, *stream.DataStream)

					}
					if maxPos < stream.GetOffset() {
						maxPos = stream.GetOffset()
					}
				} else {
					if stream.Read == false {
						stream.Read = true
					}
				}
			}
		}

		if len(response.ToDevice.StdEvent) == 0 {
			response.ToDevice.StdEvent = []types.StdEvent{}
		}
	}

	if maxPos == -1 {
		maxPos = req.marks.stdRecv
	}

	req.marks.stdProcess = maxPos
	//deviceBytes, _ := json.Marshal(response.ToDevice)
	//log.Errorf("SyncMng addSendToDevice response user:%s, dev:%s, events:%s", req.device.UserID, req.device.ID, string(deviceBytes))

	return
}

func (sm *SyncMng) addAccountData(req *request, response *syncapitypes.Response) *syncapitypes.Response {
	if req.marks.accRecv == 0 {
		return sm.addFullAccountData(req, response)
	}
	return sm.addIncrementalAccountData(req, response)
}

func (sm *SyncMng) addIncrementalAccountData(req *request, response *syncapitypes.Response) *syncapitypes.Response {
	userID := req.device.UserID
	cdsTimeLine := sm.clientDataStreamRepo.GetHistory(userID)
	if cdsTimeLine == nil {
		log.Errorf("SyncMng.addIncrementalAccountData get client data stream nil traceid:%s user:%s device:%s", req.traceId, userID, req.device.ID)
		return response
	}

	_, feedUp := cdsTimeLine.GetFeedRange()
	if feedUp <= req.marks.accRecv {
		return response
	}

	pushRuleChanged := false
	roomTagMap := make(map[string]bool)
	accountDataMap := make(map[string]bool)
	roomAccountDataMap := make(map[string]bool)

	maxPos := int64(-1)

	var feeds []feedstypes.Feed
	cdsTimeLine.ForRange(func(offset int, feed feedstypes.Feed) bool {
		if feed == nil {
			log.Errorf("SyncMng.addIncrementalAccountData traceid:%s user:%s device:%s get feed nil offset %d", req.traceId, req.device.UserID, req.device.ID, offset)
			cdsTimeLine.Console()
		} else {
			feeds = append(feeds, feed)
		}
		return true
	})
	for _, feed := range feeds {
		if feed != nil {
			stream := feed.(*feedstypes.ClientDataStream)

			if stream.GetOffset() > req.marks.accRecv {
				if maxPos < stream.GetOffset() {
					maxPos = stream.GetOffset()
				}

				cds := stream.GetDataStream()
				switch cds.StreamType {
				case "roomTag":
					roomTagMap[cds.RoomID] = true
				case "accountData":
					changeKey := fmt.Sprintf("%s:%s:%s", "account_data", userID, cds.DataType)
					accountDataMap[changeKey] = true
				case "roomAccountData":
					changeKey := fmt.Sprintf("%s:%s:%s:%s", "room_account_data", userID, cds.RoomID, cds.DataType)
					roomAccountDataMap[changeKey] = true
				case "pushRule":
					pushRuleChanged = true
				}
			}
		}
	}

	allTagIDs, _ := sm.cache.GetUserRoomTagIds(userID)

	var tagIDs []string
	var missRooms []string
	for changeKey := range roomTagMap {
		tagKey := fmt.Sprintf("%s:%s:%s", "room_tags", userID, changeKey)
		miss := true
		for _, tagID := range allTagIDs {
			if strings.HasPrefix(tagID, tagKey) {
				tagIDs = append(tagIDs, tagID)
				miss = false
			}
		}
		if miss {
			missRooms = append(missRooms, changeKey)
		}
	}

	if len(tagIDs) > 0 {
		response = sm.addRoomTags(req, response, tagIDs)
	}
	if len(missRooms) > 0 {
		response = sm.addRoomEmptyTags(req, response, missRooms)
	}

	var accountKeys []string
	for changeKey := range accountDataMap {
		accountKeys = append(accountKeys, changeKey)
	}
	if len(accountKeys) > 0 {
		response = sm.addClientAccountData(req, response, accountKeys)
	}

	var roomAccountKeys []string
	for changeKey := range roomAccountDataMap {
		roomAccountKeys = append(roomAccountKeys, changeKey)
	}
	if len(roomAccountKeys) > 0 {
		response = sm.addRoomAccountData(req, response, roomAccountKeys)
	}

	if pushRuleChanged {
		response = sm.addPushRules(req, response)
	}

	if maxPos == -1 {
		maxPos = req.marks.accRecv
	}

	req.marks.accProcess = maxPos

	//joinBytes, _ := json.Marshal(response.AccountData)
	//log.Errorf("SyncMng addIncrementalAccountData response user:%s, dev:%s, start:%d, end:%d since:%d return:%d, events:%s", req.device.UserID, req.device.ID, start, end, req.marks.accRecv, maxPos, string(joinBytes))

	return response
}

func (sm *SyncMng) addFullAccountData(req *request, response *syncapitypes.Response) *syncapitypes.Response {
	userID := req.device.UserID
	response = sm.addPushRules(req, response)

	if allTagIDs, ok := sm.cache.GetUserRoomTagIds(userID); ok {
		response = sm.addRoomTags(req, response, allTagIDs)
	}

	if clientActIDs, ok := sm.cache.GetUserAccountDataIds(userID); ok {
		response = sm.addClientAccountData(req, response, clientActIDs)
	}

	if roomActIDs, ok := sm.cache.GetUserRoomAccountDataIds(userID); ok {
		response = sm.addRoomAccountData(req, response, roomActIDs)
	}

	cdsTimeLine := sm.clientDataStreamRepo.GetHistory(userID)
	if cdsTimeLine == nil {
		return response
	}

	_, feedUp := cdsTimeLine.GetFeedRange()
	maxPos := req.marks.accRecv
	if feedUp > 0 && feedUp > maxPos {
		maxPos = feedUp
	}

	if maxPos == 0 {
		//避免持续full sync account data
		maxPos = 1
	}
	req.marks.accProcess = maxPos

	return response
}

func (sm *SyncMng) addClientAccountData(req *request, response *syncapitypes.Response, accountIDs []string) *syncapitypes.Response {
	for _, accountID := range accountIDs {
		actData, _ := sm.cache.GetAccountDataCacheData(accountID)
		event := gomatrixserverlib.ClientEvent{
			Type:    actData.Type,
			Content: []byte(actData.Content),
		}
		response.AccountData.Events = append(response.AccountData.Events, event)
	}
	return response
}

func (sm *SyncMng) addRoomAccountData(req *request, response *syncapitypes.Response, accountIDs []string) *syncapitypes.Response {
	joinRooms, err := sm.userTimeLine.GetJoinRooms(req.device.UserID)
	if err != nil {
		return response
	}
	for _, accountID := range accountIDs {
		actData, _ := sm.cache.GetRoomAccountDataCacheData(accountID)
		if _, ok := joinRooms.Load(actData.RoomID); !ok {
			continue
		}
		event := gomatrixserverlib.ClientEvent{
			Type:    actData.Type,
			Content: []byte(actData.Content),
		}

		jr := response.Rooms.Join[actData.RoomID]
		events := jr.AccountData.Events
		events = append(events, event)
		jr.AccountData.Events = events
		response.Rooms.Join[actData.RoomID] = jr
	}
	return response
}

func (sm *SyncMng) rpcGetPushData(userID, deviceID string, reqType string) push.RpcResponse {
	resp := push.RpcResponse{}
	payload, err := json.Marshal(push.ReqPushUser{
		UserID:   userID,
		DeviceID: deviceID,
	})
	if err != nil {
		resp.Error = fmt.Sprintf("reqType:%s json.Marshal payload err:%v", reqType, err)
		return resp
	}
	byte, err := json.Marshal(push.PushDataRequest{
		Payload: payload,
		ReqType: reqType,
		Slot:    common.CalcStringHashCode(userID) % sm.cfg.MultiInstance.SyncServerTotal,
	})
	if err != nil {
		resp.Error = fmt.Sprintf("reqType:%s json.Marshal request err:%v", reqType, err)
		return resp
	}
	data, err := sm.rpcClient.Request(types.PushDataTopicDef, byte, 15000)
	if err != nil {
		resp.Error = fmt.Sprintf("reqType:%s rpc request err:%v", reqType, err)
		return resp
	}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		resp.Error = fmt.Sprintf("reqType:%s rpc response json.Unmarshal err:%v", reqType, err)
		return resp
	} else {
		return resp
	}
}

func (sm *SyncMng) addPushRules(req *request, response *syncapitypes.Response) *syncapitypes.Response {
	if req.filter != nil {
		if len(req.filter.AccountData.NotSenders) > 0 {
			for _, sender := range req.filter.AccountData.NotSenders {
				if sender == req.device.UserID {
					return response
				}
			}
		}
		if len(req.filter.AccountData.Types) > 0 {
			for _, types := range req.filter.AccountData.Types {
				if types == "m.push_rules" {
					return response
				}
			}
		}
	}
	resp := sm.rpcGetPushData(req.device.UserID, req.device.ID, types.GET_PUSHRULE_BY_USER)
	if resp.Error != "" {
		log.Errorf("traceid:%s user:%s device:%s error:%s", req.traceId, req.device.UserID, req.device.ID, resp.Error)
		return response
	}
	rules := push.Rules{}
	err := json.Unmarshal(resp.Payload, &rules)
	if err != nil {
		log.Errorf("traceid:%s user:%s device:%s json.Unmarshal error:%v", req.traceId, req.device.UserID, req.device.ID, err)
		return response
	}
	global := push.GlobalRule{}
	//rules := routing.GetUserPushRules(req.device.UserID, sm.cache, true, nil)
	formatted := routing.FormatRuleResponse(rules)
	global.Global = formatted
	global.Device = map[string]interface{}{}

	value, err := json.Marshal(global)
	if err != nil {
		log.Errorf("addPushRules for traceid:%s user:%s device:%s error, err:%v", req.traceId, req.device.UserID, req.device.ID, err)
		return response
	}

	event := gomatrixserverlib.ClientEvent{
		Type:    "m.push_rules",
		Content: value,
	}
	response.AccountData.Events = append(response.AccountData.Events, event)

	return response
}

func (sm *SyncMng) addRoomEmptyTags(req *request, response *syncapitypes.Response, roomIDs []string) *syncapitypes.Response {
	var roomTag authtypes.RoomTags
	roomTag.Tags = make(map[string]interface{})

	event := gomatrixserverlib.ClientEvent{}
	event.Type = "m.tag"
	contentBytes, _ := json.Marshal(roomTag)
	event.Content = contentBytes

	for _, roomID := range roomIDs {
		jr := response.Rooms.Join[roomID]
		events := jr.AccountData.Events
		events = append(events, event)
		jr.AccountData.Events = events
		response.Rooms.Join[roomID] = jr
	}

	return response
}

func (sm *SyncMng) addRoomTags(req *request, response *syncapitypes.Response, tagIDs []string) *syncapitypes.Response {
	roomTags := make(map[string]interface{})
	var tagContent interface{}
	joinRooms, err := sm.userTimeLine.GetJoinRooms(req.device.UserID)
	if err != nil {
		return response
	}

	for _, tagID := range tagIDs {
		tag, _ := sm.cache.GetRoomTagCacheData(tagID)
		if _, ok := joinRooms.Load(tag.RoomID); !ok {
			continue
		}

		err := json.Unmarshal([]byte(tag.Content), &tagContent)
		if err != nil {
			log.Error("addRoomTags for traceid:%s user:%s device:%s error:%v", req.traceId, req.device.UserID, req.device.ID, err)
			continue
		}
		var tagMap map[string]interface{}
		if _, ok := roomTags[tag.RoomID]; !ok {
			tagMap = make(map[string]interface{})
			tagMap[tag.Tag] = tagContent
			roomTags[tag.RoomID] = tagMap
		} else {
			tagMap = roomTags[tag.RoomID].(map[string]interface{})
			tagMap[tag.Tag] = tagContent
			roomTags[tag.RoomID] = tagMap
		}
	}

	var roomTag authtypes.RoomTags
	for roomID, tags := range roomTags {
		roomTag.Tags = tags.(map[string]interface{})

		event := gomatrixserverlib.ClientEvent{}
		event.Type = "m.tag"
		contentBytes, _ := json.Marshal(roomTag)
		event.Content = contentBytes

		jr := response.Rooms.Join[roomID]
		events := jr.AccountData.Events
		events = append(events, event)
		jr.AccountData.Events = events
		response.Rooms.Join[roomID] = jr
	}

	return response
}

func (sm *SyncMng) addOneTimeKeyCountInfo(req *request, res *syncapitypes.Response) {
	alCountMap, err := sm.keyChangeRepo.GetOneTimeKeyCount(req.device.UserID, req.device.ID)
	if err != nil {
		log.Errorf("SyncMng add OneTimeKeyCountInfo, traceid:%s, user:%s, device:%s, err:%v ", req.traceId, req.device.UserID, req.device.ID, err)
		return
	}
	res.SignNum = alCountMap
	return
}

func (sm *SyncMng) addPresence(req *request, response *syncapitypes.Response) {
	maxPos := int64(-1)
	if sm.presenceStreamRepo.ExistsPresence(req.device.UserID, req.marks.preRecv) {
		log.Infof("add presence for %s", req.device.UserID)
		friendShipMap := sm.userTimeLine.GetFriendShip(req.device.UserID, true)
		if friendShipMap != nil {
			var presenceEvent gomatrixserverlib.ClientEvent
			friendShipMap.Range(func(key, _ interface{}) bool {
				feed := sm.presenceStreamRepo.GetHistoryByUserID(key.(string))
				if feed != nil && feed.GetOffset() > req.marks.preRecv {
					err := json.Unmarshal(feed.DataStream.Content, &presenceEvent)
					if err != nil {
						log.Errorf("addReceipt: Unmarshal json error for presence traceid:%s userID:%s dev:%s  err:%v", req.traceId, key.(string), req.device.ID, err)
						return true
					}

					response.Presence.Events = append(response.Presence.Events, presenceEvent)
					data, _ := json.Marshal(presenceEvent)
					log.Infof("add presence for %s %d %d %s", req.device.UserID, feed.GetOffset(), req.marks.preRecv, data)

					if maxPos < feed.GetOffset() {
						maxPos = feed.GetOffset()
					}
				}
				return true
			})
		}
	}

	if maxPos == -1 {
		maxPos = req.marks.preRecv
	}

	req.marks.preProcess = maxPos
	log.Infof("process precense user:%s cur:%d", req.device.UserID, req.marks.preProcess)
	return
}

func (sm *SyncMng) addKeyChangeInfo(req *request, response *syncapitypes.Response) {
	maxPos := int64(-1)
	if req.marks.utlRecv > 0 {
		if sm.keyChangeRepo.ExistsKeyChange(req.marks.kcRecv, req.device.UserID) {
			kcMap := sm.keyChangeRepo.GetHistory()
			if kcMap != nil {
				friendShipMap := sm.userTimeLine.GetFriendShip(req.device.UserID, true)
				if friendShipMap != nil {
					friendShipMap.Range(func(key, _ interface{}) bool {
						if val, ok := kcMap.Load(key.(string)); ok {
							feed := val.(*feedstypes.KeyChangeStream)
							if feed.GetOffset() > req.marks.kcRecv {
								response.DeviceList.Changed = append(response.DeviceList.Changed, key.(string))
								if maxPos < feed.GetOffset() {
									maxPos = feed.GetOffset()
								}
							}
						}
						return true
					})
				}
			}
		}
	} else {
		maxPos = sm.keyChangeRepo.GetUserLatestOffset(req.device.UserID)
	}

	if maxPos == -1 {
		maxPos = req.marks.kcRecv
	}

	req.marks.kcProcess = maxPos
	return
}

func (sm *SyncMng) addTyping(req *request, response *syncapitypes.Response, curRoomID string) {
	if sm.typingConsumer.ExistsTyping(req.device.UserID, req.device.ID, curRoomID) {
		events := sm.typingConsumer.GetTyping(req.device.UserID, req.device.ID, curRoomID)
		joinRooms, err := sm.userTimeLine.GetJoinRooms(req.device.UserID)
		if err != nil {
			return
		}
		if response.Rooms.Join == nil {
			log.Warnf("traceid:%s addTyping rooms join is nil", req.traceId)
			response.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
		}
		if curRoomID == "" {
			for _, event := range events {
				var jr *syncapitypes.JoinResponse
				roomID := event.RoomID
				if _, ok := joinRooms.Load(roomID); ok {
					if joinResponse, ok := response.Rooms.Join[roomID]; ok {
						jr = &joinResponse
					} else {
						jr = syncapitypes.NewJoinResponse()
					}
					event.RoomID = ""
					jr.Ephemeral.Events = append(jr.Ephemeral.Events, event)
					response.Rooms.Join[roomID] = *jr
				}
			}
		} else {
			for _, event := range events {
				var jr *syncapitypes.JoinResponse
				roomID := event.RoomID
				if roomID != curRoomID {
					continue
				}
				if _, ok := joinRooms.Load(roomID); ok {
					if joinResponse, ok := response.Rooms.Join[roomID]; ok {
						jr = &joinResponse
					} else {
						jr = syncapitypes.NewJoinResponse()
					}
					event.RoomID = ""
					jr.Ephemeral.Events = append(jr.Ephemeral.Events, event)
					response.Rooms.Join[roomID] = *jr
				}
			}
		}
	}
	return
}
