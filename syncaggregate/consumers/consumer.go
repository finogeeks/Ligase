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
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type UtlEvent struct {
	Event        gomatrixserverlib.ClientEvent
	RelateJoined []string
	Joined       []string
}

type UtlContent struct {
	ev   *gomatrixserverlib.ClientEvent
	user string
}

type EventFeedConsumer struct {
	channel            core.IChannel
	db                 model.SyncAPIDatabase
	userTimeLine       *repos.UserTimeLineRepo
	presenceStreamRepo *repos.PresenceDataStreamRepo
	cfg                *config.Dendrite
	chanSize           uint32
	msgChan            []chan *UtlEvent
	utlChanSize        uint32
	utlChan            []chan *UtlContent
	cache              service.Cache
}

func NewEventFeedConsumer(
	cfg *config.Dendrite,
	store model.SyncAPIDatabase,
	cache service.Cache,
) *EventFeedConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputRoomEventSyncAggregate.Underlying,
		cfg.Kafka.Consumer.OutputRoomEventSyncAggregate.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &EventFeedConsumer{
			channel:     channel,
			db:          store,
			cfg:         cfg,
			chanSize:    64,
			utlChanSize: 64,
			cache:       cache,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *EventFeedConsumer) SetUserTimeLine(userTimeLine *repos.UserTimeLineRepo) *EventFeedConsumer {
	s.userTimeLine = userTimeLine
	return s
}

func (s *EventFeedConsumer) SetPresenceStreamRepo(presenceRepo *repos.PresenceDataStreamRepo) *EventFeedConsumer {
	s.presenceStreamRepo = presenceRepo
	return s
}

func (s *EventFeedConsumer) startWorker(msgChan chan *UtlEvent) {
	for data := range msgChan {
		timespend := common.NewTimeSpend()
		log.Debugf("EventFeedConsumer start handler")
		s.onNewRoomEvent(context.TODO(), data)
		timespend.Logf(1000, "EventFeedConsumer end handler")
	}
}

func (s *EventFeedConsumer) startUtlWorker(utlChan chan *UtlContent) {
	for data := range utlChan {
		bs := time.Now().UnixNano() / 1000000
		s.onInsertUserTimeLine(data)
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("onInsertUserTimeLine roomID:%s user:%s eventID:%s event_offset:%d spend:%d", data.ev.RoomID, data.user, data.ev.EventID, data.ev.EventOffset, spend)
	}
}

func (s *EventFeedConsumer) dispthInsertUserTimeLine(ev *gomatrixserverlib.ClientEvent, user string) {
	idx := common.CalcStringHashCode(user) % s.utlChanSize
	s.utlChan[idx] <- &UtlContent{
		ev:   ev,
		user: user,
	}
}

func (s *EventFeedConsumer) onInsertUserTimeLine(data *UtlContent) {
	if adapter.GetDebugLevel() == adapter.DEBUG_LEVEL_DEBUG {
		delay := utils.GetRandomSleepSecondsForDebug()
		log.Debugf("roomId:%s event_id:%s user:%s sleep %fs", data.ev.RoomID, data.ev.EventID, data.user, delay)
		time.Sleep(time.Duration(delay*1000) * time.Millisecond)
	}
	s.userTimeLine.AddP2PEv(data.ev, data.user)
}

func (s *EventFeedConsumer) Start() error {
	s.msgChan = make([]chan *UtlEvent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *UtlEvent, 512)
		go s.startWorker(s.msgChan[i])
	}
	s.utlChan = make([]chan *UtlContent, s.chanSize)
	for i := uint32(0); i < s.utlChanSize; i++ {
		s.utlChan[i] = make(chan *UtlContent, 20480)
		go s.startUtlWorker(s.utlChan[i])
	}
	//s.channel.Start()
	return nil
}

