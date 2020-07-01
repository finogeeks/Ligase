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

package consumers

import (
	"github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common/config"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/dbtypes"
	"sync"
)

var regMu sync.RWMutex
var newHandler = make(map[int64]func() ConsumerInterface)

type ConsumerInterface interface {
	OnMessage(*dbtypes.DBEvent) error
	Prepare(*config.Dendrite)
	Report(monitor.LabeledGauge)
	Start()
}

func Register(name int64, f func() ConsumerInterface) {
	regMu.Lock()
	defer regMu.Unlock()

	if f == nil {
		log.Panicf("Register: %s func nil", dbtypes.DBCategoryToStr(name))
	}

	if _, ok := newHandler[name]; ok {
		log.Panicf("Register: %s already registered", dbtypes.DBCategoryToStr(name))
	}

	newHandler[name] = f
}
