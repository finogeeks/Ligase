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

package api

import (
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/plugins/message/internals"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/nats.go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InputRpcCB struct {
	topic   string
	handler nats.MsgHandler
}

func (i *InputRpcCB) GetTopic() string {
	return i.topic
}

func (i *InputRpcCB) GetCB() nats.MsgHandler {
	return i.handler
}

func (i *InputRpcCB) Clean() {
	//do nothing
}

// consumes events that originated in the client api server.
type InputConsumer struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	cbs       map[string]*InputRpcCB
	handlers  sync.Map
}

func NewInputConsumer(
	cfg *config.Dendrite,
	client *common.RpcClient,
) *InputConsumer {
	s := &InputConsumer{
		cfg:       cfg,
		rpcClient: client,
	}

	input := new(InputRpcCB)
	input.topic = ""
	input.handler = s.cb
	s.cbs[input.topic] = input
	client.SubRaw(input)
	return s
}

type IMshHandler interface {
	ProcessInputMsg(*internals.InputMsg)
}

func (s *InputConsumer) RegisterHandler(key int32, handler IMshHandler) {
	s.handlers.Store(key, handler)
}

//when nats, write data to chan, when process done need to replay, called by nats transport
func (s *InputConsumer) cb(msg *nats.Msg) {
	var input internals.InputMsg

	if err := input.Decode(msg.Data); err != nil {
		log.Errorf("cb: parse failure %v, input:%s", err, string(msg.Data))
		return
	}

	val, ok := s.handlers.Load(input.GetCategory())
	if ok {
		handler := val.(IMshHandler)
		handler.ProcessInputMsg(&input)
		if msg.Reply != "" {
			s.rpcClient.PubObj(msg.Reply, nil)
		}
	}
	return
}
