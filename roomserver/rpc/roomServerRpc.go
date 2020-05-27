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
	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/go-nats"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RoomserverRpcConsumer struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	DB        model.RoomServerDatabase
	Repo      *repos.RoomServerCurStateRepo
	UmsRepo   *repos.RoomServerUserMembershipRepo
	Proc      roomserverapi.RoomserverQueryAPI

	//msgChan1 chan *roomserverapi.RoomserverRpcRequest
	msgChan1 chan common.ContextMsg
	msgChan2 chan common.ContextMsg
	msgChan3 chan common.ContextMsg
	msgChan4 chan common.ContextMsg
	msgChan5 chan common.ContextMsg
	msgChan6 chan common.ContextMsg
	msgChan7 chan common.ContextMsg
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

	s.msgChan1 = make(chan common.ContextMsg, 1024)
	s.msgChan2 = make(chan common.ContextMsg, 1024)
	s.msgChan3 = make(chan common.ContextMsg, 1024)
	s.msgChan4 = make(chan common.ContextMsg, 1024)
	s.msgChan5 = make(chan common.ContextMsg, 1024)
	s.msgChan6 = make(chan common.ContextMsg, 1024)
	s.msgChan7 = make(chan common.ContextMsg, 1024)
	return s
}

func (s *RoomserverRpcConsumer) Start() error {
	go func() {
		for msg := range s.msgChan1 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryRoomState(msg.Ctx, data.QueryRoomState, data.Reply)
		}
	}()

	go func() {
		for msg := range s.msgChan2 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryEventsByID(msg.Ctx, data.QueryEventsByID, data.Reply)
		}
	}()

	go func() {
		for msg := range s.msgChan3 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryRoomEventByID(msg.Ctx, data.QueryRoomEventByID, data.Reply)
		}
	}()

	go func() {
		for msg := range s.msgChan4 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryJoinRooms(msg.Ctx, data.QueryJoinRooms, data.Reply)
		}
	}()

	go func() {
		for msg := range s.msgChan5 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryBackFillEvents(msg.Ctx, data.QueryBackFillEvents, data.Reply)
		}
	}()

	go func() {
		for msg := range s.msgChan6 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryEventAuth(msg.Ctx, data.QueryEventAuth, data.Reply)
		}
	}()

	go func() {
		for msg := range s.msgChan7 {
			data := msg.Msg.(*roomserverapi.RoomserverRpcRequest)
			s.processQueryEventsByDomainOffset(data.QueryEventsByDomainOffset, data.Reply)
		}
	}()

	s.rpcClient.ReplyGrpWithContext(s.GetTopic(), types.ROOMQRY_PRC_GROUP, s.cb)
	return nil
}

func (s *RoomserverRpcConsumer) GetTopic() string {
	return s.cfg.Rpc.RsQryTopic
}

func (s *RoomserverRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var request roomserverapi.RoomserverRpcRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Errorf("rpc roomserverqry unmarshal error %v", err)
		return
	}
	log.Debugf("RoomserverRpcConsumer on message, topic:%s  replay:%s data:%v", msg.Subject, msg.Reply, request)
	request.Reply = msg.Reply

	if request.QueryEventsByID != nil {
		s.msgChan2 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	} else if request.QueryRoomEventByID != nil {
		s.msgChan3 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	} else if request.QueryJoinRooms != nil {
		s.msgChan4 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	} else if request.QueryRoomState != nil {
		s.msgChan1 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	} else if request.QueryBackFillEvents != nil {
		s.msgChan5 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	} else if request.QueryEventAuth != nil {
		s.msgChan6 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	} else if request.QueryEventsByDomainOffset != nil {
		s.msgChan7 <- common.ContextMsg{Ctx: ctx, Msg: &request}
	}
}

func (s *RoomserverRpcConsumer) processQueryRoomState(
	ctx context.Context,
	request *roomserverapi.QueryRoomStateRequest,
	reply string,
) {
	var response roomserverapi.QueryRoomStateResponse
	log.Infof("processQueryRoomState recv request recv data:%s", request.RoomID)
	s.Proc.QueryRoomState(ctx, request, &response)

	log.Infof("processQueryRoomState recv process room:%s, data:%v, reply:%s", request.RoomID, response, reply)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryEventsByID(
	ctx context.Context,
	request *roomserverapi.QueryEventsByIDRequest,
	reply string,
) {
	var response roomserverapi.QueryEventsByIDResponse
	s.Proc.QueryEventsByID(ctx, request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryRoomEventByID(
	ctx context.Context,
	request *roomserverapi.QueryRoomEventByIDRequest,
	reply string,
) {
	var response roomserverapi.QueryRoomEventByIDResponse
	s.Proc.QueryRoomEventByID(ctx, request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryJoinRooms(
	ctx context.Context,
	request *roomserverapi.QueryJoinRoomsRequest,
	reply string,
) {
	var response roomserverapi.QueryJoinRoomsResponse
	s.Proc.QueryJoinRooms(ctx, request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryBackFillEvents(
	ctx context.Context,
	request *roomserverapi.QueryBackFillEventsRequest,
	reply string,
) {
	var response roomserverapi.QueryBackFillEventsResponse
	s.Proc.QueryBackFillEvents(ctx, request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryEventAuth(
	ctx context.Context,
	request *roomserverapi.QueryEventAuthRequest,
	reply string,
) {
	var response roomserverapi.QueryEventAuthResponse
	s.Proc.QueryEventAuth(ctx, request, &response)

	s.rpcClient.PubObj(reply, response)
}

func (s *RoomserverRpcConsumer) processQueryEventsByDomainOffset(
	request *roomserverapi.QueryEventsByDomainOffsetRequest,
	reply string,
) {
	var response roomserverapi.QueryEventsByDomainOffsetResponse
	s.Proc.QueryEventsByDomainOffset(context.Background(), request, &response)

	s.rpcClient.PubObj(reply, response)
}
