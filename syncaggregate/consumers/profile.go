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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type ProfileConsumer struct {
	channel            core.IChannel
	presenceStreamRepo *repos.PresenceDataStreamRepo
	chanSize           uint32
	//msgChan            []chan *types.ProfileStreamUpdate
	msgChan      []chan common.ContextMsg
	userTimeLine *repos.UserTimeLineRepo
	olRepo       *repos.OnlineUserRepo
	cfg          *config.Dendrite
	syncDB       model.SyncAPIDatabase
	idg          *uid.UidGenerator
	cache        service.Cache
}

func NewProfileConsumer(
	cfg *config.Dendrite,
	userTimeLine *repos.UserTimeLineRepo,
	syncDB model.SyncAPIDatabase,
	idg *uid.UidGenerator,
	cache service.Cache,
) *ProfileConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputProfileSyncAggregate.Underlying,
		cfg.Kafka.Consumer.OutputProfileSyncAggregate.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &ProfileConsumer{
			channel:      channel,
			chanSize:     4,
			cfg:          cfg,
			userTimeLine: userTimeLine,
			syncDB:       syncDB,
			idg:          idg,
			cache:        cache,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *ProfileConsumer) SetPresenceStreamRepo(presenceRepo *repos.PresenceDataStreamRepo) *ProfileConsumer {
	s.presenceStreamRepo = presenceRepo
	return s
}

func (s *ProfileConsumer) SetOnlineUserRepo(ol *repos.OnlineUserRepo) {
	s.olRepo = ol
}

func (s *ProfileConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.ProfileStreamUpdate)
		s.onMessage(msg.Ctx, data)
	}
}

func (s *ProfileConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	//s.channel.Start()
	return nil
}

func (s *ProfileConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var output types.ProfileStreamUpdate
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("output profile consumer log: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	idx := common.CalcStringHashCode(output.UserID) % s.chanSize
	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &output}
	return
}

