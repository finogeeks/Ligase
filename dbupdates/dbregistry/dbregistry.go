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
