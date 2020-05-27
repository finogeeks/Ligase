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

package feedstypes

import (
	"sync"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

type TimeLinesAtomicData struct {
	Size   int
	Offset int
	Limit  bool

	Start int
	End   int
	Lower int64
	Upper int64
	Data  []Feed
	Ofo   bool
}

type TimeLines struct {
	size   int
	offset int
	limit  bool

	lower int64
	upper int64
	data  []Feed
	ofo   bool //标记是否乱序，对于有限timeline有用

	mutex sync.RWMutex
}

func NewEvTimeLines(size int, limit bool) *TimeLines {
	tl := new(TimeLines)

	tl.size = size
	tl.limit = limit
	tl.data = make([]Feed, size)
	tl.offset = 0
	tl.lower = 0
	tl.upper = 0
	tl.ofo = false

	return tl
}

func (tl *TimeLines) calc() {
	if tl.limit == false {
		return
	}

	if tl.offset <= tl.size {
		return
	}

	if tl.ofo == false {
		feed := tl.data[(tl.offset-tl.size)%tl.size]
		tl.lower = feed.GetOffset()
		feed = tl.data[(tl.offset-1)%tl.size]
		tl.upper = feed.GetOffset()
	} else {
		base := tl.offset - tl.size
		for i := 0; i < tl.size-1; i++ {
			for j := 0; j < tl.size-1-i; j++ {
				cur := tl.data[(base+j)%tl.size]
				next := tl.data[(base+j+1)%tl.size]
				if cur.GetOffset() > next.GetOffset() {
					tl.data[(base+j)%tl.size] = next
					tl.data[(base+j+1)%tl.size] = cur
				}
			}
		}

		feed := tl.data[(tl.offset-tl.size)%tl.size]
		tl.lower = feed.GetOffset()
		feed = tl.data[(tl.offset-1)%tl.size]
		tl.upper = feed.GetOffset()
	}
}

func (tl *TimeLines) checkOutOfOrder() {
	if tl.limit == false {
		return
	}

	if tl.offset <= 1 {
		return
	}

	cur := tl.data[(tl.offset-1)%tl.size]
	pre := tl.data[(tl.offset-2)%tl.size]

	if tl.ofo == false && cur != nil && pre != nil && cur.GetOffset() < pre.GetOffset() {
		tl.ofo = true
	}
}

func (tl *TimeLines) Console() {
	log.Errorf("timeline Console time.size %d, time.offset %d, tl.lower %d, tl.upper %d, len(tl.data) %d", tl.size, tl.offset, tl.lower, tl.upper, len(tl.data))
}

func (tl *TimeLines) Expanse() int {
	newsize := tl.size
	oldsize := tl.size
	if tl.offset >= tl.size {
		for {
			newsize = oldsize * 2
			oldsize = newsize
			if newsize > tl.offset {
				return newsize
			}
		}
	} else {
		return newsize
	}
}

func (tl *TimeLines) Add(feed Feed) Feed {
	if feed == nil {
		log.Errorf("timeline.Add nil time.size %d, time.offset %d, tl.lower %d, tl.upper %d, len(tl.data) %d", tl.size, tl.offset, tl.lower, tl.upper, len(tl.data))
		return nil
	}

	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	var remove Feed
	if tl.limit {
		remove = tl.data[tl.offset%tl.size]
		tl.data[tl.offset%tl.size] = feed
		tl.offset = tl.offset + 1
	} else {
		if tl.offset < tl.size {
			tl.data[tl.offset] = feed
			tl.offset = tl.offset + 1
		} else {
			//resize
			newsize := tl.Expanse()
			newdata := make([]Feed, newsize)
			for i := 0; i < tl.size; i++ {
				newdata[i] = tl.data[i]
			}
			tl.data = newdata
			tl.size = newsize
			tl.data[tl.offset] = feed
			tl.offset = tl.offset + 1
		}
		remove = nil
	}

	if tl.lower == 0 { //初始化
		tl.lower = feed.GetOffset()
		tl.upper = feed.GetOffset()
	} else {
		offset := feed.GetOffset()

		if offset < tl.lower {
			tl.lower = offset
		} else if offset > tl.upper {
			tl.upper = offset
		}

		tl.checkOutOfOrder()

		if remove != nil { //有序集合的删除&插入
			removeOffset := remove.GetOffset()

			if removeOffset == tl.lower || removeOffset == tl.upper { //recalc
				tl.calc()
			}
		}
	}

	return remove
}

func (tl *TimeLines) ForRange(f func(offset int, feed Feed) bool) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	start, end := tl.GetRange()
	for i := start; i < end; i++ {
		feed := tl.getFeed(i)
		if !f(i, feed) {
			break
		}
	}
}

