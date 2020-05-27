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

package rpc

import (
	"context"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncaggregate/consumers"
	"github.com/json-iterator/go"
	"github.com/nats-io/go-nats"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TypingRpcConsumer struct {
	rpcClient *common.RpcClient
	consumer  *consumers.TypingConsumer
	chanSize  uint32
	//msgChan   []chan *syncapitypes.TypingUpdate
	msgChan []chan common.ContextMsg
	cfg     *config.Dendrite
}

func NewTypingRpcConsumer(
	consumer *consumers.TypingConsumer,
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *TypingRpcConsumer {
	s := &TypingRpcConsumer{
		consumer:  consumer,
		rpcClient: rpcClient,
		chanSize:  16,
		cfg:       cfg,
	}

	return s
}

func (s *TypingRpcConsumer) GetTopic() string {
	return types.TypingUpdateTopicDef
}

func (s *TypingRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result syncapitypes.TypingUpdate
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc typing update cb error %v", err)
		return
	}
	for _, user := range result.RoomUsers {
		if common.IsRelatedRequest(user, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			idx := common.CalcStringHashCode(result.UserID) % s.chanSize
			s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
			break
		}
	}
}

func (s *TypingRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*syncapitypes.TypingUpdate)
		s.processTypingUpdate(msg.Ctx, data)
	}
}

func (s *TypingRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}

func (s *TypingRpcConsumer) processTypingUpdate(ctx context.Context, data *syncapitypes.TypingUpdate) {
	s.consumer.AddRoomJoined(data.RoomID, data.RoomUsers)
	switch data.Type {
	case "add":
		s.consumer.AddTyping(data.RoomID, data.UserID)
	case "remove":
		s.consumer.RemoveTyping(data.RoomID, data.UserID)
	}
}
