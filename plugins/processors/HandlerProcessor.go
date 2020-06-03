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
	"github.com/finogeeks/ligase/core"
)

type HandlerProcessor struct {
}

func init() {
	core.RegisterProcessor("handler", newHandlerProcessor)
}

func newHandlerProcessor(conf interface{}) (core.IProcessor, error) {
	k := new(HandlerProcessor)
	return k, nil
}

func (p *HandlerProcessor) OnMultiplexerMessage(serviceID string, msg interface{}) (interface{}, error) {
	return nil, nil
}

func (m *HandlerProcessor) OnMessage(topic string, partition int32, data []byte) ([]byte, error) {
	return nil, nil
}

func (m *HandlerProcessor) PreStart() {
}

func (m *HandlerProcessor) Start() {
}
