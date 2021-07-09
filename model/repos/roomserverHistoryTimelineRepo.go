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
	"math"
	"sync"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"

	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type Events struct {
	size    int
	lru     *Lru
	persist model.RoomServerDatabase
	events  sync.Map
}

func NewEvents(size, gcPerNum int, persist model.RoomServerDatabase) *Events {
	es := new(Events)
	es.size = size
	es.lru = NewLru(size, gcPerNum)
	es.persist = persist
	return es
}

func (es *Events) addEvent(event *gomatrixserverlib.Event) {
	eventID := event.EventID()
	if _, ok := es.events.Load(eventID); ok {
		return
	}

	toRemove := es.lru.Add(eventID)
	if toRemove != nil {
		if _, ok := es.events.Load(toRemove); ok {
			es.events.Delete(toRemove)
		}
	}

	es.events.Store(eventID, event)
}

func (es Events) getEventsByID(eventIDs []string) ([]*gomatrixserverlib.Event, error) {
	events := make([]*gomatrixserverlib.Event, len(eventIDs))
	notFound := false
	for i, v := range eventIDs {
		if val, ok := es.events.Load(v); ok {
			events[i] = val.(*gomatrixserverlib.Event)
		} else {
			notFound = true
			break
		}
	}

	if !notFound {
		return events, nil
	}

	eventNIDMap, err := es.persist.EventNIDs(context.TODO(), eventIDs)
	if err != nil {
		log.Errorf("Failed to get eventNID by eventID: %v\n", err)
		return nil, err
	}

	var eventNIDs []int64
	for _, nid := range eventNIDMap {
		eventNIDs = append(eventNIDs, nid)
	}

	evs, _, err := es.persist.Events(context.TODO(), eventNIDs)
	if err != nil {
		log.Errorf("Failed to get event by eventID: %v\n", err)
		return nil, err
	}

	for _, v := range evs {
		es.addEvent(v)
	}

	return evs, nil
}

type RoomServerHistoryTimeLineRepo struct {
	repo            *TimeLineRepo
	persist         model.RoomServerDatabase
	cache           service.Cache
	loading         sync.Map
	ready           sync.Map
	roomMutex       *sync.Mutex
	roomIDToNID     sync.Map
	roomNIDToID     sync.Map
	events          *Events
	queryHitCounter mon.LabeledCounter
}

func NewRoomServerHistoryTimeLineRepo(
	bukSize,
	maxEntries,
	gcPerNum int,
	persist model.RoomServerDatabase,
) *RoomServerHistoryTimeLineRepo {
	tls := new(RoomServerHistoryTimeLineRepo)
	tls.repo = NewTimeLineRepo(bukSize, 128, true, maxEntries, gcPerNum)
	tls.roomMutex = new(sync.Mutex)
	tls.persist = persist
	tls.events = NewEvents(128*maxEntries, gcPerNum, persist)

	return tls
}

func (tl *RoomServerHistoryTimeLineRepo) GetLoadedRoomNumber() (int, int) {
	return tl.repo.GetKeyNumbers()
}

func (tl *RoomServerHistoryTimeLineRepo) SetCache(cache service.Cache) {
	tl.cache = cache
}

func (tl *RoomServerHistoryTimeLineRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *RoomServerHistoryTimeLineRepo) GetEventsByID(eventIDs []string) ([]*gomatrixserverlib.Event, error) {
	return tl.events.getEventsByID(eventIDs)
}

func (tl *RoomServerHistoryTimeLineRepo) GetRoomIDByRoomNID(roomNID int64) string {
	val, ok := tl.roomNIDToID.Load(roomNID)
	if ok {
		return val.(string)
	}

	tl.loadRooms()

	val, ok = tl.roomNIDToID.Load(roomNID)
	if ok {
		return val.(string)
	}

	return ""
}

func (tl *RoomServerHistoryTimeLineRepo) GetRoomNIDByRoomID(roomID string) int64 {
	val, ok := tl.roomIDToNID.Load(roomID)
	if ok {
		return val.(int64)
	}

	tl.loadRooms()

	val, ok = tl.roomIDToNID.Load(roomID)
	if ok {
		return val.(int64)
	}

	return 0
}

