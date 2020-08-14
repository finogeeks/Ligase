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

package cache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/model/service"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type LocalCacheRepo struct {
	repos   []*sync.Map
	nameMap sync.Map
	offset  int
	regMtx  sync.Mutex

	cap      int
	cnt      int32
	duration int32
	cleanIdx int32
	ticker   *time.Timer

	cleanBukSize int32
	cleanStep    int32
	cleanCap     int32
	cleanItems   []*cleanItem

	liveCnt int
}

type cleanItem struct {
	mux      sync.Mutex
	cleanBuk []*sync.Map
}

func (ci *cleanItem) init(size int32) {
	ci.cleanBuk = make([]*sync.Map, size)
	for i := int32(0); i < size; i++ {
		ci.cleanBuk[i] = new(sync.Map)
	}
}

//cap: clean total slot cnt = slot_cnt*step
//step: slot width
//cur: clean slot
//old: last slot
//now: now slot
func (ci *cleanItem) update(lc *LocalCacheRepo, item *service.CacheItem, cap, step, cur, target int32) {
	tgtSlot := (target % cap) / step
	old := atomic.LoadInt32(&(item.Offset))
	oldSlot := (old % cap) / step
	curSlot := (cur % cap) / step
	key := item.Key

	repo := lc.repos[item.Repo]
	if _, ok := repo.Load(key); !ok {
		repo.Store(key, item)
		log.Errorf("update item may missing key:%s repo:%s", key, lc.GetRegisterReverse(item.Repo))
	}

	if (oldSlot == tgtSlot) && (old != -1) {
		if atomic.CompareAndSwapInt32(&(item.Offset), old, target) == false {
			log.Errorf("update swap key:%s old:%d target:%d actual: %d repo:%s", key, old, target, atomic.LoadInt32(&(item.Offset)), lc.GetRegisterReverse(item.Repo))
		}
		return
	}

	item.Mux.Lock()
	defer item.Mux.Unlock()

	old = atomic.LoadInt32(&(item.Offset))
	oldSlot = (old % cap) / step
	ci.cleanBuk[tgtSlot].Store(key, true)

	//log.Errorf("update set clean store key:%s slot:%d cur:%d repo:%s\, key, tgtSlot, curSlot, lc.GetRegisterReverse(item.repo))
	if old == -1 {
		//log.Errorf("add item key:%s slot:%d repo:%s", key, tgtSlot, lc.GetRegisterReverse(item.repo))
		return
	}

	atomic.StoreInt32(&(item.Offset), target)
	if curSlot == oldSlot {
		ci.mux.Lock()
		repo := lc.repos[item.Repo]
		if _, ok := repo.Load(key); !ok {
			repo.Store(key, item)
		}
		ci.cleanBuk[curSlot].Delete(key)
		//log.Errorf("update set clean del key:%s slot:%d cur:%d repo:%s", key, curSlot, curSlot, lc.GetRegisterReverse(item.repo))
		ci.mux.Unlock()
	} else {
		ci.cleanBuk[oldSlot].Delete(key)
		//log.Errorf("update set clean del key:%s slot:%d cur:%d repo:%s", key, oldSlot, curSlot, lc.GetRegisterReverse(item.repo))
	}

}

func (lc *LocalCacheRepo) Register(repoName string) int {
	val, ok := lc.nameMap.Load(repoName)
	if ok {
		return val.(int)
	} else {
		lc.regMtx.Lock()
		defer lc.regMtx.Unlock()
		ret := lc.offset
		if ret >= lc.cap {
			log.Fatalf("Register %s exceed repo capacity %d", repoName, lc.cap)
			return -1
		} else {
			lc.offset++
			lc.nameMap.Store(repoName, ret)
			return ret
		}

	}
}

func (lc *LocalCacheRepo) GetRegister(repoName string) int {
	if repoName == "" {
		log.Panic("can't register without name")
	}

	val, ok := lc.nameMap.Load(repoName)
	if ok {
		return val.(int)
	} else {
		return -1
	}
}

func (lc *LocalCacheRepo) GetRegisterReverse(repoName int) string {
	res := ""
	lc.nameMap.Range(func(key, value interface{}) bool {
		if value.(int) == repoName {
			res = key.(string)
		}
		return true
	})

	return res
}

func (lc *LocalCacheRepo) Put(repoName int, key, val interface{}) *service.CacheItem {
	if val == nil {
		log.Panic("Put nil value")
	}
	return lc.putWithDuration(repoName, key, val, lc.duration)
}

func (lc *LocalCacheRepo) Tie(repoName int, key interface{}, ref *service.CacheItem) {
	if repoName < 0 || repoName >= lc.cap {
		return
	}

	if ref.Ref != nil {
		log.Panic("can't ref more than 1 level")
	}

	repo := lc.repos[repoName]

	item, ok := repo.Load(key)
	if ok {
		item.(*service.CacheItem).Ref = ref
	}
}

/*
func (lc *LocalCacheRepo) Touch(repoName int, key interface{}) {
	if repoName < 0 || repoName >= lc.cap {
		return
	}

	repo := lc.repos[repoName]

	item, ok := repo.Load(key)
	if ok {
		atomic.StoreInt32(&item.(*CacheItem).offset, atomic.LoadInt32(&lc.cnt)+lc.duration)
	}
}
*/

