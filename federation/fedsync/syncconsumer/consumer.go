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
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/model/backfilltypes"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/nats.go"
)

type FedEventExtra struct {
	FedEvent roomserverapi.FederationEvent
	Subject  string
}

type SyncConsumer struct {
	cfg        *config.Dendrite
	fedClient  *client.FedClientWrap
	rpcClient  *common.RpcClient
	feddomains *common.FedDomains
	backfill   backfilltypes.BackFillProcessor
	msgChan    []chan *FedEventExtra
	chanSize   int
}

func NewSyncConsumer(
	cfg *config.Dendrite,
	fedClient *client.FedClientWrap,
	rpcClient *common.RpcClient,
	feddomains *common.FedDomains,
	backfill backfilltypes.BackFillProcessor,
) *SyncConsumer {
	s := &SyncConsumer{
		cfg:        cfg,
		fedClient:  fedClient,
		rpcClient:  rpcClient,
		feddomains: feddomains,
		backfill:   backfill,
	}

	// s.msgChan = make(chan *FedEventExtra, 1024)
	return s
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

	switch request.Subject {
	case s.cfg.Rpc.FedAliasTopic:
		var aliasReq external.GetDirectoryRoomAliasRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &aliasReq); err != nil {
			log.Errorf("federation GetAliasRoomIDRequest unmarshal error: %v", err)
			response = external.GetDirectoryRoomAliasResponse{}
		} else {
			log.Infof("extra: %s, aliasreq: %v", string(request.FedEvent.Extra), aliasReq)
			response = GetAliasRoomID(s.fedClient, &aliasReq, destination)
		}
	case s.cfg.Rpc.FedProfileTopic:
		var profileReq external.GetProfileRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &profileReq); err != nil {
			log.Errorf("federation GetProfile unmarshal error: %v", err)
			response = external.GetProfileResponse{}
		} else {
			response = GetProfile(s.fedClient, &profileReq, destination)
		}
	case s.cfg.Rpc.FedAvatarTopic:
		var profileReq external.GetProfileRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &profileReq); err != nil {
			log.Errorf("federation GetProfile unmarshal error: %v", err)
			response = external.GetAvatarURLResponse{}
		} else {
			response = GetAvatar(s.fedClient, &profileReq, destination)
		}
	case s.cfg.Rpc.FedDisplayNameTopic:
		var profileReq external.GetProfileRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &profileReq); err != nil {
			log.Errorf("federation GetProfile unmarshal error: %v", err)
			response = external.GetDisplayNameResponse{}
		} else {
			response = GetDisplayName(s.fedClient, &profileReq, destination)
		}
	case s.cfg.Rpc.FedRsQryTopic:
		var stateReq external.GetFedRoomStateRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &stateReq); err != nil {
			log.Errorf("federation GetRoomState unmarshal error: %v", err)
			response = gomatrixserverlib.RespState{}
		} else {
			log.Infof("extra: %s, statereq: %v", string(request.FedEvent.Extra), stateReq)
			response = GetRoomState(s.fedClient, &stateReq, destination, s.backfill)
		}
	case s.cfg.Rpc.FedDownloadTopic:
		var req external.GetFedDownloadRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &req); err != nil {
			log.Errorf("federation Download unmarshal error: %v", err)
			response = external.GetFedDownloadResponse{
				StatusCode: http.StatusInternalServerError,
			}
		} else {
			response = Download(s.fedClient, &req, destination, request.FedEvent.Destination, func(reqID string, data []byte) error {
				s.rpcClient.Pub(reqID, data)
				return nil
			}, func(reqID string) {
				s.rpcClient.Pub(reqID, []byte{})
			})
		}
	case s.cfg.Rpc.FedUserInfoTopic:
		var userInfoReq external.GetUserInfoRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &userInfoReq); err != nil {
			log.Errorf("federation GetUserInfo unmarshal error: %v", err)
			response = external.GetUserInfoResponse{}
		} else {
			response = GetUserInfo(s.fedClient, &userInfoReq, destination)
		}
	case s.cfg.Rpc.FedMakeJoinTopic:
		var req external.GetMakeJoinRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &req); err != nil {
			log.Errorf("federation make join unmarshal error: %v", err)
			response = gomatrixserverlib.RespMakeJoin{}
		} else {
			response = MakeJoin(s.fedClient, &req, destination)
		}
	case s.cfg.Rpc.FedSendJoinTopic:
		var req external.PutSendJoinRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &req); err != nil {
			log.Errorf("federation send join unmarshal error: %v", err)
			response = gomatrixserverlib.RespSendJoin{}
		} else {
			response = SendJoin(s.fedClient, &req, destination, s.backfill)
		}
	case s.cfg.Rpc.FedMakeLeaveTopic:
		var req external.GetMakeLeaveRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &req); err != nil {
			log.Errorf("federation make leave unmarshal error: %v", err)
			response = gomatrixserverlib.RespMakeLeave{}
		} else {
			response = MakeLeave(s.fedClient, &req, destination)
		}
	case s.cfg.Rpc.FedSendLeaveTopic:
		var req external.PutSendLeaveRequest
		if err := json.Unmarshal(request.FedEvent.Extra, &req); err != nil {
			log.Errorf("federation send leave unmarshal error: %v", err)
			response = gomatrixserverlib.RespSendLeave{}
		} else {
			response = SendLeave(s.fedClient, &req, destination, s.backfill)
		}
	case s.cfg.Rpc.FedInviteTopic:
		var event gomatrixserverlib.Event
		if err := json.Unmarshal(request.FedEvent.Extra, &event); err != nil {
			log.Errorf("federation send invite unmarshal error: %v", err)
			response = gomatrixserverlib.RespInvite{Code: 400}
		} else {
			response = SendInvite(s.fedClient, &event, destination)
		}
	}

	s.rpcClient.PubObj(request.FedEvent.Reply, &RpcResponse{Payload: response})
}