func (tl *RoomServerHistoryTimeLineRepo) loadRooms() {
	tl.roomMutex.Lock()
	defer tl.roomMutex.Unlock()

	rooms, err := tl.persist.GetAllRooms(context.TODO(), math.MaxInt64, 0)
	if err != nil {
		log.Errorf("Failed to get all rooms: %v\n", err)
		return
	}

	for _, v := range rooms {
		tl.roomNIDToID.Store(v.RoomNID, v.RoomID)
		tl.roomIDToNID.Store(v.RoomID, v.RoomNID)
	}
}

func (tl *RoomServerHistoryTimeLineRepo) AddRoom(roomID string) (int64, error) {
	roomNID, err := tl.persist.AssignRoomNID(context.TODO(), roomID)
	if err != nil {
		return 0, err
	}

	tl.roomNIDToID.Store(roomNID, roomID)
	tl.roomIDToNID.Store(roomID, roomNID)

	return roomNID, nil
}

func (tl *RoomServerHistoryTimeLineRepo) AddEv(ev *gomatrixserverlib.Event, roomNID, curSnap int64, refId string, refHash []byte) {
	tl.addEv(ev, true)
	tl.persist.StoreEvent(context.TODO(), ev, roomNID, curSnap, refId, refHash)
}

func (tl *RoomServerHistoryTimeLineRepo) addEv(ev *gomatrixserverlib.Event, load bool) {
	if load == true && ev.Type() != "m.room.create" {
		tl.LoadHistory(ev.RoomID(), true)
	}

	e := new(feedstypes.RoomserverEvent)
	e.Ev = ev
	tl.repo.add(ev.RoomID(), e)
	tl.events.addEvent(ev)
}

func (tl *RoomServerHistoryTimeLineRepo) loadHistory(roomID string) {
	defer tl.loading.Delete(roomID)

	roomNID := tl.GetRoomNIDByRoomID(roomID)
	bs := time.Now().UnixNano() / 1000000
	evs, _, err := tl.persist.GetHistoryEvents(context.TODO(), roomNID, 128) //注意是倒序的event，需要排列下
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed RoomServerHistoryTimeLineRepo load room:%s history spend:%d ms err:%v", roomID, spend, err)
		return
	}

	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomServerHistoryTimeLineRepo.loadHistory finished room:%s spend:%d ms", types.DB_EXCEED_TIME, roomID, spend)
	} else {
		log.Infof("load db succ RoomServerHistoryTimeLineRepo.loadHistory finished room:%s spend:%d ms", roomID, spend)
	}

	for i := len(evs); i > 0; i-- {
		tl.addEv(evs[i], false)
	}

	tl.ready.Store(roomID, true)

}

func (tl *RoomServerHistoryTimeLineRepo) LoadHistory(roomID string, sync bool) {
	if _, ok := tl.ready.Load(roomID); !ok {
		if _, ok := tl.loading.Load(roomID); !ok {
			tl.loading.Store(roomID, true)
			if sync == false {
				go tl.loadHistory(roomID)
			} else {
				tl.loadHistory(roomID)
			}

			tl.queryHitCounter.WithLabelValues("db", "RoomServerHistoryTimeLineRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckLoadReady(roomID, true)
		}
	} else {
		res := tl.repo.getTimeLine(roomID)
		if res == nil {
			tl.ready.Delete(roomID)
			tl.LoadHistory(roomID, sync)
		} else {
			tl.queryHitCounter.WithLabelValues("db", "RoomServerHistoryTimeLineRepo", "LoadHistory").Add(1)
		}
	}
}

func (tl *RoomServerHistoryTimeLineRepo) CheckLoadReady(roomID string, sync bool) bool {
	_, ok := tl.ready.Load(roomID)
	if ok || sync == false {
		if sync == false {
			tl.LoadHistory(roomID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.ready.Load(roomID); ok {
			break
		}

		tl.LoadHistory(roomID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed RoomServerHistoryTimeLineRepo.CheckLoadReady room %s spend:%d s but still not ready, break", roomID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(roomID)
	return ok
}

func (tl *RoomServerHistoryTimeLineRepo) GetHistory(roomID string) *feedstypes.TimeLines {
	tl.LoadHistory(roomID, true)
	return tl.repo.getTimeLine(roomID)
}
