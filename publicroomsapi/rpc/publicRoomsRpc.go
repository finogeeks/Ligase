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
	"strconv"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/nats.go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type PublicRoomsRpcConsumer struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	DB        model.PublicRoomAPIDatabase
	Repo      *repos.RoomServerCurStateRepo
	UmsRepo   *repos.RoomServerUserMembershipRepo
	Proc      roomserverapi.RoomserverQueryAPI

	msgChan1 chan *publicroomsapi.PublicRoomsRpcRequest
}

func NewPublicRoomsRpcConsumer(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	db model.PublicRoomAPIDatabase,
) *PublicRoomsRpcConsumer {
	s := &PublicRoomsRpcConsumer{
		cfg:       cfg,
		rpcClient: rpcClient,
		DB:        db,
	}

	s.msgChan1 = make(chan *publicroomsapi.PublicRoomsRpcRequest, 1024)
	return s
}

func (s *PublicRoomsRpcConsumer) Start() error {
	go func() {
		for data := range s.msgChan1 {
			s.processQueryRoomState(data.QueryPublicRooms, data.Reply)
		}
	}()

	s.rpcClient.Reply(s.GetTopic(), s.cb)
	return nil
}

func (s *PublicRoomsRpcConsumer) GetTopic() string {
	return s.cfg.Rpc.PrQryTopic
}

func (s *PublicRoomsRpcConsumer) cb(msg *nats.Msg) {
	var request publicroomsapi.PublicRoomsRpcRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Errorf("rpc roomserverqry unmarshal error %v", err)
		return
	}
	log.Debugf("PublicRoomsRpcConsumer on message, topic:%s  replay:%s data:%v", msg.Subject, msg.Reply, request)
	request.Reply = msg.Reply

	if request.QueryPublicRooms != nil {
		s.msgChan1 <- &request
	}
}

func (s *PublicRoomsRpcConsumer) processQueryRoomState(
	request *publicroomsapi.QueryPublicRoomsRequest,
	reply string,
) {
	ctx := context.TODO()
	limit := request.Limit
	since := request.Since
	filter := request.Filter
	log.Infof("processQueryPublicRooms recv request recv data, limit:%d, since:%s, filter:%s", limit, since, filter)

	offset := int64(0)
	if len(since) > 0 {
		offset, _ = strconv.ParseInt(since, 10, 64)
	}

	var response publicroomsapi.QueryPublicRoomsResponse
	defer func(response *publicroomsapi.QueryPublicRoomsResponse) {
		s.rpcClient.PubObj(reply, response)
	}(&response)

	count, err := s.DB.CountPublicRooms(ctx)
	if err != nil {
		log.Errorf("CountPublicRooms err %v", err)
		return
	}

	publicRooms, err := s.DB.GetPublicRooms(ctx, offset, limit, filter)
	if err != nil {
		log.Errorf("GetPublicRooms err %v", err)
		return
	}

	if offset > 0 {
		response.PrevBatch = strconv.Itoa(int(offset) - 1)
	}

	response.Chunk = publicRooms
	response.NextBatch = strconv.FormatInt(offset+limit, 10)
	response.Estimate = count

	log.Infof("processQueryPublicRooms recv process data:%v, reply:%s", response, reply)
}
