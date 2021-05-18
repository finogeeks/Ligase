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

package syncconsumer

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/federation/model/backfilltypes"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/nats.go"
)

type FedEventExtra struct {
	FedEvent roomserverapi.FederationEvent
	Subject  string
}

type SyncConsumer struct {
	cfg        *config.Fed
	fedClient  *client.FedClientWrap
	rpcClient  *common.RpcClient
	feddomains *common.FedDomains
	backfill   backfilltypes.BackFillProcessor
	msgChan    []chan *FedEventExtra
	chanSize   int
}

func NewSyncConsumer(
	cfg *config.Fed,
	fedClient *client.FedClientWrap,
	rpcClient *common.RpcClient,
	feddomains *common.FedDomains,
) *SyncConsumer {
	s := &SyncConsumer{
		cfg:        cfg,
		fedClient:  fedClient,
		rpcClient:  rpcClient,
		feddomains: feddomains,
	}

	// s.msgChan = make(chan *FedEventExtra, 1024)
	return s
}

func (s *SyncConsumer) SetBackfill(bfp backfilltypes.BackFillProcessor) {
	s.backfill = bfp
}

func (s *SyncConsumer) Start() error {
	rand.Seed(time.Now().UnixNano())

	s.chanSize = 8
	s.msgChan = make([]chan *FedEventExtra, s.chanSize)

	for i := 0; i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *FedEventExtra, 1024)
		go s.startWorker(s.msgChan[i])
	}

	// subscribes all the subjects(topics) start with "fed"
	subject := fmt.Sprintf("%s.%s", s.cfg.Rpc.FedTopic, ">")
	s.rpcClient.Reply(subject, s.cb)
	return nil
}

func (s *SyncConsumer) startWorker(msgChan chan *FedEventExtra) {
	for msg := range msgChan {
		s.processRequest(msg)
	}
}

func (s *SyncConsumer) cb(msg *nats.Msg) {
	var request FedEventExtra
	if err := json.Unmarshal(msg.Data, &request.FedEvent); err != nil {
		log.Errorf("roomAliasRpcConsumer federationEvent unmarshal error %v", err)
		return
	}
	request.FedEvent.Reply = msg.Reply
	request.Subject = msg.Subject

	s.msgChan[rand.Intn(s.chanSize)] <- &request
}

type RpcResponse struct {
	Error   string
	Payload interface{}
}

func (s *SyncConsumer) processRequest(request *FedEventExtra) {
	var response interface{}
	destination, ok := s.feddomains.GetDomainHost(request.FedEvent.Destination)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s, topic: %s", request.FedEvent.Destination, request.Subject)
		s.rpcClient.PubObj(request.FedEvent.Reply, &RpcResponse{Error: "FedSync processRequest invalid destination " + request.FedEvent.Destination})
		return
	}
	log.Infof("source dest: %s, topic: %s", destination, request.Subject)

	if request.Subject == s.cfg.Rpc.FedAliasTopic {
		response = GetAliasRoomID(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedProfileTopic {
		response = GetProfile(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedAvatarTopic {
		response = GetAvatar(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedDisplayNameTopic {
		response = GetDisplayName(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedRsQryTopic {
		response = GetRoomState(s.fedClient, &request.FedEvent, destination, s.backfill)
	} else if request.Subject == s.cfg.Rpc.FedRsDownloadTopic {
		response = Download(s.fedClient, &request.FedEvent, destination, request.FedEvent.Destination, s.rpcClient)
	} else if request.Subject == s.cfg.Rpc.FedUserInfoTopic {
		response = GetUserInfo(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedRsMakeJoinTopic {
		response = MakeJoin(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedRsSendJoinTopic {
		response = SendJoin(s.fedClient, &request.FedEvent, destination, s.backfill)
	} else if request.Subject == s.cfg.Rpc.FedRsMakeLeaveTopic {
		response = MakeLeave(s.fedClient, &request.FedEvent, destination)
	} else if request.Subject == s.cfg.Rpc.FedRsSendLeaveTopic {
		response = SendLeave(s.fedClient, &request.FedEvent, destination, s.backfill)
	} else if request.Subject == s.cfg.Rpc.FedRsInviteTopic {
		response = SendInvite(s.fedClient, &request.FedEvent, destination)
	}

	s.rpcClient.PubObj(request.FedEvent.Reply, &RpcResponse{Payload: response})
}
