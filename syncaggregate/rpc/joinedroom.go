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
	"fmt"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/model/repos"
	syncapi "github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/nats.go"
)

type JoinedRoomRpcConsumer struct {
	rpcClient    *common.RpcClient
	userTimeLine *repos.UserTimeLineRepo
	chanSize     uint32
	msgChan      []chan *types.JoinedRoomContent
	cfg          *config.Dendrite
}

func NewJoinedRoomRpcConsumer(
	rpcClient *common.RpcClient,
	userTimeLine *repos.UserTimeLineRepo,
	cfg *config.Dendrite,
) *JoinedRoomRpcConsumer {
	s := &JoinedRoomRpcConsumer{
		rpcClient:    rpcClient,
		userTimeLine: userTimeLine,
		chanSize:     16,
		cfg:          cfg,
	}

	return s
}

func (s *JoinedRoomRpcConsumer) GetTopic() string {
	return types.JoinedRoomTopicDef
}

func (s *JoinedRoomRpcConsumer) cb(msg *nats.Msg) {
	var result types.JoinedRoomContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc joined room cb error %v", err)
		return
	}

	if common.IsRelatedRequest(result.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		s.msgChan[idx] <- &result
	}
}

func (s *JoinedRoomRpcConsumer) startWorker(i uint32) {
	idx := i
	for data := range s.msgChan[idx] {
		s.processOnJoinedRoom(data.UserID, data.Reply)
	}
}

func (s *JoinedRoomRpcConsumer) Start() error {
	s.msgChan = make([]chan *types.JoinedRoomContent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.JoinedRoomContent, 512)
		go s.startWorker(i)
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)

	return nil
}

func (s *JoinedRoomRpcConsumer) processOnJoinedRoom(userID, reply string) {
	resp := syncapi.JoinedRoomsResp{}
	var err error
	resp.JoinedRooms, err = s.userTimeLine.GetJoinRoomsArr(userID)
	if err != nil {
		resp := util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: jsonerror.NotFound(fmt.Sprintf("Could not find user joined rooms %s", userID)),
		}
		s.rpcClient.PubObj(reply, resp)
		return
	}

	for _, roomId := range resp.JoinedRooms {
		log.Debugf("OnIncomingJoinedRoomMessagesRequest user:%s load join room :%s", userID, roomId)
	}

	respResult := util.JSONResponse{
		Code: http.StatusOK,
		JSON: resp,
	}
	s.rpcClient.PubObj(reply, respResult)
}
