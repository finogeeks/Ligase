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
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"net/http"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/nats-io/go-nats"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type UnReadRpcConsumer struct {
	rpcClient    *common.RpcClient
	userTimeLine *repos.UserTimeLineRepo
	chanSize     uint32
	//msgChan      []chan *types.UnreadReqContent
	msgChan []chan common.ContextMsg
	cfg     *config.Dendrite
}

func NewUnReadRpcConsumer(
	rpcClient *common.RpcClient,
	userTimeLine *repos.UserTimeLineRepo,
	cfg *config.Dendrite,
) *UnReadRpcConsumer {
	s := &UnReadRpcConsumer{
		rpcClient:    rpcClient,
		userTimeLine: userTimeLine,
		chanSize:     4,
		cfg:          cfg,
	}

	return s
}

func (s *UnReadRpcConsumer) GetTopic() string {
	return types.UnreadReqTopicDef
}

func (s *UnReadRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result types.UnreadReqContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc unread cb error %v", err)
		return
	}
	if common.IsRelatedRequest(result.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
	}
}

func (s *UnReadRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.UnreadReqContent)
		s.processOnUnread(msg.Ctx, data.UserID, data.Reply)
	}
}

func (s *UnReadRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}

func (s *UnReadRpcConsumer) processOnUnread(ctx context.Context, userID, reply string) {
	joinMap, err := s.userTimeLine.GetJoinRooms(ctx, userID)
	if err != nil {
		resp := util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: jsonerror.Unknown(err.Error()),
		}
		s.rpcClient.PubObj(reply, resp)
		return
	}

	count := int64(0)
	countMap := new(sync.Map)
	requestMap := make(map[uint32]*syncapitypes.SyncUnreadRequest)
	if joinMap != nil {
		joinMap.Range(func(key, value interface{}) bool {
			instance := common.GetSyncInstance(key.(string), s.cfg.MultiInstance.SyncServerTotal)
			var request *syncapitypes.SyncUnreadRequest
			if data, ok := requestMap[instance]; ok {
				request = data
			} else {
				request = &syncapitypes.SyncUnreadRequest{}
				requestMap[instance] = request
			}
			request.JoinRooms = append(request.JoinRooms, key.(string))
			request.UserID = userID
			return true
		})
	}

	var wg sync.WaitGroup
	for instance, syncReq := range requestMap {
		wg.Add(1)
		go func(
			instance uint32,
			syncReq *syncapitypes.SyncUnreadRequest,
			countMap *sync.Map,
		) {
			syncReq.SyncInstance = instance
			bytes, err := json.Marshal(*syncReq)
			if err == nil {
				data, err := s.rpcClient.Request(types.SyncUnreadTopicDef, bytes, 30000)
				if err == nil {
					var result syncapitypes.SyncUnreadResponse
					err = json.Unmarshal(data, &result)
					if err != nil {
						log.Errorf("sync unread response Unmarshal error %v", err)
					} else {
						countMap.Store(syncReq.SyncInstance, result.Count)
					}
				} else {
					log.Errorf("call rpc for syncServer unread user %s error %v", syncReq.UserID, err)
				}
			} else {
				log.Errorf("marshal call sync unread content error, user %s error %v", syncReq.UserID, err)
			}
			wg.Done()
		}(instance, syncReq, countMap)
	}
	wg.Wait()

	countMap.Range(func(key, value interface{}) bool {
		count = count + value.(int64)
		return true
	})

	resp := util.JSONResponse{
		Code: http.StatusOK,
		JSON: struct {
			Count int64 `json:"count"`
		}{count},
	}
	s.rpcClient.PubObj(reply, resp)
}
