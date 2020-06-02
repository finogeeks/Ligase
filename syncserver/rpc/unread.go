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

type SyncUnreadRpcConsumer struct {
	rpcClient *common.RpcClient
	chanSize  uint32
	//msgChan       []chan *syncapitypes.SyncUnreadRequest
	msgChan       []chan common.ContextMsg
	readCountRepo *repos.ReadCountRepo
	cfg           *config.Dendrite
}

func NewSyncUnreadRpcConsumer(
	rpcClient *common.RpcClient,
	readCountRepo *repos.ReadCountRepo,
	cfg *config.Dendrite,
) *SyncUnreadRpcConsumer {
	s := &SyncUnreadRpcConsumer{
		rpcClient:     rpcClient,
		readCountRepo: readCountRepo,
		chanSize:      16,
		cfg:           cfg,
	}

	return s
}

func (s *SyncUnreadRpcConsumer) GetCB() common.MsgHandlerWithContext {
	return s.cb
}

func (s *SyncUnreadRpcConsumer) GetTopic() string {
	return types.SyncUnreadTopicDef
}

func (s *SyncUnreadRpcConsumer) Clean() {
}

func (s *SyncUnreadRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result syncapitypes.SyncUnreadRequest
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc unread cb error %v", err)
		return
	}
	if common.IsRelatedSyncRequest(result.SyncInstance, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
	}
}

func (s *SyncUnreadRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*syncapitypes.SyncUnreadRequest)
		s.onUnreadRequest(msg.Ctx, data)
	}
}

func (s *SyncUnreadRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}

func (s *SyncUnreadRpcConsumer) onUnreadRequest(ctx context.Context, req *syncapitypes.SyncUnreadRequest) {
	count := int64(0)
	for _, roomID := range req.JoinRooms {
		unread, _ := s.readCountRepo.GetRoomReadCount(roomID, req.UserID)
		count = count + unread
	}
	result := syncapitypes.SyncUnreadResponse{
		Count: count,
	}
	s.rpcClient.PubObj(req.Reply, result)
}
