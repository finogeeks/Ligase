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

package consumers

import (
	"context"
	goSync "sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	pushapi "github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type ReceiptConsumer struct {
	container       *goSync.Map
	notifyRoom      *goSync.Map
	delay           int64
	countRepo       *repos.ReadCountRepo
	rsTimeline      *repos.RoomStateTimeLineRepo
	receiptRepo     *repos.ReceiptDataStreamRepo
	userReceiptRepo *repos.UserReceiptRepo
	roomHistory     *repos.RoomHistoryTimeLineRepo
	rpcClient       *common.RpcClient
	roomCurState    *repos.RoomCurStateRepo
	cfg             *config.Dendrite
	idg             *uid.UidGenerator
}

func NewReceiptConsumer(
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
	idg *uid.UidGenerator,
) *ReceiptConsumer {
	s := &ReceiptConsumer{}
	s.container = new(goSync.Map)
	s.notifyRoom = new(goSync.Map)
	s.delay = cfg.ReceiptDelay
	s.rpcClient = rpcClient
	s.cfg = cfg
	s.idg = idg

	return s
}

func (s *ReceiptConsumer) SetRoomCurState(roomCurState *repos.RoomCurStateRepo) *ReceiptConsumer {
	s.roomCurState = roomCurState
	return s
}

func (s *ReceiptConsumer) SetRoomHistory(roomHistory *repos.RoomHistoryTimeLineRepo) *ReceiptConsumer {
	s.roomHistory = roomHistory
	return s
}

func (s *ReceiptConsumer) SetUserReceiptRepo(userReceiptRepo *repos.UserReceiptRepo) *ReceiptConsumer {
	s.userReceiptRepo = userReceiptRepo
	return s
}

func (s *ReceiptConsumer) SetReceiptRepo(receiptRepo *repos.ReceiptDataStreamRepo) *ReceiptConsumer {
	s.receiptRepo = receiptRepo
	return s
}

func (s *ReceiptConsumer) SetCountRepo(countRepo *repos.ReadCountRepo) *ReceiptConsumer {
	s.countRepo = countRepo
	return s
}

func (s *ReceiptConsumer) SetRsTimeline(rsTimeline *repos.RoomStateTimeLineRepo) *ReceiptConsumer {
	s.rsTimeline = rsTimeline
	return s
}

//it's a up to markers
func (s *ReceiptConsumer) OnReceipt(ctx context.Context, req *types.ReceiptContent) {
	log.Infof("OnReceipt roomID %s receiptType %s eventID %s userID %s deviceID %s source %s", req.RoomID, req.ReceiptType, req.EventID, req.UserID, req.DeviceID, req.Source)
	rs := s.roomCurState.GetRoomState(req.RoomID)
	if rs == nil {
		s.rsTimeline.LoadStreamStates(ctx, req.RoomID, true)
		rs = s.roomCurState.GetRoomState(req.RoomID)
		if rs == nil {
			log.Warnf("OnReceipt RoomState empty roomID %s userID %s", req.RoomID, req.UserID)
			return
		}
	}
	_, isJoinedMember := rs.GetJoinMap().Load(req.UserID)
	if !isJoinedMember {
		log.Warnf("OnReceipt the user is not the joined member roomID %s userID %s", req.RoomID, req.UserID)
		return
	}

	lastEventID := ""
	stream := s.roomHistory.GetLastEvent(ctx, req.RoomID)
	if stream != nil {
		lastEventID = stream.GetEv().EventID
	}

	if req.EventID == "" {
		req.EventID = lastEventID
	}

	if req.EventID == "" {
		log.Warnf("OnReceipt eventID is null roomID %s receiptType %s eventID %s userID %s deviceID %s", req.RoomID, req.ReceiptType, req.EventID, req.UserID, req.DeviceID)
		return
	}

	receiptOffSet := int64(-1)
	stream = s.roomHistory.GetStreamEv(ctx, req.RoomID, req.EventID)
	if stream != nil {
		receiptOffSet = stream.Offset
	}

	if lastEventID == req.EventID || lastEventID == "" {
		var receipt *pushapi.RoomReceipt
		item, ok := s.container.Load(req.RoomID)
		if !ok {
			receipt = new(pushapi.RoomReceipt)
			receipt.RoomID = req.RoomID
			receipt.EvID = req.EventID
			receipt.EvOffSet = receiptOffSet
			receipt.Content = new(goSync.Map)
			item, _ = s.container.LoadOrStore(req.RoomID, receipt)
			receipt = item.(*pushapi.RoomReceipt)
		} else {
			receipt = item.(*pushapi.RoomReceipt)
		}

		if receipt.EvID == req.EventID {
			readTs, isRead := receipt.Content.Load(req.UserID)
			if isRead {
				readCount, hlCount := s.countRepo.GetRoomReadCount(req.RoomID, req.UserID)
				if readCount == 0 && hlCount == 0 {
					log.Infof("-----OnReceipt ready notifyRoom room:%s, userID:%s type:%s evID:%s isRead:%t readTime:%d readCount:%d hlCount:%d", req.RoomID, req.UserID, req.ReceiptType, req.EventID, isRead, readTs, readCount, hlCount)
					return
				}
			}
		} else {
			//new msg should have a new req content
			receipt.EvID = req.EventID
			receipt.EvOffSet = receiptOffSet
			receipt.Content = new(goSync.Map)
		}

		log.Infof("-----OnReceipt notifyRoom room:%s, userID:%s type:%s evID:%s time:%d", req.RoomID, req.UserID, req.ReceiptType, req.EventID, time.Now().Unix()*1000)
		receipt.Content.Store(req.UserID, time.Now().Unix()*1000)
		s.notifyRoom.Store(req.RoomID, true)

		//重置未读计数
		s.countRepo.UpdateRoomReadCount(req.RoomID, req.EventID, req.UserID, "reset")

		//federation
		if s.roomCurState.GetRoomState(req.RoomID) != nil {
			domainMap := make(map[string]bool)
			s.roomCurState.GetRoomState(req.RoomID).GetJoinMap().Range(func(key, value interface{}) bool {
				domain, _ := common.DomainFromID(key.(string))
				if common.CheckValidDomain(domain, s.cfg.Matrix.ServerName) == false {
					domainMap[domain] = true
				}
				return true
			})

			senderDomain, _ := common.DomainFromID(req.UserID)
			if common.CheckValidDomain(senderDomain, s.cfg.Matrix.ServerName) {
				content, _ := json.Marshal(req)
				for domain := range domainMap {
					edu := gomatrixserverlib.EDU{
						Type:        "receipt",
						Origin:      senderDomain,
						Destination: domain,
						Content:     content,
					}
					bytes, err := json.Marshal(edu)
					if err == nil {
						s.rpcClient.Pub(types.EduTopicDef, bytes)
					} else {
						log.Errorf("ReceiptConsumer pub receipt edu error %v", err)
					}
				}
			}
		}
	} else {
		log.Infof("OnReceipt not latest event %s roomID %s receiptType %s eventID %s userID %s deviceID %s source %s", lastEventID, req.RoomID, req.ReceiptType, req.EventID, req.UserID, req.DeviceID, req.Source)
	}
}

// Start consuming from room servers
func (s *ReceiptConsumer) Start() error {
	go func() {
		t := time.NewTimer(time.Millisecond * time.Duration(s.delay))
		for {
			select {
			case <-t.C:
				func() {
					span, ctx := common.StartSobSomSpan(context.Background(), "ReceiptConsumer.Start")
					defer span.Finish()
					s.fireReceipt(ctx)
				}()
				t.Reset(time.Millisecond * time.Duration(s.delay))
			}
		}
	}()

	return nil
}

func (s *ReceiptConsumer) fireReceipt(ctx context.Context) {
	s.notifyRoom.Range(func(key, _ interface{}) bool {
		s.notifyRoom.Delete(key)

		roomID := key.(string)
		log.Infof("-----OnReceipt fireReceipt roomID:%s", roomID)
		s.fireRoomReceipt(ctx, roomID)
		return true
	})
}

func (s *ReceiptConsumer) fireRoomReceipt(ctx context.Context, roomID string) {
	res, _ := s.container.Load(roomID)
	receipt := res.(*pushapi.RoomReceipt)
	user := new(pushapi.ReceiptUser)
	user.Users = make(map[string]pushapi.ReceiptTs)
	content := make(map[string]pushapi.ReceiptUser)

	receipt.Content.Range(func(key, val interface{}) bool {
		receipt.Content.Delete(key)

		uid := key.(string)
		ts := val.(int64)

		user.Users[uid] = pushapi.ReceiptTs{
			Ts: ts,
		}
		return true
	})

	content[receipt.EvID] = *user

	event := &gomatrixserverlib.ClientEvent{}
	event.Type = "m.receipt"
	event.Content, _ = json.Marshal(content)

	eventJson, err := json.Marshal(event)
	if err != nil {
		log.Errorw("fireRoomReceipt: Marshal json error for receipt", log.KeysAndValues{"roomID", roomID, "eventJson", string(eventJson), "error", err})
		return
	}

	offset, _ := s.idg.Next()
	log.Infof("flushReceiptUpdate roomID:%s evoffset:%d offset:%d", roomID, receipt.EvOffSet, offset)
	s.flushReceiptUpdate(ctx, roomID, eventJson, receipt.EvOffSet, offset)
}

func (s *ReceiptConsumer) flushReceiptUpdate(ctx context.Context, roomID string, content []byte, evOffset, offset int64) {
	var receipt pushapi.ReceiptContent
	var receiptEvent gomatrixserverlib.ClientEvent

	err := json.Unmarshal(content, &receiptEvent)
	if err != nil {
		log.Errorf("ReceiptConsumer receipt unmarshal err: %v", err)
		return
	}

	err = json.Unmarshal(receiptEvent.Content, &receipt.Map)
	if err != nil {
		log.Errorf("sync receipt unmarshal err: %v", err)
		return
	}

	for evtID, content := range receipt.Map {
		for uid, ts := range content.Users {
			user := new(pushapi.ReceiptUser)
			user.Users = make(map[string]pushapi.ReceiptTs)
			user.Users[uid] = pushapi.ReceiptTs{
				Ts: ts.Ts,
			}

			content := make(map[string]pushapi.ReceiptUser)
			content[evtID] = *user

			event := &gomatrixserverlib.ClientEvent{}
			event.Type = "m.receipt"
			event.Content, _ = json.Marshal(content)

			eventJson, _ := json.Marshal(event)

			var receipt types.UserReceipt
			receipt.RoomID = roomID
			receipt.UserID = uid
			receipt.EvtOffset = evOffset
			receipt.Content = eventJson
			s.userReceiptRepo.AddUserReceipt(&receipt)
		}
	}

	receiptStream := types.ReceiptStream{}
	receiptStream.RoomID = roomID
	receiptStream.Content = content
	receiptStream.ReceiptOffset = evOffset

	s.receiptRepo.AddReceiptDataStream(ctx, &receiptStream, offset)
	s.pubReceiptUpdate(roomID, offset)
}

func (s *ReceiptConsumer) pubReceiptUpdate(roomID string, offset int64) {
	roomState := s.roomCurState.GetRoomState(roomID)
	if roomState != nil {
		roomUpdate := new(syncapitypes.ReceiptUpdate)
		roomUpdate.RoomID = roomID
		roomUpdate.Offset = offset
		joinMap := roomState.GetJoinMap()
		if joinMap != nil {
			joinMap.Range(func(key, _ interface{}) bool {
				roomUpdate.Users = append(roomUpdate.Users, key.(string))
				return true
			})
		}

		bytes, err := json.Marshal(roomUpdate)
		if err == nil {
			s.rpcClient.Pub(types.ReceiptUpdateTopicDef, bytes)
		} else {
			log.Errorf("ReceiptConsumer pub receipt update error %v", err)
		}
	} else {
		log.Errorf("ReceiptConsumer.pubReceiptUpdate %s get room state nil", roomID)
	}
}
