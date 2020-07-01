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
	"fmt"
	"github.com/finogeeks/ligase/syncserver/extra"
	"time"

	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/types"
	jsoniter "github.com/json-iterator/go"

	jsonRaw "encoding/json"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// OutputRoomEventConsumer consumes events that originated in the room server.
type RoomEventFeedConsumer struct {
	channel               core.IChannel
	db                    model.SyncAPIDatabase
	roomStateTimeLine     *repos.RoomStateTimeLineRepo
	roomHistoryTimeLine   *repos.RoomHistoryTimeLineRepo
	roomCurState          *repos.RoomCurStateRepo
	receiptDataStreamRepo *repos.ReceiptDataStreamRepo
	displayNameRepo       *repos.DisplayNameRepo
	pushConsumer          *PushConsumer
	cfg                   *config.Dendrite
	rpcClient             *common.RpcClient
	chanSize              uint32
	msgChan               []chan roomserverapi.OutputEvent
	idg                   *uid.UidGenerator
}

func NewRoomEventFeedConsumer(
	cfg *config.Dendrite,
	store model.SyncAPIDatabase,
	pushConsumer *PushConsumer,
	rpcClient *common.RpcClient,
	idg *uid.UidGenerator,
) *RoomEventFeedConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputRoomEventSyncServer.Underlying,
		cfg.Kafka.Consumer.OutputRoomEventSyncServer.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &RoomEventFeedConsumer{
			channel:      channel,
			db:           store,
			pushConsumer: pushConsumer,
			cfg:          cfg,
			rpcClient:    rpcClient,
			chanSize:     64,
			idg:          idg,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *RoomEventFeedConsumer) SetReceiptRepo(receiptDataStreamRepo *repos.ReceiptDataStreamRepo) *RoomEventFeedConsumer {
	s.receiptDataStreamRepo = receiptDataStreamRepo
	return s
}

func (s *RoomEventFeedConsumer) SetRoomHistory(roomHistory *repos.RoomHistoryTimeLineRepo) *RoomEventFeedConsumer {
	s.roomHistoryTimeLine = roomHistory
	return s
}

func (s *RoomEventFeedConsumer) SetRsTimeline(rsTimeline *repos.RoomStateTimeLineRepo) *RoomEventFeedConsumer {
	s.roomStateTimeLine = rsTimeline
	return s
}

func (s *RoomEventFeedConsumer) SetRsCurState(rsCurState *repos.RoomCurStateRepo) *RoomEventFeedConsumer {
	s.roomCurState = rsCurState
	return s
}

func (s *RoomEventFeedConsumer) SetDisplayNameRepo(displayNameRepo *repos.DisplayNameRepo) *RoomEventFeedConsumer {
	s.displayNameRepo = displayNameRepo
	return s
}

func (s *RoomEventFeedConsumer) startWorker(msgChan chan roomserverapi.OutputEvent) {
	for data := range msgChan {
		switch data.Type {
		case roomserverapi.OutputTypeNewRoomEvent:
			s.onNewRoomEvent(context.TODO(), data.NewRoomEvent)
		case roomserverapi.OutputBackfillRoomEvent:
			s.onBackFillEvent(context.TODO(), data.NewRoomEvent)
		}
	}
}

func (s *RoomEventFeedConsumer) Start() error {
	s.msgChan = make([]chan roomserverapi.OutputEvent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan roomserverapi.OutputEvent, 1024)
		go s.startWorker(s.msgChan[i])
	}

	//s.channel.Start()
	return nil
}

