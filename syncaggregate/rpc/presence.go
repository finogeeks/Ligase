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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/types"
	"github.com/nats-io/nats.go"
)

type PresenceRpcConsumer struct {
	rpcClient    *common.RpcClient
	cfg          *config.Dendrite
	chanSize     uint32
	msgChan      []chan *types.OnlinePresence
	olRepo       *repos.OnlineUserRepo
	presenceRepo *repos.PresenceDataStreamRepo
}

func NewPresenceRpcConsumer(
	rpcClient *common.RpcClient,
	olRepo *repos.OnlineUserRepo,
	presenceRepo *repos.PresenceDataStreamRepo,
	cfg *config.Dendrite,
) *PresenceRpcConsumer {
	s := &PresenceRpcConsumer{
		rpcClient:    rpcClient,
		olRepo:       olRepo,
		presenceRepo: presenceRepo,
		chanSize:     16,
		cfg:          cfg,
	}

	return s
}

func (s *PresenceRpcConsumer) GetCB() nats.MsgHandler {
	return s.cb
}

func (s *PresenceRpcConsumer) GetTopic() string {
	return types.PresenceTopicDef
}

func (s *PresenceRpcConsumer) Clean() {
}

func (s *PresenceRpcConsumer) cb(msg *nats.Msg) {
	var result types.OnlinePresence

	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc sync cb error %v", err)
		return
	}

	if common.IsRelatedRequest(result.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		s.msgChan[idx] <- &result
	}
}

func (s *PresenceRpcConsumer) startWorker(msgChan chan *types.OnlinePresence) {
	for data := range msgChan {
		s.getOnlinePresence(data)
	}
}

func (s *PresenceRpcConsumer) getOnlinePresence(data *types.OnlinePresence) {
	feed := s.presenceRepo.GetHistoryByUserID(data.UserID)
	if feed == nil {
		data.Found = false
		data.Presence = "offline"
	} else {
		data.Found = true
		var presenceEvent gomatrixserverlib.ClientEvent
		var presenceContent types.PresenceJSON
		json.Unmarshal(feed.DataStream.Content, &presenceEvent)
		json.Unmarshal([]byte(presenceEvent.Content), &presenceContent)
		data.Presence = presenceContent.Presence
		data.StatusMsg = presenceContent.StatusMsg
		data.ExtStatusMsg = presenceContent.ExtStatusMsg
	}
	s.rpcClient.PubObj(data.Reply, data)
}

func (s *PresenceRpcConsumer) Start() error {
	s.msgChan = make([]chan *types.OnlinePresence, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.OnlinePresence, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)
	return nil
}
