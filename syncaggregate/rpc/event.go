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
	"fmt"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/types"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/nats-io/go-nats"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type EventRpcConsumer struct {
	rpcClient    *common.RpcClient
	userTimeLine *repos.UserTimeLineRepo
	db           model.SyncAPIDatabase
	chanSize     uint32
	msgChan      []chan *types.EventContent
	cfg          *config.Dendrite
}

func NewEventRpcConsumer(
	rpcClient *common.RpcClient,
	userTimeLine *repos.UserTimeLineRepo,
	syncDB model.SyncAPIDatabase,
	cfg *config.Dendrite,
) *EventRpcConsumer {
	s := &EventRpcConsumer{
		rpcClient:    rpcClient,
		userTimeLine: userTimeLine,
		db:           syncDB,
		chanSize:     4,
		cfg:          cfg,
	}

	return s
}

func (s *EventRpcConsumer) GetTopic() string {
	return types.EventTopicDef
}

func (s *EventRpcConsumer) cb(msg *nats.Msg) {
	var result types.EventContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc event cb error %v", err)
		return
	}
	if common.IsRelatedRequest(result.UserID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		s.msgChan[idx] <- &result
	}
}

func (s *EventRpcConsumer) startWorker(msgChan chan *types.EventContent) {
	for data := range msgChan {
		s.processOnEvent(data.EventID, data.UserID, data.Reply)
	}
}

func (s *EventRpcConsumer) Start() error {
	s.msgChan = make([]chan *types.EventContent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.EventContent, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)

	return nil
}

func (s *EventRpcConsumer) processOnEvent(eventID, userID, reply string) {
	//TODO 可见性校验
	event, _, err := s.db.StreamEvents(context.TODO(), []string{eventID})
	if err != nil {
		resp := util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: jsonerror.Unknown(err.Error()),
		}
		s.rpcClient.PubObj(reply, resp)
		return
	}

	if len(event) > 0 {
		roomID := event[0].RoomID
		joined, err := s.userTimeLine.GetJoinRooms(userID)
		if err != nil {
			resp := util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.NotFound(fmt.Sprintf("Could not find user joined rooms %s", userID)),
			}
			s.rpcClient.PubObj(reply, resp)
			return
		}

		isJoin := false
		if joined != nil {
			if _, ok := joined.Load(roomID); ok {
				isJoin = true
			}
		}

		if isJoin == false {
			resp := util.JSONResponse{
				Code: http.StatusForbidden,
				JSON: jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room or just forget the room"),
			}
			s.rpcClient.PubObj(reply, resp)
			return
		}

		resp := util.JSONResponse{
			Code: http.StatusOK,
			JSON: event[0],
		}
		s.rpcClient.PubObj(reply, resp)
		return
	}

	resp := util.JSONResponse{
		Code: http.StatusNotFound,
		JSON: jsonerror.NotFound(fmt.Sprintf("Could not find event %s", eventID)),
	}
	s.rpcClient.PubObj(reply, resp)
}
