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
	"github.com/finogeeks/ligase/model/service"
	"sync"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"

	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type RoomHistoryTimeLineRepo struct {
	repo                   *TimeLineRepo
	persist                model.SyncAPIDatabase
	cache  				   service.Cache
	loading                sync.Map
	ready                  sync.Map
	roomLatest             sync.Map //room latest offset
	roomMutex              *sync.Mutex
	roomMinStream          sync.Map
	domainMaxOffset        sync.Map
	loadingDomainMaxOffset sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewRoomHistoryTimeLineRepo(
	bukSize,
	maxEntries,
	gcPerNum int,
) *RoomHistoryTimeLineRepo {
	tls := new(RoomHistoryTimeLineRepo)
	tls.repo = NewTimeLineRepo(bukSize, 128, true, maxEntries, gcPerNum)
	tls.roomMutex = new(sync.Mutex)

	return tls
}

func (tl *RoomHistoryTimeLineRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *RoomHistoryTimeLineRepo) SetCache(cache service.Cache){
	tl.cache = cache
}

func (tl *RoomHistoryTimeLineRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *RoomHistoryTimeLineRepo) AddEv(ctx context.Context, ev *gomatrixserverlib.ClientEvent, offset int64, load bool) {
	if load == true && ev.Type != "m.room.create" {
		tl.LoadHistory(ctx, ev.RoomID, true)
	}

	sev := new(feedstypes.StreamEvent)
	sev.Ev = ev
	sev.Offset = offset
	tl.repo.add(ev.RoomID, sev)
	if sev.Ev.StateKey != nil {
		log.Infof("update roomHistoryTimeineRepo sender:%s,statekey:%s,room:%s,offset:%d", sev.Ev.Sender, *sev.Ev.StateKey, sev.Ev.RoomID, sev.Offset)
	} else {
		log.Infof("update roomHistoryTimeineRepo sender:%s,room:%s,offset:%d", sev.Ev.Sender, sev.Ev.RoomID, sev.Offset)
	}
	tl.setRoomLatest(ev.RoomID, offset)
}

func (tl *RoomHistoryTimeLineRepo) loadHistory(ctx context.Context, roomID string) {
	defer tl.loading.Delete(roomID)

	bs := time.Now().UnixNano() / 1000000
	evs, offsets, err := tl.persist.GetHistoryEvents(ctx, roomID, 30) //注意是倒序的event，需要排列下
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed RoomHistoryTimeLineRepo load room:%s history spend:%d ms err:%v", roomID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomHistoryTimeLineRepo.loadHistory finished room:%s spend:%d ms", types.DB_EXCEED_TIME, roomID, spend)
	} else {
		log.Infof("load db succ RoomHistoryTimeLineRepo.loadHistory finished room:%s spend:%d ms", roomID, spend)
	}
	length := len(evs)
	for i := 0; i < length/2; i++ {
		ev := evs[i]
		evs[i] = evs[length-1-i]
		evs[length-1-i] = ev

		off := offsets[i]
		offsets[i] = offsets[length-1-i]
		offsets[length-1-i] = off
	}

	for idx := range evs {
		tl.AddEv(ctx, &evs[idx], offsets[idx], false)
	}

	tl.ready.Store(roomID, true)

}

