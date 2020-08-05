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

package bridge

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/service/roomserverapi"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/skunkworks/util/id"

	// "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model"
	wp "github.com/finogeeks/ligase/skunkworks/util/workerpool"
	pool "github.com/jolestar/go-commons-pool"
)

func init() {
	// must register the encode type first
	gob.Register(model.TxnReq{})
	gob.Register(map[string]string{})
	// gob.Register(api.GetAliasRoomIDResponse{})
	gob.Register(&roomserverapi.GetAliasRoomIDResponse{})
	gob.Register(&external.GetProfileResponse{})
	gob.Register(&external.GetAvatarURLResponse{})
	gob.Register(&external.GetDisplayNameResponse{})
	gob.Register(&gomatrixserverlib.FederationRequest{})
	gob.Register(&gomatrixserverlib.Transaction{})
	gob.Register(&roomserverapi.FederationEvent{})
	gob.Register(&roomserverapi.RawEvent{})
	gob.Register(&external.GetFedBackFillRequest{})
	gob.Register(&roomserverapi.QueryBackFillEventsResponse{})
}

var bridge Bridge

func newReplyChan(context.Context) (interface{}, error) {
	return make(chan *model.GobMessage), nil
}

type Bridge struct {
	w         *wp.WorkerPool
	replyChan sync.Map
	objPool   *pool.ObjectPool
	ctx       context.Context
	cfg       *config.Dendrite
}

// handle request and response
func (b *Bridge) processRequest(gobMsg *model.GobMessage) error {
	prod := b.cfg.Kafka.Producer.FedBridgeOutHs
	if gobMsg.Cmd >= model.CMD_FED {
		prod = b.cfg.Kafka.Producer.FedBridgeOut
	}
	// subject := topic //fmt.Sprintf("%s.%d", topic, id.GetNodeId())
	// log.Infof("connector pub addr:%s subject: %s data:%v", nc.addrs, subject, gobMsg)
	// return nc.econn.Publish(subject, gobMsg)

	keys := gobMsg.Key
	if gobMsg.Key == nil {
		keys = []byte{}
	}

	return common.GetTransportMultiplexer().SendAndRecvWithRetry(
		prod.Underlying,
		prod.Name,
		&core.TransportPubMsg{
			//Format: core.FORMAT_GOB,
			Keys: keys,
			Obj:  gobMsg,
		})
}

func (b *Bridge) processReply(gobMsg *model.GobMessage) error {
	ch, ok := b.replyChan.Load(gobMsg.MsgSeq)
	if !ok {
		return errors.New("Bridge reply MsgSeq not found in map. " + gobMsg.MsgSeq)
	}
	ch.(chan *model.GobMessage) <- gobMsg
	return nil
}

// 推送等单向消息
func (b *Bridge) processInstantMessage(gobMsg *model.GobMessage) error {
	return nil
}

func (b *Bridge) process(payload model.Payload) error {
	msg := payload.(model.GobMessage)
	// fmt.Printf("process, msg type: %d, msg: %v\n", msg.MsgType, msg)
	switch msg.MsgType {
	case model.REQUEST:
		b.processRequest(&msg)
	case model.REPLY:
		b.processReply(&msg)
	case model.MESSAGE:
		b.processInstantMessage(&msg)
	}
	return nil
}

func (b *Bridge) SetupBridge(cfg *config.Dendrite) {
	b.cfg = cfg

	factory := pool.NewPooledObjectFactorySimple(newReplyChan)
	b.ctx = context.Background()
	b.objPool = pool.NewObjectPoolWithDefaultConfig(b.ctx, factory)

	b.w = wp.NewWorkerPool(10)
	// defer w.Stop()
	b.w.SetHandler(b.process).Run()

	val, ok := common.GetTransportMultiplexer().GetChannel(
		b.cfg.Kafka.Consumer.FedBridgeOutRes.Underlying,
		b.cfg.Kafka.Consumer.FedBridgeOutRes.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		channel.SetHandler(b)
		channel.Start()
	}
}

func (b *Bridge) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	//dec := gob.NewDecoder(bytes.NewReader(data))
	msg := &model.GobMessage{}
	//err := dec.Decode(msg)
	err := json.Unmarshal(data, msg)
	if err != nil {
		log.Errorf("decode error: %s", err.Error())
		return
	}

	log.Infof("fed bridge response recv msg, MsgType:%d , MsgSeq:%s, NodeId:%d cmd:%d Body:%s", msg.MsgType, msg.MsgSeq, msg.NodeId, msg.Cmd, msg.Body)
	// msg := payload.(*model.GobMessage)
	msg.MsgType = model.REPLY
	b.w.FeedPayload(*msg)
}

func (b *Bridge) SendAndRecv(msg model.GobMessage, timeout int) (reply *model.GobMessage, err error) {
	// send msg
	Send(msg)

	// recv msg
	var obj interface{}
	if replyCh, ok := b.replyChan.Load(msg.MsgSeq); !ok {
		obj, err = b.objPool.BorrowObject(b.ctx)
		if err == nil && obj != nil {
			actual, loaded := b.replyChan.LoadOrStore(msg.MsgSeq, obj)
			if loaded {
				log.Errorf("Bridge reply channel exists, %s", msg.MsgSeq)
				b.objPool.ReturnObject(b.ctx, obj)
				obj = actual
			}
		} else {
			return nil, errors.New("server memory alloc failed")
		}
	} else {
		log.Errorf("Bridge reply channel exists, %s", msg.MsgSeq)
		obj = replyCh
	}
	ch := obj.(chan *model.GobMessage)

	select {
	case <-time.After(time.Millisecond * time.Duration(timeout)):
		err = model.ErrRequestTimedOut
	case m := <-ch:
		reply = m
	}

	b.replyChan.Delete(msg.MsgSeq)
	b.objPool.ReturnObject(b.ctx, obj)

	return
}

func (b *Bridge) Send(msg model.GobMessage) {
	msg.MsgType = model.REQUEST
	msg.NodeId = id.GetNodeId()
	b.w.FeedPayload(msg)
}

func SetupBridge(cfg *config.Dendrite) {
	bridge.SetupBridge(cfg)
}

func SendAndRecv(msg model.GobMessage, timeout int) (reply *model.GobMessage, err error) {
	return bridge.SendAndRecv(msg, timeout)
}

func Send(msg model.GobMessage) {
	bridge.Send(msg)
}
