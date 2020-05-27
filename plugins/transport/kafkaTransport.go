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
	core.RegisterTransport("kafka", NewKafkaTransport)
}

func NewKafkaTransport(conf interface{}) (core.ITransport, error) {
	k := new(KafkaTransport)
	return k, nil
}

type KafkaTransport struct {
	baseTransport
}

func (t *KafkaTransport) AddChannel(dir int, id, topic, grp string, conf interface{}) bool {
	_, ok := t.channels.Load(id)
	if ok {
		return true
	}

	channel, err := core.GetChannel("kafka", conf)
	if err != nil {
		return false
	}
	channel.Init(t.logPorf)
	channel.SetDir(dir)
	channel.SetTopic(topic)
	channel.SetID(id)
	channel.SetGroup(grp)

	t.channels.Store(id, channel)

	log.Printf("KafkaTransport AddChannel broker:%s dir:%d id:%s topic:%s grp:%s", t.brokers, dir, id, topic, grp)

	return true
}
