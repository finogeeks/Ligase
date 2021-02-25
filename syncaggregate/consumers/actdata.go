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

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type ActDataConsumer struct {
	channel              core.IChannel
	db                   model.SyncAPIDatabase
	clientDataStreamRepo *repos.ClientDataStreamRepo
	chanSize             uint32
	msgChan              []chan *types.ActDataStreamUpdate
	cfg                  *config.Dendrite
}

func NewActDataConsumer(
	cfg *config.Dendrite,
	store model.SyncAPIDatabase,
) *ActDataConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputClientData.Underlying,
		cfg.Kafka.Consumer.OutputClientData.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &ActDataConsumer{
			channel:  channel,
			db:       store,
			cfg:      cfg,
			chanSize: 4,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *ActDataConsumer) SetClientDataStreamRepo(clientDataStreamRepo *repos.ClientDataStreamRepo) *ActDataConsumer {
	s.clientDataStreamRepo = clientDataStreamRepo
	return s
}

func (s *ActDataConsumer) startWorker(msgChan chan *types.ActDataStreamUpdate) {
	for data := range msgChan {
		s.onMessage(data)
	}
}

func (s *ActDataConsumer) Start() error {
	s.msgChan = make([]chan *types.ActDataStreamUpdate, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.ActDataStreamUpdate, 512)
		go s.startWorker(s.msgChan[i])
	}

	//s.channel.Start()
	return nil
}

func (s *ActDataConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	// Parse out the event JSON
	var output types.ActDataStreamUpdate
	if err := json.Unmarshal(data, &output); err != nil {
		// If the message was invalid, log it and move on to the next message in the stream
		log.Errorw("client API server output log: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	if common.IsRelatedRequest(output.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		idx := common.CalcStringHashCode(output.UserID) % s.chanSize
		s.msgChan[idx] <- &output
	}

	return
}

func (s *ActDataConsumer) onMessage(output *types.ActDataStreamUpdate) {
	if output.StreamType != "" {
		log.Infow("sync client data stream consumer received data", log.KeysAndValues{
			"userID", output.UserID, "roomID", output.RoomID, "dataType", output.DataType, "streamType", output.StreamType,
		})

		syncStreamPos, err := s.db.UpsertClientDataStream(
			context.TODO(), output.UserID, output.RoomID, output.DataType, output.StreamType,
		)
		if err != nil {
			log.Errorw("could not save account data stream", log.KeysAndValues{
				"userID", output.UserID, "roomID", output.RoomID, "dataType", output.DataType, "streamType", output.StreamType, "error", err,
			})
		}

		s.clientDataStreamRepo.AddClientDataStream(output, int64(syncStreamPos))
	} else {
		log.Errorw("sync client data stream consumer received data with nil stream", log.KeysAndValues{
			"userID", output.UserID, "roomID", output.RoomID, "dataType", output.DataType})
	}

	return
}
