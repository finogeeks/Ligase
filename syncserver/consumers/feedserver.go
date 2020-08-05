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
	"time"

	"github.com/finogeeks/ligase/syncserver/extra"

	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/types"
	jsoniter "github.com/json-iterator/go"

	jsonRaw "encoding/json"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
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
	//msgChan               []chan roomserverapi.OutputEvent
	msgChan []chan common.ContextMsg
	idg     *uid.UidGenerator
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

func (s *RoomEventFeedConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		ctx := msg.Ctx
		data := msg.Msg.(roomserverapi.OutputEvent)
		switch data.Type {
		case roomserverapi.OutputTypeNewRoomEvent:
			s.onNewRoomEvent(ctx, data.NewRoomEvent)
		case roomserverapi.OutputBackfillRoomEvent:
			s.onBackFillEvent(ctx, data.NewRoomEvent)
		}
	}
}

func (s *RoomEventFeedConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 1024)
		go s.startWorker(s.msgChan[i])
	}

	//s.channel.Start()
	return nil
}

func (s *RoomEventFeedConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
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
			s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: output}
		}
	case roomserverapi.OutputBackfillRoomEvent:
		if common.IsRelatedRequest(output.NewRoomEvent.Event.RoomID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			log.Infow("sync writer received back fill event from room server", log.KeysAndValues{"type", output.NewRoomEvent.Event.Type, "event_id", output.NewRoomEvent.Event.EventID, "room_id", output.NewRoomEvent.Event.RoomID})
			idx := common.CalcStringHashCode(output.NewRoomEvent.Event.RoomID) % s.chanSize
			s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: output}
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

