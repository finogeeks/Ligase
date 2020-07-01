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
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/storage/model"
	"sync"
	"time"
)

type ClientDataStreamRepo struct {
	persist model.SyncAPIDatabase
	repo    *TimeLineRepo
	ready   sync.Map
	loading sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewClientDataStreamRepo(
	bukSize,
	maxEntries,
	gcPerNum int,
) *ClientDataStreamRepo {
	tls := new(ClientDataStreamRepo)
	tls.repo = NewTimeLineRepo(bukSize, 500, true, maxEntries, gcPerNum)

	return tls
}

func (tl *ClientDataStreamRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *ClientDataStreamRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *ClientDataStreamRepo) AddClientDataStream(dataStream *types.ActDataStreamUpdate, offset int64) {
	tl.LoadHistory(dataStream.UserID, true)
	tl.addClientDataStream(dataStream, offset)
}

func (tl *ClientDataStreamRepo) addClientDataStream(dataStream *types.ActDataStreamUpdate, offset int64) {
	clientDataStream := new(feedstypes.ClientDataStream)
	clientDataStream.DataStream = dataStream
	clientDataStream.Offset = offset

	tl.repo.add(dataStream.UserID, clientDataStream)
}

func (tl *ClientDataStreamRepo) LoadHistory(userID string, sync bool) {
	if _, ok := tl.ready.Load(userID); !ok {
		if _, ok := tl.loading.Load(userID); !ok {
			tl.loading.Store(userID, true)
			if sync == false {
				go tl.loadHistory(userID)
			} else {
				tl.loadHistory(userID)
			}

			tl.queryHitCounter.WithLabelValues("db", "ClientDataStreamRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckLoadReady(userID, true)
		}
	} else {
		res := tl.repo.getTimeLine(userID)
		if res == nil {
			tl.ready.Delete(userID)
			tl.LoadHistory(userID, sync)
		} else {
			tl.queryHitCounter.WithLabelValues("cache", "ClientDataStreamRepo", "LoadHistory").Add(1)
		}
	}
}

func (tl *ClientDataStreamRepo) CheckLoadReady(userID string, sync bool) bool {
	_, ok := tl.ready.Load(userID)
	if ok || sync == false {
		if sync == false {
			tl.LoadHistory(userID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.ready.Load(userID); ok {
			break
		}
		tl.LoadHistory(userID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed ClientDataStreamRepo.CheckLoadReady user %s spend:%d s but still not ready, break", userID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(userID)
	return ok
}

func (tl *ClientDataStreamRepo) loadHistory(userID string) {
	defer tl.loading.Delete(userID)
	bs := time.Now().UnixNano() / 1000000
	streams, offsets, err := tl.persist.GetHistoryClientDataStream(context.TODO(), userID, 100)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed ClientDataStreamRepo load user:%s history spend:%d ms err: %v", userID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms ClientDataStreamRepo.loadHistory finished user:%s spend:%d ms", types.DB_EXCEED_TIME, userID, spend)
	} else {
		log.Infof("load db succ ClientDataStreamRepo.loadHistory finished user:%s spend:%d ms", userID, spend)
	}
	length := len(streams)
	empty := true
	for i := 0; i < length/2; i++ {
		stream := streams[i]
		streams[i] = streams[length-1-i]
		streams[length-1-i] = stream

		off := offsets[i]
		offsets[i] = offsets[length-1-i]
		offsets[length-1-i] = off
	}

	if streams != nil {
		for idx := range streams {
			empty = false
			tl.addClientDataStream(&streams[idx], offsets[idx])
		}
	}

	if empty {
		tl.repo.setDefault(userID)
	}

	tl.ready.Store(userID, true)
}

func (tl *ClientDataStreamRepo) GetHistory(user string) *feedstypes.TimeLines {
	tl.LoadHistory(user, true)
	return tl.repo.getTimeLine(user)
}

func (tl *ClientDataStreamRepo) ExistsAccountDataUpdate(position int64, userID string) bool {
	cdsTimeLine := tl.GetHistory(userID)

	if cdsTimeLine == nil {
		tl.repo.setDefault(userID)
		return false
	}
	_, feedUp := cdsTimeLine.GetFeedRange()
	if feedUp > position {
		return true
	}

	return false
}
