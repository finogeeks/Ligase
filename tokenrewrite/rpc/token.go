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
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/tokenrewrite/storage"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/nats.go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TokenRpcConsumer struct {
	rpcClient     *common.RpcClient
	chanSize      uint32
	msgChan       []chan *types.LoginInfoContent
	idg           *uid.UidGenerator
	staffPersist  *storage.TokenRewriteDataBase
	retailPersist *storage.TokenRewriteDataBase
	cfg           *config.Dendrite
}

func NewTokenRpcConsumer(
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
) *TokenRpcConsumer {
	s := &TokenRpcConsumer{
		rpcClient: rpcClient,
		chanSize:  16,
	}
	idg, _ := uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	s.idg = idg

	staffDB, err := storage.NewTokenRewriteDataBase(cfg.TokenRewrite.StaffDB)
	if err != nil {
		log.Panicf("create staffDB err %v", err)
	}

	retailDB, err := storage.NewTokenRewriteDataBase(cfg.TokenRewrite.RetailDB)
	if err != nil {
		log.Panicf("create retailDB err %v", err)
	}

	s.staffPersist = staffDB
	s.retailPersist = retailDB
	s.cfg = cfg

	return s
}

func (s *TokenRpcConsumer) GetCB() nats.MsgHandler {
	return s.cb
}

func (s *TokenRpcConsumer) GetTopic() string {
	return types.LoginTopicDef
}

func (s *TokenRpcConsumer) Clean() {
}

func (s *TokenRpcConsumer) cb(msg *nats.Msg) {
	var result types.LoginInfoContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc token cb error %v", err)
		return
	}
	idx := common.CalcStringHashCode(result.UserID) % s.chanSize
	s.msgChan[idx] <- &result
}

func (s *TokenRpcConsumer) startWorker(msgChan chan *types.LoginInfoContent) {
	for data := range msgChan {
		s.process(data)
	}
}

func (s *TokenRpcConsumer) Start() error {
	log.Infof("TokenRpcConsumer start")
	s.msgChan = make([]chan *types.LoginInfoContent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.LoginInfoContent, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)
	return nil
}

func (s *TokenRpcConsumer) process(data *types.LoginInfoContent) {
	log.Infof("start process for %s %s %s", data.UserID, data.DeviceID, data.Token)
	domain, _ := common.DomainFromID(data.UserID)
	id, _ := s.idg.Next()
	switch domain {
	case s.cfg.TokenRewrite.StaffDomain:
		err := s.staffPersist.UpsertDevice(context.TODO(), data.UserID, data.DeviceID, data.DisplayName)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", data.UserID, data.DeviceID, data.Token, err)
		}

		err = s.staffPersist.UpsertUser(context.TODO(), data.UserID)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", data.UserID, data.DeviceID, data.Token, err)
		}

		err = s.staffPersist.UpsertToken(context.TODO(), id, data.UserID, data.DeviceID, data.Token)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", data.UserID, data.DeviceID, data.Token, err)
		}
	case s.cfg.TokenRewrite.RetailDomain:
		err := s.staffPersist.UpsertDevice(context.TODO(), data.UserID, data.DeviceID, data.DisplayName)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", data.UserID, data.DeviceID, data.Token, err)
		}

		err = s.staffPersist.UpsertUser(context.TODO(), data.UserID)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", data.UserID, data.DeviceID, data.Token, err)
		}

		err = s.staffPersist.UpsertToken(context.TODO(), id, data.UserID, data.DeviceID, data.Token)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", data.UserID, data.DeviceID, data.Token, err)
		}
	}
}
