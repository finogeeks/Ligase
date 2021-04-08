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
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

type KeyChangeStreamRepo struct {
	syncDB              model.SyncAPIDatabase
	userTimeLine        *UserTimeLineRepo
	cache               service.Cache
	repo                *sync.Map
	OneTimeKeyCountInfo *sync.Map
	maxPosition         sync.Map
	ready               sync.Map
	loading             sync.Map

	queryHitCounter mon.LabeledCounter
}

func NewKeyChangeStreamRepo(
	userTimeLine *UserTimeLineRepo,
) *KeyChangeStreamRepo {
	tls := new(KeyChangeStreamRepo)
	tls.repo = new(sync.Map)
	tls.userTimeLine = userTimeLine
	tls.OneTimeKeyCountInfo = new(sync.Map)

	return tls
}

func (tl *KeyChangeStreamRepo) SetSyncDB(db model.SyncAPIDatabase) {
	tl.syncDB = db
}

func (tl *KeyChangeStreamRepo) SetCache(cache service.Cache) {
	tl.cache = cache
}

func (tl *KeyChangeStreamRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *KeyChangeStreamRepo) GetOneTimeKeyCount(userID string, deviceID string) (alCountMap map[string]int, err error) {
	key := fmt.Sprintf("%s:%s", userID, deviceID)

	if val, ok := tl.OneTimeKeyCountInfo.Load(key); ok {
		return val.(map[string]int), nil
	} else {
		err := tl.UpdateOneTimeKeyCount(userID, deviceID)
		if err != nil {
			return nil, err
		}
		return tl.GetOneTimeKeyCount(userID, deviceID)
	}
}

func (tl *KeyChangeStreamRepo) UpdateOneTimeKeyCount(userID string, deviceID string) error {
	key := fmt.Sprintf("%s:%s", userID, deviceID)

	alCountMap := common.InitKeyCount()
	keyIDs, ok := tl.cache.GetOneTimeKeyIDs(userID, deviceID)
	if ok {
		for _, keyID := range keyIDs {
			key, exists := tl.cache.GetOneTimeKey(keyID)
			if exists && key.UserID != "" {
				if _, ok := alCountMap[key.KeyAlgorithm]; ok {
					count := alCountMap[key.KeyAlgorithm] + 1
					alCountMap[key.KeyAlgorithm] = count
				} else {
					alCountMap[key.KeyAlgorithm] = 1
				}
			}
		}
	}

	tl.OneTimeKeyCountInfo.Store(key, alCountMap)
	return nil
}

func (tl *KeyChangeStreamRepo) AddKeyChangeStream(dataStream *types.KeyChangeStream, offset int64, broadCast bool) {
	keyChangeStream := new(feedstypes.KeyChangeStream)
	keyChangeStream.DataStream = dataStream
	keyChangeStream.Offset = offset

	tl.repo.Store(dataStream.ChangedUserID, keyChangeStream)

	if broadCast {
		userMap := tl.userTimeLine.GetFriendShip(dataStream.ChangedUserID, true)
		if userMap != nil {
			userMap.Range(func(key, _ interface{}) bool {
				if maxPos, ok := tl.maxPosition.Load(key.(string)); ok {
					if maxPos.(int64) < offset {
						tl.maxPosition.Store(key.(string), offset)
					}
				} else {
					tl.maxPosition.Store(key.(string), offset)
				}
				return true
			})
		}
	}
}

func (tl *KeyChangeStreamRepo) LoadHistory(userID string, sync bool) {
	if _, ok := tl.ready.Load(userID); !ok {
		if _, loaded := tl.loading.LoadOrStore(userID, true); !loaded {
			if sync == false {
				go tl.loadHistory(userID)
			} else {
				tl.loadHistory(userID)
			}

			tl.queryHitCounter.WithLabelValues("db", "KeyChangeStreamRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckLoadReady(userID, true)
		}
	} else {
		tl.queryHitCounter.WithLabelValues("cache", "KeyChangeStreamRepo", "LoadHistory").Add(1)
	}
}

func (tl *KeyChangeStreamRepo) loadHistory(userID string) {
	defer tl.loading.Delete(userID)
	userMap := tl.userTimeLine.GetFriendShip(userID, true)
	if userMap != nil {
		var users []string
		maxPos := int64(0)
		userMap.Range(func(key, _ interface{}) bool {
			if presence, ok := tl.repo.Load(key.(string)); !ok {
				users = append(users, key.(string))
			} else {
				if maxPos < presence.(*feedstypes.KeyChangeStream).Offset {
					maxPos = presence.(*feedstypes.KeyChangeStream).Offset
				}
			}
			return true
		})
		if len(users) > 0 {
			bs := time.Now().UnixNano() / 1000000
			streams, offsets, err := tl.syncDB.GetHistoryKeyChangeStream(context.TODO(), users)
			spend := time.Now().UnixNano()/1000000 - bs
			if err != nil {
				log.Errorf("load db failed KeyChangeStreamRepo history user:%s spend:%d ms err:%v", userID, spend, err)
				return
			}
			if spend > types.DB_EXCEED_TIME {
				log.Warnf("load db exceed %d ms KeyChangeStreamRepo user:%s spend:%d ms", types.DB_EXCEED_TIME, userID, spend)
			} else {
				log.Infof("load db succ KeyChangeStreamRepo user:%s spend:%d ms", userID, spend)
			}
			for idx := range streams {
				tl.AddKeyChangeStream(&streams[idx], offsets[idx], false)
				if offsets[idx] > maxPos {
					maxPos = offsets[idx]
				}
			}
		}
		if val, ok := tl.maxPosition.Load(userID); ok {
			if val.(int64) < maxPos {
				tl.maxPosition.Store(userID, maxPos)
			}
		} else {
			tl.maxPosition.Store(userID, maxPos)
		}
	}
	tl.ready.Store(userID, true)
}

func (tl *KeyChangeStreamRepo) CheckLoadReady(userID string, sync bool) bool {
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
			log.Errorf("checkloadready failed KeyChangeStreamRepo.CheckLoadReady user %s spend:%d s but still not ready, break", userID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(userID)
	return ok
}

func (tl *KeyChangeStreamRepo) GetHistory() *sync.Map {
	return tl.repo
}

func (tl *KeyChangeStreamRepo) ExistsKeyChange(position int64, userID string) bool {
	if maxPos, ok := tl.maxPosition.Load(userID); ok {
		return maxPos.(int64) > position
	}
	return false
}

func (tl *KeyChangeStreamRepo) GetUserLatestOffset(userID string) int64 {
	if maxPos, ok := tl.maxPosition.Load(userID); ok {
		return maxPos.(int64)
	}
	return int64(-1)
}
