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
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/tidwall/gjson"
)

type PresenceDataStreamRepo struct {
	persist      model.SyncAPIDatabase
	userTimeLine *UserTimeLineRepo
	repo         *sync.Map
	maxPosition  sync.Map
	ready        sync.Map
	loading      sync.Map
	cfg          *config.Dendrite
	onlineRepo   *OnlineUserRepo

	queryHitCounter mon.LabeledCounter
}

func NewPresenceDataStreamRepo(
	userTimeLine *UserTimeLineRepo,
) *PresenceDataStreamRepo {
	tls := new(PresenceDataStreamRepo)
	tls.userTimeLine = userTimeLine
	tls.repo = new(sync.Map)

	return tls
}

func (tl *PresenceDataStreamRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *PresenceDataStreamRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	tl.queryHitCounter = queryHitCounter
}

func (tl *PresenceDataStreamRepo) SetCfg(cfg *config.Dendrite) {
	tl.cfg = cfg
}

func (tl *PresenceDataStreamRepo) SetOnlineRepo(repo *OnlineUserRepo) {
	tl.onlineRepo = repo
}

func (tl *PresenceDataStreamRepo) AddPresenceDataStream(ctx context.Context,
	dataStream *types.PresenceStream, offset int64, broadCast bool) {
	if val, ok := tl.repo.Load(dataStream.UserID); ok && val != nil {
		feed := val.(*feedstypes.PresenceDataStream)
		if feed.GetOffset() > offset {
			return
		}
	}
	presenceDataStream := new(feedstypes.PresenceDataStream)
	presenceDataStream.DataStream = dataStream
	presenceDataStream.Offset = offset

	tl.repo.Store(dataStream.UserID, presenceDataStream)

	if broadCast {
		userMap := tl.userTimeLine.GetFriendShip(ctx, dataStream.UserID, true)
		if userMap != nil {
			userMap.Range(func(key, _ interface{}) bool {
				if maxPos, ok := tl.maxPosition.Load(key.(string)); ok {
					log.Infof("change user:%s check update presence user:%s offset:%d exsit:%d", dataStream.UserID, key.(string), offset, maxPos.(int64))
					if maxPos.(int64) < offset {
						log.Infof("change user:%s update presence is men user:%s offset:%d", dataStream.UserID, key.(string), offset)
						tl.maxPosition.Store(key.(string), offset)
					}
				} else {
					tl.maxPosition.Store(key.(string), offset)
					log.Infof("change user:%s update presence no mem user:%s offset:%d", dataStream.UserID, key.(string), offset)
				}
				return true
			})
		}
	}
}

func (tl *PresenceDataStreamRepo) UpdateUserMaxPos(userID string, offset int64) {
	if maxPos, ok := tl.maxPosition.Load(userID); ok {
		log.Infof("change user update presence user:%s offset:%d exsit:%d", userID, offset, maxPos.(int64))
		if maxPos.(int64) < offset {
			log.Infof("change user update presence is men user:%s offset:%d", userID, offset)
			tl.maxPosition.Store(userID, offset)
		}
	} else {
		tl.maxPosition.Store(userID, offset)
		log.Infof("change user update presence no mem user:%s offset:%d", userID, offset)
	}
}

func (tl *PresenceDataStreamRepo) LoadHistory(ctx context.Context, userID string, sync bool) {
	if _, ok := tl.ready.Load(userID); !ok {
		if _, ok := tl.loading.Load(userID); !ok {
			tl.loading.Store(userID, true)
			if sync == false {
				go tl.loadHistory(ctx, userID)
			} else {
				tl.loadHistory(ctx, userID)
			}

			tl.queryHitCounter.WithLabelValues("db", "PresenceDataStreamRepo", "LoadHistory").Add(1)
		} else {
			if sync == false {
				return
			}
			tl.CheckLoadReady(ctx, userID, true)
		}
	} else {
		tl.queryHitCounter.WithLabelValues("cache", "PresenceDataStreamRepo", "LoadHistory").Add(1)
	}
}

