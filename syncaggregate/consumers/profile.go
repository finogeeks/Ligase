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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
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

func (s *ProfileConsumer) checkUpdate(output *types.ProfileStreamUpdate) bool {
	//not only base prop change, update presence
	if !output.IsUpdateBase {
		return true
	}
	//only has base prop change, using /presence/{userID}/status update presence
	//get precense from cache has err, update presence
	presence, ok := s.cache.GetPresences(output.UserID)
	if !ok {
		return true
	}
	log.Infof("update base user:%s server status is:%s last presence:%s set presence:%s", output.UserID, presence.ServerStatus, output.Presence.LastPresence, output.Presence.Presence)
	//other base prop has change, update presence
	if output.Presence.LastStatusMsg != output.Presence.StatusMsg || output.Presence.LastExtStatusMsg != output.Presence.ExtStatusMsg {
		if output.Presence.Presence == "offline" && presence.ServerStatus != "offline" {
			output.Presence.Presence = presence.ServerStatus
		}
		s.cache.SetPresencesServerStatus(output.UserID,output.Presence.Presence)
		return true
	}
	//set presence is same to serverStatus, not update presence
	if output.Presence.Presence == presence.ServerStatus {
		return false
	}
	//use is online, set offline, not update presence
	if presence.ServerStatus != "offline" && output.Presence.Presence == "offline" {
		return false
	}
	s.cache.SetPresencesServerStatus(output.UserID,output.Presence.Presence)
	log.Infof("user:%s server status is:%s set precense from:%s to:%s", output.UserID, presence.ServerStatus, output.Presence.LastPresence, output.Presence.Presence)
	return true
}

func (s *ProfileConsumer) onMessage(ctx context.Context, output *types.ProfileStreamUpdate) {
	log.Infof("recv Profile update, user:%s presence:%v", output.UserID,  output.Presence.Presence)

	if !s.checkUpdate(output) {
		return
	}
	log.Infof("do Profile update, user:%s presence:%v", output.UserID,  output.Presence.Presence)
	event := &gomatrixserverlib.ClientEvent{}
	event.Type = "m.presence"
	event.Sender = output.UserID
	event.Content, _ = json.Marshal(output.Presence)

	eventJson, err := json.Marshal(event)
	if err != nil {
		log.Errorw("Marshal json error for profile update", log.KeysAndValues{"userID", output.UserID, "error", err})
		return
	}
	isRelate := common.IsRelatedRequest(output.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite)
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
	s.presenceStreamRepo.AddPresenceDataStream(ctx, &presenceStream, pos, true)

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
	senderDomain, _ := common.DomainFromID(output.UserID)
	if common.CheckValidDomain(senderDomain, s.cfg.Matrix.ServerName) && isRelate {
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
			State:     output.Presence.State,
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
			common.GetTransportMultiplexer().SendWithRetry(
				s.cfg.Kafka.Producer.FedEduUpdate.Underlying,
				s.cfg.Kafka.Producer.FedEduUpdate.Name,
				&core.TransportPubMsg{
					Keys: userIDData,
					Obj:  edu,
				})
		}
	}
}
