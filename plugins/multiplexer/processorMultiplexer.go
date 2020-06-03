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

package transport

import (
	"errors"
	"sync"

	"github.com/finogeeks/ligase/core"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	core.RegisterMultiplexer("processor", NewProcessorMultiplexer)
}

func NewProcessorMultiplexer(conf interface{}) (core.IMultiplexer, error) {
	log.Printf("NewProcessorMultiplexer")
	k := new(ProcessorMultiplexer)
	return k, nil
}

type ProcessorMultiplexer struct {
	processorMap sync.Map
}

func (m *ProcessorMultiplexer) AddNode(nodeName string, node interface{}) bool {
	proc, ok := m.processorMap.Load(nodeName)
	if !ok {
		log.Errorf("Failed to addnode %s, processor already exits", nodeName)
		return false
	}

	proc, ok = node.(core.IProcessor)
	if !ok {
		log.Errorf("Failed to addnode %s, not a processor", nodeName)
		return false
	}

	m.processorMap.Store(nodeName, proc)

	return true
}

func (m *ProcessorMultiplexer) GetNode(nodeName string) (interface{}, bool) {
	return m.processorMap.Load(nodeName)
}

func (m *ProcessorMultiplexer) GetChannel(nodeName, serviceID string) (interface{}, bool) {
	return m.processorMap.Load(nodeName)
}

func (m *ProcessorMultiplexer) PreStartChannel(nodeName, serviceID string) bool {
	return true
}

func (m *ProcessorMultiplexer) SendNode(nodeName, serviceID string, obj interface{}) error {
	val, ok := m.processorMap.Load(nodeName)
	if !ok {
		log.Errorf("Failed to get processor %s not exits", nodeName)
		return errors.New("unknown processor")
	}

	proc := val.(core.IProcessor)
	proc.OnMultiplexerMessage(serviceID, obj)

	return nil
}

func (m *ProcessorMultiplexer) SendRecvNode(nodeName, serviceID string, obj interface{}) (interface{}, error) {
	val, ok := m.processorMap.Load(nodeName)
	if !ok {
		log.Errorf("Failed to get transport %s not exits", nodeName)
		return nil, errors.New("unknown processor")
	}

	proc := val.(core.IProcessor)
	return proc.OnMultiplexerMessage(serviceID, obj)
}

func (m *ProcessorMultiplexer) PreStart() {
	m.processorMap.Range(func(key, value interface{}) bool {
		tran := value.(core.IProcessor)
		tran.PreStart()
		return true
	})
}

func (m *ProcessorMultiplexer) Start() {
	m.processorMap.Range(func(key, value interface{}) bool {
		tran := value.(core.IProcessor)
		tran.Start()
		return true
	})
}

func (m *ProcessorMultiplexer) Send(nodeName, serviceID string, obj interface{}) error {
	return errors.New("unsupported commond Send")
}

func (m *ProcessorMultiplexer) SendAndRecv(nodeName, serviceID string, obj interface{}) error {
	return errors.New("unsupported commond SendAndRecv")
}

func (m *ProcessorMultiplexer) SendWithRetry(nodeName, serviceID string, obj interface{}) error {
	return errors.New("unsupported commond SendWithRetry")
}

func (m *ProcessorMultiplexer) SendAndRecvWithRetry(nodeName, serviceID string, obj interface{}) error {
	return errors.New("unsupported commond SendAndRecvWithRetry")
}
