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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	log "github.com/finogeeks/ligase/skunkworks/log"

	// "github.com/confluentinc/confluent-kafka-go/kafka"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

type KafkaConsumerConf interface {
	EnableAutoCommit() *bool
	AutoCommitIntervalMS() *int
	TopicAutoOffsetReset() *string
	GoChannelEnable() *bool
}

const (
	DefaultTimeOut          = 10
	DefaultEnableAutoCommit = true
)

type msgContext struct {
	result chan error
	msg    kafka.Message
}

type KafkaChannel struct {
	start   bool
	logPorf bool
	dir     int
	id      string
	//default topic
	topic    string
	grp      string
	handler  core.IChannelConsumer
	producer *kafka.Producer
	consumer *kafka.Consumer
	slot     uint32
	chanSize int
	msgChan  []chan msgContext
	// cache custom topic when send, such as dispatch-out.domain
	cacheTopics sync.Map
	broker      string
	conf        interface{}
	subTopics   []string
}

func init() {
	core.RegisterChannel("kafka", NewKafkaChannel)
}

func NewKafkaChannel(conf interface{}) (core.IChannel, error) {
	k := new(KafkaChannel)
	k.conf = conf
	return k, nil
}

func (c *KafkaChannel) Init(logPorf bool) {
	c.start = false
	c.logPorf = logPorf
}

func (c *KafkaChannel) SetTopic(topic string) {
	c.topic = topic
}

func (c *KafkaChannel) SetGroup(group string) {
	c.grp = group
}

func (c *KafkaChannel) SetID(id string) {
	c.id = id
}

func (c *KafkaChannel) GetID() string {
	return c.id
}

func (c *KafkaChannel) SetDir(dir int) {
	c.dir = dir
}

func (c *KafkaChannel) GetDir() int {
	return c.dir
}

func (c *KafkaChannel) SetHandler(handler core.IChannelConsumer) {
	c.handler = handler
}

//all topic create by this method, cancel auto create
func (c *KafkaChannel) createTopic(broker, topic string) error {
	if topic == "" {
		log.Warn("create topic is empty")
		return nil
	}
	a, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": broker})
	if err != nil {
		log.Errorln("Failed to create Admin client: ", err)
		return err
	}
	defer a.Close()

	ctx := context.Background()
	md, err := a.GetMetadata(&topic, false, 5000)
	if err != nil {
		log.Errorln("Failed to get kafka meta data: ", err)
		return err
	}

	numPartitions := adapter.GetKafkaNumPartitions()
	replicationFactor := adapter.GetKafkaReplicaFactor()

	if topicInfo, ok := md.Topics[topic]; ok && len(topicInfo.Partitions) > 0 {
		log.Infof("kafka topic %s already create", topic)
		if len(topicInfo.Partitions) != numPartitions {
			log.Warnf("kafka topic:%s partition %d expect %d", topic, len(topicInfo.Partitions), numPartitions)
		}
		for _, vv := range topicInfo.Partitions {
			if len(vv.Replicas) != replicationFactor {
				log.Warnf("kafka topic:%s partition:%d reeplicas %d expect %d", topic, vv.ID, len(vv.Replicas), replicationFactor)
			}
		}
		if topic != c.topic {
			c.cacheTopics.Store(topic, true)
		}
		return nil
	}

	maxDur, err := time.ParseDuration("60s")
	if err != nil {
		log.Errorln("ParseDuration err: ", err)
		return err
	}
	results, err := a.CreateTopics(
		ctx,
		[]kafka.TopicSpecification{{
			Topic:             topic,
			ReplicationFactor: replicationFactor,
			NumPartitions:     numPartitions,
		}},
		kafka.SetAdminOperationTimeout(maxDur))
	if err != nil {
		log.Errorln("Failed to create topic: ", err)
		return err
	}
	// check results
	for _, result := range results {
		if result.Error.Code() != kafka.ErrNoError && result.Error.Code() != kafka.ErrTopicAlreadyExists {
			log.Errorf("Create toipc:%s err result:%v ", topic, result)
			return result.Error
		}
	}
	if topic != c.topic {
		c.cacheTopics.Store(topic, true)
	}
	return nil
}

func (c *KafkaChannel) SetChan(slot uint32, chanSize int) {
	c.slot = slot
	c.chanSize = chanSize
}

func (c *KafkaChannel) startChan() {
	c.msgChan = make([]chan msgContext, c.slot)
	for i := uint32(0); i < c.slot; i++ {
		c.msgChan[i] = make(chan msgContext, c.chanSize)
		go c.startWorker(c.msgChan[i])
	}
}

