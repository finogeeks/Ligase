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

package fedsender

import (
	"context"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/go-nats"
)

type EduSender struct {
	sender    *FederationSender
	cfg       *config.Fed
	channel   core.IChannel
	rpcClient *common.RpcClient
	chanSize  uint32
	//msgChan   []chan *gomatrixserverlib.EDU
	msgChan []chan common.ContextMsg
}

func NewEduSender(
	cfg *config.Fed,
	rpcClient *common.RpcClient,
) *EduSender {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.EduSenderInput.Underlying,
		cfg.Kafka.Consumer.EduSenderInput.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		eduSender := &EduSender{
			channel:   channel,
			cfg:       cfg,
			rpcClient: rpcClient,
			chanSize:  4,
		}
		channel.SetHandler(eduSender)
		return eduSender
	}
	return nil
}

func (e *EduSender) Start() error {
	e.msgChan = make([]chan common.ContextMsg, e.chanSize)
	for i := uint32(0); i < e.chanSize; i++ {
		e.msgChan[i] = make(chan common.ContextMsg, 512)
		go e.startWorker(e.msgChan[i])
	}

	e.rpcClient.ReplyWithContext(e.GetTopic(), e.cb)

	e.channel.Start()
	return nil
}

func (e *EduSender) SetSender(sender *FederationSender) {
	e.sender = sender
}

func (e *EduSender) GetCB() common.MsgHandlerWithContext {
	return e.cb
}

func (e *EduSender) GetTopic() string {
	return types.EduTopicDef
}

func (e *EduSender) Clean() {
}

func (e *EduSender) cb(ctx context.Context, msg *nats.Msg) {
	var result gomatrixserverlib.EDU
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc receipt cb error %v", err)
		return
	}

	idx := common.CalcStringHashCode(result.Destination) % e.chanSize
	e.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
}

func (e *EduSender) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*gomatrixserverlib.EDU)
		e.sender.sendEdu(msg.Ctx, data)
	}
}

func (e *EduSender) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	log.Infof("fed-edu-sender received data topic:%s, data:%s", topic, string(data))
	var output gomatrixserverlib.EDU
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorf("fed-dispatch: message parse failure err:%v", err)
		return
	}
	span := common.StartSpanFromMsgAfterReceived(topic, rawMsg)
	span.Finish()
	ctx = common.ContextWithSpan(ctx, span)
	e.sender.sendEdu(ctx, &output)
}
