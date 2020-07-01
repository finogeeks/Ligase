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

package apiconsumer

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/json-iterator/go"
	"github.com/nats-io/go-nats"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	APITypeMin int8 = iota

	APITypeAuth
	APITypeExternal
	APITypeInternal
	APITypeInternalAuth
	APITypeDownload
	APITypeUpload
	APITypeFed

	APITypeMax
)

var (
	processorsMap sync.Map
)

func SetAPIProcessor(p APIProcessor) {
	apiType := p.GetAPIType()
	if apiType <= APITypeMin || apiType >= APITypeMax {
		log.Panicf("invalid api type %d for [%s]", apiType, p.GetRoute())
		return
	}
	prefix := p.GetPrefix()
	for _, v := range prefix {
		if v != "r0" && v != "v1" && v != "inr0" && v != "sys" && v != "unstable" && v != "mediaR0" && v != "mediaV1" && v != "fedV1" {
			log.Panicf("invalid prefix type %s for [%s]", v, p.GetRoute())
			return
		}
	}
	msgType := p.GetMsgType()
	if _, ok := processorsMap.Load(msgType); ok {
		log.Panicf("msgtype register duplicate %x", msgType)
		return
	}
	processorsMap.Store(msgType, p)
}

func GetAPIProcessor(msgType int32) APIProcessor {
	v, _ := processorsMap.Load(msgType)
	return v.(APIProcessor)
	// if ok {
	// 	return v.(APIProcessor)
	// }
	// return nil
}

func ForeachAPIProcessor(h func(p APIProcessor) bool) {
	processorsMap.Range(func(k, v interface{}) bool {
		return h(v.(APIProcessor))
	})
}

type APIProcessor interface {
	GetRoute() string
	GetMetricsName() string
	GetMsgType() int32
	GetAPIType() int8
	GetMethod() []string
	GetTopic(cft *config.Dendrite) string
	NewRequest() core.Coder
	FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error
	NewResponse(code int) core.Coder
	// GetPath valid: r0 v1 inr0 sys unstable mediaR0 mediaV1 fedV1
	GetPrefix() []string

	Process(ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder)
}

type APIEvent struct {
	reply     string
	topic     string
	partition int32
	data      []byte
}

type APIConsumer struct {
	name     string
	userData interface{}
	handlers sync.Map
	msgChan  chan APIEvent
	Cfg      config.Dendrite
	RpcCli   *common.RpcClient
}

func (c *APIConsumer) GetCfg() config.Dendrite {
	return c.Cfg
}

func (c *APIConsumer) GetRpcCli() *common.RpcClient {
	return c.RpcCli
}

func (c *APIConsumer) Init(name string, ud interface{}, topic string) {
	c.name = name
	c.userData = ud
	c.msgChan = make(chan APIEvent, 4096)
	c.setupReply(topic)

	ForeachAPIProcessor(func(p APIProcessor) bool {
		if p.GetTopic(&c.Cfg) == topic {
			c.RegisterHandler(p.GetMsgType(), p)
		}
		return true
	})
}

func (c *APIConsumer) InitGroup(name string, ud interface{}, topic, grp string) {
	c.name = name
	c.userData = ud
	c.msgChan = make(chan APIEvent, 4096)
	c.setupGroupReply(topic, grp)

	ForeachAPIProcessor(func(p APIProcessor) bool {
		if p.GetTopic(&c.Cfg) == topic {
			c.RegisterHandler(p.GetMsgType(), p)
		}
		return true
	})
}

func (c *APIConsumer) Start() {
	c.startWorkder(c.msgChan)
}

func (c *APIConsumer) SetupTransport() {
	// val, ok := common.GetTransportMultiplexer().GetNode("n0")
	// if ok {
	// 	tran := val.(core.ITransport)
	// 	tran.AddChannel(core.CHANNEL_SUB, "proxyData", "proxyData", "")
	// } else {
	// 	log.Errorf("addConsumer can't find transport %s", "n0")
	// }

	// cfg := &c.Cfg.Kafka.Consumer.ProxyHandle
	// val, ok := common.GetTransportMultiplexer().GetChannel(
	// 	cfg.Underlying,
	// 	cfg.Name,
	// )
	// if ok {
	// 	fmt.Println("=================", ok)
	// 	channel := val.(core.IChannel)
	// 	channel.SetHandler(c)
	// 	//channel.PreStart("nats://nats1:4222") // TODO: cjw
	// 	channel.Start()
	// }
}

func (c *APIConsumer) RegisterHandler(msgType int32, p APIProcessor) {
	if _, ok := c.handlers.Load(msgType); ok {
		log.Warnf("%s msgType[%x] has register\n", c.name, msgType)
		return
	}
	c.handlers.Store(msgType, p)
}

