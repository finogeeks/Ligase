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
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"

	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

type ReceiptStreamContent struct {
	CurrentOffset int64
	Content       *feedstypes.TimeLines
}

type ReceiptDataStreamRepo struct {
	roomCurState  *RoomCurStateRepo
	rsTimeline    *RoomStateTimeLineRepo
	persist       model.SyncAPIDatabase
	container     *sync.Map
	loading       sync.Map
	ready         sync.Map //ready for loading
	userMaxStream *sync.Map
	roomLatest    *sync.Map
	delay         int
	tlSize        int
	updatedRoom   *sync.Map
	roomMutex     *sync.Mutex
	userMutex     *sync.Mutex
	flushDB       bool

	queryHitCounter mon.LabeledCounter
}

func NewReceiptDataStreamRepo(
	delay int,
	tlSize int,
	flushDB bool,
) *ReceiptDataStreamRepo {
	tls := new(ReceiptDataStreamRepo)
	tls.container = new(sync.Map)
	tls.userMaxStream = new(sync.Map)
	tls.roomLatest = new(sync.Map)
	tls.updatedRoom = new(sync.Map)
	tls.delay = delay
	tls.tlSize = tlSize
	tls.roomMutex = new(sync.Mutex)
	tls.userMutex = new(sync.Mutex)
	tls.flushDB = flushDB

	if flushDB {
		tls.startFlush()
	}
	return tls
}

func (tl *ReceiptDataStreamRepo) startFlush() error {
	go func() {
		t := time.NewTimer(time.Millisecond * time.Duration(tl.delay))
		for {
			select {
			case <-t.C:
				func() {
					span, ctx := common.StartSobSomSpan(context.Background(), "ReceiptDataStreamRepo.startFlush")
					defer span.Finish()
					tl.flush(ctx)
				}()
				t.Reset(time.Millisecond * time.Duration(tl.delay))
			}
		}
	}()

	return nil
}

func (tl *ReceiptDataStreamRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *ReceiptDataStreamRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *ReceiptDataStreamRepo) SetRsCurState(rsCurState *RoomCurStateRepo) {
	tl.roomCurState = rsCurState
}

func (tl *ReceiptDataStreamRepo) SetRsTimeline(rsTimeline *RoomStateTimeLineRepo){
	tl.rsTimeline = rsTimeline
}

func (tl *ReceiptDataStreamRepo) AddReceiptDataStream(ctx context.Context, dataStream *types.ReceiptStream, offset int64) {
	roomID := dataStream.RoomID
	tl.LoadHistory(ctx, roomID, true)

	tl.addReceiptDataStream(dataStream, offset, false)

	return
}

func (tl *ReceiptDataStreamRepo) addReceiptDataStream(dataStream *types.ReceiptStream, offset int64, loaded bool) {
	roomID := dataStream.RoomID
	eventOffset := dataStream.ReceiptOffset

	receiptDataStream := new(feedstypes.ReceiptDataStream)
	receiptDataStream.DataStream = dataStream
	receiptDataStream.Offset = offset
	receiptDataStream.Written = loaded

	var receipt *ReceiptStreamContent
	if item, ok := tl.container.Load(roomID); !ok {
		receipt = new(ReceiptStreamContent)
		receipt.CurrentOffset = eventOffset
		receipt.Content = feedstypes.NewEvTimeLines(tl.tlSize, false)
		val, loaded := tl.container.LoadOrStore(roomID, receipt)
		if loaded {
			receipt = val.(*ReceiptStreamContent)
		}
	} else {
		receipt = item.(*ReceiptStreamContent)
	}
	if receipt.CurrentOffset != eventOffset {
		receipt.CurrentOffset = eventOffset
		receipt.Content = feedstypes.NewEvTimeLines(tl.tlSize, false)
	}
	receipt.Content.Add(receiptDataStream)
	tl.setJoinUsersLatest(roomID, offset)
	if loaded == false {
		tl.updatedRoom.Store(roomID, true)
	}

	tl.setRoomLatest(roomID, offset)
}

