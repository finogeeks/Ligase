// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package transport

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

type consumerMon struct {
	mon   mon.Monitor
	gauge mon.LabeledGauge
	items sync.Map
}

type kafkaMonItem struct {
	topic     string
	group     string
	partition int32
	offset    int64
}

var once sync.Once
var instance *consumerMon

func getConsumeMonInstance() *consumerMon {
	once.Do(func() {
		instance = new(consumerMon)
		instance.mon = mon.GetInstance()
		instance.gauge = instance.mon.NewLabeledGauge("dendrite_kafka_counter", []string{"app", "topic", "group", "partition"})

		go instance.start()
	})

	return instance
}

func (c *consumerMon) start() {
	t := time.NewTimer(time.Second * time.Duration(1))
	for {
		select {
		case <-t.C:
			c.items.Range(func(key, value interface{}) bool {
				item := value.(*kafkaMonItem)
				c.gauge.WithLabelValues("monolith", item.topic, item.group, strconv.FormatInt(int64(item.partition), 10)).Set(float64(item.offset))
				return true
			})
			t.Reset(time.Second * 5)
		}
	}
}

func (c *consumerMon) Mark(topic string, group string, partition int32, offset int64) {
	key := fmt.Sprintf("%s:%s:%d", topic, group, partition)
	var item *kafkaMonItem

	val, ok := c.items.Load(key)
	if ok {
		item = val.(*kafkaMonItem)
	} else {
		item = new(kafkaMonItem)
		c.items.Store(key, item)
		item.topic = topic
		item.group = group
		item.partition = partition
	}

	item.offset = offset
}
