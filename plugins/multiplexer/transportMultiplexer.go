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

package transport

import (
	"errors"
	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/finogeeks/ligase/core"
	log "github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func init() {
	core.RegisterMultiplexer("transport", NewTransportMultiplexer)
}

func NewTransportMultiplexer(conf interface{}) (core.IMultiplexer, error) {
	log.Printf("NewTransportMultiplexer")
	k := new(TransportMultiplexer)
	return k, nil
}

type TransportMultiplexer struct {
	transMap sync.Map
}

func (m *TransportMultiplexer) AddNode(nodeName string, node interface{}) bool {
	_, ok := m.transMap.Load(nodeName)
	if ok {
		log.Errorf("TransportMultiplexer Failed to addnode %s, transport already exits", nodeName)
		return false
	}

	tran, ok := node.(core.ITransport)
	if !ok {
		log.Errorf("TransportMultiplexer Failed to addnode %s, not a transport", nodeName)
		return false
	}

	log.Infof("TransportMultiplexer add transport %s, broker:%s ", nodeName, tran.GetBrokers())
	m.transMap.Store(nodeName, tran)

	return true
}

func (m *TransportMultiplexer) GetNode(nodeName string) (interface{}, bool) {
	return m.transMap.Load(nodeName)
}

func (m *TransportMultiplexer) GetChannel(nodeName, serviceID string) (interface{}, bool) {
	val, ok := m.transMap.Load(nodeName)
	if !ok {
		log.Errorf("TransportMultiplexer Failed to get transport %s not exits", nodeName)
		return nil, false
	}

	tran := val.(core.ITransport)
	return tran.GetChannel(serviceID)
}

func (m *TransportMultiplexer) PreStartChannel(nodeName, serviceID string) bool {
	val, ok := m.transMap.Load(nodeName)
	if !ok {
		log.Errorf("TransportMultiplexer Failed to get transport %s not exits", nodeName)
		return false
	}

	tran := val.(core.ITransport)
	return tran.PreStartChannel(serviceID)
}

func (m *TransportMultiplexer) getNode(nodeName, serviceID string) (error, core.IChannel) {
	val, ok := m.transMap.Load(nodeName)
	if !ok {
		log.Errorf("TransportMultiplexer Failed to get transport %s:%s not exits", nodeName, serviceID)
		debug.PrintStack()
		return errors.New("can't find transport"), nil
	}

	tran := val.(core.ITransport)
	channnel, ok := tran.GetChannel(serviceID)
	if !ok {
		log.Errorf("TransportMultiplexer Failed to get channel %s:%s not exits", nodeName, serviceID)
		debug.PrintStack()
		return errors.New("can't find channel"), nil
	}
	return nil, channnel
}

func (m *TransportMultiplexer) SendNode(nodeName, serviceID string, obj interface{}) error {
	return m.Send(nodeName, serviceID, obj)
}

func (m *TransportMultiplexer) SendRecvNode(nodeName, serviceID string, obj interface{}) (interface{}, error) {
	msg := obj.(*core.TransportPubMsg)
	serviceID = m.getNodeInst(serviceID, msg)
	err, channnel := m.getNode(nodeName, serviceID)
	if err != nil {
		return nil, err
	}
	value, err := msg.Encode()
	if err != nil {
		return nil, err
	}
	if channnel != nil {
		return channnel.SendRecv(msg.Topic, value, msg.Timeout, msg.Headers)
	}

	return nil, errors.New("can't find channel")
}

func (m *TransportMultiplexer) getNodeInst(serviceID string, msg *core.TransportPubMsg) string {
	inst := msg.Inst
	if inst <= 0 {
		inst = adapter.GetKafkaNumProducers()
	}
	key := ""
	if msg.Keys != nil {
		key = string(msg.Keys)
	}
	if key == "" {
		key = strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
		msg.Keys = []byte(key)
	}
	if inst > 1 {
		serviceID = serviceID + strconv.Itoa(int(common.CalcStringHashCode(key))%inst)
	}
	return serviceID
}

func (m *TransportMultiplexer) Send(nodeName, serviceID string, obj interface{}) error {
	msg := obj.(*core.TransportPubMsg)
	serviceID = m.getNodeInst(serviceID, msg)
	err, channnel := m.getNode(nodeName, serviceID)
	if err != nil {
		return err
	}
	value, err := msg.Encode()
	if err != nil {
		log.Errorf("TransportMultiplexer Failed to encodemsg:%v, err:%v", obj, err)
		return err
	}
	return channnel.Send(msg.Topic, msg.Partition, msg.Keys, value, msg.Headers)
}

func (m *TransportMultiplexer) SendAndRecv(nodeName, serviceID string, obj interface{}) error {
	msg := obj.(*core.TransportPubMsg)
	serviceID = m.getNodeInst(serviceID, msg)
	err, channnel := m.getNode(nodeName, serviceID)
	if err != nil {
		return err
	}
	value, err := msg.Encode()
	if err != nil {
		log.Errorf("TransportMultiplexer Failed to encodemsg:%v, err:%v", obj, err)
		return err
	}
	return channnel.SendAndRecv(msg.Topic, msg.Partition, msg.Keys, value, msg.Headers)
}

func (m *TransportMultiplexer) SendWithRetry(nodeName, serviceID string, obj interface{}) error {
	msg := obj.(*core.TransportPubMsg)
	serviceID = m.getNodeInst(serviceID, msg)
	err, channnel := m.getNode(nodeName, serviceID)
	if err != nil {
		return err
	}
	value, err := msg.Encode()
	if err != nil {
		log.Errorf("TransportMultiplexer Failed to encodemsg:%v, err:%v", obj, err)
		return err
	}
	return channnel.SendWithRetry(msg.Topic, msg.Partition, msg.Keys, value, msg.Headers)
}

func (m *TransportMultiplexer) SendAndRecvWithRetry(nodeName, serviceID string, obj interface{}) error {
	msg := obj.(*core.TransportPubMsg)
	serviceID = m.getNodeInst(serviceID, msg)
	err, channnel := m.getNode(nodeName, serviceID)
	if err != nil {
		return err
	}
	value, err := msg.Encode()
	if err != nil {
		log.Errorf("TransportMultiplexer Failed to encodemsg:%v, err:%v", obj, err)
		return err
	}
	return channnel.SendAndRecvWithRetry(msg.Topic, msg.Partition, msg.Keys, value, msg.Headers)
}

func (m *TransportMultiplexer) PreStart() {
	m.transMap.Range(func(key, value interface{}) bool {
		tran := value.(core.ITransport)
		tran.PreStart()
		return true
	})
}

func (m *TransportMultiplexer) Start() {
	m.transMap.Range(func(key, value interface{}) bool {
		tran := value.(core.ITransport)
		tran.Start()
		return true
	})
}