func (c *KafkaChannel) startWorker(channel chan msgContext) {
	for msg := range channel {
		deliveryChan := make(chan kafka.Event)
		c.pubSync(msg.msg, deliveryChan, msg.result)
	}
}

func (c *KafkaChannel) dispatch(msg kafka.Message, result chan error) {
	hash := common.CalcStringHashCode(string(msg.Key))
	slot := hash % c.slot
	c.msgChan[slot] <- msgContext{
		result: result,
		msg:    msg,
	}
}

func (c *KafkaChannel) PreStart(broker string, statsInterval int) {
	c.broker = broker
	if c.dir == core.CHANNEL_PUB {
		c.preStartProducer(broker, statsInterval)
	} else {
		c.preStartConsumer(broker, statsInterval)
	}
}

func (c *KafkaChannel) Start() {
	if c.start == false {
		c.start = true

		if c.dir == core.CHANNEL_PUB {
			c.startProducer()
		} else {
			c.startConsumer()
		}
	}
}

func (c *KafkaChannel) Stop() {
	c.start = false
}

func (c *KafkaChannel) Close() {
	if c.producer != nil {
		c.producer.Close()
	}

	if c.consumer != nil {
		c.consumer.Close()
	}
}

func (c *KafkaChannel) Commit(rawMsgs []interface{}) error {
	if rawMsgs == nil || len(rawMsgs) == 0 || c.consumer == nil {
		return nil
	}

	partitionsMsgArr := []*kafka.Message{}
	partitionsMsg := make(map[string]map[int32]bool)
	for i := len(rawMsgs) - 1; i >= 0; i-- {
		rawMsg := rawMsgs[i]
		if v, ok := rawMsg.(*kafka.Message); ok {
			topic := *v.TopicPartition.Topic
			tempMap := partitionsMsg[topic]
			if tempMap == nil {
				tempMap = make(map[int32]bool)
				partitionsMsg[topic] = tempMap
			}
			if !tempMap[v.TopicPartition.Partition] {
				tempMap[v.TopicPartition.Partition] = true
				partitionsMsgArr = append(partitionsMsgArr, v)
			}
		}
	}
	var err error
	for i := len(partitionsMsgArr) - 1; i >= 0; i-- {
		v := partitionsMsgArr[i]
		_, err0 := c.consumer.CommitMessage(v)
		if err0 != nil {
			log.Errorf("kafka commit offset error %s", v)
			err = err0
		}
	}
	return err
}

func (c *KafkaChannel) startProducer() error {
	c.startChan()
	go func() {
		for e := range c.producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				m := ev
				if m.TopicPartition.Error != nil {
					log.Errorf("async Failed to delivery topic:%s partition:%d msg:%s err:%s", *m.TopicPartition.Topic, m.TopicPartition.Partition, string(m.Value), m.TopicPartition.Error.Error())
					//kafka异常报此错误时，需要重试，不重试会造成丢消息, 由配置确定是程序自动重试还是由调用者重试
					if retry, retries := c.pubRetry(m); retry {
						c.pubAsync(*m)
						log.Errorf("async re produce failed msg:%s to topic:%s retries:%d again", string(m.Value), *m.TopicPartition.Topic, retries)
					}
				} else {
					log.Debugf("async Delivered message:%s to topic %s [%d] at offset %v", string(m.Value), *m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
				}
			case *kafka.Stats:
				// https://github.com/edenhill/librdkafka/blob/master/STATISTICS.md
				var stats map[string]interface{}
				err := json.Unmarshal([]byte(ev.String()), &stats)
				if err != nil {
					log.Infof("kafka producer stats: json unmarshall error: %s, stats: %v", err, ev.String())
				}
				log.Infof("kafka producer stats, tx: %v, rx: %v, txmsgs: %v, rxmsgs: %v", stats["tx"], stats["rx"], stats["txmsgs"], stats["rxmsgs"])
				log.Debugf("Kafka producer stats (all): %v", ev.String())

				// try to do sth on "callback" when the stat is unusual
				// go statsCallback(e.String())
			default:
				if c.start == false {
					return
				}
			}
		}
	}()

	return nil
}

