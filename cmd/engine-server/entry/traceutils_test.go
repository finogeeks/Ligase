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

package entry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/nats.go"
	"github.com/opentracing/opentracing-go"
)

func passContext(ctx context.Context) {
	log.Info("b1:", ctx)
	log.Info("b2:", &ctx)

}

func TestPassContextByCall(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key001", "value001")
	log.Info("a1:", ctx)
	passContext(ctx)
	log.Info("a2:", ctx)
	log.Info("a3:", &ctx)
}

func TestPassContextByChannel(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key001", "value001")
	log.Info("a1:", ctx)
	log.Info("a3:", &ctx)
	ctxChan := make(chan context.Context, 10)
	ctxChan <- ctx
	ctx2 := <-ctxChan
	log.Info("a2:", ctx2)
	log.Info("a4:", &ctx2)
}

func passSpan(span opentracing.Span) {
	log.Info("b2:", &span)
	span.SetBaggageItem("key001", "bbb001")
	log.Info("b1:", common.InjectSpanToHeader(span))
	span.Finish() // for test
}

func TestSpan(t *testing.T) {
	configPath := "/Users/yuwu/svn/finochat/dendrite/config/dendrite_test_zs.yaml"
	err := config.LoadMonolithic(configPath)
	if err != nil {
		log.Fatalf("Invalid config file: %s", err)
	}

	cfg := config.GetConfig()
	_, err = cfg.SetupTracing("Dendrite" + "test")
	if err != nil {
		log.Errorf("failed to start opentracing err:%v", err)
	}

	span := opentracing.StartSpan("test span")
	span.Finish()
	span.SetBaggageItem("key001", "value001")
	log.Info("a3:", &span)
	log.Info("a1:", common.InjectSpanToHeader(span))
	passSpan(span)
	log.Info("a2:", common.InjectSpanToHeader(span))

}

func TestRequestWithSpan(t *testing.T) {
	configPath := "/Users/yuwu/svn/finochat/dendrite/config/dendrite_test_zs.yaml"
	err := config.LoadMonolithic(configPath)
	if err != nil {
		log.Fatalf("Invalid config file: %s", err)
	}

	cfg := config.GetConfig()
	_, err = cfg.SetupTracing("Dendrite" + "test")
	if err != nil {
		log.Errorf("failed to start opentracing err:%v", err)
	}

	idg, _ := uid.NewDefaultIdGenerator(0)
	rpcClient := common.NewRpcClient("nats://127.0.0.1:4222", idg)
	rpcClient.Start(true)

	topic := "test_topic_001"

	go func() {
		cb := func(ctx context.Context, msg *nats.Msg) {
			span, ctx := common.StartSpanFromContext(ctx, "callback")
			log.Info(fmt.Sprintf("received Data[%s], Sub[%s], Reply[%s], span[%s]",
				string(msg.Data), msg.Subject, msg.Reply, span))
			rpcClient.Pub(msg.Reply, []byte("goodbye world!!!"))
		}

		//rpcClient.ReplyWithContext(topic, cb, nil)
		rpcClient.ReplyGrpWithContext(topic, "test_group", cb)
	}()

	time.Sleep(time.Second * 1)

	go func() {
		for true {
			func() {
				span, ctx := common.StartSobSomSpan(context.Background(),
					"TestRequestWithSpan")
				defer span.Finish()
				res, err := rpcClient.RequestWithContext(ctx, topic,
					[]byte("hello world!!!"), 30000)
				//res, err := rpcClient.Request(topic,
				//	[]byte("hello world!!!"), 30000)
				//res, err := rpcClient.Request(topic,
				//	[]byte("h"), 30000)
				//res, err := rpcClient.Request(topic,
				//	[]byte(`{"cs": "1587036620000", "cr": "1587036620000"}`), 30000)
				if err != nil {
					log.Error("rpcClient.RequestWithContext failed")
				}
				log.Info("result: ", string(res))
			}()
			time.Sleep(time.Second * 1)
		}
	}()

	for i := 0; i < 1000; i++ {
		time.Sleep(time.Second * 1)
	}
}

type kafkaConsumer001 struct {
}

func (s *kafkaConsumer001) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	span, ctx := common.StartSpanFromContext(ctx, "kafkaConsumer001")
	defer span.Finish()
	log.Info("span:", span)
	log.Info("ctx: ", ctx)
}

func TestKafkaWithSpan(t *testing.T) {
	configPath := "/Users/yuwu/svn/finochat/dendrite/config/dendrite_test_zs.yaml"
	err := config.LoadMonolithic(configPath)
	if err != nil {
		log.Fatalf("Invalid config file: %s", err)
	}

	cfg := config.GetConfig()
	_, err = cfg.SetupTracing("Dendrite" + "test")
	if err != nil {
		log.Errorf("failed to start opentracing err:%v", err)
	}

	transportMultiplexer, _ := core.GetMultiplexer("transport", nil)
	for _, v := range cfg.TransportConfs {
		tran, err := core.GetTransport(v.Name, v.Underlying, v)
		if err != nil {
			log.Fatalf("get transport name:%s underlying:%s fail err:%v", v.Name, v.Underlying, err)
		}
		tran.Init(false)
		tran.SetBrokers(v.Addresses)
		transportMultiplexer.AddNode(v.Name, tran)
	}
	common.SetTransportMultiplexer(transportMultiplexer)
	transportMultiplexer = common.GetTransportMultiplexer()

	addProducer(transportMultiplexer, config.ProducerConf{
		Topic: "topic001", Underlying: "kafka", Name: "producer001", Inst: 3, LingerMs: nil})
	addConsumer(transportMultiplexer, config.ConsumerConf{
		Topic: "topic001", Underlying: "kafka", Name: "consumer001", Group: "group001"},
		0)
	transportMultiplexer.PreStart()

	val, ok := common.GetTransportMultiplexer().GetChannel(
		"kafka",
		"consumer001",
	)

	if ok {
		channel := val.(core.IChannel)
		s := &kafkaConsumer001{}
		channel.SetHandler(s)
	}

	transportMultiplexer.Start()

	time.Sleep(time.Second * 1)

	go func() {
		i := 0
		for true {
			i += 1
			func() {
				span := opentracing.StartSpan("TestKafkaWithSpan")
				defer span.Finish()
				common.ExportMetricsBeforeSending(span, "TestKafkaWithSpan", "kafka")
				common.GetTransportMultiplexer().SendWithRetry(
					"kafka",
					"producer001",
					&core.TransportPubMsg{
						Keys:    []byte(fmt.Sprintf("key[%d]", i)),
						Obj:     fmt.Sprintf("hello world[%d]", i),
						Headers: common.InjectSpanToHeaderForSending(span),
					})
			}()
			time.Sleep(time.Second * 1)
		}
	}()

	for i := 0; i < 1000; i++ {
		time.Sleep(time.Second * 1)
	}
}