func (s *EventFeedConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	timespend := common.NewTimeSpend()
	log.Debugf("EventFeedConsumer onMessage start")
	var output roomserverapi.OutputEvent
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("sync aggregate: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	log.Infow("sync aggregate received data", log.KeysAndValues{"type", output.Type, "topic", topic})

	switch output.Type {
	case roomserverapi.OutputTypeNewRoomEvent:
		s.userTimeLine.UpdateRoomOffset(output.NewRoomEvent.Event.RoomID, output.NewRoomEvent.Event.EventOffset)
		log.Infof("onNewRoomEvent onMessage roomID:%s eventID:%s eventOffset:%d", output.NewRoomEvent.Event.RoomID, output.NewRoomEvent.Event.EventID, output.NewRoomEvent.Event.EventOffset)
		utlEvent := &UtlEvent{
			Event:        output.NewRoomEvent.Event,
			RelateJoined: []string{},
			Joined:       []string{},
		}
		for _, user := range output.NewRoomEvent.Joined {
			utlEvent.Joined = append(utlEvent.Joined, user)
			if common.IsRelatedRequest(user, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
				utlEvent.RelateJoined = append(utlEvent.RelateJoined, user)
			}
		}
		if len(utlEvent.RelateJoined) > 0 || (len(utlEvent.RelateJoined) <= 0 && utlEvent.Event.Type == "m.room.member") {
			log.Debugf("sync aggregate received data instance:%d IsRelatedRequest event_id:%s room:%s RelateJoined len:%d", s.cfg.MultiInstance.Instance, utlEvent.Event.EventID, utlEvent.Event.RoomID, len(utlEvent.RelateJoined))
			idx := common.CalcStringHashCode(utlEvent.Event.RoomID) % s.chanSize
			s.msgChan[idx] <- utlEvent
		} else {
			log.Debugf("sync aggregate received data instance:%d not IsRelatedRequest and not m.room.member event_id:%s room:%s joined:%v", s.cfg.MultiInstance.Instance, utlEvent.Event.EventID, utlEvent.Event.RoomID, output.NewRoomEvent.Joined)
		}
	default:
		log.Debugw("sync aggregate: ignoring unknown output type", log.KeysAndValues{"type", output.Type})
	}
	timespend.Logf(1000, "EventFeedConsumer onMessage end")
}

func (s *EventFeedConsumer) onNewRoomEvent(
	ctx context.Context, msg *UtlEvent,
) error {
	defer func() {
		if e := recover(); e != nil {
			stack := common.PanicTrace(4)
			log.Panicf("%v\n%s\n", e, stack)
		}
	}()
	bs := time.Now().UnixNano() / 1000000
	s.addUserTimeLineEvent(&msg.Event, msg.RelateJoined, msg.Joined)
	spend := time.Now().UnixNano()/1000000 - bs
	log.Infof("onNewRoomEvent addUserTimeLineEvent roomID:%s eventID:%s eventOffset:%d spend:%d", msg.Event.RoomID, msg.Event.EventID, msg.Event.EventOffset, spend)
	return nil
}

func (s *EventFeedConsumer) addUserTimeLineEvent(ev *gomatrixserverlib.ClientEvent, relateUsers []string, users []string) {
	var updateProfileUser map[string]map[string]struct{}
	if ev.Type == "m.room.member" && ev.StateKey != nil {
		member := external.MemberContent{}
		json.Unmarshal(ev.Content, &member)
		if member.Membership != "join" {
			if common.IsRelatedRequest(*ev.StateKey, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
				bs := time.Now().UnixNano() / 1000000
				s.dispthInsertUserTimeLine(ev, *ev.StateKey)
				spend := time.Now().UnixNano()/1000000 - bs
				log.Debugf("m.room.member not join add to timeline roomId:%s event_id:%s user_id:%s spend:%dms", ev.RoomID, ev.EventID, *ev.StateKey, spend)
			}
		} else {
			bs := time.Now().UnixNano() / 1000000
			updateProfileUser = map[string]map[string]struct{}{}
			domain, _ := common.DomainFromID(*ev.StateKey)
			isSelfDomain := common.CheckValidDomain(domain, s.cfg.Matrix.ServerName)
			for _, member := range relateUsers {
				hasLoad, hasFriendship := s.userTimeLine.AddFriendShip(member, *ev.StateKey)
				domainCheck, _ := common.DomainFromID(member)
				isSelfDomainCheck := common.CheckValidDomain(domainCheck, s.cfg.Matrix.ServerName)
				if (!hasLoad || !hasFriendship) && isSelfDomainCheck != isSelfDomain {
					var domainA, userB string
					if isSelfDomain {
						domainA = domainCheck
						userB = *ev.StateKey
					} else {
						domainA = domain
						userB = member
					}
					m, ok := updateProfileUser[domainA]
					if !ok {
						m = map[string]struct{}{}
						updateProfileUser[domainA] = m
					}
					m[userB] = struct{}{}
				}
			}
			if common.IsRelatedRequest(*ev.StateKey, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
				for _, member := range users {
					s.userTimeLine.AddFriendShip(*ev.StateKey, member)
				}
			}
			spend := time.Now().UnixNano()/1000000 - bs
			log.Debugf("addUserTimeLineEvent update friends ship roomID:%s eventID:%s spend:%d", ev.RoomID, ev.EventID, spend)
		}
	}
	bs := time.Now().UnixNano() / 1000000
	for _, user := range relateUsers {
		s.dispthInsertUserTimeLine(ev, user)
	}
	spend := time.Now().UnixNano()/1000000 - bs
	log.Debugf("update relateUsers roomID:%s eventID:%s len(relateUsers):%d spend:%d", ev.RoomID, ev.EventID, len(relateUsers), spend)
	if updateProfileUser != nil {
		bs := time.Now().UnixNano() / 1000000
		for domain, users := range updateProfileUser {
			for user := range users {
				feed := s.presenceStreamRepo.GetHistoryByUserID(user)
				if feed != nil {
					senderDomain, _ := common.DomainFromID(user)

					var presenceEvent gomatrixserverlib.ClientEvent
					var presenceContent types.PresenceJSON
					json.Unmarshal(feed.DataStream.Content, &presenceEvent)
					json.Unmarshal(presenceEvent.Content, &presenceContent)

					fedProfile := types.ProfileContent{
						UserID:      user,
						DisplayName: presenceContent.DisplayName,
						AvatarUrl:   presenceContent.AvatarURL,
						Presence:    presenceContent.Presence,
						UserName:    presenceContent.UserName,
						JobNumber:   presenceContent.JobNumber,
						Mobile:      presenceContent.Mobile,
						Landline:    presenceContent.Landline,
						Email:       presenceContent.Email,
						State:       presenceContent.State,
					}
					content, _ := json.Marshal(fedProfile)
					log.Debugf("send profile to new domain, user:%s, domain:%s, profile:%s", user, domain, content)
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
							Keys: []byte(user),
							Obj:  edu,
						})
				}
			}
		}
		spend := time.Now().UnixNano()/1000000 - bs
		log.Debugf("update updateProfileUser roomID:%s eventID:%s spend:%d", ev.RoomID, ev.EventID, spend)
	}
}