func (tl *RoomHistoryTimeLineRepo) LoadHistory(ctx context.Context, roomID string, sync bool) {
	if _, ok := tl.ready.Load(roomID); !ok {
		if _, ok := tl.loading.Load(roomID); !ok {
			tl.loading.Store(roomID, true)
			if sync == false {
				go tl.loadHistory(ctx, roomID)
			} else {
				tl.loadHistory(ctx, roomID)
			}

			tl.queryHitCounter.WithLabelValues("db", "RoomHistoryTimeLineRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckLoadReady(ctx, roomID, true)
		}
	} else {
		res := tl.repo.getTimeLine(roomID)
		if res == nil {
			tl.ready.Delete(roomID)
			tl.LoadHistory(ctx, roomID, sync)
		} else {
			tl.queryHitCounter.WithLabelValues("db", "RoomHistoryTimeLineRepo", "LoadHistory").Add(1)
		}
	}
}

func (tl *RoomHistoryTimeLineRepo) CheckLoadReady(ctx context.Context, roomID string, sync bool) bool {
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
			log.Errorf("checkloadready failed RoomHistoryTimeLineRepo.CheckLoadReady room %s spend:%d s but still not ready, break", roomID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(roomID)
	return ok
}

func (tl *RoomHistoryTimeLineRepo) GetHistory(ctx context.Context, roomID string) *feedstypes.TimeLines {
	tl.LoadHistory(ctx, roomID, true)
	return tl.repo.getTimeLine(roomID)
}

func (tl *RoomHistoryTimeLineRepo) GetStreamEv(ctx context.Context, roomID, eventId string) *feedstypes.StreamEvent {
	history := tl.GetHistory(ctx, roomID)
	if history == nil {
		return nil
	}

	var sev *feedstypes.StreamEvent
	history.ForRangeReverse(func(offset int, feed feedstypes.Feed) bool {
		if feed != nil {
			stream := feed.(*feedstypes.StreamEvent)
			if stream.GetEv().EventID == eventId {
				sev = stream
				return false
			}
			return true
		}
		return true
	})

	return sev
}

func (tl *RoomHistoryTimeLineRepo) GetLastEvent(ctx context.Context, roomID string) *feedstypes.StreamEvent {
	history := tl.GetHistory(ctx, roomID)
	if history == nil {
		return nil
	}

	var sev *feedstypes.StreamEvent
	history.ForRangeReverse(func(offset int, feed feedstypes.Feed) bool {
		if feed != nil {
			sev = feed.(*feedstypes.StreamEvent)
			return false
		}
		return true
	})
	return sev
}

func (tl *RoomHistoryTimeLineRepo) GetRoomLastOffset(roomID string) int64 {
	if val, ok := tl.roomLatest.Load(roomID); ok {
		return val.(int64)
	}

	return int64(-1)
}

func (tl *RoomHistoryTimeLineRepo) GetRoomMinStream(ctx context.Context, roomID string) int64 {
	if val, ok := tl.roomMinStream.Load(roomID); ok {
		tl.queryHitCounter.WithLabelValues("cache", "RoomHistoryTimeLineRepo", "GetRoomMinStream").Add(1)
		return val.(int64)
	}
	bs := time.Now().UnixNano() / 1000000
	pos, err := tl.persist.SelectOutputMinStream(ctx, roomID)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed RoomHistoryTimeLineRepo.SelectOutputMinStream roomID:%s spend:%d ms err %v", roomID, spend, err)
		return -1
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomHistoryTimeLineRepo.SelectOutputMinStream roomID:%s spend:%d ms", types.DB_EXCEED_TIME, roomID, spend)
	} else {
		log.Infof("load db succ RoomHistoryTimeLineRepo.SelectOutputMinStream roomID:%s spend:%d ms", roomID, spend)
	}
	val, _ := tl.roomMinStream.LoadOrStore(roomID, pos)
	pos = val.(int64)

	tl.queryHitCounter.WithLabelValues("db", "RoomHistoryTimeLineRepo", "GetRoomMinStream").Add(1)

	return pos
}

func (tl *RoomHistoryTimeLineRepo) SetRoomMinStream(roomID string, minStream int64) {
	tl.roomMinStream.Store(roomID, minStream)
}

func (tl *RoomHistoryTimeLineRepo) GetDomainMaxStream(ctx context.Context, roomID, domain string) int64 {
	for {
		if val, ok := tl.domainMaxOffset.Load(roomID); !ok {
			if _, loaded := tl.loadingDomainMaxOffset.LoadOrStore(roomID, true); loaded {
				time.Sleep(time.Millisecond * 3)
				continue
			} else {
				defer tl.loadingDomainMaxOffset.Delete(roomID)
				domains, offsets, err := tl.persist.SelectDomainMaxOffset(ctx, roomID)
				if err != nil {
					log.Errorf("RoomHistoryTimeLineRepo GetDomainMaxStream roomID %s err %v", roomID, err)
					return -1
				}
				domainMap := new(sync.Map)
				for index, domain := range domains {
					domainMap.Store(domain, offsets[index])
				}
				tl.domainMaxOffset.Store(roomID, domainMap)
				if val, ok := domainMap.Load(domain); ok {
					tl.queryHitCounter.WithLabelValues("db", "RoomHistoryTimeLineRepo", "GetDomainMaxStream").Add(1)

					return val.(int64)
				}
				return -1
			}
		} else {
			if val, ok := val.(*sync.Map).Load(domain); ok {
				tl.queryHitCounter.WithLabelValues("cache", "RoomHistoryTimeLineRepo", "GetDomainMaxStream").Add(1)

				return val.(int64)
			}
			return -1
		}
	}
}

func (tl *RoomHistoryTimeLineRepo) SetDomainMaxStream(roomID, domain string, offset int64) {
	val, ok := tl.domainMaxOffset.Load(roomID)
	if !ok {
		val, _ = tl.domainMaxOffset.LoadOrStore(roomID, new(sync.Map))
	}
	val.(*sync.Map).Store(domain, offset)
}

func (tl *RoomHistoryTimeLineRepo) setRoomLatest(roomID string, offset int64) {
	tl.roomMutex.Lock()
	defer tl.roomMutex.Unlock()

	val, ok := tl.roomLatest.Load(roomID)
	if ok {
		lastoffset := val.(int64)
		if offset > lastoffset {
			log.Infof("update roomId:%s lastoffset:%d,offset:%d", roomID, lastoffset, offset)
			tl.roomLatest.Store(roomID, offset)
			/*err := tl.cache.SetRoomLatestOffset(roomID, offset)
			if err != nil {
				log.Errorf("set roomID:%s offset:%d lastoffset:%d err:%v", roomID, offset, lastoffset,err)
			}*/
		}
	} else {
		log.Infof("update roomId:%s first offset:%d ", roomID, offset)
		tl.roomLatest.Store(roomID, offset)
		/*err := tl.cache.SetRoomLatestOffset(roomID, offset)
		if err != nil {
			log.Errorf("set roomID:%s first offset:%d err:%v", roomID, offset, err)
		}*/
	}
}

func (tl *RoomHistoryTimeLineRepo) LoadRoomLatest(ctx context.Context, rooms []syncapitypes.SyncRoom) error {
	var loadRooms []string
	for _, room := range rooms {
		_, ok := tl.roomLatest.Load(room.RoomID)
		if !ok {
			loadRooms = append(loadRooms, room.RoomID)
		}
	}

	if len(loadRooms) > 0 {
		bs := time.Now().UnixNano() / 1000000
		roomMap, err := tl.persist.GetRoomLastOffsets(ctx, loadRooms)
		spend := time.Now().UnixNano()/1000000 - bs
		if err != nil {
			log.Errorf("load db failed RoomHistoryTimeLineRepo.LoadRoomLatest spend:%d ms err:%v", spend, err)
			return err
		}
		if spend > types.DB_EXCEED_TIME {
			log.Warnf("load db exceed %d ms RoomHistoryTimeLineRepo.LoadRoomLatest spend:%d ms", types.DB_EXCEED_TIME, spend)
		} else {
			log.Infof("load db succ RoomHistoryTimeLineRepo.LoadRoomLatest spend:%d ms", spend)
		}
		if roomMap != nil {
			for roomID, offset := range roomMap {
				tl.setRoomLatest(roomID, offset)
			}
		}

		tl.queryHitCounter.WithLabelValues("db", "RoomHistoryTimeLineRepo", "LoadRoomLatest").Add(1)
	} else {
		tl.queryHitCounter.WithLabelValues("cache", "RoomHistoryTimeLineRepo", "LoadRoomLatest").Add(1)
	}
	return nil
}