//todo mannual commit message
func (c *KafkaChannel) startConsumer() error {
	c.subTopics = []string{c.topic}
	err := c.consumer.SubscribeTopics(c.subTopics, nil)
	if err != nil {
		log.Errorf("StartConsumer sub err: %v", err)
		return nil
	}

	log.Infof("StartConsumer topic:%s group:%s", c.topic, c.grp)
	onMessage := func(consumer core.IChannelConsumer, msg *kafka.Message) {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("channel consumer panic: %#v", e)
			}
		}()
		consumer.OnMessage(context.Background(), *msg.TopicPartition.Topic, msg.TopicPartition.Partition, msg.Value, msg)
	}
	var evHandler func()
	evHandler = func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("consumer panic: %#v", e)
				go evHandler()
			}
		}()
		for c.start == true {
			select {
			case ev := <-c.consumer.Events():
				switch e := ev.(type) {
				case kafka.AssignedPartitions:
					c.consumer.Assign(e.Partitions)
					log.Infof("consumer assigned partitions: %v", e)
				case kafka.RevokedPartitions:
					c.consumer.Unassign()
					log.Infof("consumer unassigned partitions: %v", e)
				case *kafka.Message:
					//log.Infof("consumer %% Message on topic:%s partition:%d offset:%d val:%s grp:%s", *e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value), t.consumerGroup)
					onMessage(c.handler, e)
				case kafka.PartitionEOF:
					log.Infof("consumer Reached: %v", e)
				case kafka.Error:
					log.Errorf("consumer Error: %v", e)
				case *kafka.Stats:
					// https://github.com/edenhill/librdkafka/blob/master/STATISTICS.md
					var stats map[string]interface{}
					err = json.Unmarshal([]byte(e.String()), &stats)
					if err != nil {
						log.Warnf("Kafka consumer stats: json unmarshall error: %s, stats: %v", err, e.String())
					}
					log.Debugf("Kafka consumer stats, tx: %v, rx: %v, rxmsgs: %v, txmsgs: %v", stats["tx"], stats["rx"], stats["rxmsgs"], stats["txmsgs"])
					log.Debugf("kafka consumer stats (all): %v", e.String())

					brokerRaw := stats["brokers"].(map[string]interface{})
					bInfo := make(map[string]map[string]interface{})
					for k, v := range brokerRaw {
						bInfo[k] = v.(map[string]interface{})
						log.Debugf("kafka consumer stats for brokers: %v, state: %v, tx: %v, rx: %v", k, bInfo[k]["state"], bInfo[k]["tx"], bInfo[k]["rx"])
					}
					// try to do sth on "callback" when the stat is unusual
					// go statsCallback(e.String())
				}
			}
		}
	}
	go evHandler()

	return nil
}

func (c *KafkaChannel) SubscribeTopic(topic string) error {
	c.subTopics = append(c.subTopics, topic)
	err := c.consumer.SubscribeTopics(c.subTopics, nil)
	if err != nil {
		log.Errorf("SubscribeTopic sub err: %v", err)
		return err
	}
	return nil
}

func (c *KafkaChannel) preStartProducer(broker string, statsInterval int) error {
	if c.producer == nil {
		err := c.createTopic(broker, c.topic)
		if err != nil {
			log.Errorf("Failed to create topic:%s err:%v", c.topic, err)
			return err
		}
		pc := kafka.ConfigMap{
			"bootstrap.servers":        broker,
			"go.produce.channel.size":  10000,
			"go.batch.producer":        false,
			"socket.timeout.ms":        5000,
			"session.timeout.ms":       5000,
			"reconnect.backoff.max.ms": 5000,
			"default.topic.config": kafka.ConfigMap{
				"message.timeout.ms": 6000,
			},
		}
		if adapter.GetKafkaEnableIdempotence() {
			pc.SetKey("enable.idempotence", true)
		}
		p, err := kafka.NewProducer(&pc)
		if err != nil {
			log.Errorf("Failed to create producer: %v", err)
			return err
		}
		c.slot = 64
		c.chanSize = 64
		c.producer = p
	}

	return nil
}

