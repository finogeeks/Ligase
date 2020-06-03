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
	"context"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/json-iterator/go"
	"github.com/nats-io/go-nats"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type FilterTokenCB struct {
	rpcClient   *common.RpcClient
	tokenFilter *filter.SimpleFilter
	chanSize    uint32
	//msgChan     []chan *types.FilterTokenContent
	msgChan []chan common.ContextMsg
}

func (f *FilterTokenCB) GetTopic() string {
	return types.FilterTokenTopicDef
}

func (f *FilterTokenCB) GetCB() common.MsgHandlerWithContext {
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

func (f *FilterTokenCB) cb(ctx context.Context, msg *nats.Msg) {
	var result types.FilterTokenContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc filter token cb error %v", err)
		return
	}
	idx := common.CalcStringHashCode(result.UserID) % f.chanSize
	f.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
}

func (f *FilterTokenCB) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*types.FilterTokenContent)
		f.process(msg.Ctx, data)
	}
}

func (f *FilterTokenCB) Start() error {
	log.Infof("FilterTokenConsumer start")
	f.msgChan = make([]chan common.ContextMsg, f.chanSize)
	for i := uint32(0); i < f.chanSize; i++ {
		f.msgChan[i] = make(chan common.ContextMsg, 512)
		go f.startWorker(f.msgChan[i])
	}

	f.rpcClient.ReplyWithContext(f.GetTopic(), f.cb)
	return nil
}

func (f *FilterTokenCB) process(ctx context.Context, data *types.FilterTokenContent) {
	switch data.FilterType {
	case types.FILTERTOKENADD:
		f.tokenFilter.Insert(data.UserID, data.DeviceID)
	case types.FILTERTOKENDEL:
		f.tokenFilter.Delete(data.UserID, data.DeviceID)
	default:
		log.Infof("unknown filter token type:%s", data.FilterType)
	}
}
