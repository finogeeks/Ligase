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

package dbregistry

import (
	"sync"

	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type DBRegistry struct {
	Persist dbupdatetypes.ProcessorFactory
	Cache   dbupdatetypes.CacheFactory
}

var (
	registry sync.Map
)

func Register(key string, persist dbupdatetypes.ProcessorFactory, cache dbupdatetypes.CacheFactory) {
	if persist == nil && cache == nil {
		return
	}
	_, loaded := registry.LoadOrStore(key, DBRegistry{Persist: persist, Cache: cache})
	if loaded {
		log.Errorf("dbupdates registry already exists %s", key)
	}
}

func GetAllKeys() []string {
	keys := []string{}
	registry.Range(func(k, v interface{}) bool {
		keys = append(keys, k.(string))
		return true
	})
	return keys
}

func GetRegistryProc(key string) (DBRegistry, bool) {
	v, ok := registry.Load(key)
	if !ok {
		return DBRegistry{}, false
	}
	return v.(DBRegistry), true
}
