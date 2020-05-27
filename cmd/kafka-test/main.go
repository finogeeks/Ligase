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

package main

import (
	"bufio"
	"fmt"
	sarama "github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	dendriteLog "github.com/finogeeks/ligase/skunkworks/log"
	// "github.com/confluentinc/confluent-kafka-go/kafka"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

var addr = []string{"kafka1:9091", "kafka2:9092", "kafka3:9093"}
var addr2 = "kafka1:9091, kafka2:9092, kafka3:9093"
var topic = "test2"

func startCofluentProducer() {
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": addr2})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create producer: %s", err)
		os.Exit(1)
	}

	doneChan := make(chan bool)

	go func() {
		defer close(doneChan)
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				m := ev
				if m.TopicPartition.Error != nil {
					fmt.Fprintf(os.Stderr, "Delivery failed: %v", m.TopicPartition.Error)
				} else {
					fmt.Fprintf(os.Stderr, "Delivered message to topic %s [%d] at offset %v",
						*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
				}
				return

			default:
				fmt.Fprintf(os.Stderr, "Ignored event: %s", ev)
			}
		}
	}()

	keys := []byte{0x00, 0x01, 0x02}
	for i := 0; i < 9; i++ {
		value := "Hello Go!" + strconv.Itoa(i)
		//kafka.PartitionAny
		partition := kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}
		p.ProduceChannel() <- &kafka.Message{
			TopicPartition: partition,
			Value:          []byte(value),
			Key:            []byte{keys[i%3]},
		}
		fmt.Fprintf(os.Stderr, "send producer topic:%s key:%s msg: %s", *partition.Topic, strconv.Itoa(i%3), value)
		time.Sleep(time.Second)
	}

	// wait for delivery report goroutine to finish
	_ = <-doneChan

	p.Close()

}

func startCofluentConsumer(groupId string) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":               addr[1],
		"group.id":                        groupId,
		"session.timeout.ms":              20000,
		"go.events.channel.enable":        true,
		"go.application.rebalance.enable": true,
		"default.topic.config":            kafka.ConfigMap{"auto.offset.reset": "earliest"}})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create consumer: %s", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Created Consumer %v", c)
	topics := []string{topic}
	err = c.SubscribeTopics(topics, nil)

	run := true
	for run == true {
		select {
		case sig := <-sigchan:
			fmt.Fprintf(os.Stderr, "Caught signal %v: terminating", sig)
			run = false

		case ev := <-c.Events():
			switch e := ev.(type) {
			case kafka.AssignedPartitions:
				fmt.Fprintf(os.Stderr, "consumer %% %v", e)
				c.Assign(e.Partitions)
			case kafka.RevokedPartitions:
				fmt.Fprintf(os.Stderr, "consumer %% %v", e)
				c.Unassign()
			case *kafka.Message:
				fmt.Fprintf(os.Stderr, "consumer %% Message on topic:%s partition:%d offset:%d val:%s grp:%s",
					*e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value), groupId)
			case kafka.PartitionEOF:
				fmt.Fprintf(os.Stderr, "consumer %% Reached %v", e)
			case kafka.Error:
				fmt.Fprintf(os.Stderr, "consumer %% Error: %v", e)
				c.SubscribeTopics(topics, nil)
				//run = false
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Closing consumer")
	c.Close()
}

func startProducer() {
	p, err := sarama.NewAsyncProducer(addr, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create producer: %s", err)
		os.Exit(1)
	}

	for i := 0; i < 1000; i++ {
		value := "Hello Go!" + strconv.Itoa(i)
		msg := &sarama.ProducerMessage{
			Topic: "test",
			Key:   sarama.StringEncoder("123"),
			Value: sarama.ByteEncoder([]byte(value)),
		}

		p.Input() <- msg
		fmt.Fprintf(os.Stderr, "send producer msg: %s", value)
		time.Sleep(time.Second)
	}

	// wait for delivery report goroutine to finish

	p.Close()

}

func startConsumer() {
	sarama.Logger = log.New(os.Stderr, "[TEST]", log.LstdFlags)
	config := cluster.NewConfig()
	config.Group.Mode = cluster.ConsumerModeMultiplex
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	sarama.MaxRequestSize = int32(config.Producer.MaxMessageBytes + 1)

	consumer, err := cluster.NewConsumer(addr, "group1", []string{"test"}, config)
	dendriteLog.Errorf("------start consumer ")
	if err != nil {
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// consume partitions
	go func() {
		for {
			select {
			case part, ok := <-consumer.Partitions():
				if !ok {
					dendriteLog.Errorf("failed process part")
					return
				}

				for message := range part.Messages() {
					dendriteLog.Errorf("-------kafka topic:%s part:%d offset:%d key:%s val:%s", message.Topic, message.Partition, message.Offset, message.Key, message.Value)
					consumer.MarkOffset(message, "")
				}
				part.Close()
			case <-signals:
				dendriteLog.Errorf("kafka recv signals")
				return
			}
		}
	}()
}

func main() {
	logCfg := new(dendriteLog.LogConfig)
	logCfg.Level = "info"
	logCfg.Underlying = "zap"
	logCfg.ZapConfig.JsonFormat = true
	logCfg.ZapConfig.BtEnabled = false
	dendriteLog.Setup(logCfg)

	ncpu := runtime.NumCPU()
	runtime.GOMAXPROCS(ncpu)

	//go startConsumer()
	//go startProducer()

	//go startCofluentConsumer("group-1")
	//go startCofluentConsumer("group-2")
	//go startCofluentProducer()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Fprintln(os.Stderr, "bye....")
		os.Exit(1)
	}
}
