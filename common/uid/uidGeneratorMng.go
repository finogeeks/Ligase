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

package uid

import (
	"sync"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var once sync.Once
var instance *UidGeneratorMng

type UidGeneratorMng struct {
	generators sync.Map
	lock       sync.Mutex
}

func GetUidGeneratorMng() *UidGeneratorMng {
	once.Do(func() {
		instance = new(UidGeneratorMng)
	})

	return instance
}

func (m *UidGeneratorMng) Register(key string, start, end int64) {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.generators.Load(key)
	if !ok {
		idg, _ := NewIdGenerator(start, end)
		m.generators.Store(key, idg)
	} else {
		log.Errorf("UidGeneratorMng Register conflict key:%s", key)
	}
}

func (m *UidGeneratorMng) GetIdg(key string) *UidGenerator {
	val, ok := m.generators.Load(key)
	if ok {
		return val.(*UidGenerator)
	}

	return nil
}
