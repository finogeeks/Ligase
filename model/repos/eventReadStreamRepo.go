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

	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

type EventReadStreamRepo struct {
	persist           model.SyncAPIDatabase
	userReceiptOffset *sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewEventReadStreamRepo() *EventReadStreamRepo {
	tls := new(EventReadStreamRepo)
	tls.userReceiptOffset = new(sync.Map)

	return tls
}

func (tl *EventReadStreamRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *EventReadStreamRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *EventReadStreamRepo) AddUserReceiptOffset(userID, roomID string, receiptOffset int64) {
	key := fmt.Sprintf("%s:%s", userID, roomID)
	tl.userReceiptOffset.Store(key, receiptOffset)
}

func (tl *EventReadStreamRepo) GetUserLastOffset(ctx context.Context, userID, roomID string) int64 {
	key := fmt.Sprintf("%s:%s", userID, roomID)

	if lastOffset, ok := tl.userReceiptOffset.Load(key); ok {
		tl.queryHitCounter.WithLabelValues("cache", "EventReadStreamRepo", "GetUserLastOffset").Add(1)
		return lastOffset.(int64)
	} else {
		tl.queryHitCounter.WithLabelValues("db", "EventReadStreamRepo", "GetUserLastOffset").Add(1)

		offset, _, err := tl.persist.GetUserHistoryReceiptData(ctx, roomID, userID)
		if err != nil {
			log.Errorf("EventReadStreamRepo GetUserLastOffset userID %s roomID %s err: %v", userID, roomID, err)
		} else {
			tl.userReceiptOffset.Store(key, offset)
			return offset
		}
	}
	return int64(0)
}
