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
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/go-nats"
	"strconv"
)

type KeyUpdateRpcConsumer struct {
	rpcClient     *common.RpcClient
	keyChangeRepo *repos.KeyChangeStreamRepo
	chanSize      uint32
	//msgChan       []chan *types.KeyUpdateContent
	msgChan   []chan common.ContextMsg
	cfg       *config.Dendrite
	keyFilter *filter.Filter
}

func NewKeyUpdateRpcConsumer(
	keyChangeRepo *repos.KeyChangeStreamRepo,
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *KeyUpdateRpcConsumer {
	s := &KeyUpdateRpcConsumer{
		keyChangeRepo: keyChangeRepo,
		rpcClient:     rpcClient,
		chanSize:      2,
		cfg:           cfg,
	}

	keyFilter := filter.GetFilterMng().Register("keySender", nil)
	s.keyFilter = keyFilter

	return s
}

func (s *KeyUpdateRpcConsumer) GetTopic() string {
	return types.KeyUpdateTopicDef
}

func (s *KeyUpdateRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result types.KeyUpdateContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc key update cb error %v", err)
		return
	}
	result.Reply = msg.Reply
	idx := common.CalcStringHashCode(result.Type) % s.chanSize
	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
}

func (s *KeyUpdateRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.KeyUpdateContent)
		s.processKeyUpdate(msg.Ctx, data)
	}
}

func (s *KeyUpdateRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}

func (s *KeyUpdateRpcConsumer) processKeyUpdate(ctx context.Context, data *types.KeyUpdateContent) {
	switch data.Type {
	case types.ONETIMEKEYUPDATE:
		if common.IsRelatedRequest(data.OneTimeKeyChangeUserId, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			s.keyChangeRepo.UpdateOneTimeKeyCount(data.OneTimeKeyChangeUserId, data.OneTimeKeyChangeDeviceId)
		}
	case types.DEVICEKEYUPDATE:
		if data.EventNID > 0 {
			if s.keyFilter.Lookup([]byte(strconv.FormatInt(data.EventNID, 10))) {
				return
			}
			s.keyFilter.Insert([]byte(strconv.FormatInt(data.EventNID, 10)))
		}
		for _, changed := range data.DeviceKeyChanges {
			keyStream := types.KeyChangeStream{
				ChangedUserID: changed.ChangedUserID,
			}
			s.keyChangeRepo.AddKeyChangeStream(ctx, &keyStream, changed.Offset, true)
		}
	default:
		return
	}
}
