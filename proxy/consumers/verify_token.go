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

package consumers

import (
	"math/rand"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/nats-io/go-nats"
)

type VerifyToken struct {
	reply      string
	token      string
	requestURI string
}

type VerifyTokenCB struct {
	rpcClient   *common.RpcClient
	tokenFilter *filter.SimpleFilter
	cache       service.Cache
	cfg         *config.Dendrite
	chanSize    uint32
	msgChan     []chan VerifyToken
}

func (f *VerifyTokenCB) GetTopic() string {
	return types.VerifyTokenTopicDef
}

func (f *VerifyTokenCB) GetCB() nats.MsgHandler {
	return f.cb
}

func (f *VerifyTokenCB) Clean() {
	//do nothing
}

func NewVerifyTokenConsumer(
	client *common.RpcClient,
	tokenFilter *filter.SimpleFilter,
	cache service.Cache,
	cfg *config.Dendrite,
) *VerifyTokenCB {
	f := &VerifyTokenCB{
		rpcClient:   client,
		tokenFilter: tokenFilter,
		cache:       cache,
		cfg:         cfg,
		chanSize:    16,
	}
	return f
}

func (f *VerifyTokenCB) cb(msg *nats.Msg) {
	var req types.VerifyTokenRequest
	err := json.Unmarshal(msg.Data, &req)
	if err != nil {
		log.Errorf("VerifyToken unmarshal %v, err: %v", msg.Data, err)
		return
	}
	idx := rand.Intn(len(f.msgChan))
	f.msgChan[idx] <- VerifyToken{
		reply:      msg.Reply,
		token:      req.Token,
		requestURI: req.RequestURI,
	}
}

func (f *VerifyTokenCB) startWorker(msgChan chan VerifyToken) {
	for data := range msgChan {
		f.process(&data)
	}
}

func (f *VerifyTokenCB) Start() error {
	log.Infof("FilterTokenConsumer start")
	f.msgChan = make([]chan VerifyToken, f.chanSize)
	for i := uint32(0); i < f.chanSize; i++ {
		f.msgChan[i] = make(chan VerifyToken, 512)
		go f.startWorker(f.msgChan[i])
	}

	f.rpcClient.Reply(f.GetTopic(), f.cb)
	return nil
}

func (f *VerifyTokenCB) process(data *VerifyToken) {
	device, resErr := common.VerifyToken(data.token, data.requestURI, f.cache, *f.cfg, f.tokenFilter)
	resp := types.VerifyTokenResponse{}
	if device != nil {
		resp.Device = *device
	}
	if resErr != nil {
		data, _ := json.Marshal(resErr.JSON)
		resp.Error = string(data)
	}
	f.rpcClient.PubObj(data.reply, resp)
}
