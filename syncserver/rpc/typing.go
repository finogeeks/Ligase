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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/json-iterator/go"
	"github.com/nats-io/go-nats"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TypingRpcConsumer struct {
	roomCurState *repos.RoomCurStateRepo
	rpcClient    *common.RpcClient
	chanSize     uint32
	//msgChan      []chan *types.TypingContent
	msgChan []chan common.ContextMsg
	cfg     *config.Dendrite
}

func NewTypingRpcConsumer(
	roomCurState *repos.RoomCurStateRepo,
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *TypingRpcConsumer {
	s := &TypingRpcConsumer{
		roomCurState: roomCurState,
		rpcClient:    rpcClient,
		chanSize:     16,
		cfg:          cfg,
	}

	return s
}

func (s *TypingRpcConsumer) GetCB() common.MsgHandlerWithContext {
	return s.cb
}

func (s *TypingRpcConsumer) GetTopic() string {
	return types.TypingTopicDef
}

func (s *TypingRpcConsumer) Clean() {
}

func (s *TypingRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result types.TypingContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc typing cb error %v", err)
		return
	}
	if common.IsRelatedRequest(result.RoomID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		idx := common.CalcStringHashCode(result.RoomID) % s.chanSize
		s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
	}
}

func (s *TypingRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.TypingContent)
		state := s.roomCurState.GetRoomState(data.RoomID)
		if state != nil {
			update := syncapitypes.TypingUpdate{
				Type:   data.Type,
				UserID: data.UserID,
				RoomID: data.RoomID,
			}
			domainMap := make(map[string]bool)
			state.GetJoinMap().Range(func(key, value interface{}) bool {
				update.RoomUsers = append(update.RoomUsers, key.(string))
				domain, _ := common.DomainFromID(key.(string))
				if common.CheckValidDomain(domain, s.cfg.Matrix.ServerName) == false {
					domainMap[domain] = true
				}
				return true
			})

			bytes, err := json.Marshal(update)
			if err == nil {
				s.rpcClient.Pub(types.TypingUpdateTopicDef, bytes)
			} else {
				log.Errorf("TypingRpcConsumer pub typing update error %v", err)
			}

			senderDomain, _ := common.DomainFromID(data.UserID)
			if common.CheckValidDomain(senderDomain, s.cfg.Matrix.ServerName) {
				content, _ := json.Marshal(data)
				for domain := range domainMap {
					edu := gomatrixserverlib.EDU{
						Type:        "typing",
						Origin:      senderDomain,
						Destination: domain,
						Content:     content,
					}
					bytes, err := json.Marshal(edu)
					if err == nil {
						s.rpcClient.Pub(types.EduTopicDef, bytes)
					} else {
						log.Errorf("TypingRpcConsumer pub typing edu error %v", err)
					}
				}
			}
		}
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
