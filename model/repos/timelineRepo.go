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
	//log "github.com/finogeeks/ligase/skunkworks/log"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/feedstypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type TimeLineRepo struct {
	bukSize    int
	tlSize     int
	tlLimit    bool
	buks       []*sync.Map
	lru        *Lru
	entryLimit bool
}

func NewTimeLineRepo(
	bukSize int,
	tlSize int,
	tlLimit bool,
	maxEntries,
	gcPerNum int,
) *TimeLineRepo {
	tl := new(TimeLineRepo)

	tl.bukSize = bukSize
	tl.tlSize = tlSize
	tl.tlLimit = tlLimit
	tl.buks = make([]*sync.Map, bukSize)
	if maxEntries > 0 {
		tl.lru = NewLru(maxEntries, gcPerNum)
		tl.entryLimit = true
	}

	for i := 0; i < bukSize; i++ {
		tl.buks[i] = new(sync.Map)
	}

	return tl
}

func (tl *TimeLineRepo) GetKeyNumbers() (int, int) {
	count := 0
	for i := 0; i < tl.bukSize; i++ {
		tl.buks[i].Range(func(key interface{}, value interface{}) bool {
			count++
			return true
		})
	}

	if tl.lru != nil {
		return count, tl.lru.maxEntries
	}

	return count, -1
}

func (tl *TimeLineRepo) getSlot(key string) int {
	return int(common.CalcStringHashCode(key)) % tl.bukSize
}

//return removed feed
func (tl *TimeLineRepo) add(key string, feed feedstypes.Feed) feedstypes.Feed {
	if tl.entryLimit {
		toRemove := tl.lru.Add(key)
		if toRemove != nil {
			removeSlot := tl.getSlot(toRemove.(string))
			if _, ok := tl.buks[removeSlot].Load(toRemove); ok {
				tl.buks[removeSlot].Delete(toRemove)
				log.Infof("TimeLineRepo clean memory remove feeds for key: %s", toRemove)
			}
		}
	}

	slot := tl.getSlot(key)

	val, ok := tl.buks[slot].Load(key)
	var tls *feedstypes.TimeLines
	if ok {
		tls = val.(*feedstypes.TimeLines)
	} else {
		tls = feedstypes.NewEvTimeLines(tl.tlSize, tl.tlLimit)
		actual, loaded := tl.buks[slot].LoadOrStore(key, tls)
		if loaded {
			tls = actual.(*feedstypes.TimeLines)
		}
	}

	return tls.Add(feed)
}

func (tl *TimeLineRepo) setDefault(key string) {
	if tl.entryLimit {
		toRemove := tl.lru.Add(key)
		if toRemove != nil {
			removeSlot := tl.getSlot(toRemove.(string))
			if _, ok := tl.buks[removeSlot].Load(toRemove); ok {
				tl.buks[removeSlot].Delete(toRemove)
				log.Errorf("TimeLineRepo clean memory remove feeds for key: %s", toRemove)
			}
		}
	}

	slot := tl.getSlot(key)
	if _, ok := tl.buks[slot].Load(key); !ok {
		tls := feedstypes.NewEvTimeLines(tl.tlSize, tl.tlLimit)
		tl.buks[slot].LoadOrStore(key, tls)
	}
}

func (tl *TimeLineRepo) getTimeLine(key string) *feedstypes.TimeLines {
	if tl.entryLimit {
		tl.lru.Get(key)
	}

	slot := tl.getSlot(key)

	if val, ok := tl.buks[slot].Load(key); ok {
		return val.(*feedstypes.TimeLines)
	} else {
		return nil
	}
}

func (tl *TimeLineRepo) exits(key string) bool {
	slot := tl.getSlot(key)
	_, ok := tl.buks[slot].Load(key)

	return ok
}

func (tl *TimeLineRepo) remove(key string) {
	slot := tl.getSlot(key)
	tl.buks[slot].Delete(key)
}
