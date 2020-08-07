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
	"context"
	"fmt"
	"strconv"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/nats-io/nats.go"
)

type ProfileRpcConsumer struct {
	cfg       *config.Fed
	fedClient *client.FedClientWrap
	rpcClient *common.RpcClient
	feddomain *common.FedDomains
	msgChan   chan *roomserverapi.FederationEvent
}

func NewProfileRpcConsumer(
	cfg *config.Fed,
	fedClient *client.FedClientWrap,
	rpcClient *common.RpcClient,
	feddomain *common.FedDomains,
) *ProfileRpcConsumer {
	s := &ProfileRpcConsumer{
		cfg:       cfg,
		fedClient: fedClient,
		rpcClient: rpcClient,
		feddomain: feddomain,
	}

	s.msgChan = make(chan *roomserverapi.FederationEvent, 1024)
	return s
}

func (s *ProfileRpcConsumer) Start() error {
	go func() {
		for data := range s.msgChan {
			s.processGetProfile(data, data.Reply)
		}
	}()

	subject := fmt.Sprintf("%s.%s", s.GetTopic(), ">")
	s.rpcClient.Reply(subject, s.cb)
	return nil
}

func (s *ProfileRpcConsumer) GetTopic() string {
	return s.cfg.Rpc.FedProfileTopic
}

func (s *ProfileRpcConsumer) cb(msg *nats.Msg) {
	var request roomserverapi.FederationEvent
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Errorf("profileRpcConsumer federationEvent unmarshal error %v", err)
		return
	}
	request.Reply = msg.Reply

	s.msgChan <- &request
}

func (s *ProfileRpcConsumer) processGetProfile(
	request *roomserverapi.FederationEvent,
	reply string,
) {
	var profileReq external.GetProfileRequest
	if err := json.Unmarshal(request.Extra, &profileReq); err != nil {
		log.Errorf("federation GetProfile unmarshal error: %v", err)
		return
	}

	info, ok := s.feddomain.GetDomainInfo(request.Destination)
	if !ok {
		log.Errorf("federation GetProfile get destination domain failed %s", request.Destination)
		return
	}
	destination := info.Host + ":" + strconv.Itoa(info.Port)
	log.Infof("source dest: %s, userID: %s", destination, profileReq.UserID)
	response, err := s.fedClient.LookupProfile(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation processGetProfile error: %v", err)
		return
	}

	log.Infof("LookupProfile return: %v", response)
	s.rpcClient.PubObj(reply, response)
}

func GetProfile(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) external.GetProfileResponse {
	var profileReq external.GetProfileRequest
	if err := json.Unmarshal(request.Extra, &profileReq); err != nil {
		log.Errorf("federation GetProfile unmarshal error: %v", err)
		return external.GetProfileResponse{}
	}

	// destination := fmt.Sprintf("%s:%s", request.Destination, s.cfg.GetConnectorPort())
	response, err := fedClient.LookupProfile(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation LookupProfile error: %v", err)
		return external.GetProfileResponse{}
	}

	log.Infof("LookupProfile return: %v", response)
	resp := external.GetProfileResponse{
		AvatarURL:   response.AvatarURL,
		DisplayName: response.DisplayName,
	}
	return resp
}

func GetAvatar(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) external.GetAvatarURLResponse {
	var profileReq external.GetProfileRequest
	if err := json.Unmarshal(request.Extra, &profileReq); err != nil {
		log.Errorf("federation GetProfile unmarshal error: %v", err)
		return external.GetAvatarURLResponse{}
	}

	response, err := fedClient.LookupAvatarURL(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation avatar error: %v", err)
		return external.GetAvatarURLResponse{}
	}

	log.Infof("LookupProfile return: %v", response)
	resp := external.GetAvatarURLResponse{
		AvatarURL: response.AvatarURL,
	}
	return resp
}

func GetDisplayName(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) external.GetDisplayNameResponse {
	var profileReq external.GetProfileRequest
	if err := json.Unmarshal(request.Extra, &profileReq); err != nil {
		log.Errorf("federation GetProfile unmarshal error: %v", err)
		return external.GetDisplayNameResponse{}
	}

	response, err := fedClient.LookupDisplayName(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation display name error: %v", err)
		return external.GetDisplayNameResponse{}
	}

	log.Infof("LookupProfile return: %v", response)
	resp := external.GetDisplayNameResponse{
		DisplayName: response.DisplayName,
	}
	return resp
}

func GetUserInfo(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination string,
) external.GetUserInfoResponse {
	var userInfoReq external.GetUserInfoRequest
	if err := json.Unmarshal(request.Extra, &userInfoReq); err != nil {
		log.Errorf("federation GetUserInfo unmarshal error: %v", err)
		return external.GetUserInfoResponse{}
	}

	response, err := fedClient.LookupUserInfo(context.Background(), destination, userInfoReq.UserID)
	if err != nil {
		log.Errorf("federation LookupUserInfo error: %v", err)
		return external.GetUserInfoResponse{}
	}

	log.Infof("LookupUserInfo return: %v", response)
	resp := external.GetUserInfoResponse{
		UserName:  response.UserName,
		JobNumber: response.JobNumber,
		Mobile:    response.Mobile,
		Landline:  response.Landline,
		Email:     response.Email,
		State:     response.State,
	}
	return resp
}
