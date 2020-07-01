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
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

//存储state timeline, 没有房间timeline 负责加载数据
type RoomStateTimeLineRepo struct {
	repo          *TimeLineRepo
	streamRepo    *TimeLineRepo
	rsRepo        *RoomCurStateRepo
	persist       model.SyncAPIDatabase
	stateLoading  sync.Map
	stateReady    sync.Map
	streamLoading sync.Map
	streamReady   sync.Map

	QueryHitCounter mon.LabeledCounter
}

//room state time line 不限制大小
func NewRoomStateTimeLineRepo(
	bukSize int,
	rsRepo *RoomCurStateRepo,
	maxEntries,
	gcPerNum int,
) *RoomStateTimeLineRepo {
	tls := new(RoomStateTimeLineRepo)
	tls.repo = NewTimeLineRepo(bukSize, 128, false, maxEntries, gcPerNum)
	tls.streamRepo = NewTimeLineRepo(bukSize, 128, false, maxEntries, gcPerNum)
	tls.rsRepo = rsRepo

	return tls
}

func (tl *RoomStateTimeLineRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *RoomStateTimeLineRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.QueryHitCounter = queryHitCounter
}

func (tl *RoomStateTimeLineRepo) DelEv(roomID string, removedEventIDs []string) {
	states := tl.GetStates(roomID)

	if states != nil {
		feeds, _, _, _, _ := states.GetAllFeeds()
		for _, feed := range feeds {
			if feed != nil {
				stream := (feed).(*feedstypes.StreamEvent)
				if !stream.IsDeleted {
					ev := stream.GetEv()
					for _, eventID := range removedEventIDs {
						if ev.EventID == eventID {
							stream.IsDeleted = true
						}
					}
				}
			}
		}
	}
}

//kafka 消费数据，load设置为true
func (tl *RoomStateTimeLineRepo) AddEv(ev *gomatrixserverlib.ClientEvent, offset int64, load bool) {
	if load == true { //
		if ev.Type != "m.room.create" {
			tl.LoadStates(ev.RoomID, true)
		} else {
			// 如果syncwriter比这里syncserver快，timeline中feed了之后，又向db取数据，导致会部分事件重复
			tl.stateReady.Store(ev.RoomID, true)
		}
	}

	if common.IsStateClientEv(ev) {
		sev := new(feedstypes.StreamEvent)
		sev.Ev = ev
		sev.Offset = offset

		tl.repo.add(ev.RoomID, sev)

		if load {
			states := tl.repo.getTimeLine(ev.RoomID)
			if states != nil {
				feeds, _, _, _, _ := states.GetAllFeeds()
				for _, feed := range feeds {
					if feed != nil {
						stream := (feed).(*feedstypes.StreamEvent)
						if !stream.IsDeleted {
							if stream.Ev.Type == ev.Type && stream.Ev.EventID != ev.EventID {
								if ev.Type == "m.room.member" {
									if stream.Ev.Type == "m.room.member" {
										if *stream.Ev.StateKey == *ev.StateKey {
											stream.IsDeleted = true
										}
									}
								} else {
									stream.IsDeleted = true
								}
							}
						}
					}
				}
			}
		}

		//log.Errorf("RoomStateTimeLineRepo AddEv evid:%s offset:%d sev:%v type:%s", ev.EventID(), offset, sev, ev.Type())
	}
}

func (tl *RoomStateTimeLineRepo) AddStreamEv(ev *gomatrixserverlib.ClientEvent, offset int64, load bool) {
	if load == true && ev.Type != "m.room.create" { //
		tl.LoadStreamStates(ev.RoomID, true)
	}

	if common.IsStateClientEv(ev) {
		sev := new(feedstypes.StreamEvent)
		sev.Ev = ev
		sev.Offset = offset

		tl.rsRepo.onEvent(ev, offset, false)
		tl.streamRepo.add(ev.RoomID, sev)
	}
}

func (tl *RoomStateTimeLineRepo) AddBackfillEv(ev *gomatrixserverlib.ClientEvent, offset int64, load bool) {
	if load == true && ev.Type != "m.room.create" { //
		tl.LoadStreamStates(ev.RoomID, true)
	}
	if common.IsStateClientEv(ev) {
		tl.rsRepo.onEvent(ev, offset, true)
	}
}

func (tl *RoomStateTimeLineRepo) loadStates(roomID string) {
	defer tl.stateLoading.Delete(roomID)

	bs := time.Now().UnixNano() / 1000000
	evs, offsets, err := tl.persist.GetStateEventsForRoom(context.TODO(), roomID)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed RoomStateTimeLineRepo load room %s state spend:%d ms err: %v", roomID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomStateTimeLineRepo.loadStates finished %s spend:%d ms", types.DB_EXCEED_TIME, roomID, spend)
	} else {
		log.Infof("load db succ RoomStateTimeLineRepo.loadStates finished %s spend:%d ms", roomID, spend)
	}
	for idx := range evs {
		tl.AddEv(&evs[idx], offsets[idx], false)
	}

	tl.stateReady.Store(roomID, true)
}

func (tl *RoomStateTimeLineRepo) loadStateStreams(roomID string) {
	defer tl.streamLoading.Delete(roomID)

	bs := time.Now().UnixNano() / 1000000
	evs, offsets, err := tl.persist.GetStateEventsStreamForRoom(context.TODO(), roomID)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed RoomStateTimeLineRepo load room %s state stream spend:%d ms err: %v", roomID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomStateTimeLineRepo.loadStateStreams finished %s spend:%d ms", types.DB_EXCEED_TIME, roomID, spend)
	} else {
		log.Infof("load db succ RoomStateTimeLineRepo.loadStateStreams finished %s spend:%d ms", roomID, spend)
	}
	for idx := range evs {
		tl.AddStreamEv(&evs[idx], offsets[idx], false)
	}
	tl.streamReady.Store(roomID, true)
}

