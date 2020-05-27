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
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncaggregate/sync"
	"github.com/nats-io/go-nats"
)

type SyncRpcConsumer struct {
	rpcClient *common.RpcClient
	sm        *sync.SyncMng
	chanSize  uint32
	//msgChan        []chan *types.SyncContent
	msgChan        []chan common.ContextMsg
	compressLength int64
	cfg            *config.Dendrite
}

func NewSyncRpcConsumer(
	rpcClient *common.RpcClient,
	sm *sync.SyncMng,
	cfg *config.Dendrite,
) *SyncRpcConsumer {
	s := &SyncRpcConsumer{
		rpcClient: rpcClient,
		sm:        sm,
		chanSize:  16,
		cfg:       cfg,
	}

	s.compressLength = config.DefaultCompressLength
	if cfg.CompressLength != 0 {
		s.compressLength = cfg.CompressLength
	}

	return s
}

func (s *SyncRpcConsumer) GetCB() common.MsgHandlerWithContext {
	return s.cb
}

func (s *SyncRpcConsumer) GetTopic() string {
	return types.SyncTopicDef
}

func (s *SyncRpcConsumer) Clean() {
}

func (s *SyncRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result types.SyncContent

	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc sync cb error %v", err)
		return
	}

	if common.IsRelatedRequest(result.Device.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.Device.UserID) % s.chanSize
		s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
	}
}

func (s *SyncRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.SyncContent)
		//必须异步，否则sync的等待机制会阻塞其他请求
		go s.callSync(msg.Ctx, data)
	}
}

func (s *SyncRpcConsumer) callSync(ctx context.Context, data *types.SyncContent) {
	code, coder := s.sm.OnSyncRequest(ctx, &data.Request, &data.Device)
	resp := util.JSONResponse{
		Code: code,
		JSON: coder,
	}
	contentBytes, _ := json.Marshal(resp)
	msgSize := int64(len(contentBytes))

	result := types.CompressContent{
		Compressed: false,
	}
	if msgSize > s.compressLength {
		contentBytes = common.DoCompress(contentBytes)
		result.Compressed = true
		log.Infof("nats pub message with content, before compress %d after compress %d", msgSize, len(contentBytes))
	}
	result.Content = contentBytes
	s.rpcClient.PubObj(data.Reply, result)
}

func (s *SyncRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)
	return nil
}
