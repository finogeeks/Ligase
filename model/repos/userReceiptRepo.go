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
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

type UserReceiptRepo struct {
	persist model.SyncAPIDatabase
	receipt *sync.Map
	updated *sync.Map
	delay   int
	ready   sync.Map
	loading sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewUserReceiptRepo(
	delay int,
) *UserReceiptRepo {
	tls := new(UserReceiptRepo)
	tls.receipt = new(sync.Map)
	tls.updated = new(sync.Map)
	tls.delay = delay

	tls.startFlush()
	return tls
}

func (tl *UserReceiptRepo) startFlush() error {
	go func() {
		t := time.NewTimer(time.Millisecond * time.Duration(tl.delay))
		for {
			select {
			case <-t.C:
				func() {
					span, ctx := common.StartSobSomSpan(context.Background(), "UserReceiptRepo.startFlush")
					defer span.Finish()
					tl.flush(ctx)
				}()
				t.Reset(time.Millisecond * time.Duration(tl.delay))
			}
		}
	}()

	return nil
}

func (tl *UserReceiptRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *UserReceiptRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *UserReceiptRepo) AddUserReceipt(receipt *types.UserReceipt) {
	key := fmt.Sprintf("%s:%s", receipt.RoomID, receipt.UserID)
	tl.receipt.Store(key, receipt)
	tl.updated.Store(key, true)
}

func (tl *UserReceiptRepo) LoadHistory(ctx context.Context, userID, roomID string, sync bool) {
	key := fmt.Sprintf("%s:%s", roomID, userID)
	if _, ok := tl.ready.Load(key); !ok {
		if _, ok := tl.loading.Load(key); !ok {
			tl.loading.Store(key, true)
			if sync == false {
				go tl.loadHistory(ctx, userID, roomID)
			} else {
				tl.loadHistory(ctx, userID, roomID)
			}

			tl.queryHitCounter.WithLabelValues("db", "UserReceiptRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			} else {
				tl.CheckLoadReady(ctx, userID, roomID, true)
			}
		}
	} else {
		tl.queryHitCounter.WithLabelValues("cache", "UserReceiptRepo", "LoadHistory").Add(1)
	}
}

func (tl *UserReceiptRepo) CheckLoadReady(ctx context.Context, userID, roomID string, sync bool) bool {
	key := fmt.Sprintf("%s:%s", roomID, userID)

	_, ok := tl.ready.Load(key)
	if ok || sync == false {
		if sync == false {
			tl.LoadHistory(ctx, userID, roomID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.ready.Load(key); ok {
			break
		}

		tl.LoadHistory(ctx, userID, roomID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed UserReceiptRepo.CheckLoadReady user %s room %s spend:%d s but still not ready, break", userID, roomID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(key)
	return ok
}

func (tl *UserReceiptRepo) loadHistory(ctx context.Context, userID, roomID string) {
	key := fmt.Sprintf("%s:%s", roomID, userID)
	defer tl.loading.Delete(key)

	bs := time.Now().UnixNano() / 1000000
	evtOffset, content, err := tl.persist.GetUserHistoryReceiptData(ctx, roomID, userID)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed UserReceiptRepo load history roomID:%s user:%s spend:%d ms err: %v", roomID, userID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms UserReceiptRepo.loadHistory finished room:%s user:%s spend:%d ms", types.DB_EXCEED_TIME, roomID, userID, spend)
	} else {
		log.Infof("load db succ UserReceiptRepo.loadHistory finished room:%s user:%s spend:%d ms", roomID, userID, spend)
	}
	var receipt types.UserReceipt
	receipt.RoomID = roomID
	receipt.UserID = userID
	receipt.EvtOffset = evtOffset
	receipt.Content = content
	receipt.Written = true

	tl.receipt.Store(key, &receipt)

	tl.ready.Store(key, true)

}

func (tl *UserReceiptRepo) GetLatestOffset(ctx context.Context, userID, roomID string) int64 {
	key := fmt.Sprintf("%s:%s", roomID, userID)

	if _, ok := tl.ready.Load(key); !ok {
		tl.LoadHistory(ctx, userID, roomID, true)
	}

	if val, ok := tl.receipt.Load(key); ok {
		tl.queryHitCounter.WithLabelValues("cache", "UserReceiptRepo", "GetLatestOffset").Add(1)

		return val.(*types.UserReceipt).EvtOffset
	}

	return 0
}

func (tl *UserReceiptRepo) GetLatestReceipt(ctx context.Context, userID, roomID string) []byte {
	key := fmt.Sprintf("%s:%s", roomID, userID)

	if _, ok := tl.ready.Load(key); !ok {
		tl.LoadHistory(ctx, userID, roomID, true)
	}

	if val, ok := tl.receipt.Load(key); ok {
		tl.queryHitCounter.WithLabelValues("cache", "UserReceiptRepo", "GetLatestReceipt").Add(1)

		return val.(*types.UserReceipt).Content
	}

	return nil
}

func (tl *UserReceiptRepo) flush(ctx context.Context) {
	log.Infof("UserReceiptRepo start flush")
	tl.updated.Range(func(key, _ interface{}) bool {
		tl.updated.Delete(key)

		if item, ok := tl.receipt.Load(key); ok {
			receipt := item.(*types.UserReceipt)
			if !receipt.Written {
				receipt.Written = true
				tl.flushToDB(ctx, receipt.RoomID, receipt.UserID, receipt.Content, receipt.EvtOffset)
			}
		}

		return true
	})
	log.Infof("UserReceiptRepo finished flush")
}

func (tl *UserReceiptRepo) flushToDB(ctx context.Context, roomID, userID string, content []byte, offset int64) {
	err := tl.persist.UpsertUserReceiptData(ctx, roomID, userID, string(content), offset)
	if err != nil {
		log.Errorw("UserReceiptRepo flushToDB could not save user receipt data", log.KeysAndValues{
			"roomID", roomID, "userID", userID, "error", err,
		})
	}
}
