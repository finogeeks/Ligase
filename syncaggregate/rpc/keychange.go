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
	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/repos"
	syncapi "github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/nats-io/nats.go"
	"net/http"
)

type KeyChangeRpcConsumer struct {
	rpcClient     *common.RpcClient
	keyChangeRepo *repos.KeyChangeStreamRepo
	userTimeLine  *repos.UserTimeLineRepo
	chanSize      uint32
	msgChan       []chan *types.KeyChangeContent
	cfg           *config.Dendrite
}

func NewKeyChangeRpcConsumer(
	keyChangeRepo *repos.KeyChangeStreamRepo,
	userTimeLine *repos.UserTimeLineRepo,
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *KeyChangeRpcConsumer {
	s := &KeyChangeRpcConsumer{
		keyChangeRepo: keyChangeRepo,
		userTimeLine:  userTimeLine,
		rpcClient:     rpcClient,
		chanSize:      4,
		cfg:           cfg,
	}

	return s
}

func (s *KeyChangeRpcConsumer) GetTopic() string {
	return types.KcTopicDef
}

func (s *KeyChangeRpcConsumer) cb(msg *nats.Msg) {
	var result types.KeyChangeContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc key change cb error %v", err)
		return
	}
	if common.IsRelatedRequest(result.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		s.msgChan[idx] <- &result
	}
}

func (s *KeyChangeRpcConsumer) startWorker(msgChan chan *types.KeyChangeContent) {
	for data := range msgChan {
		s.processKc(data.FromPos, data.ToPos, data.Reply, data.UserID)
	}
}

func (s *KeyChangeRpcConsumer) Start() error {
	s.msgChan = make([]chan *types.KeyChangeContent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.KeyChangeContent, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)

	return nil
}

func (s *KeyChangeRpcConsumer) processKc(fromPos, toPos int64, reply, userID string) {
	kcTimeLine := s.keyChangeRepo.GetHistory()
	if kcTimeLine == nil {
		resp := util.JSONResponse{
			Code: http.StatusOK,
			JSON: syncapi.KeyChanged{
				Changes: []string{},
			},
		}
		s.rpcClient.PubObj(reply, resp)
		return
	}

	var changedUserIDs []string
	changedUserIDs = []string{}

	kcMap := s.keyChangeRepo.GetHistory()
	if kcMap != nil {
		friendShipMap := s.userTimeLine.GetFriendShip(userID, true)
		if friendShipMap != nil {
			friendShipMap.Range(func(key, _ interface{}) bool {
				if val, ok := kcMap.Load(key.(string)); ok {
					feed := val.(*feedstypes.KeyChangeStream)
					if feed.GetOffset() >= fromPos && feed.GetOffset() <= toPos {
						changedUserIDs = append(changedUserIDs, key.(string))
					}
				}
				return true
			})
		}
	}

	resp := util.JSONResponse{
		Code: http.StatusOK,
		JSON: syncapi.KeyChanged{
			Changes: changedUserIDs,
		},
	}
	s.rpcClient.PubObj(reply, resp)
}
