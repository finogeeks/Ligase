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
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/storage/model"
)

type STDEventStreamRepo struct {
	persist    model.SyncAPIDatabase
	repo       *TimeLineRepo
	ready      sync.Map
	loading    sync.Map
	idg        *uid.UidGenerator
	delay      int
	updatedKey *sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewSTDEventStreamRepo(
	cfg *config.Dendrite,
	bukSize,
	maxEntries,
	gcPerNum,
	delay int,
) *STDEventStreamRepo {
	tls := new(STDEventStreamRepo)
	tls.repo = NewTimeLineRepo(bukSize, 500, true, maxEntries, gcPerNum)
	tls.idg, _ = uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	tls.updatedKey = new(sync.Map)
	tls.delay = delay

	tls.startFlush()
	return tls
}

func (tl *STDEventStreamRepo) startFlush() error {
	go func() {
		t := time.NewTimer(time.Millisecond * time.Duration(tl.delay))
		for {
			select {
			case <-t.C:
				tl.flush()
				t.Reset(time.Millisecond * time.Duration(tl.delay))
			}
		}
	}()

	return nil
}

func (tl *STDEventStreamRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *STDEventStreamRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *STDEventStreamRepo) AddSTDEventStream(dataStream *types.StdEvent, targetUserID, targetDeviceID string) {
	tl.LoadHistory(targetUserID, targetDeviceID, true)
	offset, _ := tl.idg.Next()
	bytes, _ := json.Marshal(dataStream)
	log.Infof("STDEventStreamRepo.AddSTDEventStream offset:%d targetUserID %s targetDeviceID %s content %s", offset, targetUserID, targetDeviceID, string(bytes))
	tl.addSTDEventStream(dataStream, offset, targetUserID, targetDeviceID, false)
}

func (tl *STDEventStreamRepo) addSTDEventStream(dataStream *types.StdEvent, offset int64, targetUserID, targetDeviceID string, loaded bool) {
	key := fmt.Sprintf("%s:%s", targetUserID, targetDeviceID)

	stdStream := new(feedstypes.STDEventStream)
	stdStream.DataStream = dataStream
	stdStream.Offset = offset
	stdStream.Written = loaded
	stdStream.TargetUserID = targetUserID
	stdStream.TargetDeviceID = targetDeviceID

	tl.repo.add(key, stdStream)

	if loaded == false {
		tl.updatedKey.Store(key, true)
	}
}

func (tl *STDEventStreamRepo) loadHistory(targetUserID, targetDeviceID string) {
	key := fmt.Sprintf("%s:%s", targetUserID, targetDeviceID)
	defer tl.loading.Delete(key)
	bs := time.Now().UnixNano() / 1000000
	streams, offsets, err := tl.persist.GetHistoryStdStream(context.TODO(), targetUserID, targetDeviceID, 100)
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed STDEventStreamRepo load user:%s dev:%s history spend:%d ms err: %v", targetUserID, targetDeviceID, spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms STDEventStreamRepo.loadHistory finished userID:%s dev:%s spend:%d ms", types.DB_EXCEED_TIME, targetUserID, targetDeviceID, spend)
	} else {
		log.Infof("load db succ STDEventStreamRepo.loadHistory finished userID:%s dev:%s spend:%d ms", targetUserID, targetDeviceID, spend)
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
			tl.addSTDEventStream(&streams[idx], offsets[idx], targetUserID, targetDeviceID, true)
		}
	}

	if empty {
		tl.repo.setDefault(key)
	}

	tl.ready.Store(key, true)
}

