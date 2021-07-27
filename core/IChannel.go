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
	"context"
	"errors"
	"log"
	"sync"
)

type IChannel interface {
	Init(logProf bool)
	SetTopic(topic string)
	SetGroup(group string)
	SetID(id string)
	GetID() string
	SetDir(dir int)
	GetDir() int
	SetHandler(handler IChannelConsumer)
	Commit(rawMsg []interface{}) error

	Send(topic string, partition int32, keys, bytes []byte) error
	SendAndRecv(topic string, partition int32, keys, bytes []byte) error
	SendWithRetry(topic string, partition int32, keys, bytes []byte) error
	SendAndRecvWithRetry(topic string, partition int32, keys, bytes []byte) error
	SendRecv(topic string, bytes []byte, timeout int) ([]byte, error)
	PreStart(broker string, statsInterval int)
	Start()
	Stop()
	Close()
}

type IChannelConsumer interface {
	OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{})
}

const CHANNEL_PUB = 0
const CHANNEL_SUB = 1

var regChannelMu sync.RWMutex
var newChannelHandler = make(map[string]func(conf interface{}) (IChannel, error))

func RegisterChannel(name string, f func(conf interface{}) (IChannel, error)) {
	regChannelMu.Lock()
	defer regChannelMu.Unlock()

	log.Printf("IChannel Register: %s\n", name)
	if f == nil {
		log.Panicf("IChannel Register: %s func nil\n", name)
	}

	if _, ok := newChannelHandler[name]; ok {
		log.Panicf("IChannel Register: %s already registered\n", name)
	}

	newChannelHandler[name] = f
}

func GetChannel(name string, conf interface{}) (IChannel, error) {
	f := newChannelHandler[name]
	if f == nil {
		return nil, errors.New("unknown transport " + name)
	}

	return f(conf)
}