func (tl *TimeLines) ForRangeReverse(f func(offset int, feed Feed) bool) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	start, end := tl.GetRange()
	for i := end - 1; i >= start; i-- {
		feed := tl.getFeed(i)
		if !f(i, feed) {
			break
		}
	}
}

func (tl *TimeLines) GetAllFeeds() ([]Feed, int, int, int64, int64) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	start, end := tl.GetRange()
	if start >= end {
		return []Feed{}, start, end, tl.lower, tl.upper
	}
	res := make([]Feed, 0, end-start)
	for i := start; i < end; i++ {
		v := tl.getFeed(i)
		res = append(res, v)
	}
	return res, start, end, tl.lower, tl.upper
}

func (tl *TimeLines) GetFeeds(fromPos, endPos int64) ([]Feed, int, int, int64, int64) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	start, end := tl.GetRange()
	if start >= end {
		return []Feed{}, start, end, tl.lower, tl.upper
	}
	if fromPos > 0 && endPos > 0 && fromPos >= endPos {
		return []Feed{}, start, end, tl.lower, tl.upper
	}

	lowerPos := tl.lower
	upperPos := tl.upper
	if fromPos > tl.lower && fromPos < tl.upper {
		lowerPos = fromPos
	}
	if endPos > 0 && endPos < tl.upper {
		upperPos = endPos
	}

	res := make([]Feed, 0, end-start)
	for i := start; i < end; i++ {
		v := tl.getFeed(i)
		if v.GetOffset() >= lowerPos && v.GetOffset() <= upperPos {
			res = append(res, v)
		}
	}

	return res, start, end, tl.lower, tl.upper
}

func (tl *TimeLines) GetAllFeedsReverse() ([]Feed, int, int, int64, int64) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	start, end := tl.GetRange()
	if start >= end {
		return []Feed{}, start, end, tl.lower, tl.upper
	}
	res := make([]Feed, 0, end-start)
	for i := end - 1; i >= start; i-- {
		v := tl.getFeed(i)
		res = append(res, v)
	}
	return res, start, end, tl.lower, tl.upper
}

// RAtomic 原子性读操作，尽量在回调中减少计算量，否则锁的时间加长
func (tl *TimeLines) RAtomic(f func(data *TimeLinesAtomicData)) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	start, end := tl.GetRange()

	f(&TimeLinesAtomicData{
		Size:   tl.size,
		Offset: tl.offset,
		Limit:  tl.limit,
		Start:  start,
		End:    end,
		Lower:  tl.lower,
		Upper:  tl.upper,
		Data:   tl.data,
		Ofo:    tl.ofo,
	})
}

func (tl *TimeLines) GetRange() (int, int) {
	if tl.offset < tl.size {
		return 0, tl.offset
	} else {
		return tl.offset - tl.size, tl.offset
	}
}

func (tl *TimeLines) getFeed(offset int) Feed {
	// tl.mutex.RLock()
	// defer tl.mutex.RUnlock()
	return tl.data[offset%tl.size]
}

func (tl *TimeLines) GetFeedRange() (int64, int64) {
	tl.mutex.RLock()
	defer tl.mutex.RUnlock()
	return tl.lower, tl.upper
}
