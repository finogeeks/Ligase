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

package consumers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/dbtypes"
	jsoniter "github.com/json-iterator/go"

	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// DBEventDataConsumer consumes db events for roomserver.
type DBEventDataConsumer struct {
	channel      core.IChannel
	dbGauge      mon.LabeledGauge
	monItem      *DBMonItem
	consumerRepo sync.Map
}

type DBMonItem struct {
	recv       int32
	process    int32
	fail       int32
	tablenamse string
	method     string
	duration   int64
}

// NewDBUpdateDataConsumer creates a new DBUpdateData consumer. Call Start() to begin consuming from room servers.
func NewDBEventDataConsumer(
	cfg *config.Dendrite,
) *DBEventDataConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.DBUpdates.Underlying,
		cfg.Kafka.Consumer.DBUpdates.Name,
	)

	if ok {
		channel := val.(core.IChannel)
		s := &DBEventDataConsumer{
			channel: channel,
		}
		channel.SetHandler(s)

		item := new(DBMonItem)
		item.tablenamse = "all"
		item.method = "all"
		s.monItem = item

		monitor := mon.GetInstance()
		s.dbGauge = monitor.NewLabeledGauge("dendrite_dbop_counter", []string{"app", "tablename", "method", "state"})

		//load instance
		for key, f := range newHandler {
			instance := f()
			instance.Prepare(cfg)
			log.Infof("NewDBEventDataConsumer: load instance %s", dbtypes.DBCategoryToStr(key))
			s.consumerRepo.Store(key, instance)
		}

		go func() {
			t := time.NewTimer(time.Second * time.Duration(1))
			for {
				select {
				case <-t.C:
					s.dbGauge.WithLabelValues("monolith", s.monItem.tablenamse, s.monItem.method, "recv").Set(float64(atomic.LoadInt32(&s.monItem.recv)))
					s.consumerRepo.Range(func(key, value interface{}) bool {
						handler := value.(ConsumerInterface)
						handler.Report(s.dbGauge)
						return true
					})
					t.Reset(time.Second * 5)
				}
			}
		}()

		return s
	}

	return nil
}

// Start consuming from room servers
func (s *DBEventDataConsumer) Start() error {
	s.consumerRepo.Range(func(key, value interface{}) bool {
		handler := value.(ConsumerInterface)
		handler.Start()

		return true
	})

	//s.channel.Start()
	return nil
}

func (s *DBEventDataConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var output dbtypes.DBEvent
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("dbevent: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	if output.IsRecovery {
		return //recover from db, just need to write cache
	}

	atomic.AddInt32(&s.monItem.recv, 1)

	category := output.Category

	val, ok := s.consumerRepo.Load(category)
	if ok {
		consumer := val.(ConsumerInterface)
		consumer.OnMessage(&output)
	}

	return
}

func (s *DBEventDataConsumer) ConsumerMessage(rawMsg []interface{}) {
	err := s.channel.Commit(rawMsg)
	if err != nil {
		log.Errorf("DBEventDataConsumer commit error %v", err)
	}
}