func (tl *ReceiptDataStreamRepo) setJoinUsersLatest(roomID string, offset int64){
	rs := tl.roomCurState.GetRoomState(roomID)
	if rs != nil {
		joined := rs.GetJoinMap()
		joined.Range(func(key, _ interface{}) bool {
			tl.setUserLatest(key.(string), offset)
			return true
		})
	}else{
		log.Infof("load roomID:%s stream state sync to init user latest receipt", roomID)
		tl.rsTimeline.LoadStreamStates(context.TODO(), roomID, true)
		rs = tl.roomCurState.GetRoomState(roomID)
		if rs == nil {
			log.Errorf("after sync LoadStreamStates roomID:%s rs is still nil", roomID)
		}else{
			joined := rs.GetJoinMap()
			joined.Range(func(key, _ interface{}) bool {
				tl.setUserLatest(key.(string), offset)
				return true
			})
		}
	}
}

func (tl *ReceiptDataStreamRepo) LoadHistory(ctx context.Context, roomID string, sync bool) {
	if _, ok := tl.ready.Load(roomID); !ok {
		if _, ok := tl.loading.Load(roomID); !ok {
			tl.loading.Store(roomID, true)
			if sync == false {
				go tl.loadHistory(ctx, roomID)
			} else {
				tl.loadHistory(ctx, roomID)
			}

			tl.queryHitCounter.WithLabelValues("db", "ReceiptDataStreamRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			} else {
				tl.CheckLoadReady(ctx, roomID, true)
			}
		}
	} else {
		tl.queryHitCounter.WithLabelValues("cache", "ReceiptDataStreamRepo", "LoadHistory").Add(1)
	}
}

func (tl *ReceiptDataStreamRepo) LoadRoomLatest(ctx context.Context, rooms []string) {
	var loadRooms []string
	for _, roomID := range rooms {
		_, ok := tl.roomLatest.Load(roomID)
		if !ok {
			loadRooms = append(loadRooms, roomID)
		}
	}

	if len(loadRooms) > 0 {
		bs := time.Now().UnixNano() / 1000000
		roomMap, err := tl.persist.GetRoomReceiptLastOffsets(ctx, loadRooms)
		spend := time.Now().UnixNano()/1000000 - bs
		rooms := strings.Join(loadRooms, ",")
		if err != nil {
			log.Errorf("load db failed ReceiptDataStreamRepo.LoadRoomLatest loadrooms:%s spend:%d ms  err:%v", rooms, spend, err)
		} else {
			if spend > types.DB_EXCEED_TIME {
				log.Warnf("load db exceed %d ms ReceiptDataStreamRepo.LoadRoomLatest loadrooms:%s spend:%d ms", types.DB_EXCEED_TIME, rooms, spend)
			} else {
				log.Infof("load db succ ReceiptDataStreamRepo.LoadRoomLatest  loadrooms:%s spend:%d ms", rooms, spend)
			}
		}
		if roomMap != nil {
			for roomID, offset := range roomMap {
				log.Infof("store receipt data stream from db roomId:%s offset:%d", roomID, offset)
				tl.setRoomLatest(roomID, offset)
			}
		}
		for _, roomID := range loadRooms {
			if _, ok := roomMap[roomID]; !ok {
				log.Infof("store receipt data stream default roomId:%s offset:%d", roomID, 0)
				tl.setRoomLatest(roomID, 0)
			}
		}

		tl.queryHitCounter.WithLabelValues("db", "ReceiptDataStreamRepo", "LoadRoomLatest").Add(1)
	} else {
		tl.queryHitCounter.WithLabelValues("cache", "ReceiptDataStreamRepo", "LoadRoomLatest").Add(1)
	}
}