func (tl *PresenceDataStreamRepo) CheckLoadReady(ctx context.Context, userID string, sync bool) bool {
	_, ok := tl.ready.Load(userID)
	if ok || sync == false {
		if sync == false {
			tl.LoadHistory(ctx, userID, false)
		}
		return ok
	}

	start := time.Now().Unix()
	for {
		if _, ok := tl.ready.Load(userID); ok {
			break
		}

		tl.LoadHistory(ctx, userID, false)

		now := time.Now().Unix()
		if now-start > 35 {
			log.Errorf("checkloadready failed PresenceDataStreamRepo.CheckLoadReady user %s spend:%d s but still not ready, break", userID, now-start)
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	_, ok = tl.ready.Load(userID)
	return ok
}

func (tl *PresenceDataStreamRepo) loadHistory(ctx context.Context, userID string) {
	defer tl.loading.Delete(userID)
	userMap := tl.userTimeLine.GetFriendShip(ctx, userID, true)
	if userMap != nil {
		var users []string
		maxPos := int64(0)
		userMap.Range(func(key, _ interface{}) bool {
			if presence, ok := tl.repo.Load(key.(string)); !ok {
				users = append(users, key.(string))
			} else {
				if maxPos < presence.(*feedstypes.PresenceDataStream).Offset {
					maxPos = presence.(*feedstypes.PresenceDataStream).Offset
				}
			}
			return true
		})
		if len(users) > 0 {
			bs := time.Now().UnixNano() / 1000000
			streams, offsets, err := tl.persist.GetUserPresenceDataStream(ctx, users)
			spend := time.Now().UnixNano()/1000000 - bs
			if err != nil {
				log.Errorf("load db failed PresenceDataStreamRepo history user:%s spend:%d ms err:%v", userID, spend, err)
				return
			}
			if spend > types.DB_EXCEED_TIME {
				log.Warnf("load db exceed %d ms PresenceDataStreamRepo history user:%s spend:%d ms", types.DB_EXCEED_TIME, userID, spend)
			} else {
				log.Infof("load db succ PresenceDataStreamRepo history user:%s spend:%d ms", userID, spend)
			}

			for idx := range streams {
				tl.AddPresenceDataStream(ctx, &streams[idx], offsets[idx], false)
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

func (tl *PresenceDataStreamRepo) GetHistoryByUserID(userID string) *feedstypes.PresenceDataStream {
	if tl.repo == nil {
		return nil
	}
	val, ok := tl.repo.Load(userID)
	if !ok || val == nil {
		return nil
	}
	return val.(*feedstypes.PresenceDataStream)
}

func (tl *PresenceDataStreamRepo) ExistsPresence(userID string, position int64) bool {
	if maxPos, ok := tl.maxPosition.Load(userID); ok {
		return maxPos.(int64) > position
	}
	return false
}

func (tl *PresenceDataStreamRepo) LoadOnlinePresence() {
	limit := 1000
	offset := 0
	exists := true

	span, ctx := common.StartSobSomSpan(context.Background(), "PresenceDataStreamRepo.LoadOnlinePresence")
	defer span.Finish()
	for exists {
		exists = false
		streams, offsets, err := tl.persist.GetHistoryPresenceDataStream(ctx, limit, offset)
		if err != nil {
			log.Panicf("PresenceDataStreamRepo load history err: %v", err)
			return
		}

		for idx := range streams {
			dataStream := streams[idx]
			exists = true
			offset = offset + 1

			if !common.IsRelatedRequest(dataStream.UserID, tl.cfg.MultiInstance.Instance, tl.cfg.MultiInstance.Total, tl.cfg.MultiInstance.MultiWrite) {
				continue
			}

			domain, _ := common.DomainFromID(dataStream.UserID)
			if !common.CheckValidDomain(domain, tl.cfg.Matrix.ServerName) {
				continue
			}

			value := gjson.Get(string(dataStream.Content), "content.presence")
			if value.String() != "offline" {
				tl.onlineRepo.Pet(ctx, dataStream.UserID, "virtual-restore", -1, 0)
				stream := new(types.PresenceStream)
				*stream = dataStream
				presenceDataStream := new(feedstypes.PresenceDataStream)
				presenceDataStream.DataStream = stream
				presenceDataStream.Offset = offsets[idx]
				tl.repo.LoadOrStore(dataStream.UserID, presenceDataStream)
				log.Debugf("restore presence stream user: %s, presence: %s", dataStream.UserID, value.String())
			}
		}
	}
}
