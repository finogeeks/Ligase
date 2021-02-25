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

package channel

import (
	"context"
	"errors"
	"time"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/nats.go"
)

type NatsChannel struct {
	start   bool
	logPorf bool
	dir     int
	id      string
	topic   string
	grp     string
	handler core.IChannelConsumer
	conn    *nats.Conn
	sub     *nats.Subscription
}

func init() {
	core.RegisterChannel("nats", NewNatsChannel)
}

func NewNatsChannel(conf interface{}) (core.IChannel, error) {
	k := new(NatsChannel)
	return k, nil
}

func (c *NatsChannel) Init(logPorf bool) {
	c.start = false
	c.logPorf = logPorf
}

func (c *NatsChannel) SetTopic(topic string) {
	c.topic = topic
}

func (c *NatsChannel) SetGroup(group string) {
	c.grp = group
}

func (c *NatsChannel) SetID(id string) {
	c.id = id
}

func (c *NatsChannel) GetID() string {
	return c.id
}

func (c *NatsChannel) SetDir(dir int) {
	c.dir = dir
}

func (c *NatsChannel) GetDir() int {
	return c.dir
}

func (c *NatsChannel) SetHandler(handler core.IChannelConsumer) {
	c.handler = handler
}

func (c *NatsChannel) SetConn(conn *nats.Conn) {
	c.conn = conn
}

func (c *NatsChannel) PreStart(broker string, statsInterval int) {
	if c.conn == nil {
		conn, err := nats.Connect(broker)
		if err != nil {
			log.Fatalf("NatsTransport: start fail %s %v", broker, err)
			return
		}
		c.conn = conn
	}
}

func (c *NatsChannel) Start() {
	if c.conn == nil {
		log.Fatalf("NatsTransport: start fail conn nil")
		return
	}

	if c.start == false {
		c.start = true
		if c.dir == core.CHANNEL_SUB {
			if c.grp != "" {
				c.sub, _ = c.conn.QueueSubscribe(c.topic, c.grp, c.cb)
			} else {
				c.sub, _ = c.conn.Subscribe(c.topic, c.cb)
			}
		}
	}
}

func (c *NatsChannel) Stop() {
	c.start = false
	if c.dir == core.CHANNEL_SUB {
		c.sub.Drain()
		c.sub.Unsubscribe()
	}
}

func (c *NatsChannel) Close() {
	c.conn.Close()
}

func (c *NatsChannel) Commit(rawMsg []interface{}) error {
	return nil
}

func (c *NatsChannel) Send(topic string, partition int32, keys, bytes []byte) error {
	if topic == "" {
		topic = c.topic
	}

	err := c.conn.Publish(topic, bytes)
	if err != nil {
		log.Errorf("Nats pub fail, topic:%s", topic)
	}
	return err
}

func (c *NatsChannel) SendRecv(topic string, bytes []byte, timeout int) ([]byte, error) {
	if topic == "" {
		topic = c.topic
	}

	msg, err := c.conn.Request(topic, bytes, time.Duration(timeout)*time.Millisecond)

	if err != nil {
		return nil, err
	}

	return msg.Data, nil
}

func (c *NatsChannel) cb(msg *nats.Msg) {
	c.handler.OnMessage(context.Background(), msg.Subject, -1, msg.Data, msg)
}

func (c *NatsChannel) SendAndRecv(topic string, partition int32, keys, bytes []byte) error {
	return errors.New("unsupported commond SendAndRecv")
}
func (c *NatsChannel) SendWithRetry(topic string, partition int32, keys, bytes []byte) error {
	return errors.New("unsupported commond SendWithRetry")
}
func (c *NatsChannel) SendAndRecvWithRetry(topic string, partition int32, keys, bytes []byte) error {
	return errors.New("unsupported commond SendAndRecvWithRetry")
}