func (c *KafkaChannel) preStartConsumer(broker string, statsInterval int) error {
	if c.consumer == nil {
		enableAutoCommit := DefaultEnableAutoCommit
		autoCommitIntervalMS := 5000
		topicAutoOffsetReset := "latest"
		goChannelEnable := true
		if c.conf != nil {
			conf := c.conf.(KafkaConsumerConf)
			if conf.EnableAutoCommit() != nil {
				enableAutoCommit = *conf.EnableAutoCommit()
			}
			if conf.AutoCommitIntervalMS() != nil {
				autoCommitIntervalMS = *conf.AutoCommitIntervalMS()
			}
			if conf.TopicAutoOffsetReset() != nil {
				topicAutoOffsetReset = *conf.TopicAutoOffsetReset()
			}
			if conf.GoChannelEnable() != nil {
				goChannelEnable = *conf.GoChannelEnable()
			}
		}
		log.Infof("kafka consumer config topic:%s enableAutoCommit:%t autoCommitIntervalMS:%d topicAutoOffsetReset:%s goChannelEnable:%t", c.topic, enableAutoCommit, autoCommitIntervalMS, topicAutoOffsetReset, goChannelEnable)
		s, err := kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers":               broker,
			"group.id":                        c.grp,
			"session.timeout.ms":              20000,
			"go.events.channel.enable":        goChannelEnable,
			"go.application.rebalance.enable": true,
			"enable.auto.commit":              enableAutoCommit,
			"auto.commit.interval.ms":         autoCommitIntervalMS,
			"default.topic.config":            kafka.ConfigMap{"auto.offset.reset": topicAutoOffsetReset}, //earliest
			"statistics.interval.ms":          statsInterval,
			// "enable.partition.eof":            true
		})

		if err != nil {
			log.Errorf("Failed to create consumer: %v", err)
			return err
		}

		c.consumer = s
	}

	return nil
}

func (c *KafkaChannel) getAssignTopic(topic string) (string, error) {
	if topic == "" || topic == c.topic {
		return c.topic, nil
	}
	//custom topic
	if _, ok := c.cacheTopics.Load(topic); ok {
		return topic, nil
	} else {
		return topic, c.createTopic(c.broker, topic)
	}
}

//异步发送消息
func (c *KafkaChannel) Send(topic string, partition int32, keys, bytes []byte) error {
	if c.start == false {
		return errors.New("Kafka producer not start yet")
	}
	topic, err := c.getAssignTopic(topic)
	if err != nil {
		return errors.New(fmt.Sprintf("Kafka producer create custom topic:%s err:%v", topic, err))
	}
	if keys != nil {
		return c.pubWithKey(topic, keys, bytes, false, false)
	} else {
		return c.pubWithPartition(topic, partition, bytes, false, false)
	}
}

//异步发送消息失败重试
func (c *KafkaChannel) SendWithRetry(topic string, partition int32, keys, bytes []byte) error {
	if c.start == false {
		return errors.New("Kafka producer not start yet")
	}
	topic, err := c.getAssignTopic(topic)
	if err != nil {
		return errors.New(fmt.Sprintf("Kafka producer create custom topic:%s err:%v", topic, err))
	}
	if keys != nil {
		return c.pubWithKey(topic, keys, bytes, false, true)
	} else {
		return c.pubWithPartition(topic, partition, bytes, false, true)
	}
}

//同步发送消息
func (c *KafkaChannel) SendAndRecv(topic string, partition int32, keys, bytes []byte) error {
	if adapter.GetKafkaForceAsyncSend() {
		return c.Send(topic, partition, keys, bytes)
	}
	if c.start == false {
		return errors.New("Kafka producer not start yet")
	}
	topic, err := c.getAssignTopic(topic)
	if err != nil {
		return errors.New(fmt.Sprintf("Kafka producer create custom topic:%s err:%v", topic, err))
	}
	if keys != nil {
		return c.pubWithKey(topic, keys, bytes, true, false)
	} else {
		return c.pubWithPartition(topic, partition, bytes, true, false)
	}
}

//同步发送消息失败重试
func (c *KafkaChannel) SendAndRecvWithRetry(topic string, partition int32, keys, bytes []byte) error {
	if adapter.GetKafkaForceAsyncSend() {
		return c.SendWithRetry(topic, partition, keys, bytes)
	}
	if c.start == false {
		return errors.New("Kafka producer not start yet")
	}
	topic, err := c.getAssignTopic(topic)
	if err != nil {
		return errors.New(fmt.Sprintf("Kafka producer create custom topic:%s err:%v", topic, err))
	}
	if keys != nil {
		return c.pubWithKey(topic, keys, bytes, true, true)
	} else {
		return c.pubWithPartition(topic, partition, bytes, true, true)
	}
}

