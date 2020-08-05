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
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncserver/consumers"
	"github.com/nats-io/go-nats"
)

type ReceiptRpcConsumer struct {
	rpcClient       *common.RpcClient
	receiptConsumer *consumers.ReceiptConsumer
	chanSize        uint32
	//msgChan         []chan *types.ReceiptContent
	msgChan []chan common.ContextMsg
	cfg     *config.Dendrite
}

func NewReceiptRpcConsumer(
	receiptConsumer *consumers.ReceiptConsumer,
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *ReceiptRpcConsumer {
	s := &ReceiptRpcConsumer{
		receiptConsumer: receiptConsumer,
		rpcClient:       rpcClient,
		chanSize:        16,
		cfg:             cfg,
	}

	return s
}

func (s *ReceiptRpcConsumer) GetCB() common.MsgHandlerWithContext {
	return s.cb
}

func (s *ReceiptRpcConsumer) GetTopic() string {
	return types.ReceiptTopicDef
}

func (s *ReceiptRpcConsumer) Clean() {
}

func (s *ReceiptRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result types.ReceiptContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc receipt cb error %v", err)
		return
	}
	if common.IsRelatedRequest(result.RoomID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		idx := common.CalcStringHashCode(result.RoomID) % s.chanSize
		s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
	}
}

func (s *ReceiptRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.ReceiptContent)
		data.Source = "rpc"
		s.receiptConsumer.OnReceipt(msg.Ctx, data)
	}
}

func (s *ReceiptRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}