func (s *RoomEventFeedConsumer) OnMessage(topic string, partition int32, data []byte) {
	var output roomserverapi.OutputEvent
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("syncapi: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	log.Debugw("syncpi received data", log.KeysAndValues{"type", output.Type, "topic", topic})

	switch output.Type {
	case roomserverapi.OutputTypeNewRoomEvent:
		if common.IsRelatedRequest(output.NewRoomEvent.Event.RoomID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			bytes, _ := json.Marshal(output.NewRoomEvent.Event)
			log.Infow("sync server received event from room server", log.KeysAndValues{"type", output.NewRoomEvent.Event.Type, "event_id", output.NewRoomEvent.Event.EventID, "room_id", output.NewRoomEvent.Event.RoomID, "instance", s.cfg.MultiInstance.Instance, "data", string(bytes)})
			idx := common.CalcStringHashCode(output.NewRoomEvent.Event.RoomID) % s.chanSize
			s.msgChan[idx] <- output
		}
	case roomserverapi.OutputBackfillRoomEvent:
		if common.IsRelatedRequest(output.NewRoomEvent.Event.RoomID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			log.Infow("sync writer received back fill event from room server", log.KeysAndValues{"type", output.NewRoomEvent.Event.Type, "event_id", output.NewRoomEvent.Event.EventID, "room_id", output.NewRoomEvent.Event.RoomID})
			idx := common.CalcStringHashCode(output.NewRoomEvent.Event.RoomID) % s.chanSize
			s.msgChan[idx] <- output
		}
	default:
		log.Debugw("syncapi: ignoring unknown output type", log.KeysAndValues{"type", output.Type})
	}
}

func (s *RoomEventFeedConsumer) processStateEv(ev *gomatrixserverlib.ClientEvent) (gomatrixserverlib.ClientEvent, error) {
	rs := s.roomCurState.GetRoomState(ev.RoomID)

	if rs != nil {
		statekey := ""
		if ev.StateKey != nil {
			statekey = *ev.StateKey
		}
		pre := rs.GetState(ev.Type, statekey)
		if pre != nil { //set unigned
			prev := syncapitypes.PrevEventRef{
				PrevContent:   jsonRaw.RawMessage(pre.GetEv().Content),
				ReplacesState: pre.GetEv().EventID,
				PrevSender:    pre.GetEv().Sender,
				PreOffset:     pre.GetOffset(),
			}

			prevBytes, err := jsonRaw.Marshal(prev)
			if err != nil {
				return *ev, err
			}

			ev.Unsigned = prevBytes
		}
	}

	return *ev, nil
}

func (s *RoomEventFeedConsumer) processRedactEv(ev *gomatrixserverlib.ClientEvent) {
	var redactEv gomatrixserverlib.ClientEvent

	stream := s.roomHistoryTimeLine.GetStreamEv(ev.RoomID, ev.Redacts)
	if stream != nil {
		redactEv = *stream.Ev
	} else {
		evs, err := s.db.Events(context.TODO(), []string{ev.Redacts})
		//log.Infof("redact redact:%s evs:%v, err:%v", ev.Redacts, evs, err)
		if err == nil && len(evs) > 0 {
			redactEv = evs[0]
		} else {
			if err != nil {
				log.Errorf("redact redact:%s evs:%v, err:%v", ev.Redacts, evs, err)
			}
			return
		}
	}
	unsigned := types.RedactUnsigned{}
	if ev.Type == "m.room.redaction" {
		content := map[string]interface{}{}
		empty, _ := json.Marshal(content)
		redactEv.Content = empty
		redactEv.Hint = fmt.Sprintf("%s撤回了一条消息", extra.GetDisplayName(s.displayNameRepo, ev.Sender))
		unsigned.RedactedBecause = ev
	} else {
		redactEv.Content = ev.Content
		unsigned.UpdatedBecause = ev
	}
	unsignedBytes, err := json.Marshal(unsigned)
	if err != nil {
		log.Errorf("redact redact Marshal:%s evs:%v, err:%v", ev.Redacts, unsigned, err)
	}
	redactEv.Unsigned = unsignedBytes
	log.Infof("syncserver, edactEv.Hint: %s", redactEv.Hint)

	if stream != nil {
		stream.Ev = &redactEv //更新timeline
	}
}

func (s *RoomEventFeedConsumer) onNewRoomEvent(
	ctx context.Context, msg *roomserverapi.OutputNewRoomEvent,
) error {
	defer func() {
		if e := recover(); e != nil {
			stack := common.PanicTrace(4)
			log.Panicf("%v\n%s\n", e, stack)
		}
	}()

	ev := msg.Event
	log.Infof("feedserver onNewRoomEvent start roomID:%s eventID:%s sender:%s type:%s eventoffset:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset)
	domain, _ := common.DomainFromID(ev.Sender)
	if ev.Type != "m.room.create" {
		bs := time.Now().UnixNano() / 1000000
		s.roomStateTimeLine.GetStates(ev.RoomID)       //load state timeline& state
		s.roomStateTimeLine.GetStateStreams(ev.RoomID) //load state stream timeline& state
		s.roomHistoryTimeLine.LoadHistory(ev.RoomID, true)
		s.receiptDataStreamRepo.LoadHistory(ev.RoomID, false)
		preOffset := s.roomHistoryTimeLine.GetDomainMaxStream(ev.RoomID, domain)
		if preOffset != -1 && preOffset+1 != ev.DomainOffset {
			log.Infof("feedserver onNewRoomEvent SetRoomMinStream roomID:%s preOffset:%d domainOffset:%d eventOffset:%d", ev.RoomID, preOffset, ev.DomainOffset, ev.EventOffset)
			s.roomHistoryTimeLine.SetRoomMinStream(ev.RoomID, ev.EventOffset)
		}
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("feedserver onNewRoomEvent load room state roomID:%s eventID:%s sender:%s type:%s eventoffset:%d spend:%dms", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset, spend)
	}
	s.roomHistoryTimeLine.SetDomainMaxStream(ev.RoomID, domain, ev.DomainOffset)

	if common.IsStateClientEv(&ev) == true { //state ev
		ev, _ = s.processStateEv(&ev)
	} else if ev.Type == "m.room.redaction" || ev.Type == "m.room.update" {
		s.processRedactEv(&ev)
	}
	log.Infof("feedserver onNewRoomEvent update unsigned roomID:%s eventID:%s sender:%s type:%s eventoffset:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset)
	transId := ""
	if msg.TransactionID != nil {
		transId = msg.TransactionID.TransactionID
	}

	if (ev.Type == "m.room.message" || ev.Type == "m.room.encrypted") && transId != "" {
		unsigned := types.Unsigned{}
		unsigned.TransactionID = transId
		unsignedBytes, err := json.Marshal(unsigned)
		if err != nil {
			log.Errorf("RoomEventFeedConsumer.onNewRoomEvent Marshal:%s evs:%v, err:%v", ev.Redacts, unsigned, err)
		}
		ev.Unsigned = unsignedBytes
	}

	if ev.StateKey != nil {
		msg.TransactionID = &roomservertypes.TransactionID{
			DeviceID:      *ev.StateKey,
			TransactionID: transId,
		}
	}

	membership := ""
	if common.IsStateClientEv(&ev) == true { //state ev
		log.Infof("feedserver onNewRoomEvent update state event roomID:%s eventID:%s sender:%s type:%s eventoffset:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset)
		s.roomStateTimeLine.AddEv(&ev, ev.EventOffset, true)       //state 也会更新
		s.roomStateTimeLine.AddStreamEv(&ev, ev.EventOffset, true) //保留state stream

		if ev.Type == "m.room.member" {
			con := external.MemberContent{}
			json.Unmarshal(ev.Content, &con)
			membership = con.Membership
		}

		switch ev.Type {
		case "m.room.member":
			if membership == "join" {
				rs := s.roomCurState.GetRoomState(ev.RoomID)
				if rs != nil {
					if rs.IsEncrypted() {
						var changes []types.DeviceKeyChanges
						offset, _ := s.idg.Next()
						err := s.db.InsertKeyChange(ctx, ev.Sender, offset)
						if err != nil {
							log.Errorf("RoomEventFeedConsumer insert key change error %v", err)
						} else {
							changed := types.DeviceKeyChanges{
								ChangedUserID: ev.Sender,
								Offset:        offset,
							}
							changes = append(changes, changed)
							s.pubKeyUpdate(&changes, ev.EventNID)
						}
					}
				}
			} else if membership == "invite" {
				senderDomain, _ := common.DomainFromID(*ev.StateKey)
				rs := s.roomCurState.GetRoomState(ev.RoomID)
				if rs != nil {
					if common.CheckValidDomain(senderDomain, s.cfg.Matrix.ServerName) {
						domainMap := make(map[string]bool)
						rs.GetJoinMap().Range(func(key, value interface{}) bool {
							domain, _ := common.DomainFromID(key.(string))
							if common.CheckValidDomain(domain, s.cfg.Matrix.ServerName) == false {
								domainMap[domain] = true
							}
							return true
						})

						fedProfile := types.ProfileContent{
							UserID: *ev.StateKey,
						}
						fedProfile.DisplayName = s.displayNameRepo.GetOriginDisplayName(*ev.StateKey)
						fedProfile.AvatarUrl = s.displayNameRepo.GetAvatarUrl(*ev.StateKey)
						fedProfile.UserName = s.displayNameRepo.GetUserName(*ev.StateKey)
						fedProfile.JobNumber = s.displayNameRepo.GetJobNumber(*ev.StateKey)
						fedProfile.Mobile = s.displayNameRepo.GetMobile(*ev.StateKey)
						fedProfile.Landline = s.displayNameRepo.GetLandline(*ev.StateKey)
						fedProfile.Email = s.displayNameRepo.GetEmail(*ev.StateKey)

						content, _ := json.Marshal(fedProfile)
						stateKeyData := []byte(*ev.StateKey)
						for domain := range domainMap {
							edu := gomatrixserverlib.EDU{
								Type:        "profile",
								Origin:      senderDomain,
								Destination: domain,
								Content:     content,
							}
							common.GetTransportMultiplexer().SendWithRetry(
								s.cfg.Kafka.Producer.FedEduUpdate.Underlying,
								s.cfg.Kafka.Producer.FedEduUpdate.Name,
								&core.TransportPubMsg{
									Keys: stateKeyData,
									Obj:  edu,
								})
						}
					} else {
						rs.GetJoinMap().Range(func(key, value interface{}) bool {
							domain, _ := common.DomainFromID(key.(string))
							if common.CheckValidDomain(domain, s.cfg.Matrix.ServerName) {
								fedProfile := types.ProfileContent{
									UserID: key.(string),
								}
								fedProfile.DisplayName = s.displayNameRepo.GetOriginDisplayName(key.(string))
								fedProfile.AvatarUrl = s.displayNameRepo.GetAvatarUrl(key.(string))
								fedProfile.UserName = s.displayNameRepo.GetUserName(key.(string))
								fedProfile.JobNumber = s.displayNameRepo.GetJobNumber(key.(string))
								fedProfile.Mobile = s.displayNameRepo.GetMobile(key.(string))
								fedProfile.Landline = s.displayNameRepo.GetLandline(key.(string))
								fedProfile.Email = s.displayNameRepo.GetEmail(key.(string))
								content, _ := json.Marshal(fedProfile)

								edu := gomatrixserverlib.EDU{
									Type:        "profile",
									Origin:      domain,
									Destination: senderDomain,
									Content:     content,
								}
								common.GetTransportMultiplexer().SendWithRetry(
									s.cfg.Kafka.Producer.FedEduUpdate.Underlying,
									s.cfg.Kafka.Producer.FedEduUpdate.Name,
									&core.TransportPubMsg{
										Keys: []byte(key.(string)),
										Obj:  edu,
									})
							}
							return true
						})
					}
				}
			}
		case "m.room.encryption":
			rs := s.roomCurState.GetRoomState(ev.RoomID)
			if rs != nil {
				joined := rs.GetJoinMap()
				if joined != nil {
					var changes []types.DeviceKeyChanges
					joined.Range(func(key, _ interface{}) bool {
						offset, _ := s.idg.Next()
						err := s.db.InsertKeyChange(ctx, key.(string), offset)
						if err != nil {
							log.Errorf("RoomEventFeedConsumer insert key change error %v", err)
							return true
						}

						changed := types.DeviceKeyChanges{
							ChangedUserID: key.(string),
							Offset:        offset,
						}
						changes = append(changes, changed)

						return true
					})
					s.pubKeyUpdate(&changes, ev.EventNID)
				}
			}
		}
	}
	log.Infof("feedserver onNewRoomEvent add history timeline roomID:%s eventID:%s sender:%s type:%s eventoffset:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset)
	s.roomHistoryTimeLine.AddEv(&ev, ev.EventOffset, true) //更新room timeline

	if s.cfg.CalculateReadCount {
		s.pushConsumer.OnEvent(&ev, ev.EventOffset)
	}

	return nil
}

func (s *RoomEventFeedConsumer) pubKeyUpdate(changes *[]types.DeviceKeyChanges, evtNid int64) {
	if len(*changes) > 0 {
		content := types.KeyUpdateContent{
			Type:             types.DEVICEKEYUPDATE,
			DeviceKeyChanges: *changes,
			EventNID:         evtNid,
		}
		bytes, err := json.Marshal(content)
		if err == nil {
			s.rpcClient.Pub(types.KeyUpdateTopicDef, bytes)
		} else {
			log.Errorf("RoomEventConsumer pub key change error %v", err)
		}
	}
}

func (s *RoomEventFeedConsumer) onBackFillEvent(
	ctx context.Context, msg *roomserverapi.OutputNewRoomEvent,
) error {
	ev := msg.Event
	domain, _ := common.DomainFromID(ev.Sender)
	preOffset := s.roomHistoryTimeLine.GetDomainMaxStream(ev.RoomID, domain)
	if preOffset <= ev.DomainOffset {
		log.Infof("feedserver onBackFillEvent SetRoomMinStream roomID:%s preOffset:%d domainOffset:%d eventOffset:%d", ev.RoomID, preOffset, ev.DomainOffset, ev.EventOffset)
		s.roomHistoryTimeLine.SetRoomMinStream(ev.RoomID, ev.EventOffset)
	}

	if common.IsStateClientEv(&ev) {
		ev, _ = s.processStateEv(&ev)
		s.roomStateTimeLine.AddBackfillEv(&ev, ev.EventOffset, true)
	} else if ev.Type == "m.room.redaction" || ev.Type == "m.room.update" {
		s.processRedactEv(&ev)
	}

	return nil
}