func (s *ProfileConsumer) onMessage(ctx context.Context, output *types.ProfileStreamUpdate) {
	domain, _ := common.DomainFromID(output.UserID)
	isSelfDomain := common.CheckValidDomain(domain, s.cfg.Matrix.ServerName)
	isRelate := common.IsRelatedRequest(output.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite)
	if (!isRelate && output.IsMasterHndle && isSelfDomain) || (isRelate && !output.IsMasterHndle && isSelfDomain) {
		return
	}

	log.Infof("Profile update, user:%s device:%s presence:%s %t", output.UserID, output.DeviceID, output.Presence.Presence, output.IsUpdateStauts)
	if isSelfDomain && isRelate && output.IsUpdateStauts {
		if output.Presence.Presence == "offline" {
			log.Infof("Profile offline user:%s device:%s", output.UserID, output.DeviceID)
			s.olRepo.UpdateState(ctx, output.UserID, output.DeviceID, repos.OFFLINE_STATE)
			if s.olRepo.IsUserOnline(output.UserID) && output.Presence.Presence == "offline" {
				// 整合在线状态
				deviceIDs, pos, ts := s.olRepo.GetRemainDevice(output.UserID)
				log.Infof("Profile update user:%s offline aggregate to online, deviceIDs:%v, pos:%v, ts:%v", output.UserID, deviceIDs, pos, ts)
				output.Presence.Presence = "online"
				feed := s.presenceStreamRepo.GetHistoryByUserID(output.UserID)
				if feed != nil {
					var ev gomatrixserverlib.ClientEvent
					var content types.PresenceJSON
					json.Unmarshal(feed.DataStream.Content, &ev)
					json.Unmarshal(ev.Content, &content)
					if content.Presence != "offline" {
						deviceIDs, pos, ts := s.olRepo.GetRemainDevice(output.UserID)
						log.Infof("Profile update user:%s online aggregate to %s, deviceIDs:%v, pos:%v, ts:%v", output.UserID, content.Presence, deviceIDs, pos, ts)
						output.Presence.Presence = content.Presence
					}
				}
				s.cache.SetPresences(output.UserID, output.Presence.Presence, output.Presence.StatusMsg, output.Presence.ExtStatusMsg)
			}
			// } else if output.Presence.Presence == "online" {
			// 	feed := s.presenceStreamRepo.GetHistoryByUserID(output.UserID)
			// 	if feed != nil {
			// 		var ev gomatrixserverlib.ClientEvent
			// 		var content types.PresenceJSON
			// 		json.Unmarshal(feed.DataStream.Content, &ev)
			// 		json.Unmarshal(ev.Content, &content)
			// 		if content.Presence != "offline" && content.Presence != "online" {
			// 			deviceIDs, pos, ts := s.olRepo.GetRemainDevice(output.UserID)
			// 			log.Infof("Profile update user:%s online aggregate to %s, deviceIDs:%v, pos:%v, ts:%v", output.UserID, content.Presence, deviceIDs, pos, ts)
			// 			output.Presence.Presence = content.Presence
			// 		}
			// 	}
		}
	}
	if isSelfDomain && isRelate && output.IsMasterHndle {
		stream := *output
		stream.IsMasterHndle = false
		span, _ := common.StartSpanFromContext(ctx, s.cfg.Kafka.Producer.OutputProfileData.Name)
		defer span.Finish()
		common.ExportMetricsBeforeSending(span, s.cfg.Kafka.Producer.OutputProfileData.Name,
			s.cfg.Kafka.Producer.OutputProfileData.Underlying)
		common.GetTransportMultiplexer().SendWithRetry(
			s.cfg.Kafka.Producer.OutputProfileData.Underlying,
			s.cfg.Kafka.Producer.OutputProfileData.Name,
			&core.TransportPubMsg{
				Keys:    []byte(output.UserID),
				Obj:     stream,
				Headers: common.InjectSpanToHeaderForSending(span),
			})
	}

	if !output.IsUpdateStauts {
		feed := s.presenceStreamRepo.GetHistoryByUserID(output.UserID)
		if feed != nil {
			var ev gomatrixserverlib.ClientEvent
			var content types.PresenceJSON
			json.Unmarshal(feed.DataStream.Content, &ev)
			json.Unmarshal(ev.Content, &content)
			output.Presence.Presence = content.Presence
			output.Presence.StatusMsg = content.StatusMsg
			output.Presence.ExtStatusMsg = content.ExtStatusMsg
		}
		if output.Presence.Presence == "" {
			output.Presence.Presence = "offline"
		}
	}

	event := &gomatrixserverlib.ClientEvent{}
	event.Type = "m.presence"
	event.Sender = output.UserID
	event.Content, _ = json.Marshal(output.Presence)

	eventJson, err := json.Marshal(event)
	if err != nil {
		log.Errorw("Marshal json error for profile update", log.KeysAndValues{"userID", output.UserID, "error", err})
		return
	}

	var pos int64
	if isRelate {
		offset, _ := s.syncDB.UpsertPresenceDataStream(ctx, output.UserID, string(eventJson))
		pos = int64(offset)
	} else {
		pos, _ = s.idg.Next()
	}

	presenceStream := types.PresenceStream{}
	presenceStream.UserID = output.UserID
	presenceStream.Content = eventJson
	log.Infow("sync profile stream consumer received data", log.KeysAndValues{"userID", output.UserID, "pos", pos, "eventJson", string(eventJson)})
	s.presenceStreamRepo.AddPresenceDataStream(ctx, &presenceStream, pos, true)
	if !isRelate || !isSelfDomain {
		friendReverseMap := s.userTimeLine.GetFriendshipReverse(output.UserID)
		if friendReverseMap != nil {
			friendReverseMap.Range(func(k, _ interface{}) bool {
				s.presenceStreamRepo.UpdateUserMaxPos(k.(string), pos)
				return true
			})
		}
	}

	friendMap := s.userTimeLine.GetFriendShip(ctx, output.UserID, true)
	domainMap := make(map[string]bool)
	if friendMap != nil {
		friendMap.Range(func(key, _ interface{}) bool {
			domain, _ := common.DomainFromID(key.(string))
			if !common.CheckValidDomain(domain, s.cfg.Matrix.ServerName) {
				domainMap[domain] = true
			}
			return true
		})
	}
	senderDomain := domain
	if isSelfDomain && isRelate && output.IsMasterHndle {
		fedProfile := types.ProfileContent{
			UserID:       output.UserID,
			DisplayName:  output.Presence.DisplayName,
			AvatarUrl:    output.Presence.AvatarURL,
			Presence:     output.Presence.Presence,
			StatusMsg:    output.Presence.StatusMsg,
			ExtStatusMsg: output.Presence.ExtStatusMsg,

			// ext user info
			UserName:  output.Presence.UserName,
			JobNumber: output.Presence.JobNumber,
			Mobile:    output.Presence.Mobile,
			Landline:  output.Presence.Landline,
			Email:     output.Presence.Email,
		}

		content, _ := json.Marshal(fedProfile)
		userIDData := []byte(output.UserID)
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
						Keys:    userIDData,
						Obj:     edu,
						Headers: common.InjectSpanToHeaderForSending(span),
					})
			}()
		}
	}
}
