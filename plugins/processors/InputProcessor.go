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

package processor

import (
	"log"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

type InputProcessorHandler = func(msg core.Coder, input *internals.InputMsg) (int, core.Coder)

type IProcessor interface {
	OnMessage(serviceID string, msg interface{}) ([]byte, error)
	PreStart()
	Start()
}

func init() {
	core.RegisterProcessor("input", NewInputProcessor)
}

func NewInputProcessor(conf interface{}) (core.IProcessor, error) {
	log.Printf("NewTransportMultiplexer")
	k := new(InputProcessor)
	k.h = make(map[int32]InputProcessorHandler)
	return k, nil
}

type InputProcessor struct {
	h map[int32]InputProcessorHandler
}

func (m *InputProcessor) OnMultiplexerMessage(serviceID string, msg interface{}) (interface{}, error) {
	return nil, nil
}

func (m *InputProcessor) OnMessage(topic string, partition int32, data []byte) ([]byte, error) {
	input := &internals.InputMsg{}
	err := input.Decode(data)
	if err != nil {
		return nil, err
	}
	multiplexer, err := core.GetMultiplexer("processor", nil)
	if err != nil {
		return nil, err
	}
	resp, err := multiplexer.SendRecvNode("", "", input)
	var respData []byte
	if resp != nil {
		output := resp.(*internals.OutputMsg)
		respData, err = output.Encode()
		if err != nil {
			return nil, err
		}
	}
	return respData, err
}

func (m *InputProcessor) PreStart() {

}

func (m *InputProcessor) Start() {

}

func (m *InputProcessor) RegisterHandler(msgType int32, handler InputProcessorHandler) {
	if _, ok := m.h[msgType]; ok {
		// TODO: log warn
		return
	}
	m.h[msgType] = handler
}

func (m *InputProcessor) GetHandler(msgType int32) (InputProcessorHandler, bool) {
	h, ok := m.h[msgType]
	return h, ok
}