func (c *APIConsumer) setupReply(topic string) {
	c.RpcCli.Reply(topic, c.onRpcMsg)
}

func (c *APIConsumer) setupGroupReply(topic, grp string) {
	c.RpcCli.ReplyGrp(topic, grp, c.onRpcMsg)
}

func (c *APIConsumer) onRpcMsg(msg *nats.Msg) {
	c.msgChan <- APIEvent{
		topic:     msg.Subject,
		partition: -1,
		data:      msg.Data,
		reply:     msg.Reply,
	}
}

func (c *APIConsumer) startWorkder(msgChan chan APIEvent) {
	go func() {
		for ev := range msgChan {
			go func(ev APIEvent) {
				data := c.OnMessage(ev.topic, ev.partition, ev.data)
				output := &internals.OutputMsg{}
				err := output.Decode(data)
				if err != nil {
					log.Warnf("output msg decode err:%s", err.Error())
					output := &internals.OutputMsg{
						MsgType: internals.MSG_RESP_ERROR,
						Code:    http.StatusInternalServerError,
					}
					output.Body, _ = jsonerror.Unknown("output msg decode err").Encode()
					data, _ := output.Encode()
					c.RpcCli.Pub(ev.reply, data)
				} else {
					if output.Code != internals.HTTP_RESP_DISCARD {
						c.RpcCli.Pub(ev.reply, data)
					} else {
						//do not response to nats
					}
				}
			}(ev)
		}
	}()
}

func (c *APIConsumer) OnMessage(topic string, partition int32, data []byte) []byte {
	input := &internals.InputMsg{}
	err := input.Decode(data)
	if err != nil {
		output := &internals.OutputMsg{
			MsgType: internals.MSG_RESP_ERROR,
			Code:    http.StatusInternalServerError,
		}
		output.Body, _ = jsonerror.Unknown("input msg decode error").Encode()
		resp, _ := output.Encode()
		return resp
	}
	output, err := c.process(input)
	if err != nil {
		log.Errorf("process %s msg err: %s", c.name, err.Error())
		output := &internals.OutputMsg{
			MsgType: internals.MSG_RESP_ERROR,
			Code:    http.StatusInternalServerError,
		}
		output.Body, _ = jsonerror.Unknown("process input error").Encode()
		resp, _ := output.Encode()
		return resp
	}

	resp, err := output.Encode()
	if err != nil {
		output := &internals.OutputMsg{
			MsgType: internals.MSG_RESP_ERROR,
			Code:    http.StatusInternalServerError,
		}
		output.Body, _ = jsonerror.Unknown("output msg encode error").Encode()
		resp, _ := output.Encode()
		return resp
	}
	// common.GetTransportMultiplexer().SendWithRetry(
	// 	c.Cfg.Kafka.Producer.OutputClientData.Underlying,
	// 	c.Cfg.Kafka.Producer.OutputClientData.Name,
	// 	&core.TransportPubMsg{
	// 		Keys: []byte{},
	// 		Obj:  output,
	// 	},
	// )
	return resp
}

func (c *APIConsumer) process(input *internals.InputMsg) (*internals.OutputMsg, error) {
	if input == nil {
		return nil, errors.New("input is empty")
	}

	val, ok := c.handlers.Load(input.MsgType)
	if !ok {
		return nil, fmt.Errorf("msg[%x] hasn't register", input.MsgType)
	}

	processor := val.(APIProcessor)
	msg := processor.NewRequest()
	if msg != nil {
		if err := msg.Decode(input.Payload); err != nil {
			return nil, err
		}
	}
	var device *authtypes.Device
	if input.Device != nil {
		device = &authtypes.Device{
			ID:           input.Device.ID,
			UserID:       input.Device.UserID,
			DisplayName:  input.Device.DisplayName,
			DeviceType:   input.Device.DeviceType,
			IsHuman:      input.Device.IsHuman,
			Identifier:   input.Device.Identifier,
			CreateTs:     input.Device.CreateTs,
			LastActiveTs: input.Device.LastActiveTs,
		}
	}
	code, resp := processor.Process(c.userData, msg, device)
	output := &internals.OutputMsg{}
	switch resp.(type) {
	case *internals.RespMessage:
		output.MsgType = internals.MSG_RESP_MESSAGE
	case *jsonerror.MatrixError:
		output.MsgType = internals.MSG_RESP_ERROR
	default:
		output.MsgType = input.MsgType
	}
	output.Code = code
	var err error
	if resp != nil {
		output.Body, err = resp.Encode()
	}
	return output, err
}
