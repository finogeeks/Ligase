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
	"errors"
	"log"
	"sync"
)

type IProcessor interface {
	OnMultiplexerMessage(serviceID string, msg interface{}) (interface{}, error) //as receiver for multiplexer
	OnMessage(topic string, partition int32, data []byte) ([]byte, error)        //as receiver for transport
	PreStart()
	Start()
}

type ProcessorMsg struct {
	MsgType  int32
	Msg      interface{}
	UserID   string
	DeviceID string
}

var regProcessorMu sync.RWMutex
var newProcessorHandler = make(map[string]func(conf interface{}) (IProcessor, error))
var processorMap sync.Map

func RegisterProcessor(name string, f func(conf interface{}) (IProcessor, error)) {
	regProcessorMu.Lock()
	defer regProcessorMu.Unlock()

	log.Printf("IProcessor Register: %s func\n", name)
	if f == nil {
		log.Panicf("IProcessor Register: %s func nil\n", name)
	}

	if _, ok := newProcessorHandler[name]; ok {
		log.Panicf("IProcessor Register: %s already registered\n", name)
	}

	newProcessorHandler[name] = f
}

func GetProcessor(name, typeName string, conf interface{}) (IProcessor, error) {
	f := newProcessorHandler[typeName]
	if f == nil {
		return nil, errors.New("unknown processor " + typeName)
	}

	val, ok := processorMap.Load(name)
	if ok {
		return val.(IProcessor), nil
	}

	sel, err := f(conf)
	if err == nil {
		processorMap.Store(name, sel)
	}

	return sel, err
}

type IMsgHandler interface {
}
