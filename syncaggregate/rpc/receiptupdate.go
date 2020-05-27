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
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/go-nats"
)

type UpdateReceiptOffset struct {
	Users  []string
	Offset int64
}

type ReceiptUpdateRpcConsumer struct {
	rpcClient    *common.RpcClient
	userTimeLine *repos.UserTimeLineRepo
	chanSize     uint32
	//msgChan      []chan *UpdateReceiptOffset
	msgChan []chan common.ContextMsg
	cfg     *config.Dendrite
}

func NewReceiptUpdateRpcConsumer(
	userTimeLine *repos.UserTimeLineRepo,
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *ReceiptUpdateRpcConsumer {
	s := &ReceiptUpdateRpcConsumer{
		userTimeLine: userTimeLine,
		rpcClient:    rpcClient,
		chanSize:     16,
		cfg:          cfg,
	}

	return s
}

func (s *ReceiptUpdateRpcConsumer) GetTopic() string {
	return types.ReceiptUpdateTopicDef
}

func (s *ReceiptUpdateRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result syncapitypes.ReceiptUpdate
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc receipt update cb error %v", err)
		return
	}
	updateReceiptOffset := &UpdateReceiptOffset{
		Users:  []string{},
		Offset: result.Offset,
	}
	for _, user := range result.Users {
		if common.IsRelatedRequest(user, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			updateReceiptOffset.Users = append(updateReceiptOffset.Users, user)
		}
	}
	if len(updateReceiptOffset.Users) > 0 {
		idx := common.CalcStringHashCode(result.RoomID) % s.chanSize
		s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: updateReceiptOffset}
	}
}

func (s *ReceiptUpdateRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*UpdateReceiptOffset)
		s.processReceiptUpdate(msg.Ctx, data)
	}
}

func (s *ReceiptUpdateRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}

func (s *ReceiptUpdateRpcConsumer) processReceiptUpdate(ctx context.Context, data *UpdateReceiptOffset) {
	for _, userID := range data.Users {
		log.Infof("process update receipt user:%s offset:%d", userID, data.Offset)
		s.userTimeLine.SetReceiptLatest(userID, data.Offset)
	}
}
