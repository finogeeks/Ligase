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
	"log"

	"github.com/finogeeks/ligase/core"
)

func init() {
	core.RegisterTransport("nats", NewNatsTransport)
}

func NewNatsTransport(conf interface{}) (core.ITransport, error) {
	k := new(NatsTransport)
	return k, nil
}

type NatsTransport struct {
	baseTransport
}

func (t *NatsTransport) AddChannel(dir int, id, topic, grp string, conf interface{}) bool {
	_, ok := t.channels.Load(id)
	if ok {
		log.Printf("NatsTransport AddChannel dir:%d id:%s topic:%s grp:%s already exits\n", dir, id, topic, grp)
		return true
	}

	channel, err := core.GetChannel("nats", conf)
	if err != nil {
		log.Printf("NatsTransport AddChannel dir:%d id:%s topic:%s grp:%s get channel fail\n", dir, id, topic, grp)
		return false
	}
	channel.Init(t.logPorf)
	channel.SetDir(dir)
	channel.SetTopic(topic)
	channel.SetID(id)
	channel.SetGroup(grp)

	t.channels.Store(id, channel)

	log.Printf("NatsTransport AddChannel broker:%s dir:%d id:%s topic:%s grp:%s\n", t.brokers, dir, id, topic, grp)

	return true
}
