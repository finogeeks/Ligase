package consumers

import (
	"context"
	"fmt"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type DBEventSeqManager struct {
	cfg       *config.Dendrite
	consumers sync.Map
	channel   core.IChannel
}

func NewDBEventSeqManager(cfg *config.Dendrite) *DBEventSeqManager {
	m := new(DBEventSeqManager)
	m.cfg = cfg
	return m
}

func (m *DBEventSeqManager) Start() error {
	keys := dbregistry.GetAllKeys()
	for _, key := range keys {
		poc, _ := dbregistry.GetRegistryProc(key)
		if poc.Persist == nil {
			continue
		}
		processor := poc.Persist(key, m.cfg)

		cfg := m.cfg.Kafka.Consumer.DBUpdates
		cfg.Name = cfg.Name + "_group"
		cfg.Topic = cfg.Topic + "_" + key
		cfg.AutoCommit = new(bool)
		*cfg.AutoCommit = false

		val, ok := common.GetTransportMultiplexer().GetNode(cfg.Underlying)
		if !ok {
			return fmt.Errorf("invalid kafka underlying %s", cfg.Underlying)
		}
		tran := val.(core.ITransport)

		needAddConsumer := false
		var channel core.IChannel
		val, ok = common.GetTransportMultiplexer().GetChannel(
			cfg.Underlying,
			cfg.Name,
		)
		if ok {
			channelSub, ok := val.(IKafkaChannelSub)
			if !ok {
				needAddConsumer = true
				cfg.Name = m.cfg.Kafka.Consumer.DBUpdates.Name + "_" + key
			} else {
				err := channelSub.SubscribeTopic(cfg.Topic)
				if err != nil {
					log.Errorf("kafka sub on exists consumer erro %s", err.Error())
					return err
				}
				channel = val.(core.IChannel)
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

			channel = val.(core.IChannel)
			channel.SetHandler(m)
			if !common.GetTransportMultiplexer().PreStartChannel(cfg.Underlying, cfg.Name) {
				return fmt.Errorf("kafka consumer channel prestart err %s, %s", cfg.Underlying, cfg.Name)
			}
			channel.Start()
			m.channel = channel
		}

		consumer := newDBEventSeqConsumer(key, m.cfg, processor, channel)
		m.consumers.Store(cfg.Topic, consumer)
		err := consumer.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *DBEventSeqManager) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	val, ok := m.consumers.Load(topic)
	if !ok {
		return
	}
	consumer := val.(*DBEventSeqConsumer)
	consumer.OnMessage(ctx, topic, partition, data, rawMsg)
}
