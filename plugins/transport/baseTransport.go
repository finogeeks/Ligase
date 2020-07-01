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
	"sync"

	"github.com/finogeeks/ligase/core"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type baseTransport struct {
	id            string
	brokers       string
	statsInterval int
	logPorf       bool

	channels sync.Map
}

func (t *baseTransport) Init(logPorf bool) {
	t.logPorf = logPorf
}

func (t *baseTransport) SetID(id string) {
	t.id = id
}

func (t *baseTransport) SetBrokers(broker string) {
	t.brokers = broker
}

func (t *baseTransport) GetBrokers() string {
	return t.brokers
}

func (t *baseTransport) SetStatsInterval(interval int) {
	t.statsInterval = interval
}

func (t *baseTransport) GetStatsInterval() int {
	return t.statsInterval
}

func (t *baseTransport) StartChannel(id string) {
	val, ok := t.channels.Load(id)
	if ok == true {
		channel := val.(core.IChannel)
		channel.Start()
		return
	}

	log.Errorf("Failed to StartChannel id:%d", id)
}

func (t *baseTransport) StopChannel(id string) {
	val, ok := t.channels.Load(id)
	if ok == true {
		channel := val.(core.IChannel)
		channel.Stop()
		return
	}

	log.Errorf("Failed to StopChannel id:%d", id)
}

func (t *baseTransport) GetChannel(id string) (core.IChannel, bool) {
	val, ok := t.channels.Load(id)
	if ok == true {
		channel := val.(core.IChannel)
		return channel, true
	}

	return nil, false
}

func (t *baseTransport) PreStartChannel(id string) bool {
	val, ok := t.channels.Load(id)
	if ok == true {
		channel := val.(core.IChannel)
		channel.PreStart(t.brokers, t.statsInterval)
		return true
	}

	return false
}

func (t *baseTransport) RemoveChannel(id string) {
	t.channels.Delete(id)
}

func (t *baseTransport) PreStart() {
	t.channels.Range(func(key, value interface{}) bool {
		channel := value.(core.IChannel)
		channel.PreStart(t.brokers, t.statsInterval)
		return true
	})
}

func (t *baseTransport) Start() {
	t.channels.Range(func(key, value interface{}) bool {
		channel := value.(core.IChannel)
		channel.Start()
		return true
	})
}