func (tl *RoomStateTimeLineRepo) LoadStates(roomID string, sync bool) {
	if _, ok := tl.stateReady.Load(roomID); !ok {
		if _, ok := tl.stateLoading.Load(roomID); !ok {
			tl.stateLoading.Store(roomID, true)
			if sync == false {
				go tl.loadStates(roomID)
			} else {
				tl.loadStates(roomID)
			}

			tl.QueryHitCounter.WithLabelValues("db", "RoomStateTimeLineRepo", "LoadStates").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckStateLoadReady(roomID, true)
		}
	} else {
		//加载完成又被内存置换，需要重新加载
		res := tl.repo.getTimeLine(roomID)
		if res == nil {
			tl.stateReady.Delete(roomID)
			tl.LoadStates(roomID, sync)
		} else {
			tl.QueryHitCounter.WithLabelValues("cache", "RoomStateTimeLineRepo", "LoadStates").Add(1)
		}
	}
}

func (tl *RoomStateTimeLineRepo) CheckStateLoadReady(roomID string, sync bool) bool {
	_, ok := tl.stateReady.Load(roomID)
	if ok || sync == false {
		if sync == false {
			tl.LoadStates(roomID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.stateReady.Load(roomID); ok {
			break
		}

		tl.LoadStates(roomID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed RoomHistoryTimeLineRepo.CheckStateLoadReady room %s spend:%d s but still not ready, break", roomID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.stateReady.Load(roomID)
	return ok
}

func (tl *RoomStateTimeLineRepo) GetStates(roomID string) *feedstypes.TimeLines {
	tl.LoadStates(roomID, true)
	return tl.repo.getTimeLine(roomID)
}

func (tl *RoomStateTimeLineRepo) LoadStreamStates(roomID string, sync bool) {
	if _, ok := tl.streamReady.Load(roomID); !ok {
		if _, ok := tl.streamLoading.Load(roomID); !ok {
			tl.streamLoading.Store(roomID, true)
			if sync == false {
				go tl.loadStateStreams(roomID)
			} else {
				tl.loadStateStreams(roomID)
			}

			tl.QueryHitCounter.WithLabelValues("db", "RoomStateTimeLineRepo", "LoadStreamStates").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckStreamLoadReady(roomID, true)
		}
	} else {
		//加载完成又被内存置换，需要重新加载
		res := tl.streamRepo.getTimeLine(roomID)
		if res == nil {
			tl.rsRepo.removeRoomState(roomID)
			tl.streamReady.Delete(roomID)
			tl.LoadStreamStates(roomID, sync)
		} else {
			tl.QueryHitCounter.WithLabelValues("cache", "RoomStateTimeLineRepo", "LoadStreamStates").Add(1)
		}
	}
}

func (tl *RoomStateTimeLineRepo) CheckStreamLoadReady(roomID string, sync bool) bool {
	_, ok := tl.streamReady.Load(roomID)
	if ok || sync == false {
		if sync == false {
			tl.LoadStreamStates(roomID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.streamReady.Load(roomID); ok {
			break
		}

		tl.LoadStreamStates(roomID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed RoomStateTimeLineRepo.CheckStreamLoadReady room %s spend:%d s but still not ready, break", roomID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.streamReady.Load(roomID)
	return ok
}

func (tl *RoomStateTimeLineRepo) GetStateStreams(roomID string) *feedstypes.TimeLines {
	tl.LoadStreamStates(roomID, true)
	return tl.streamRepo.getTimeLine(roomID)
}

func (tl *RoomStateTimeLineRepo) RemoveStateStreams(roomID string) {
	tl.streamRepo.remove(roomID)
	tl.rsRepo.removeRoomState(roomID)
	tl.streamReady.Delete(roomID)
}

func (tl *RoomStateTimeLineRepo) GetStateEvents(roomID string, endPos int64) ([]*feedstypes.StreamEvent, []*gomatrixserverlib.ClientEvent) {
	streamEvs := []*feedstypes.StreamEvent{}
	events := []*gomatrixserverlib.ClientEvent{}

	streams := tl.GetStateStreams(roomID)
	if streams == nil {
		return streamEvs, events
	}

	feeds, _, _, _, _ := streams.GetFeeds(-1, endPos)
	for _, feed := range feeds {
		if feed != nil {
			streamEv := (feed).(*feedstypes.StreamEvent)
			streamEvs = append(streamEvs, streamEv)
			events = append(events, streamEv.GetEv())

		}
	}

	if len(streamEvs) <= 0 {
		// get from db
		evs, offsets, err := tl.persist.GetStateEventsStreamForRoomBeforePos(context.TODO(), roomID, endPos)
		if err != nil {
			log.Errorf("RoomStateTimeLineRepo load room %s state err: %v", roomID, err)
			return streamEvs, events
		}
		for i, ev := range evs {
			streamEv := new(feedstypes.StreamEvent)
			streamEv.Ev = &ev
			streamEv.Offset = offsets[i]
			streamEvs = append(streamEvs, streamEv)
			events = append(events, streamEv.GetEv())
		}

		tl.QueryHitCounter.WithLabelValues("db", "RoomStateTimeLineRepo", "GetStateEvents").Add(1)
	} else {
		tl.QueryHitCounter.WithLabelValues("cache", "RoomStateTimeLineRepo", "GetStateEvents").Add(1)
	}

	return streamEvs, events
}
