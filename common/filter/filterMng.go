// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package filter

import (
	"context"
	"sync"

	"github.com/irfansharif/cfilter"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var once sync.Once
var filterMng *FilterMng

type FilterMng struct {
	repo sync.Map
}

func GetFilterMng() *FilterMng {
	once.Do(func() {
		filterMng = new(FilterMng)
	})

	return filterMng
}

func (mng *FilterMng) Register(key string, loader FilterLoader) *Filter {
	if val, ok := mng.repo.Load(key); !ok {
		filter := new(Filter)
		filter.f = cfilter.New()
		filter.key = key
		filter.loader = loader
		filter.ready = false
		mng.repo.Store(key, filter)

		return filter
	} else {
		log.Errorf("filter %s already registered", key)
		return val.(*Filter)
	}
}

type Filter struct {
	f      *cfilter.CFilter
	key    string
	loader FilterLoader
	ready  bool
	mu     sync.Mutex
}

func (f *Filter) Load() {
	if f.ready == false {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.ready == true {
			return
		}

		if f.loader.LoadFilterData(context.TODO(), f.key, f) {
			f.ready = true
			log.Infof("filter key:%s load %d", f.key, f.f.Count())
		}
	}
}

func (f *Filter) Insert(data []byte) {
	res := f.f.Insert(data)
	if res == false {
		log.Infof("filter key:%s insert %s error", f.key, string(data))
	}
}

//may false positive
func (f *Filter) Lookup(data []byte) bool {
	return f.f.Lookup(data)
}

func (f *Filter) Delete(data []byte) {
	if f.f.Lookup(data) {
		f.f.Delete(data)
	}
}

func (f *Filter) InsertUniq(data []byte) {
	if !(f.Lookup(data)) {
		f.Insert(data)
	}
}
