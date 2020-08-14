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
	"github.com/nats-io/nats.go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

type RoomAliasRpcConsumer struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	DB        model.RoomServerDatabase
	Repo      *repos.RoomServerCurStateRepo
	UmsRepo   *repos.RoomServerUserMembershipRepo
	Proc      roomserverapi.RoomserverAliasAPI

	msgChan1 chan *roomserverapi.RoomserverAliasRequest
	msgChan2 chan *roomserverapi.RoomserverAliasRequest
}

func NewRoomAliasRpcConsumer(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	db model.RoomServerDatabase,
	repo *repos.RoomServerCurStateRepo,
	umsRepo *repos.RoomServerUserMembershipRepo,
	proc roomserverapi.RoomserverAliasAPI,
) *RoomAliasRpcConsumer {
	s := &RoomAliasRpcConsumer{
		cfg:       cfg,
		rpcClient: rpcClient,
		DB:        db,
		Repo:      repo,
		UmsRepo:   umsRepo,
		Proc:      proc,
	}

	s.msgChan1 = make(chan *roomserverapi.RoomserverAliasRequest, 1024)
	s.msgChan2 = make(chan *roomserverapi.RoomserverAliasRequest, 1024)
	return s
}

func (s *RoomAliasRpcConsumer) Start() error {
	go func() {
		for data := range s.msgChan1 {
			if data.SetRoomAliasRequest != nil {
				s.processSetRoomAlias(data.SetRoomAliasRequest, data.Reply)
			} else if data.RemoveRoomAliasRequest != nil {
				s.processRemoveRoomAlias(data.RemoveRoomAliasRequest, data.Reply)
			} else if data.AllocRoomAliasRequest != nil {
				s.processAllocRoomAlias(data.AllocRoomAliasRequest, data.Reply)
			}
		}
	}()

	go func() {
		for data := range s.msgChan2 {
			s.processGetAliasRoomID(data.GetAliasRoomIDRequest, data.Reply)
		}
	}()

	s.rpcClient.Reply(s.GetTopic(), s.cb)
	return nil
}

func (s *RoomAliasRpcConsumer) GetTopic() string {
	return s.cfg.Rpc.AliasTopic
}

func (s *RoomAliasRpcConsumer) cb(msg *nats.Msg) {
	var request roomserverapi.RoomserverAliasRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Errorf("rpc roomserverqry unmarshal error %v", err)
		return
	}
	request.Reply = msg.Reply

	if request.GetAliasRoomIDRequest != nil {
		s.msgChan2 <- &request
	} else {
		s.msgChan1 <- &request
	}
}

func (s *RoomAliasRpcConsumer) processSetRoomAlias(
	request *roomserverapi.SetRoomAliasRequest,
	reply string,
) {
	var response roomserverapi.SetRoomAliasResponse

	s.Proc.SetRoomAlias(context.Background(), request, &response)
	s.rpcClient.PubObj(reply, response)
}

func (s *RoomAliasRpcConsumer) processGetAliasRoomID(
	request *roomserverapi.GetAliasRoomIDRequest,
	reply string,
) {
	var response roomserverapi.GetAliasRoomIDResponse

	s.Proc.GetAliasRoomID(context.Background(), request, &response)
	s.rpcClient.PubObj(reply, response)
}

func (s *RoomAliasRpcConsumer) processRemoveRoomAlias(
	request *roomserverapi.RemoveRoomAliasRequest,
	reply string,
) {
	var response roomserverapi.RemoveRoomAliasResponse

	s.Proc.RemoveRoomAlias(context.Background(), request, &response)
	s.rpcClient.PubObj(reply, response)
}

func (s *RoomAliasRpcConsumer) processAllocRoomAlias(
	request *roomserverapi.SetRoomAliasRequest,
	reply string,
) {
	var response roomserverapi.SetRoomAliasResponse

	s.Proc.AllocRoomAlias(context.Background(), request, &response)
	s.rpcClient.PubObj(reply, response)
}