//将消息发送到partition参数指定的partition，此接口可以废弃
func (c *KafkaChannel) pubWithPartition(topic string, partition int32, bytes []byte, sync bool, retry bool) error {
	if partition < 0 {
		log.Errorf("Failed to Pub partition:%d topic:%s", partition, topic)
		partition = 1
	}
	var msg kafka.Message
	msg.TopicPartition.Topic = &topic
	msg.TopicPartition.Partition = partition
	msg.Value = bytes
	if retry {
		msg.Headers = []kafka.Header{{Key: "retries", Value: []byte(strconv.Itoa(0))}}
	} else {
		msg.Headers = []kafka.Header{{Key: "retries", Value: []byte(strconv.Itoa(-1))}}
	}
	if sync {
		result := make(chan error)
		c.dispatch(msg, result)
		return <-result
	} else {
		return c.pubAsync(msg)
	}
}

//将消息发送到指定的partition,partition由kafka使用key根据内部算法得到
//(同样的key会发送到一个partition，key对应partition的算法可以通过partitioner参数改变)
func (c *KafkaChannel) pubWithKey(topic string, keys, bytes []byte, sync bool, retry bool) error {
	var msg kafka.Message
	msg.TopicPartition.Topic = &topic
	msg.TopicPartition.Partition = kafka.PartitionAny
	msg.Value = bytes
	msg.Key = keys
	if retry {
		msg.Headers = []kafka.Header{{Key: "retries", Value: []byte(strconv.Itoa(0))}}
	} else {
		msg.Headers = []kafka.Header{{Key: "retries", Value: []byte(strconv.Itoa(-1))}}
	}
	if sync {
		result := make(chan error)
		c.dispatch(msg, result)
		return <-result
	} else {
		return c.pubAsync(msg)
	}
}

func (c *KafkaChannel) pubSync(msg kafka.Message, deliveryChan chan kafka.Event, result chan error) {
	bs := time.Now().UnixNano() / 1000000
	err := c.producer.Produce(&msg, deliveryChan)
	if err != nil {
		log.Errorf("sync Produce msg:%s to topic:%s failed: %s", string(msg.Value), *msg.TopicPartition.Topic, err.Error())
		//return err
		result <- err
		return
	}
	for {
		select {
		//函数阻塞在：e := <-deliveryChan，直到broker有返回或reconnect超时
		case e := <-deliveryChan:
			spend := time.Now().UnixNano()/1000000 - bs
			m := e.(*kafka.Message)
			if m.TopicPartition.Error != nil {
				//logger.GetLogger().Errorf("sync Failed to delivery topic:%s partition:%d msg:%s err:%s", *m.TopicPartition.Topic, m.TopicPartition.Partition, string(m.Value), m.TopicPartition.Error.Error())
				//kafka异常报此错误时，需要重试，不重试会造成丢消息, 由配置确定是程序自动重试还是由调用者重试
				if retry, retries := c.pubRetry(&msg); retry {
					log.Errorf("sync re produce failed msg:%s to topic:%s retries:%d again", string(m.Value), *m.TopicPartition.Topic, retries)
					c.pubSync(msg, deliveryChan, result)
					return
				} else {
					log.Errorf("sync Delivery msg:%s to topic:%s failed: %s", string(m.Value), *m.TopicPartition.Topic, m.TopicPartition.Error.Error())
					close(deliveryChan)
					result <- m.TopicPartition.Error
					return
				}
			} else {
				log.Debugf("sync Delivered message to topic %s [%d] at offset %v succ spend:%d ms",
					*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset, spend)
				close(deliveryChan)
				result <- nil
				return
			}
		}
	}
}

func (c *KafkaChannel) pubAsync(msg kafka.Message) error {
	err := c.producer.Produce(&msg, nil)
	if err != nil {
		log.Errorf("async Kafka produce with key fail, send msg:%s to topic:%s err:%s", string(msg.Value), *msg.TopicPartition.Topic, err.Error())
	}
	return err
}

func (c *KafkaChannel) pubRetry(msg *kafka.Message) (retry bool, retries int) {
	if msg.Headers != nil {
		needRetries := false
		for idx, header := range msg.Headers {
			if header.Key == "retries" {
				retries, _ = strconv.Atoi(string(header.Value))
				if retries >= 0 {
					needRetries = true
					retries++
					msg.Headers[idx].Value = []byte(strconv.Itoa(retries))
					break
				} else {
					needRetries = false
					break
				}
			}
		}
		return needRetries, retries
	} else {
		return true, 0
	}
}

//nats methed interface
func (c *KafkaChannel) SendRecv(topic string, bytes []byte, timeout int) ([]byte, error) {
	return nil, errors.New("unsupported commond SendRecv")
}
