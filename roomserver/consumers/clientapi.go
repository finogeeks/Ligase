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
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	jsoniter "github.com/json-iterator/go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// consumes events that originated in the client api server.
type InputRoomEventConsumer struct {
	channel core.IChannel
	cfg     *config.Dendrite
	input   roomserverapi.RoomserverInputAPI
	idg     *uid.UidGenerator

	chanSize uint32
	msgChan  []chan *roomserverapi.RawEvent
}

func NewInputRoomEventConsumer(
	cfg *config.Dendrite,
	input roomserverapi.RoomserverInputAPI,
) *InputRoomEventConsumer {
	idg, _ := uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	//input use kafka,output use nats
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.InputRoomEvent.Underlying,
		cfg.Kafka.Consumer.InputRoomEvent.Name,
	)

	if ok {
		channel := val.(core.IChannel)
		s := &InputRoomEventConsumer{
			channel: channel,
			cfg:     cfg,
			input:   input,
			idg:     idg,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *InputRoomEventConsumer) Start() error {
	s.chanSize = 8
	s.msgChan = make([]chan *roomserverapi.RawEvent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *roomserverapi.RawEvent, 4096)
		go s.startWorker(s.msgChan[i])
	}

	return nil
}

func (s *InputRoomEventConsumer) startWorker(msgChan chan *roomserverapi.RawEvent) {
	for msg := range msgChan {
		err := s.processEvent(msg)
		if err != nil {
			log.Errorw("process event error", log.KeysAndValues{"err", err})
		}
	}
	//log.Panicf("InputRoomEventConsumer out of loop")
}

//when kafka, write data to chan, called by kafka transport
func (s *InputRoomEventConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var input roomserverapi.RawEvent

	if err := json.Unmarshal(data, &input); err != nil {
		log.Errorf("OnMessage: parse failure %v, input:%s", err, string(data))
		return
	}

	roomId := input.RoomID
	log.Infow("kafka received data from client api server", log.KeysAndValues{"roomid", roomId, "crc64",
		common.CalcStringHashCode(roomId), "len", len(input.BulkEvents.Events)})
	for _, event := range input.BulkEvents.Events {
		log.Infof("rpc room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID(), event.EventID(), event.DomainOffset(), event.OriginServerTS(), event.Depth())
	}
	idx := common.CalcStringHashCode(roomId) % s.chanSize

	//log.Infof("InputRoomEventConsumer room:%s slot:%d all:%d have:%d cap:%d", roomId, idx, s.chanSize, len(s.msgChan[idx]), cap(s.msgChan[idx]))
	s.msgChan[idx] <- &input
	//log.Infof("InputRoomEventConsumer room:%s write slot:%d all:%d", roomId, idx, s.chanSize)

	return
}

//worker, replay when needed
func (s *InputRoomEventConsumer) processEvent(input *roomserverapi.RawEvent) error {
	log.Infof("------------------------client-api processEvent start roomId:%s len:%d", input.RoomID, len(input.BulkEvents.Events))
	begin := time.Now()
	last := begin
	var respResult roomserverapi.InputRoomEventsResponse

	//log.Infof("------------------------client-api processEvent start ev:%v", input.BulkEvents.Events)
	for _, event := range input.BulkEvents.Events {
		log.Infof("processEvent input room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID(), event.EventID(), event.DomainOffset(), event.OriginServerTS(), event.Depth())
	}
	n, err := s.input.InputRoomEvents(context.TODO(), input)
	respResult.N = n
	if err != nil {
		return err
	}

	log.Infof("------------------------client-api processEvent use %v", time.Now().Sub(last))
	last = time.Now()

	return nil
}
