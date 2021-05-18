// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package common

import (
	"sync"
	"time"

	"github.com/finogeeks/ligase/common/uid"
	log "github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/nats.go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RpcClient struct {
	url  string
	conn *nats.Conn
	subs *sync.Map
	idg  *uid.UidGenerator
}

type Result struct {
	Index   int64  `json:"index"`
	Success bool   `json:"success"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

type RpcCB interface {
	GetTopic() string
	GetCB() nats.MsgHandler
	Clean()
}

type subHandler struct {
	sub string
	nc  *RpcClient

	toMap   *sync.Map
	chanMap *sync.Map
}

func (sh *subHandler) GetCB() nats.MsgHandler {
	return sh.cb
}

func (sh *subHandler) GetTopic() string {
	return sh.sub
}

func (sh *subHandler) Clean() {
	cur := time.Now().Unix()
	sh.toMap.Range(func(key, value interface{}) bool {
		target := value.(int64)
		if target < cur {
			sh.toMap.Delete(key)
			val, ok := sh.chanMap.Load(key)
			if ok {
				var result Result
				result.Index = key.(int64)
				result.Success = false
				result.ErrMsg = "timeout"

				channel := val.(chan *Result)
				channel <- &result
				sh.chanMap.Delete(key)
			}
		}
		return true
	})
}

func (sh *subHandler) cb(msg *nats.Msg) {
	var result Result
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		return
	}

	index := result.Index
	val, ok := sh.chanMap.Load(index)
	if ok {
		log.Debugf("nats cb called")
		channel := val.(chan *Result)
		channel <- &result

		sh.chanMap.Delete(index)
		sh.toMap.Delete(index)
	}
}

func (sh *subHandler) mark(idx int64, timeout int64, resChan chan *Result) {
	if timeout > 0 {
		sh.toMap.Store(idx, time.Now().Unix()+timeout)
	}

	sh.chanMap.Store(idx, resChan)
}

type rpcSubVal struct {
	cb  RpcCB
	sub *nats.Subscription
}

func NewRpcClient(url string, idg *uid.UidGenerator) *RpcClient {
	rpc := new(RpcClient)
	rpc.url = url
	rpc.idg = idg

	return rpc
}

func (nc *RpcClient) Start(clean bool) {
	conn, err := nats.Connect(nc.url)
	if err != nil {
		log.Fatalf("RpcClient: start fail %v", err)
	}

	nc.conn = conn
	nc.subs = new(sync.Map)

	nc.conn.SetReconnectHandler(nc.reconnectCb)

	if clean {
		go nc.clean()
	}
}

func (nc *RpcClient) reconnectCb(conn *nats.Conn) {
	log.Warn("RpcClient: reconnectCb triggered")
	nc.conn = conn
	nc.url = conn.ConnectedUrl()

	//re-sub
	nc.subs.Range(func(key, value interface{}) bool {
		topic := key.(string)
		sub := value.(rpcSubVal)

		nc.conn.Subscribe(topic, sub.cb.GetCB())
		return true
	})
}

func (nc *RpcClient) clean() {
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ticker.C:
			nc.subs.Range(func(key, value interface{}) bool {
				sub := value.(rpcSubVal)
				sub.cb.Clean()
				return true
			})
			ticker.Reset(time.Second)
		}
	}
}

func (nc *RpcClient) Pub(topic string, bytes []byte) {
	err := nc.conn.Publish(topic, bytes)
	if err != nil {
		log.Errorf("rpc pub error %v", err)
	}
}

func (nc *RpcClient) PubObj(topic string, obj interface{}) {
	bytes, _ := json.Marshal(obj)
	err := nc.conn.Publish(topic, bytes)
	if err != nil {
		log.Errorf("rpc pub error %v topic:%s data:%s", err, topic, string(bytes))
	}
}

func (nc *RpcClient) PubMsg(topic string, index int64, success bool, msg string) {
	res := new(Result)
	res.Index = index
	res.Success = success
	res.ErrMsg = msg

	bytes, _ := json.Marshal(res)
	err := nc.conn.Publish(topic, bytes)
	if err != nil {
		log.Errorf("rpc pub message error %v", err)
	}
}

func (nc *RpcClient) SubRaw(cb RpcCB) {
	topic := cb.GetTopic()
	_, ok := nc.subs.Load(topic)
	if !ok {
		sub, err := nc.conn.Subscribe(topic, cb.GetCB())
		if err != nil {
			log.Errorf("rpc sub error: %v", err)
			return
		}
		nc.subs.Store(topic, rpcSubVal{sub: sub, cb: cb})
	}
}

func (nc *RpcClient) Unsubscribe(cb RpcCB) {
	topic := cb.GetTopic()
	sub, ok := nc.subs.Load(topic)
	if ok {
		nc.subs.Delete(topic)
		sub.(rpcSubVal).sub.Unsubscribe()
	}
}

func (nc *RpcClient) Request(topic string, bytes []byte, timeout int) ([]byte, error) { //timeout in millisecond
	msg, err := nc.conn.Request(topic, bytes, time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	return msg.Data, nil
}

func (nc *RpcClient) Reply(topic string, handler nats.MsgHandler) { //timeout in millisecond
	nc.conn.Subscribe(topic, handler)
}

func (nc *RpcClient) ReplyGrp(topic, grp string, handler nats.MsgHandler) {
	nc.conn.QueueSubscribe(topic, grp, handler)

}

func (nc *RpcClient) Response(msg *nats.Msg, data []byte) { //timeout in millisecond
	nc.conn.Publish(msg.Reply, data)
}