func (tl *ReceiptDataStreamRepo) CheckLoadReady(ctx context.Context, roomID string, sync bool) bool {
	_, ok := tl.ready.Load(roomID)
	if ok || sync == false {
		if sync == false {
			tl.LoadHistory(ctx, roomID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.ready.Load(roomID); ok {
			break
		}

		tl.LoadHistory(ctx, roomID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed ReceiptDataStreamRepo.CheckLoadReady room %s spend:%d s but still not ready, break", roomID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(roomID)
	return ok
}

func (tl *ReceiptDataStreamRepo) loadHistory(ctx context.Context, roomID string) {
	defer tl.loading.Delete(roomID)

	bs := time.Now().UnixNano() / 1000000
	streams, offsets, err := tl.persist.GetHistoryReceiptDataStream(ctx, roomID)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed ReceiptDataStreamRepo load history %s spend:%d ms err: %v", roomID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms ReceiptDataStreamRepo.loadHistory finished %s spend:%d ms", types.DB_EXCEED_TIME, roomID, spend)
	} else {
		log.Infof("load db succ ReceiptDataStreamRepo.loadHistory finished %s spend:%d ms", roomID, spend)
	}

	if streams != nil {
		for idx := range streams {
			tl.addReceiptDataStream(&streams[idx], offsets[idx], true)
		}
	}

	tl.ready.Store(roomID, true)
}

func (tl *ReceiptDataStreamRepo) GetHistory(ctx context.Context, roomID string) *feedstypes.TimeLines {
	if _, ok := tl.ready.Load(roomID); !ok {
		tl.LoadHistory(ctx, roomID, true)
	}

	if item, ok := tl.container.Load(roomID); ok {
		receipt := item.(*ReceiptStreamContent)

		// tl.queryHitCounter.WithLabelValues("cache", "ReceiptDataStreamRepo", "GetHistory").Add(1)
		return receipt.Content
	}

	return nil
}

func (tl *ReceiptDataStreamRepo) GetRoomLastOffset(roomID string) int64 {
	if lastOffset, ok := tl.roomLatest.Load(roomID); ok {
		return lastOffset.(int64)
	}
	return int64(0)
}

func (tl *ReceiptDataStreamRepo) setRoomLatest(roomID string, offset int64) {
	tl.roomMutex.Lock()
	defer tl.roomMutex.Unlock()

	val, ok := tl.roomLatest.Load(roomID)
	if ok {
		if offset > val.(int64) {
			tl.roomLatest.Store(roomID, offset)
		}
	} else {
		tl.roomLatest.Store(roomID, offset)
	}
}

func (tl *ReceiptDataStreamRepo) setUserLatest(userID string, offset int64) {
	tl.userMutex.Lock()
	defer tl.userMutex.Unlock()

	val, ok := tl.userMaxStream.Load(userID)
	if ok {
		if offset > val.(int64) {
			tl.userMaxStream.Store(userID, offset)
		}
	} else {
		tl.userMaxStream.Store(userID, offset)
	}
}

func (tl *ReceiptDataStreamRepo) ExistsReceipt(position int64, userID string) bool {
	if maxStream, ok := tl.userMaxStream.Load(userID); ok {
		return maxStream.(int64) > position
	} else {
		tl.setUserLatest(userID, position)
		return true
	}

	return false
}

func (tl *ReceiptDataStreamRepo) GetUserLatestOffset(userID string) int64 {
	if maxStream, ok := tl.userMaxStream.Load(userID); ok {
		return maxStream.(int64)
	}

	return 0
}

func (tl *ReceiptDataStreamRepo) flush(ctx context.Context) {
	log.Infof("ReceiptDataStreamRepo start flush")
	tl.updatedRoom.Range(func(key, _ interface{}) bool {
		tl.updatedRoom.Delete(key)

		if item, ok := tl.container.Load(key); ok {
			receipt := item.(*ReceiptStreamContent)

			if receipt.Content != nil {
				feeds, _, _, _, _ := receipt.Content.GetAllFeeds()
				for _, feed := range feeds {
					if feed != nil {
						stream := feed.(*feedstypes.ReceiptDataStream)
						if !stream.Written {
							err := tl.persist.UpsertReceiptDataStream(
								ctx, stream.Offset, stream.DataStream.ReceiptOffset, stream.DataStream.RoomID, string(stream.DataStream.Content),
							)
							if err != nil {
								log.Errorw("ReceiptDataStreamRepo flushToDB could not save receipt stream data", log.KeysAndValues{
									"roomID", key, "error", err,
								})
							} else {
								stream.Written = true
							}
						}
					}
				}
			}
		}

		return true
	})
	log.Infof("ReceiptDataStreamRepo finished flush")
}
