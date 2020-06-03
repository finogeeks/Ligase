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

package common

import (
	"sync"

	"github.com/finogeeks/ligase/core"
)

type MultiplexerMng struct {
	transport core.IMultiplexer
	processor core.IMultiplexer
}

var MultiplexerMngInsance *MultiplexerMng
var onceTransportMng sync.Once

func getMultiplexerMngInstance() *MultiplexerMng {
	onceTransportMng.Do(func() {
		MultiplexerMngInsance = new(MultiplexerMng)
	})

	return MultiplexerMngInsance
}

func SetTransportMultiplexer(multp core.IMultiplexer) {
	instance := getMultiplexerMngInstance()
	instance.transport = multp
}

func SetProcessorMultiplexer(multp core.IMultiplexer) {
	instance := getMultiplexerMngInstance()
	instance.processor = multp
}

func GetTransportMultiplexer() core.IMultiplexer {
	instance := getMultiplexerMngInstance()
	return instance.transport
}

func GetProcessorMultiplexer() core.IMultiplexer {
	instance := getMultiplexerMngInstance()
	return instance.processor
}
