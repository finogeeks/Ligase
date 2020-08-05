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

package fedsender

import (
	"context"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type FederationDispatch struct {
	channel   core.IChannel
	cfg       *config.Fed
	sender    *FederationSender
	Repo      *repos.RoomServerCurStateRepo
	domaimMap sync.Map
}

func NewFederationDispatch(cfg *config.Fed) *FederationDispatch {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.DispatchInput.Underlying,
		cfg.Kafka.Consumer.DispatchInput.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		dispatch := &FederationDispatch{
			channel: channel,
			cfg:     cfg,
		}
		channel.SetHandler(dispatch)
		return dispatch
	}
	return nil
}

func (c *FederationDispatch) Start() error {
	c.channel.Start()
	return nil
}

func (c *FederationDispatch) SetSender(sender *FederationSender) {
	c.sender = sender
}

func (c *FederationDispatch) SetRepo(repo *repos.RoomServerCurStateRepo) {
	c.Repo = repo
}

func (c *FederationDispatch) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	log.Infof("fed-dispatch received data topic:%s, data:%s", topic, string(data))
	var output gomatrixserverlib.Event
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorf("fed-dispatch: message parse failure err:%v", err)
		return
	}

	log.Infof("fed-dispatch received data type:%s topic:%s", output.Type(), topic)

	c.onRoomEvent(ctx, output)
}

func (c *FederationDispatch) onRoomEvent(
	ctx context.Context, ev gomatrixserverlib.Event,
) error {
	rs := c.Repo.OnEventRecover(ctx, &ev, ev.EventNID())
	domains := rs.GetDomainTlMap()
	hasAddConsumer := false
	domains.Range(func(key, value interface{}) bool {
		domain := key.(string)
		log.Infof("fed-dispatch onRoomEvent check domain:%s server:%s", domain, c.cfg.GetServerName())
		if common.CheckValidDomain(domain, c.cfg.GetServerName()) == false {
			_, loaded := c.domaimMap.LoadOrStore(domain, true)
			if !loaded {
				c.sender.AddConsumer(domain)
				hasAddConsumer = true
			}
			// c.writeFedEvents(ev.RoomID(), domain, &ev)
		}
		return true
	})
	// https://docs.confluent.io/current/clients/confluent-kafka-go/index.html
	// When the group has rebalanced each client member is assigned a (sub-)set of topic+partitions. By default the
	// consumer will start fetching messages for its assigned partitions at this point, but your application may enable
	// rebalance events to get an insight into what the assigned partitions where as well as set the initial offsets. To do
	// this you need to pass `"go.application.rebalance.enable": true` to the `NewConsumer()` call mentioned above.
	// You will (eventually) see a `kafka.AssignedPartitions` event with the assigned partition set. You can optionally
	// modify the initial offsets (they'll default to stored offsets and if there are no previously stored offsets it will fall back
	// to `"auto.offset.reset"` which defaults to the `latest` message) and then call `.Assign(partitions)` to start
	// consuming. If you don't need to modify the initial offsets you will not need to call `.Assign()`, the client will do so
	// automatically for you if you dont.
	// kafka每次添加consumer到group中都会rebalance，按照理解，如果分区以前没有offset 且 auto.offset.reset是latest的话，
	// rebalance从unassing到assing之间的消息是不能被消费到的，所以有可能会导致连续多次AddConsumer之后，rebalance时，前面写进的消息刚好在rebalance期间
	// 就不能正常消费的第一条消息，导致writeFedEvents的消息没有被fedsender消费
	// 所以要把writeFedEvent的调用放在全部AddConsumer调用完之后
	domains.Range(func(key, value interface{}) bool {
		domain := key.(string)
		if common.CheckValidDomain(domain, c.cfg.GetServerName()) == false {
			c.writeFedEvents(ctx, ev.RoomID(), domain, &ev)
		}
		return true
	})

	return nil
}

func (c *FederationDispatch) writeFedEvents(ctx context.Context, roomID, domain string, update *gomatrixserverlib.Event) error {
	bytes, _ := json.Marshal(*update)
	log.Infof("fed-dispatch writeFedEvents room:%s, domain:%s ev:%v", roomID, domain, string(bytes))
	span, _ := common.StartSpanFromContext(ctx, c.cfg.Kafka.Producer.DispatchOutput.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, c.cfg.Kafka.Producer.DispatchOutput.Name,
		c.cfg.Kafka.Producer.DispatchOutput.Underlying)
	return common.GetTransportMultiplexer().SendAndRecvWithRetry(
		c.cfg.Kafka.Producer.DispatchOutput.Underlying,
		c.cfg.Kafka.Producer.DispatchOutput.Name,
		&core.TransportPubMsg{
			Topic:   c.cfg.Kafka.Producer.DispatchOutput.Topic + "." + domain,
			Keys:    []byte(roomID),
			Obj:     update,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}
