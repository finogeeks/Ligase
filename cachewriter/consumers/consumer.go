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
	"math/rand"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/gomodule/redigo/redis"
	jsoniter "github.com/json-iterator/go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// DBEventDataConsumer consumes db events for cache writer.
type DBEventCacheConsumer struct {
	channel  core.IChannel
	pools    []*redis.Pool
	poolSize int

	consumerRepo sync.Map
}

// NewDBUpdateDataConsumer creates a new DBUpdateData consumer. Call Start() to begin consuming from room servers.
func NewDBEventCacheConsumer(
	cfg *config.Dendrite,
) *DBEventCacheConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.CacheUpdates.Underlying,
		cfg.Kafka.Consumer.CacheUpdates.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &DBEventCacheConsumer{
			channel: channel,
		}
		channel.SetHandler(s)

		s.poolSize = len(cfg.Redis.Uris)
		s.pools = make([]*redis.Pool, s.poolSize)
		for i := 0; i < s.poolSize; i++ {
			addr := cfg.Redis.Uris[i]
			s.pools[i] = &redis.Pool{
				MaxIdle:     10,
				MaxActive:   200,
				Wait:        true,
				IdleTimeout: 240 * time.Second,
				Dial:        func() (redis.Conn, error) { return redis.DialURL(addr) },
			}
		}

		//load instance
		for key, f := range newHandler {
			instance := f()
			instance.SetPool(s)
			instance.Prepare(cfg)
			log.Infof("NewDBEventCacheConsumer: load instance %s", dbtypes.DBCategoryToStr(key))
			s.consumerRepo.Store(key, instance)
		}
		return s
	}

	return nil
}

func (s *DBEventCacheConsumer) Pool() *redis.Pool {
	slot := rand.Int() % s.poolSize
	return s.pools[slot]
}

// Start consuming from room servers
func (s *DBEventCacheConsumer) Start() error {
	s.consumerRepo.Range(func(key, value interface{}) bool {
		handler := value.(ConsumerInterface)
		handler.Start()

		return true
	})

	//s.channel.Start()
	return nil
}

func (s *DBEventCacheConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var output dbtypes.DBEvent
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("dbevent: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	category := output.Category

	val, ok := s.consumerRepo.Load(category)
	if ok {
		consumer := val.(ConsumerInterface)
		consumer.OnMessage(&output)
	}
}
