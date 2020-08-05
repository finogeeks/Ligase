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
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type ProfileConsumer struct {
	channel         core.IChannel
	displayNameRepo *repos.DisplayNameRepo
	chanSize        uint32
	msgChan         []chan *types.ProfileStreamUpdate
}

func NewProfileConsumer(
	cfg *config.Dendrite,
) *ProfileConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputProfileSyncServer.Underlying,
		cfg.Kafka.Consumer.OutputProfileSyncServer.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &ProfileConsumer{
			channel:  channel,
			chanSize: 4,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *ProfileConsumer) SetDisplayNameRepo(displayNameRepo *repos.DisplayNameRepo) *ProfileConsumer {
	s.displayNameRepo = displayNameRepo
	return s
}

func (s *ProfileConsumer) startWorker(msgChan chan *types.ProfileStreamUpdate) {
	for data := range msgChan {
		s.onMessage(data)
	}
}

func (s *ProfileConsumer) Start() error {
	s.msgChan = make([]chan *types.ProfileStreamUpdate, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.ProfileStreamUpdate, 512)
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
	s.msgChan[idx] <- &output
	return
}

func (s *ProfileConsumer) onMessage(output *types.ProfileStreamUpdate) {
	log.Infow("sync profile stream consumer received data", log.KeysAndValues{"userID", output.UserID})

	event := &gomatrixserverlib.ClientEvent{}
	event.Type = "m.presence"
	event.Sender = output.UserID
	event.Content, _ = json.Marshal(output.Presence)

	eventJson, err := json.Marshal(event)
	if err != nil {
		log.Errorw("Marshal json error for profile update", log.KeysAndValues{"userID", output.UserID, "error", err})
	}

	presenceStream := types.PresenceStream{}
	presenceStream.UserID = output.UserID
	presenceStream.Content = eventJson
	s.displayNameRepo.AddPresenceDataStream(&presenceStream)
	return
}
