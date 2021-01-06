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
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type downloadingConds struct {
	startDownloadCond    *sync.Cond
	finishedDownloadCond *sync.Cond
	hasStart             bool
	hasFinished          bool
}

type DownloadStateRepo struct {
	downloading sync.Map

	waiting map[string]int
	mutex   sync.Mutex

	downloadFiles sync.Map
	refMutex      sync.Mutex

	downloadingSize sync.Map
}

func NewDownloadStateRepo() *DownloadStateRepo {
	return &DownloadStateRepo{
		waiting: make(map[string]int),
	}
}

func (r *DownloadStateRepo) BuildKey(domain, netdiskID, thumbnailType string) string {
	return domain + "/" + netdiskID + "/" + thumbnailType
}

func (r *DownloadStateRepo) SplitKey(key string) (domain, netdiskID, thumbnailType string, ok bool) {
	ss := strings.SplitN(key, "/", 3)
	if len(ss) < 3 {
		return "", "", "", false
	}
	return ss[0], ss[1], ss[2], true
}

func (r *DownloadStateRepo) IsDownloading(domain, netdiskID, thumbnailType string) bool {
	key := r.BuildKey(domain, netdiskID, thumbnailType)
	_, ok := r.downloading.Load(key)
	return ok
}

func (r *DownloadStateRepo) AddDownload(key string) {
	conds := &downloadingConds{sync.NewCond(&sync.Mutex{}), sync.NewCond(&sync.Mutex{}), false, false}
	r.downloading.LoadOrStore(key, conds)
}

func (r *DownloadStateRepo) RemoveDownloading(key string) {
	val, _ := r.downloading.Load(key)
	if val != nil {
		conds := val.(*downloadingConds)
		conds.finishedDownloadCond.L.Lock()
		conds.hasFinished = true
		conds.finishedDownloadCond.Broadcast()
		conds.finishedDownloadCond.L.Unlock()
	}
	r.downloading.Delete(key)
}

