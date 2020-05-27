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

package core

import (
	"bytes"
	"encoding/gob"
	"errors"
	"log"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type IMultiplexer interface {
	AddNode(nodeName string, node interface{}) bool
	GetNode(nodeName string) (interface{}, bool)
	GetChannel(nodeName, serviceID string) (interface{}, bool)
	PreStartChannel(nodeName, serviceID string) bool
	SendNode(nodeName, serviceID string, obj interface{}) error
	SendRecvNode(nodeName, serviceID string, obj interface{}) (interface{}, error)
	Send(nodeName, serviceID string, obj interface{}) error
	SendAndRecv(nodeName, serviceID string, obj interface{}) error
	SendWithRetry(nodeName, serviceID string, obj interface{}) error
	SendAndRecvWithRetry(nodeName, serviceID string, obj interface{}) error
	PreStart()
	Start()
}

var regMultiplexerMu sync.RWMutex
var newMultiplexerHandler = make(map[string]func(conf interface{}) (IMultiplexer, error))
var multiplexerMap sync.Map

func RegisterMultiplexer(name string, f func(conf interface{}) (IMultiplexer, error)) {
	regMultiplexerMu.Lock()
	defer regMultiplexerMu.Unlock()

	log.Printf("IMultiplexer Register: %s func\n", name)
	if f == nil {
		log.Panicf("IMultiplexer Register: %s func nil\n", name)
	}

	if _, ok := newMultiplexerHandler[name]; ok {
		log.Panicf("IMultiplexer Register: %s already registered\n", name)
	}

	newMultiplexerHandler[name] = f
}

func GetMultiplexer(name string, conf interface{}) (IMultiplexer, error) {
	f := newMultiplexerHandler[name]
	if f == nil {
		return nil, errors.New("unknown transport " + name)
	}

	val, ok := multiplexerMap.Load(name)
	if ok {
		return val.(IMultiplexer), nil
	}

	mult, err := f(conf)
	if err == nil {
		multiplexerMap.Store(name, mult)
	}

	return mult, err
}

type TransportPubMsg struct {
	Partition int32
	Format    int8
	Topic     string
	Keys      []byte
	Obj       interface{}
	Timeout   int
	Inst      int
	Headers   map[string]string
}

func (msg *TransportPubMsg) Encode() ([]byte, error) {
	switch msg.Format {
	case FORMAT_JSON:
		return json.Marshal(msg.Obj)
	case FORMAT_GOB:
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(msg.Obj)
		return buf.Bytes(), err
	default:
		return nil, errors.New("un supported format")
	}
}
