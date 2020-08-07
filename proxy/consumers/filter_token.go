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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/types"
	"github.com/json-iterator/go"
	"github.com/nats-io/nats.go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type FilterTokenCB struct {
	rpcClient   *common.RpcClient
	tokenFilter *filter.SimpleFilter
	chanSize    uint32
	msgChan     []chan *types.FilterTokenContent
}

func (f *FilterTokenCB) GetTopic() string {
	return types.FilterTokenTopicDef
}

func (f *FilterTokenCB) GetCB() nats.MsgHandler {
	return f.cb
}

func (f *FilterTokenCB) Clean() {
	//do nothing
}

func NewFilterTokenConsumer(
	client *common.RpcClient,
	tokenFilter *filter.SimpleFilter,
) *FilterTokenCB {
	f := &FilterTokenCB{
		rpcClient:   client,
		tokenFilter: tokenFilter,
		chanSize:    16,
	}
	return f
}

func (f *FilterTokenCB) cb(msg *nats.Msg) {
	var result types.FilterTokenContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc filter token cb error %v", err)
		return
	}
	idx := common.CalcStringHashCode(result.UserID) % f.chanSize
	f.msgChan[idx] <- &result
}

func (f *FilterTokenCB) startWorker(msgChan chan *types.FilterTokenContent) {
	for data := range msgChan {
		f.process(data)
	}
}

func (f *FilterTokenCB) Start() error {
	log.Infof("FilterTokenConsumer start")
	f.msgChan = make([]chan *types.FilterTokenContent, f.chanSize)
	for i := uint32(0); i < f.chanSize; i++ {
		f.msgChan[i] = make(chan *types.FilterTokenContent, 512)
		go f.startWorker(f.msgChan[i])
	}

	f.rpcClient.Reply(f.GetTopic(), f.cb)
	return nil
}

func (f *FilterTokenCB) process(data *types.FilterTokenContent) {
	switch data.FilterType {
	case types.FILTERTOKENADD:
		f.tokenFilter.Insert(data.UserID, data.DeviceID)
	case types.FILTERTOKENDEL:
		f.tokenFilter.Delete(data.UserID, data.DeviceID)
	default:
		log.Infof("unknown filter token type:%s", data.FilterType)
	}
}