func (tl *STDEventStreamRepo) LoadHistory(targetUserID, targetDeviceID string, sync bool) {
	key := fmt.Sprintf("%s:%s", targetUserID, targetDeviceID)

	if _, ok := tl.ready.Load(key); !ok {
		if _, ok := tl.loading.Load(key); !ok {
			tl.loading.Store(key, true)
			if sync == false {
				go tl.loadHistory(targetUserID, targetDeviceID)
			} else {
				tl.loadHistory(targetUserID, targetDeviceID)
			}

			tl.queryHitCounter.WithLabelValues("db", "STDEventStreamRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckLoadReady(targetUserID, targetDeviceID, true)
		}
	} else {
		res := tl.repo.getTimeLine(key)
		if res == nil {
			tl.ready.Delete(key)
			tl.LoadHistory(targetUserID, targetDeviceID, sync)
		} else {
			tl.queryHitCounter.WithLabelValues("cache", "STDEventStreamRepo", "LoadHistory").Add(1)
		}
	}

	return
}

func (tl *STDEventStreamRepo) GetHistory(targetUserID, targetDeviceID string) *feedstypes.TimeLines {
	key := fmt.Sprintf("%s:%s", targetUserID, targetDeviceID)
	tl.LoadHistory(targetUserID, targetDeviceID, true)
	return tl.repo.getTimeLine(key)
}

func (tl *STDEventStreamRepo) CheckLoadReady(targetUserID, targetDeviceID string, sync bool) bool {
	key := fmt.Sprintf("%s:%s", targetUserID, targetDeviceID)

	_, ok := tl.ready.Load(key)
	if ok || sync == false {
		if sync == false {
			tl.LoadHistory(targetUserID, targetDeviceID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.ready.Load(key); ok {
			break
		}

		tl.LoadHistory(targetUserID, targetDeviceID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed STDEventStreamRepo.CheckLoadReady user %s device %s spend:%d s but still not ready, break", targetUserID, targetDeviceID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(key)
	return ok
}

func (tl *STDEventStreamRepo) ExistsSTDEventUpdate(position int64, targetUserID, targetDeviceID string) bool {
	stdTimeLine := tl.GetHistory(targetUserID, targetDeviceID)
	if stdTimeLine == nil {
		key := fmt.Sprintf("%s:%s", targetUserID, targetDeviceID)
		tl.repo.setDefault(key)
		return false
	}

	_, feedUp := stdTimeLine.GetFeedRange()
	if feedUp > position {
		return true
	}

	return false
}

func (tl *STDEventStreamRepo) flush() {
	log.Infof("STDEventStreamRepo start flush")
	tl.updatedKey.Range(func(key, _ interface{}) bool {
		tl.updatedKey.Delete(key)

		stdTimeLine := tl.repo.getTimeLine(key.(string))
		if stdTimeLine != nil {
			var feeds []feedstypes.Feed
			stdTimeLine.ForRange(func(offset int, feed feedstypes.Feed) bool {
				if feed == nil {
					log.Errorf("syncMng.addSendToDevice get feed nil offset %d", offset)
					stdTimeLine.Console()
				} else {
					feeds = append(feeds, feed)
				}
				return true
			})
			for _, feed := range feeds {
				if feed != nil {
					stream := feed.(*feedstypes.STDEventStream)
					if stream.Written == false {
						if stream.Read {
							//已经读过，无需再存储
							stream.Written = true
						} else {
							jsonBuffer, err := json.Marshal(stream.DataStream.Content)
							if err != nil {
								log.Errorf("syncMng.addSendToDevice Marshal TargetUserID %s TargetDeviceID %s error %v", stream.TargetUserID, stream.TargetDeviceID, err)
								continue
							}
							ev := syncapitypes.StdHolder{
								Sender:   stream.DataStream.Sender,
								Event:    jsonBuffer,
								EventTyp: stream.DataStream.Type,
							}
							err = tl.persist.InsertStdMessage(context.TODO(), ev, stream.TargetUserID, stream.TargetDeviceID, common.GetDeviceMac(stream.TargetDeviceID), stream.Offset)
							if err != nil {
								log.Errorf("syncMng.addSendToDevice InsertStdMessage TargetUserID %s TargetDeviceID %s error %v", stream.TargetUserID, stream.TargetDeviceID, err)
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
	log.Infof("STDEventStreamRepo finished flush")
}
