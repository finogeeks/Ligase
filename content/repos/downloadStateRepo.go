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
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type DownloadStateRepo struct {
	downloading sync.Map

	waiting map[string]int
	mutex   sync.Mutex
}

func NewDownloadStateRepo() *DownloadStateRepo {
	return &DownloadStateRepo{
		waiting: make(map[string]int),
	}
}

func (r *DownloadStateRepo) BuildKey(domain, netdiskID string) string {
	return domain + "/" + netdiskID
}

func (r *DownloadStateRepo) SplitKey(key string) (domain, netdiskID string, ok bool) {
	ss := strings.SplitN(key, "/", 2)
	if len(ss) < 2 {
		return "", "", false
	}
	return ss[0], ss[1], true
}

func (r *DownloadStateRepo) IsDownloading(domain, netdiskID string) bool {
	key := r.BuildKey(domain, netdiskID)
	_, ok := r.downloading.Load(key)
	return ok
}

func (r *DownloadStateRepo) AddDownload(key string) {
	cond := sync.NewCond(&sync.Mutex{})
	r.downloading.LoadOrStore(key, cond)
}

func (r *DownloadStateRepo) RemoveDownloading(key string) {
	val, _ := r.downloading.Load(key)
	if val != nil {
		cond := val.(*sync.Cond)
		cond.L.Lock()
		cond.Broadcast()
		cond.L.Unlock()
	}
	r.downloading.Delete(key)
}

func (r *DownloadStateRepo) Wait(ctx context.Context, domain, netdiskID string) bool {
	key := r.BuildKey(domain, netdiskID)
	if v, ok := r.downloading.Load(key); ok {
		log.Infof("wait download from remote, domain: %s, netdiskID: %s", domain, netdiskID)
		r.mutex.Lock()
		r.waiting[key] = r.waiting[key] + 1
		r.mutex.Unlock()
		defer func(key string) {
			r.mutex.Lock()
			defer r.mutex.Unlock()
			amt := r.waiting[key]
			if amt == 1 {
				delete(r.waiting, key)
			} else {
				r.waiting[key] = amt - 1
			}
		}(key)

		ch := make(chan struct{}, 1)
		go func(ch chan struct{}, cond *sync.Cond) {
			log.Infof("begin waiting download from remote, domain: %s, netdiskID: %s", domain, netdiskID)
			cond.L.Lock()
			cond.Wait()
			cond.L.Unlock()
			ch <- struct{}{}
			log.Infof("finished waiting download from remote, domain: %s, netdiskID: %s", domain, netdiskID)
		}(ch, v.(*sync.Cond))
		select {
		case <-ctx.Done():
			log.Infof("cancel download while waiting from remote, domain: %s, netdiskID: %s", domain, netdiskID)
			return true
		case <-ch:
			log.Infof("finished download while waiting from remote, domain: %s, netdiskID: %s", domain, netdiskID)
			return false
		}
	}
	return false
}

type WaitElem struct {
	key string
	amt int
}

func (we *WaitElem) GetKey() string {
	return we.key
}
func (we *WaitElem) GetAmt() int {
	return we.amt
}

type WaitListSorted []WaitElem

func (w WaitListSorted) Less(i, j int) bool { return w[i].amt < w[j].amt }
func (w WaitListSorted) Len() int           { return len(w) }
func (w WaitListSorted) Swap(i, j int)      { w[i], w[j] = w[j], w[i] }

func (r *DownloadStateRepo) GetWaitingList() WaitListSorted {
	resp := WaitListSorted{}
	r.mutex.Lock()
	for key, amt := range r.waiting {
		resp = append(resp, WaitElem{key, amt})
	}
	r.mutex.Unlock()
	sort.Sort(resp)
	return resp
}
