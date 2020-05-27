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
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"

	bt "bytes"

	"github.com/finogeeks/ligase/common/uid"
	log "github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/go-nats"
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
	//GetCB() nats.MsgHandler
	GetCB() MsgHandlerWithContext
	Clean()
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

func (nc *RpcClient) wrapGetCB(topic string, cb MsgHandlerWithContext) nats.MsgHandler {
	return NatsWrapHandlerWithContext(fmt.Sprintf("t[%s]", topic), cb)
}

func (nc *RpcClient) reconnectCb(conn *nats.Conn) {
	log.Warn("RpcClient: reconnectCb triggered")
	nc.conn = conn
	nc.url = conn.ConnectedUrl()

	//re-sub
	nc.subs.Range(func(key, value interface{}) bool {
		topic := key.(string)
		sub := value.(rpcSubVal)

		nc.conn.Subscribe(topic, nc.wrapGetCB(topic, sub.cb.GetCB()))
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

//TODO: is it necessary to add context/span/monitor like RequestWithContext for Pub()?
func (nc *RpcClient) Pub(topic string, bytes []byte) {
	err := nc.conn.Publish(topic, bytes)
	if err != nil {
		log.Errorf("rpc pub error %v", err)
	}
}

//TODO: is it necessary to add context/span/monitor like RequestWithContext for PubObj()?
func (nc *RpcClient) PubObj(topic string, obj interface{}) {
	bytes, _ := json.Marshal(obj)
	err := nc.conn.Publish(topic, bytes)
	if err != nil {
		log.Errorf("rpc pub error %v topic:%s data:%s", err, topic, string(bytes))
	}
}

func (nc *RpcClient) SubRaw(cb RpcCB) {
	topic := cb.GetTopic()
	_, ok := nc.subs.Load(topic)
	if !ok {
		sub, err := nc.conn.Subscribe(topic, nc.wrapGetCB(topic, cb.GetCB()))
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

// Request is used for rpc client, Reply/ReplyGrp is used for rpc server,
// SubRaw is used for rpc client but now is deprecated
func (nc *RpcClient) Request(topic string, bytes []byte, timeout int) ([]byte, error) { //timeout in millisecond
	msg, err := nc.conn.Request(topic, bytes, time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	return msg.Data, nil
}

func Uint32ToBytes(i uint32) []byte {
	var buf = make([]byte, 4)
	binary.BigEndian.PutUint32(buf, i)
	return buf
}

func BytesToUint32(i []byte) uint32 {
	return binary.BigEndian.Uint32(i)
}

// must be careful to use RequestWithContext, because some nats rpc server dose not support parseNatsData!!!
func (nc *RpcClient) RequestWithContext(ctx context.Context,
	topic string, bytes []byte, timeout int) ([]byte, error) { //timeout in millisecond
	span, ctx := StartSpanFromContext(ctx, fmt.Sprintf("RequestWithContext[%s]", topic))
	defer span.Finish()
	header := InjectSpanToHeaderForSending(span)
	if header == nil {
		return nc.Request(topic, bytes, timeout)
	}
	carrierStr, ok := header[SpanKey]
	if !ok {
		return nc.Request(topic, bytes, timeout)
	}
	carrier := []byte(carrierStr)
	if len(carrier) > MaxSpanBytesLength {
		return nc.Request(topic, bytes, timeout)
	}

	//log.Info("RequestWithContext, span: ", InjectSpanToHeader(span))
	ExportMetricsBeforeSending(span, topic, "nats")

	magicNum := Uint32ToBytes(0xF0F0F0F0)
	headerLen := Uint32ToBytes(uint32(len(carrier)))

	var buffer bt.Buffer
	buffer.Write(magicNum)
	buffer.Write(headerLen)
	buffer.Write(carrier)
	buffer.Write(bytes)
	msg, err := nc.conn.Request(topic, buffer.Bytes(), time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	return msg.Data, nil
}

const MagicNum = 0xF0F0F0F0
const MagicNumBytes = 4     // size of uint32
const HeaderLengthBytes = 4 // size of uint32
// MaxSpanBytesLength is at least 136 for jaeger, if a span carrier length exceed this value, it will be discarded
const MaxSpanBytesLength = 1024

func ParseNatsData(data []byte) ([]byte, opentracing.HTTPHeadersCarrier) {
	var buffer = bt.NewBuffer(data)
	magicNumBytes := make([]byte, MagicNumBytes)
	length, err := buffer.Read(magicNumBytes)
	if err != nil || length != MagicNumBytes {
		return data, nil
	}
	magicNum := BytesToUint32(magicNumBytes)
	if magicNum != MagicNum {
		return data, nil
	}
	headerLenBytes := make([]byte, HeaderLengthBytes)
	length, err = buffer.Read(headerLenBytes)
	if err != nil || length != HeaderLengthBytes {
		return data, nil
	}
	headerLen := BytesToUint32(headerLenBytes)
	if headerLen <= 0 || headerLen > MaxSpanBytesLength {
		return data, nil
	}

	headerBytes := make([]byte, headerLen)
	length, err = buffer.Read(headerBytes)
	if err != nil || uint32(length) != headerLen {
		return data, nil
	}

	var header opentracing.HTTPHeadersCarrier
	err = json.Unmarshal(headerBytes, &header)
	if err != nil {
		return data, nil
	}

	return data[MagicNumBytes+HeaderLengthBytes+headerLen:], header
}

type MsgHandlerWithContext func(ctx context.Context, msg *nats.Msg)

func NatsWrapHandlerWithContext(metricName string, handler MsgHandlerWithContext) nats.MsgHandler {
	return func(msg *nats.Msg) {
		now := time.Now().UnixNano() / 1e6
		nowStr := fmt.Sprintf("%d", now)
		data, header := ParseNatsData(msg.Data)
		var span opentracing.Span
		if header == nil {
			span = opentracing.StartSpan(metricName)
		} else {
			msg.Data = data
			tracer := opentracing.GlobalTracer()
			clientContext, err := tracer.Extract(opentracing.HTTPHeaders, header)
			if err != nil {
				span = opentracing.StartSpan(metricName)
			} else {
				span = opentracing.StartSpan(metricName, opentracing.FollowsFrom(clientContext))
			}
		}
		defer span.Finish()

		if len(span.BaggageItem("sob")) == 0 {
			span.SetBaggageItem("sob", nowStr)
		}
		span.SetBaggageItem("som", nowStr)

		if csArr := span.BaggageItem("cs"); len(csArr) != 0 {
			cs, err := strconv.Atoi(csArr)
			if err == nil {
				cs2sr.WithLabelValues("nats", metricName, "cs2sr").Observe(float64(now) - float64(cs))
			}
		}
		ctx := ContextWithSpan(context.Background(), span)
		handler(ctx, msg)
	}
}

func NatsWrapHandler(handler nats.MsgHandler) nats.MsgHandler {
	return func(msg *nats.Msg) {
		data, header := ParseNatsData(msg.Data)
		if header != nil {
			msg.Data = data
		}
		handler(msg)
	}
}

func (nc *RpcClient) Reply(topic string, handler nats.MsgHandler) { //timeout in millisecond
	nc.conn.Subscribe(topic, NatsWrapHandler(handler))
}

func (nc *RpcClient) ReplyWithContext(topic string, handler MsgHandlerWithContext) { //timeout in millisecond
	nc.conn.Subscribe(topic, NatsWrapHandlerWithContext(fmt.Sprintf("t[%s]", topic), handler))
}

func (nc *RpcClient) ReplyGrp(topic, grp string, handler nats.MsgHandler) {
	nc.conn.QueueSubscribe(topic, grp, NatsWrapHandler(handler))
}

func (nc *RpcClient) ReplyGrpWithContext(topic, grp string, handler MsgHandlerWithContext) { //timeout in millisecond
	nc.conn.QueueSubscribe(topic, grp, NatsWrapHandlerWithContext(fmt.Sprintf("t[%s]:g[%s]", topic, grp), handler))
}