func (s *RoomEventFeedConsumer) processRedactEv(ctx context.Context, ev *gomatrixserverlib.ClientEvent) {
	var redactEv gomatrixserverlib.ClientEvent

	stream := s.roomHistoryTimeLine.GetStreamEv(ctx, ev.RoomID, ev.Redacts)
	if stream != nil {
		redactEv = *stream.Ev
	} else {
		evs, err := s.db.Events(ctx, []string{ev.Redacts})
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
		reaction := s.parseRelatesContent(redactEv)
		if reaction != nil {
			s.updateReactionEvent(ctx, ev.RoomID, reaction)
		}
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

func (s *RoomEventFeedConsumer) updateReactionEvent(ctx context.Context, roomID string, reaction *types.ReactionContent){
	var originEv gomatrixserverlib.ClientEvent
	stream := s.roomHistoryTimeLine.GetStreamEv(ctx, roomID, reaction.EventID)
	if stream != nil {
		originEv = *stream.Ev
	} else {
		evs, err := s.db.Events(context.TODO(), []string{reaction.EventID})
		if err == nil && len(evs) > 0 {
			originEv = evs[0]
		} else {
			if err != nil {
				log.Errorf("updateReaction room:%s event:%s evs:%v, err:%v", roomID, reaction.EventID, evs, err)
			}
			return
		}
	}
	unsigned := types.Unsigned{}
	if originEv.Unsigned != nil {
		err := json.Unmarshal(originEv.Unsigned,&unsigned)
		if err != nil {
			log.Errorf("updateReactionEvent json.Unmarshal  origin eventID:%s unsigned err:%v", reaction.EventID, err)
			return
		}
	}
	if unsigned.Relations != nil {
		if unsigned.Relations.Anno == nil {
			return
		}else{
			if unsigned.Relations.Anno.Chunk == nil {
				return
			}else{
				for idx,item := range unsigned.Relations.Anno.Chunk{
					if item.Key == reaction.Key {
						item.Count--
						if item.Count <= 0 {
							unsigned.Relations.Anno.Chunk = append(unsigned.Relations.Anno.Chunk[:idx], unsigned.Relations.Anno.Chunk[idx+1:]...)
						}
						break
					}
				}
				if len(unsigned.Relations.Anno.Chunk) <= 0 {
					if unsigned.Relations.RelayTo == nil {
						unsigned.Relations = nil
					}else{
						unsigned.Relations.Anno = nil
					}
				}
			}
		}
	}else{
		return
	}
	unsignedBytes, err := json.Marshal(unsigned)
	if err != nil {
		log.Errorf("updateReactionEvent json.Unmarshal eventID:%s unsigned err:%v", originEv.EventID, err)
		return
	}
	originEv.Unsigned = unsignedBytes
	if stream != nil {
		stream.Ev = &originEv
	}
	log.Infof("updateReactionEvent eventID:%s  succ", originEv.EventID)
}

func (s *RoomEventFeedConsumer) parseRelatesContent(redactEv gomatrixserverlib.ClientEvent)(reaction *types.ReactionContent){
	var originContent map[string]interface{}
	err := json.Unmarshal(redactEv.Content, &originContent)
	if err != nil {
		log.Errorf("json.Unmarshal redactEv err:%v", err)
		return nil
	}
	v, ok := originContent["m.relates_to"]
	if !ok {
		return nil
	}
	b, err := json.Marshal(v)
	json.Unmarshal(b,&reaction)
	originEventID := reaction.EventID
	//is reaction relay
	if originEventID != "" && reaction.RelType == "m.annotation" {
		return reaction
	}else{
		//other ignore
		return nil
	}
}

func (s *RoomEventFeedConsumer) processMessageEv(ctx context.Context, ev *gomatrixserverlib.ClientEvent) {
	var content map[string]interface{}
	err := json.Unmarshal(ev.Content, &content)
	if err != nil {
		log.Errorf("processMessageEv Unmarshal roomId:%s eventId:%s err:%v", ev.RoomID, ev.EventID, err)
		return
	}
	v, ok := content["m.relates_to"]
	if !ok {
		return
	}
	mRelayTo := types.MInRelayTo{}
	b, err := json.Marshal(v)
	json.Unmarshal(b, &mRelayTo)
	inRelayTo := mRelayTo.MRelayTo
	originEventID := inRelayTo.EventID
	var originEv gomatrixserverlib.ClientEvent
	stream := s.roomHistoryTimeLine.GetStreamEv(ctx, ev.RoomID, originEventID)
	if stream != nil {
		originEv = *stream.Ev
	} else {
		evs, err := s.db.Events(context.TODO(), []string{originEventID})
		if err == nil && len(evs) > 0 {
			originEv = evs[0]
		} else {
			if err != nil {
				log.Errorf("eventID:%s InRelayTo origin eventID:%s get from db err:%v", ev.EventID, originEventID, err)
			} else {
				log.Warnf("can not found eventID:%s InRelayTo origin eventID:%s", ev.EventID, originEventID)
			}
			return
		}
	}
	unsigned := types.Unsigned{}
	if originEv.Unsigned != nil {
		err = json.Unmarshal(originEv.Unsigned, &unsigned)
		if err != nil {
			log.Errorf("json.Unmarshal eventID:%s InRelayTo origin eventID:%s unsigned err:%v", ev.EventID, originEventID, err)
			return
		}
	}
	if unsigned.Relations != nil {
		if unsigned.Relations.RelayTo == nil {
			unsigned.Relations.RelayTo = &types.OriginInRelayTo{}
			unsigned.Relations.RelayTo.Chunk = []string{ev.EventID}
		} else {
			if unsigned.Relations.RelayTo.Chunk == nil {
				unsigned.Relations.RelayTo.Chunk = []string{ev.EventID}
			} else {
				unsigned.Relations.RelayTo.Chunk = append(unsigned.Relations.RelayTo.Chunk, ev.EventID)
			}
		}
	} else {
		unsigned.Relations = &types.EventRelations{}
		unsigned.Relations.RelayTo = &types.OriginInRelayTo{}
		unsigned.Relations.RelayTo.Chunk = []string{ev.EventID}
	}
	unsignedBytes, err := json.Marshal(unsigned)
	if err != nil {
		log.Errorf("json.Unmarshal eventID:%s InRelayTo origin eventID:%s unsigned err:%v", ev.EventID, originEventID, err)
		return
	}
	originEv.Unsigned = unsignedBytes
	if stream != nil {
		stream.Ev = &originEv
	}
	log.Infof("eventID:%s InRelayTo origin eventID:%s succ", ev.EventID, originEventID)
}

func (s *RoomEventFeedConsumer) processReactionEv(ctx context.Context, ev *gomatrixserverlib.ClientEvent) {
	var content map[string]interface{}
	err := json.Unmarshal(ev.Content, &content)
	if err != nil {
		log.Errorf("processReactionEv Unmarshal roomId:%s eventId:%s err:%v", ev.RoomID, ev.EventID, err)
		return
	}
	v, ok := content["m.relates_to"]
	if !ok {
		return
	}
	reaction := types.ReactionContent{}
	b, err := json.Marshal(v)
	json.Unmarshal(b, &reaction)
	originEventID := reaction.EventID
	var originEv gomatrixserverlib.ClientEvent
	stream := s.roomHistoryTimeLine.GetStreamEv(ctx, ev.RoomID, originEventID)
	if stream != nil {
		originEv = *stream.Ev
	} else {
		evs, err := s.db.Events(context.TODO(), []string{originEventID})
		if err == nil && len(evs) > 0 {
			originEv = evs[0]
		} else {
			if err != nil {
				log.Errorf("eventID:%s annotation origin eventID:%s get from db err:%v", ev.EventID, originEventID, err)
			} else {
				log.Warnf("can not found eventID:%s annotation origin eventID:%s", ev.EventID, originEventID)
			}
			return
		}
	}
	unsigned := types.Unsigned{}
	if originEv.Unsigned != nil {
		err = json.Unmarshal(originEv.Unsigned, &unsigned)
		if err != nil {
			log.Errorf("json.Unmarshal eventID:%s annotation origin eventID:%s unsigned err:%v", ev.EventID, originEventID, err)
			return
		}
	}
	if unsigned.Relations != nil {
		if unsigned.Relations.Anno == nil {
			unsigned.Relations.Anno = &types.Annotations{}
			annotation := &types.Annotation{
				Type:  ev.Type,
				Key:   reaction.Key,
				Count: 1,
			}
			unsigned.Relations.Anno.Chunk = []*types.Annotation{annotation}
		} else {
			if unsigned.Relations.Anno.Chunk == nil {
				annotation := &types.Annotation{
					Type:  ev.Type,
					Key:   reaction.Key,
					Count: 1,
				}
				unsigned.Relations.Anno.Chunk = []*types.Annotation{annotation}
			} else {
				hasExsit := false
				for _, item := range unsigned.Relations.Anno.Chunk {
					if item.Key == reaction.Key {
						item.Count++
						hasExsit = true
						break
					}
				}
				if !hasExsit {
					annotation := &types.Annotation{
						Type:  ev.Type,
						Key:   reaction.Key,
						Count: 1,
					}
					unsigned.Relations.Anno.Chunk = append(unsigned.Relations.Anno.Chunk, annotation)
				}
			}
		}
	} else {
		unsigned.Relations = &types.EventRelations{}
		unsigned.Relations.Anno = &types.Annotations{}
		annotation := &types.Annotation{
			Type:  ev.Type,
			Key:   reaction.Key,
			Count: 1,
		}
		unsigned.Relations.Anno.Chunk = []*types.Annotation{annotation}
	}
	unsignedBytes, err := json.Marshal(unsigned)
	if err != nil {
		log.Errorf("json.Unmarshal eventID:%s annotation origin eventID:%s unsigned err:%v", ev.EventID, originEventID, err)
		return
	}
	originEv.Unsigned = unsignedBytes
	if stream != nil {
		stream.Ev = &originEv
	}
	log.Infof("eventID:%s annotation origin eventID:%s succ", ev.EventID, originEventID)
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
	bs := time.Now().UnixNano()/1000000
	ev := msg.Event
	log.Infof("feedserver onNewRoomEvent start roomID:%s eventID:%s sender:%s type:%s eventoffset:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset)
	domain, _ := common.DomainFromID(ev.Sender)
	if ev.Type != "m.room.create" {
		bs := time.Now().UnixNano() / 1000000
		s.roomStateTimeLine.GetStates(ctx, ev.RoomID)       //load state timeline& state
		s.roomStateTimeLine.GetStateStreams(ctx, ev.RoomID) //load state stream timeline& state
		s.roomHistoryTimeLine.LoadHistory(ctx, ev.RoomID, true)
		s.receiptDataStreamRepo.LoadHistory(ctx, ev.RoomID, false)
		preOffset := s.roomHistoryTimeLine.GetDomainMaxStream(ctx, ev.RoomID, domain)
		if preOffset != -1 && preOffset+1 != ev.DomainOffset {
			log.Infof("feedserver onNewRoomEvent SetRoomMinStream roomID:%s preOffset:%d domainOffset:%d eventOffset:%d", ev.RoomID, preOffset,ev.DomainOffset, ev.EventOffset)
			s.roomHistoryTimeLine.SetRoomMinStream(ev.RoomID, ev.EventOffset)
		}
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("feedserver onNewRoomEvent load room state roomID:%s eventID:%s sender:%s type:%s eventoffset:%d spend:%dms", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset, spend)
	}
	s.roomHistoryTimeLine.SetDomainMaxStream(ev.RoomID, domain, ev.DomainOffset)

	if common.IsStateClientEv(&ev) == true { //state ev
		ev, _ = s.processStateEv(&ev)
	} else if ev.Type == "m.room.redaction" || ev.Type == "m.room.update" {
		s.processRedactEv(ctx, &ev)
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
	if ev.Type == "m.room.message" || ev.Type == "m.room.encrypted" {
		s.processMessageEv(ctx, &ev)
	}
	if ev.Type == "m.reaction" {
		s.processReactionEv(ctx, &ev)
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
		s.roomStateTimeLine.AddEv(ctx, &ev, ev.EventOffset, true)       //state 也会更新
		s.roomStateTimeLine.AddStreamEv(ctx, &ev, ev.EventOffset, true) //保留state stream

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
							func() {
								span, _ := common.StartSpanFromContext(ctx, s.cfg.Kafka.Producer.FedEduUpdate.Name)
								defer span.Finish()
								common.ExportMetricsBeforeSending(span, s.cfg.Kafka.Producer.FedEduUpdate.Name,
									s.cfg.Kafka.Producer.FedEduUpdate.Underlying)
								common.GetTransportMultiplexer().SendWithRetry(
									s.cfg.Kafka.Producer.FedEduUpdate.Underlying,
									s.cfg.Kafka.Producer.FedEduUpdate.Name,
									&core.TransportPubMsg{
										Keys:    stateKeyData,
										Obj:     edu,
										Headers: common.InjectSpanToHeaderForSending(span),
									})
							}()
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
								span, _ := common.StartSpanFromContext(ctx, s.cfg.Kafka.Producer.FedEduUpdate.Name)
								defer span.Finish()
								common.ExportMetricsBeforeSending(span, s.cfg.Kafka.Producer.FedEduUpdate.Name,
									s.cfg.Kafka.Producer.FedEduUpdate.Underlying)
								common.GetTransportMultiplexer().SendWithRetry(
									s.cfg.Kafka.Producer.FedEduUpdate.Underlying,
									s.cfg.Kafka.Producer.FedEduUpdate.Name,
									&core.TransportPubMsg{
										Keys:    []byte(key.(string)),
										Obj:     edu,
										Headers: common.InjectSpanToHeaderForSending(span),
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
	spend := time.Now().UnixNano() / 1000000 - bs
	s.roomHistoryTimeLine.AddEv(ctx, &ev, ev.EventOffset, true) //更新room timeline
	log.Infof("feedserver onNewRoomEvent add history timeline roomID:%s eventID:%s sender:%s type:%s eventoffset:%d spend:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset, spend)
	last := time.Now().UnixNano() / 1000000
	if s.cfg.CalculateReadCount {
		s.pushConsumer.DispthEvent(ctx, &ev)
	}
	now := time.Now().UnixNano() /1000000
	log.Infof("feedserver onNewRoomEvent pushConsumer.OnEvent roomID:%s eventID:%s sender:%s type:%s eventoffset:%d push spend:%d onNewRoomEvent spend:%d", ev.RoomID, ev.EventID, ev.Sender, ev.Type, ev.EventOffset, now-last, now-bs)
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
	preOffset := s.roomHistoryTimeLine.GetDomainMaxStream(ctx, ev.RoomID, domain)
	if preOffset <= ev.DomainOffset {
		log.Infof("feedserver onBackFillEvent SetRoomMinStream roomID:%s preOffset:%d domainOffset:%d eventOffset:%d", ev.RoomID, preOffset,ev.DomainOffset, ev.EventOffset)
		s.roomHistoryTimeLine.SetRoomMinStream(ev.RoomID, ev.EventOffset)
	}

	if common.IsStateClientEv(&ev) {
		ev, _ = s.processStateEv(&ev)
		s.roomStateTimeLine.AddBackfillEv(ctx, &ev, ev.EventOffset, true)
	} else if ev.Type == "m.room.redaction" || ev.Type == "m.room.update" {
		s.processRedactEv(ctx, &ev)
	}
	if ev.Type == "m.room.message" || ev.Type == "m.room.encrypted" {
		s.processMessageEv(ctx, &ev)
	}
	if ev.Type == "m.reaction" {
		s.processReactionEv(ctx, &ev)
	}
	return nil
}