//duration= -1 means never cleaned by timer
func (lc *LocalCacheRepo) putWithDuration(repoName int, key, val interface{}, duration int32) *service.CacheItem {
	if repoName < 0 || repoName >= lc.cap {
		log.Errorf("putWithDuration range error repo:%s key %v", lc.GetRegisterReverse(repoName), key)
		return nil
	}

	repo := lc.repos[repoName]

	item, ok := repo.Load(key)
	cnt := atomic.LoadInt32(&lc.cnt)
	ci := lc.cleanItems[repoName]
	if ok {
		item.(*service.CacheItem).Val = val
		ref := item.(*service.CacheItem).Ref
		target := cnt + duration

		if target != atomic.LoadInt32(&item.(*service.CacheItem).Offset) {
			ci.update(lc, item.(*service.CacheItem), lc.cleanCap, lc.cleanStep, cnt, target)
		}

		if (ref != nil) && (target != atomic.LoadInt32(&(ref.Offset))) {
			ci = lc.cleanItems[ref.Repo]
			ci.update(lc, ref, lc.cleanCap, lc.cleanStep, cnt, target)
		}

		return item.(*service.CacheItem)
	}

	itemc := new(service.CacheItem)
	itemc.Val = val
	itemc.Key = key
	itemc.Offset = -1
	itemc.Ref = nil
	itemc.Repo = repoName
	repo.Store(key, itemc)
	ci.update(lc, itemc, lc.cleanCap, lc.cleanStep, cnt, cnt+duration)

	return itemc

}

func (lc *LocalCacheRepo) Get(repoName int, key interface{}) (interface{}, bool) {
	return lc.getWithDuration(repoName, key, lc.duration)
}

func (lc *LocalCacheRepo) getWithDuration(repoName int, key interface{}, duration int32) (interface{}, bool) {
	if repoName < 0 || repoName >= lc.cap {
		log.Errorf("getWithDuration range error repo:%s key %v", lc.GetRegisterReverse(repoName), key)
		return nil, false
	}

	repo := lc.repos[repoName]
	item, ok := repo.Load(key)
	if ok {
		if item.(*service.CacheItem).Val == nil {
			log.Panic("getWithDuration hit but val nil")
		}

		cnt := atomic.LoadInt32(&lc.cnt)
		ref := item.(*service.CacheItem).Ref
		target := cnt + duration

		if target != atomic.LoadInt32(&item.(*service.CacheItem).Offset) {
			ci := lc.cleanItems[repoName]
			ci.update(lc, item.(*service.CacheItem), lc.cleanCap, lc.cleanStep, cnt, target)
		}

		if (ref != nil) && (target != atomic.LoadInt32(&(ref.Offset))) {
			ci := lc.cleanItems[ref.Repo]
			ci.update(lc, ref, lc.cleanCap, lc.cleanStep, cnt, target)
		}
		return item.(*service.CacheItem).Val, true
	}

	return nil, false
}

/*
func (lc *LocalCacheRepo) Remove(repoName int, key interface{}) {
	if repoName < 0 || repoName >= lc.cap {
		return
	}

	repo := lc.repos[repoName]
	repo.Delete(key)
}
*/

func (lc *LocalCacheRepo) Start(cap, duration int) {
	lc.cnt = 0
	lc.duration = 300
	lc.cleanBukSize = 16
	if duration != 0 {
		lc.duration = int32(duration)
		if lc.duration < lc.cleanBukSize-1 {
			lc.duration = lc.cleanBukSize - 1
		}
	}
	lc.ticker = time.NewTimer(0)
	lc.cap = cap
	lc.repos = make([]*sync.Map, lc.cap)
	for i := 0; i < lc.cap; i++ {
		lc.repos[i] = new(sync.Map)
	}

	lc.cleanStep = lc.duration / (lc.cleanBukSize - 1)
	lc.cleanCap = lc.cleanBukSize * lc.cleanStep
	lc.cleanIdx = (lc.cleanBukSize - 1) * lc.cleanStep

	lc.cleanItems = make([]*cleanItem, lc.cleanBukSize)
	for i := 0; i < lc.cap; i++ {
		lc.cleanItems[i] = new(cleanItem)
		lc.cleanItems[i].init(lc.cleanBukSize)
	}
	go lc.clean()
}

func (lc *LocalCacheRepo) clean() {
	clean := lc.cleanIdx
	for {
		select {
		case <-lc.ticker.C:
			cnt := atomic.AddInt32(&lc.cnt, 1)
			lc.ticker.Reset(time.Second)

			if cnt == clean {
				//log.Warn("local_cache start clean")
				//repoCount := 0
				//cleanRepoCount := 0
				for i := 0; i < lc.cap; i++ {
					repo := lc.repos[i]
					ci := lc.cleanItems[i]
					slot := (cnt % lc.cleanCap) / lc.cleanStep
					cleanRepo := ci.cleanBuk[slot]
					ci.mux.Lock()
					cleanRepo.Range(func(key, value interface{}) bool {
						if value != nil {
							//log.Errorf("try key:%s slot:%d repo:%s %v %v clean:%d", key, slot, lc.GetRegisterReverse(i), value, ok, clean)
							repo.Delete(key)
							//log.Errorf("clean routine del item key:%s slot:%d repo:%s", key, slot, lc.GetRegisterReverse(i))
							//repoCount++
							cleanRepo.Delete(key)
							//log.Errorf("clean routine clean del key:%s slot:%d repo:%s", key, slot, lc.GetRegisterReverse(i))
						} else {
							//log.Panic("clean routine del item key:%s slot:%d repo:%s but value nil", key, slot, lc.GetRegisterReverse(i))
						}
						//cleanRepoCount++
						return true
					})

					ci.mux.Unlock()
				}
				//log.Warn("local_cache start clean finished: ", "repoCount: ", repoCount, " cleanRepoCount:", cleanRepoCount)
				clean = atomic.AddInt32(&lc.cleanIdx, lc.cleanStep)
			}
			//log.Errorf("cnt:%d idx:%d step:%d cap:%d", cnt, lc.cleanIdx, lc.cleanStep, lc.cleanCap)
		}
	}
}