func (r *DownloadStateRepo) Wait(ctx context.Context, domain, netdiskID, thumbnailType string) bool {
	key := r.BuildKey(domain, netdiskID, thumbnailType)
	v, ok := r.downloading.Load(key)
	if !ok {
		return false
	}
	if v.(*downloadingConds).hasFinished {
		return false
	}
	log.Infof("wait download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
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
	go func(ch chan struct{}, conds *downloadingConds) {
		log.Infof("begin waiting download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
		conds.finishedDownloadCond.L.Lock()
		if conds.hasFinished {
			conds.finishedDownloadCond.L.Unlock()
			ch <- struct{}{}
			log.Infof("finished waiting immediately download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
			return
		}
		conds.finishedDownloadCond.Wait()
		conds.finishedDownloadCond.L.Unlock()
		ch <- struct{}{}
		log.Infof("finished waiting download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
	}(ch, v.(*downloadingConds))
	select {
	case <-ctx.Done():
		log.Infof("cancel download while waiting from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
		return true
	case <-ch:
		log.Infof("finished download while waiting from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
		return false
	}
}

func (r *DownloadStateRepo) WaitStartDownload(ctx context.Context, domain, netdiskID, thumbnailType string) bool {
	key := r.BuildKey(domain, netdiskID, thumbnailType)
	v, ok := r.downloading.Load(key)
	if !ok {
		return false
	}
	if v.(*downloadingConds).hasStart {
		return false
	}
	log.Infof("wait download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
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
	go func(ch chan struct{}, conds *downloadingConds) {
		log.Infof("begin waiting download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
		conds.startDownloadCond.L.Lock()
		if conds.hasStart {
			conds.startDownloadCond.L.Unlock()
			ch <- struct{}{}
			log.Infof("finished waiting immediately download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
			return
		}
		conds.startDownloadCond.Wait()
		conds.startDownloadCond.L.Unlock()
		ch <- struct{}{}
		log.Infof("finished waiting download from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
	}(ch, v.(*downloadingConds))
	select {
	case <-ctx.Done():
		log.Infof("cancel download while waiting from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
		return true
	case <-ch:
		log.Infof("finished download while waiting from remote, domain: %s, netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
		return false
	}
}

func (r *DownloadStateRepo) StartDownload(domain, netdiskID, thumbnailType string) {
	key := r.BuildKey(domain, netdiskID, thumbnailType)
	val, _ := r.downloading.Load(key)
	if val != nil {
		conds := val.(*downloadingConds)
		conds.startDownloadCond.L.Lock()
		conds.hasStart = true
		conds.startDownloadCond.Broadcast()
		conds.startDownloadCond.L.Unlock()
	}
}

func (r *DownloadStateRepo) GetDownloadFilename(domain, netdiskID, thumbnailType string) string {
	return fmt.Sprintf("./media/%s_%s_%s", domain, netdiskID, thumbnailType)
}

func (r *DownloadStateRepo) incrDownloadRef(fn string, init bool) bool {
	r.refMutex.Lock()
	defer r.refMutex.Unlock()
	v, ok := r.downloadFiles.Load(fn)
	if !ok && init {
		i := new(int32)
		*i = 0
		v, ok = r.downloadFiles.LoadOrStore(fn, i)
	}
	if !ok {
		return false
	}
	atomic.AddInt32(v.(*int32), 1)
	return true
}

func (r *DownloadStateRepo) decrDownloadRef(fn string) {
	r.refMutex.Lock()
	defer r.refMutex.Unlock()
	v, ok := r.downloadFiles.Load(fn)
	if !ok {
		return
	}
	newVal := atomic.AddInt32(v.(*int32), -1)
	if newVal <= 0 {
		r.downloadFiles.Delete(fn)
		r.downloadingSize.Delete(fn)
		os.Remove(fn)
	}
}

type FileReader struct {
	file *os.File
	fn   string
	r    *DownloadStateRepo
}

func (r *FileReader) Read(data []byte) (int, error) {
	return r.file.Read(data)
}

func (r *FileReader) Close() error {
	go func() {
		time.Sleep(time.Second * 30)
		r.r.decrDownloadRef(r.fn)
	}()
	return r.file.Close()
}

func (r *DownloadStateRepo) WriteToFile(domain, netdiskID, thumbnailType string, reader io.Reader, contentLength int64) (io.ReadCloser, int64, error) {
	fn := r.GetDownloadFilename(domain, netdiskID, thumbnailType)
	file, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, 0, err
	}
	r.downloadingSize.Store(fn, contentLength)

	r.incrDownloadRef(fn, true)

	r.StartDownload(domain, netdiskID, thumbnailType)
	_, err = io.Copy(file, reader)
	size := int64(0)
	if stat, err := file.Stat(); err == nil {
		size = stat.Size()
	}
	file.Seek(0, os.SEEK_SET)
	return &FileReader{file, fn, r}, size, err
}

func (r *DownloadStateRepo) TryResponseFromLocal(domain, netdiskID, thumbnailType string, writer http.ResponseWriter) bool {
	fn := r.GetDownloadFilename(domain, netdiskID, thumbnailType)
	if !r.incrDownloadRef(fn, false) {
		return false
	}
	defer r.decrDownloadRef(fn)
	file, err := os.Open(fn)
	if err != nil {
		return false
	}
	log.Infof("download file from local domain: %s netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)
	filesize := int64(0)
	if sizeVal, _ := r.downloadingSize.Load(fn); sizeVal != nil {
		filesize = sizeVal.(int64)
	}
	if filesize > 0 {
		writer.Header().Set("Content-Length", strconv.FormatInt(filesize, 10))
	}
	writer.WriteHeader(http.StatusOK)
	buf := make([]byte, 409600)
	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			if !r.IsDownloading(domain, netdiskID, thumbnailType) {
				if n > 0 {
					writer.Write(buf[:n])
				}
				break
			}
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if n > 0 {
			writer.Write(buf[:n])
		}
	}
	log.Infof("download file from local finished domain: %s netdiskID: %s, thumbnailType: %s", domain, netdiskID, thumbnailType)

	return true
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
	resp := func() WaitListSorted {
		resp := WaitListSorted{}
		r.mutex.Lock()
		defer r.mutex.Unlock()
		for key, amt := range r.waiting {
			resp = append(resp, WaitElem{key, amt})
		}
		return resp
	}()
	sort.Sort(resp)
	return resp
}
