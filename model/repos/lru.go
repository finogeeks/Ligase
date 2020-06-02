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

package repos

import (
	"container/list"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

type Key interface{}

type Lru struct {
	list        *list.List
	maxEntries  int
	removeCount int64
	mutex       *sync.Mutex
	cache       map[interface{}]*list.Element
	ticker      *time.Timer
	gcPerNum    int64
}

func NewLru(maxEntries, gcPerNum int) *Lru {
	lru := Lru{
		maxEntries:  maxEntries,
		list:        list.New(),
		mutex:       new(sync.Mutex),
		cache:       make(map[interface{}]*list.Element),
		removeCount: 1,
		ticker:      time.NewTimer(10),
		gcPerNum:    int64(gcPerNum),
	}

	if gcPerNum > 0 {
		go lru.startGc()
	}

	return &lru
}

func (lru *Lru) startGc() {
	for {
		select {
		case <-lru.ticker.C:
			lru.ticker.Reset(time.Second * 10)

			if lru.removeCount%lru.gcPerNum == 0 {
				log.Infof("lru start gc removeCount: %d", lru.removeCount)
				state := new(runtime.MemStats)
				runtime.ReadMemStats(state)
				log.Infof("lru before gc mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
					state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
				debug.FreeOSMemory()
				runtime.ReadMemStats(state)
				log.Infof("lru after gc mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
					state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
			}
		}
	}
}

func (lru *Lru) Add(key Key) Key {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	if lru.cache == nil {
		lru.cache = make(map[interface{}]*list.Element)
		lru.list = list.New()
	}

	if ee, ok := lru.cache[key]; ok {
		lru.list.MoveToFront(ee)
		return nil
	}
	ele := lru.list.PushFront(key)
	lru.cache[key] = ele
	if lru.maxEntries != 0 && lru.list.Len() > lru.maxEntries {
		if lru.cache == nil {
			return nil
		}
		ele := lru.list.Back()
		if ele != nil {
			lru.list.Remove(ele)
			kv := ele.Value.(Key)
			delete(lru.cache, kv)
			lru.removeCount++
			return kv
		}
	}

	return nil
}

func (lru *Lru) Get(key Key) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	if lru.cache == nil {
		return
	}
	if ele, hit := lru.cache[key]; hit {
		lru.list.MoveToFront(ele)
	}
	return
}
