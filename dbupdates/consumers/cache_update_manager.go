package consumers

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/gomodule/redigo/redis"
)

type CacheUpdateManager struct {
	cfg       *config.Dendrite
	pools     []*redis.Pool
	poolSize  int
	consumers sync.Map
}

func NewCacheUpdateManager(cfg *config.Dendrite) *CacheUpdateManager {
	m := new(CacheUpdateManager)
	m.cfg = cfg
	m.poolSize = len(m.cfg.Redis.Uris)
	m.pools = make([]*redis.Pool, m.poolSize)
	for i := 0; i < m.poolSize; i++ {
		addr := m.cfg.Redis.Uris[i]
		m.pools[i] = &redis.Pool{
			MaxIdle:     200,
			MaxActive:   200,
			Wait:        true,
			IdleTimeout: 240 * time.Second,
			Dial:        func() (redis.Conn, error) { return redis.DialURL(addr) },
		}
	}

	return m
}

func (m *CacheUpdateManager) Start() error {
	keys := dbregistry.GetAllKeys()
	for _, key := range keys {
		poc, _ := dbregistry.GetRegistryProc(key)
		if poc.Cache == nil {
			continue
		}
		processor := poc.Cache(key, m.cfg, m)

		cfg := m.cfg.Kafka.Consumer.CacheUpdates
		cfg.Name = cfg.Name + "_group"
		cfg.Topic = cfg.Topic + "_" + key

		val, ok := common.GetTransportMultiplexer().GetNode(cfg.Underlying)
		if !ok {
			return fmt.Errorf("invalid kafka underlying %s", cfg.Underlying)
		}
		tran := val.(core.ITransport)

		needAddConsumer := false
		val, ok = common.GetTransportMultiplexer().GetChannel(
			cfg.Underlying,
			cfg.Name,
		)

		if ok {
			channelSub, ok := val.(IKafkaChannelSub)
			if !ok {
				needAddConsumer = true
				cfg.Name = m.cfg.Kafka.Consumer.CacheUpdates.Name + "_" + key
			} else {
				err := channelSub.SubscribeTopic(cfg.Topic)
				if err != nil {
					log.Errorf("kafka sub on exists consumer erro %s", err.Error())
					return err
				}
			}
		} else {
			needAddConsumer = true
		}

		if needAddConsumer {
			tran.AddChannel(core.CHANNEL_SUB, cfg.Name, cfg.Topic, cfg.Group, &cfg)
			val, ok = common.GetTransportMultiplexer().GetChannel(
				cfg.Underlying,
				cfg.Name,
			)
			if !ok {
				return fmt.Errorf("kafka consumer channel add not ok %s, %s", cfg.Underlying, cfg.Name)
			}

			channel := val.(core.IChannel)
			channel.SetHandler(m)
			if !common.GetTransportMultiplexer().PreStartChannel(cfg.Underlying, cfg.Name) {
				return fmt.Errorf("kafka consumer channel prestart err %s, %s", cfg.Underlying, cfg.Name)
			}
			channel.Start()
		}

		consumer := newCacheUpdateConsumer(key, m.cfg, processor)
		m.consumers.Store(cfg.Topic, consumer)
		err := consumer.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *CacheUpdateManager) Pool() *redis.Pool {
	slot := rand.Int() % m.poolSize
	return m.pools[slot]
}

func (m *CacheUpdateManager) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	val, ok := m.consumers.Load(topic)
	if !ok {
		return
	}
	consumer := val.(*CacheUpdateConsumer)
	consumer.OnMessage(ctx, topic, partition, data, rawMsg)
}
