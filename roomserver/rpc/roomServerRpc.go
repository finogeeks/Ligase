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
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/nats.go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RoomserverRpcConsumer struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	DB        model.RoomServerDatabase
	Repo      *repos.RoomServerCurStateRepo
	UmsRepo   *repos.RoomServerUserMembershipRepo
	Proc      roomserverapi.RoomserverQueryAPI

	msgChan1 chan *roomserverapi.RoomserverRpcRequest
	msgChan2 chan *roomserverapi.RoomserverRpcRequest
	msgChan3 chan *roomserverapi.RoomserverRpcRequest
	msgChan4 chan *roomserverapi.RoomserverRpcRequest
	msgChan5 chan *roomserverapi.RoomserverRpcRequest
	msgChan6 chan *roomserverapi.RoomserverRpcRequest
}

func NewRoomserverRpcConsumer(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	db model.RoomServerDatabase,
	repo *repos.RoomServerCurStateRepo,
	umsRepo *repos.RoomServerUserMembershipRepo,
	proc roomserverapi.RoomserverQueryAPI,
) *RoomserverRpcConsumer {
	s := &RoomserverRpcConsumer{
		cfg:       cfg,
		rpcClient: rpcClient,
		DB:        db,
		Repo:      repo,
		UmsRepo:   umsRepo,
		Proc:      proc,
	}

	s.msgChan1 = make(chan *roomserverapi.RoomserverRpcRequest, 1024)
	s.msgChan2 = make(chan *roomserverapi.RoomserverRpcRequest, 1024)
	s.msgChan3 = make(chan *roomserverapi.RoomserverRpcRequest, 1024)
	s.msgChan4 = make(chan *roomserverapi.RoomserverRpcRequest, 1024)
	s.msgChan5 = make(chan *roomserverapi.RoomserverRpcRequest, 1024)
	s.msgChan6 = make(chan *roomserverapi.RoomserverRpcRequest, 1024)
	return s
}

func (s *RoomserverRpcConsumer) Start() error {
	go func() {
		for data := range s.msgChan1 {
			s.processQueryRoomState(data.QueryRoomState, data.Reply)
		}
	}()

	go func() {
		for data := range s.msgChan2 {
			s.processQueryEventsByID(data.QueryEventsByID, data.Reply)
		}
	}()

	go func() {
		for data := range s.msgChan3 {
			s.processQueryRoomEventByID(data.QueryRoomEventByID, data.Reply)
		}
	}()

	go func() {
		for data := range s.msgChan4 {
			s.processQueryJoinRooms(data.QueryJoinRooms, data.Reply)
		}
	}()

	go func() {
		for data := range s.msgChan5 {
			s.processQueryBackFillEvents(data.QueryBackFillEvents, data.Reply)
		}
	}()

	go func() {
		for data := range s.msgChan6 {
			s.processQueryEventAuth(data.QueryEventAuth, data.Reply)
		}
	}()

	s.rpcClient.Reply(s.GetTopic(), s.cb)
	return nil
}

func (s *RoomserverRpcConsumer) GetTopic() string {
	return s.cfg.Rpc.RsQryTopic
}

func (s *RoomserverRpcConsumer) cb(msg *nats.Msg) {
	var request roomserverapi.RoomserverRpcRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Errorf("rpc roomserverqry unmarshal error %v", err)
		return
	}
	log.Debugf("RoomserverRpcConsumer on message, topic:%s  replay:%s data:%v", msg.Subject, msg.Reply, request)
	request.Reply = msg.Reply

	if request.QueryEventsByID != nil {
		s.msgChan2 <- &request
	} else if request.QueryRoomEventByID != nil {
		s.msgChan3 <- &request
	} else if request.QueryJoinRooms != nil {
		s.msgChan4 <- &request
	} else if request.QueryRoomState != nil {
		s.msgChan1 <- &request
	} else if request.QueryBackFillEvents != nil {
		s.msgChan5 <- &request
	} else if request.QueryEventAuth != nil {
		s.msgChan6 <- &request
	}
}

func (s *RoomserverRpcConsumer) processQueryRoomState(
	request *roomserverapi.QueryRoomStateRequest,
	reply string,
) {
	var response roomserverapi.QueryRoomStateResponse
	log.Infof("processQueryRoomState recv request recv data:%s", request.RoomID)
	s.Proc.QueryRoomState(context.Background(), request, &response)

	log.Infof("processQueryRoomState recv process room:%s, data:%v, reply:%s", request.RoomID, response, reply)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryEventsByID(
	request *roomserverapi.QueryEventsByIDRequest,
	reply string,
) {
	var response roomserverapi.QueryEventsByIDResponse
	s.Proc.QueryEventsByID(context.Background(), request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryRoomEventByID(
	request *roomserverapi.QueryRoomEventByIDRequest,
	reply string,
) {
	var response roomserverapi.QueryRoomEventByIDResponse
	s.Proc.QueryRoomEventByID(context.Background(), request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryJoinRooms(
	request *roomserverapi.QueryJoinRoomsRequest,
	reply string,
) {
	var response roomserverapi.QueryJoinRoomsResponse
	s.Proc.QueryJoinRooms(context.Background(), request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryBackFillEvents(
	request *roomserverapi.QueryBackFillEventsRequest,
	reply string,
) {
	var response roomserverapi.QueryBackFillEventsResponse
	s.Proc.QueryBackFillEvents(context.Background(), request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryEventAuth(
	request *roomserverapi.QueryEventAuthRequest,
	reply string,
) {
	var response roomserverapi.QueryEventAuthResponse
	s.Proc.QueryEventAuth(context.Background(), request, &response)

	s.rpcClient.PubObj(reply, response)
}
