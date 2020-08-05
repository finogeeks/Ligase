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
	"github.com/finogeeks/ligase/model/types"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/go-nats"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// consumes events that originated in the client api server.
type InputRoomEventConsumer struct {
	channel   core.IChannel
	cfg       *config.Dendrite
	input     roomserverapi.RoomserverInputAPI
	rpcClient *common.RpcClient
	idg       *uid.UidGenerator

	chanSize uint32
	//msgChan  []chan *roomserverapi.RawEvent
	msgChan []chan common.ContextMsg
}

func NewInputRoomEventConsumer(
	cfg *config.Dendrite,
	input roomserverapi.RoomserverInputAPI,
	client *common.RpcClient,
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
			channel:   channel,
			cfg:       cfg,
			input:     input,
			rpcClient: client,
			idg:       idg,
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (s *InputRoomEventConsumer) Start() error {
	s.chanSize = 8
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 4096)
		go s.startWorker(s.msgChan[i])
	}

	/*if s.useKafkaInput {
		s.channel.Start()
	}*/

	s.rpcClient.ReplyGrpWithContext(s.cfg.Rpc.RoomInputTopic, types.ROOMINPUT_RPC_GROUP, s.cb)
	return nil
}

func (s *InputRoomEventConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		err := s.processEvent(msg.Ctx, msg.Msg.(*roomserverapi.RawEvent))
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
	span := common.StartSpanFromMsgAfterReceived(topic, rawMsg)
	span.Finish()
	ctx = common.ContextWithSpan(ctx, span)
	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &input}
	//log.Infof("InputRoomEventConsumer room:%s write slot:%d all:%d", roomId, idx, s.chanSize)

	return
}

//when nats, write data to chan, when process done need to replay, called by nats transport
func (s *InputRoomEventConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var input roomserverapi.RawEvent
	if err := json.Unmarshal(msg.Data, &input); err != nil {
		log.Errorf("cb: parse failure %v, input:%s", err, string(msg.Data))
		return
	}

	input.Reply = msg.Reply
	roomId := input.RoomID

	log.Infow("rpc received data from client api server", log.KeysAndValues{
		"roomid", roomId, "crc64", common.CalcStringHashCode(roomId), "reply", input.Reply,
		"len", len(input.BulkEvents.Events),
	})

	for _, event := range input.BulkEvents.Events {
		log.Infof("rpc room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID(), event.EventID(), event.DomainOffset(), event.OriginServerTS(), event.Depth())
	}

	idx := common.CalcStringHashCode(roomId) % s.chanSize

	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &input}
	return
}

//worker, replay when needed
func (s *InputRoomEventConsumer) processEvent(ctx context.Context, input *roomserverapi.RawEvent) error {
	log.Infof("------------------------client-api processEvent start roomId:%s len:%d", input.RoomID, len(input.BulkEvents.Events))
	begin := time.Now()
	last := begin
	var respResult roomserverapi.InputRoomEventsResponse
	for _, event := range input.BulkEvents.Events {
		log.Infof("processEvent input room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID(), event.EventID(), event.DomainOffset(), event.OriginServerTS(), event.Depth())
	}
	n, err := s.input.InputRoomEvents(ctx, input)
	respResult.N = n
	if err != nil {
		if input.Reply != "" {
			respResult.ErrCode = -1
			respResult.ErrMsg = err.Error()
			s.rpcClient.PubObj(input.Reply, respResult)
		}
		return err
	}

	log.Infof("------------------------client-api processEvent use %v", time.Now().Sub(last))
	last = time.Now()

	if input.Reply != "" {
		respResult.ErrCode = 0
		s.rpcClient.PubObj(input.Reply, respResult)
	}
	return nil
}
