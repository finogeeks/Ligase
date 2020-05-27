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

type ITransport interface {
	AddChannel(dir int, id, topic, grp string, conf interface{}) bool
	GetChannel(id string) (IChannel, bool)
	PreStartChannel(id string) bool
	StartChannel(id string)
	StopChannel(id string)
	SetID(id string)
	Init(logProf bool)
	SetBrokers(broker string)
	GetBrokers() string
	SetStatsInterval(interval int)
	GetStatsInterval() int
	PreStart()
	Start()
}

var regTransportMu sync.RWMutex
var newTransportHandler = make(map[string]func(conf interface{}) (ITransport, error))
var transportMap sync.Map

func RegisterTransport(name string, f func(conf interface{}) (ITransport, error)) {
	regTransportMu.Lock()
	defer regTransportMu.Unlock()

	log.Printf("ITransport Register: %s func\n", name)
	if f == nil {
		log.Panicf("ITransport Register: %s func nil\n", name)
	}

	if _, ok := newTransportHandler[name]; ok {
		log.Panicf("ITransport Register: %s already registered\n", name)
	}

	newTransportHandler[name] = f
}

func GetTransport(name, typeName string, conf interface{}) (ITransport, error) {
	f := newTransportHandler[typeName]
	if f == nil {
		return nil, errors.New("unknown transport " + typeName)
	}

	val, ok := transportMap.Load(name)
	if ok {
		return val.(ITransport), nil
	}

	sel, err := f(conf)
	if err == nil {
		transportMap.Store(name, sel)
	}

	return sel, err
}
