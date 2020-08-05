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
	"fmt"
	"github.com/finogeeks/ligase/model/service"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"sync"
	"time"
)

type ReadCountRepo struct {
	cache     service.Cache
	readCount *sync.Map
	hlCount   *sync.Map
	updated   *sync.Map
	delay     int
}

type UpdatedCountKey struct {
	RoomID string
	UserID string
}

func NewReadCountRepo(
	delay int,
) *ReadCountRepo {
	tls := new(ReadCountRepo)
	tls.readCount = new(sync.Map)
	tls.hlCount = new(sync.Map)
	tls.updated = new(sync.Map)

	tls.delay = delay
	tls.startFlush()
	return tls
}

func (tl *ReadCountRepo) startFlush() error {
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

func (tl *ReadCountRepo) SetCache(cache service.Cache) {
	tl.cache = cache
}

func (tl *ReadCountRepo) GetRoomReadCount(roomID, userID string) (int64, int64) {
	key := fmt.Sprintf("%s:%s", roomID, userID)
	readCount := int64(0)
	hlCount := int64(0)

	if val, ok := tl.readCount.Load(key); ok {
		readCount = val.(int64)
	} else {
		tl.UpdateRoomReadCountFromCache(roomID, userID)
		if val, ok := tl.readCount.Load(key); ok {
			readCount = val.(int64)
		}
	}

	if val, ok := tl.hlCount.Load(key); ok {
		hlCount = val.(int64)
	} else {
		tl.UpdateRoomReadCountFromCache(roomID, userID)
		if val, ok := tl.hlCount.Load(key); ok {
			hlCount = val.(int64)
		}
	}

	return readCount, hlCount
}

func (tl *ReadCountRepo) UpdateRoomReadCountFromCache(roomID, userID string) {
	key := fmt.Sprintf("%s:%s", roomID, userID)
	hl, count, _ := tl.cache.GetRoomUnreadCount(userID, roomID)
	tl.readCount.Store(key, count)
	tl.hlCount.Store(key, hl)
}

func (tl *ReadCountRepo) UpdateRoomReadCount(roomID, eventID, userID, updateType string) {
	log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s", roomID, eventID, userID, updateType)
	key := fmt.Sprintf("%s:%s", roomID, userID)

	switch updateType {
	case "reset":
		tl.readCount.Store(key, int64(0))
		tl.hlCount.Store(key, int64(0))
		log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d hlCount:%d", roomID, eventID, userID, updateType, 0, 0)
	case "increase":
		if val, ok := tl.readCount.Load(key); ok {
			tl.readCount.Store(key, val.(int64)+1)
			log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d", roomID, eventID, userID, updateType, val.(int64)+1)
		} else {
			tl.UpdateRoomReadCountFromCache(roomID, userID)
			if val, ok := tl.readCount.Load(key); ok {
				tl.readCount.Store(key, val.(int64)+1)
				log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d", roomID, eventID, userID, updateType, val.(int64)+1)
			} else {
				tl.readCount.Store(key, int64(1))
				log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d", roomID, eventID, userID, updateType, 1)
			}
		}
	case "increase_hl":
		if val, ok := tl.hlCount.Load(key); ok {
			tl.hlCount.Store(key, val.(int64)+1)
			log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s hlCount:%d", roomID, eventID, userID, updateType, val.(int64)+1)
		} else {
			tl.UpdateRoomReadCountFromCache(roomID, userID)
			if val, ok := tl.hlCount.Load(key); ok {
				tl.hlCount.Store(key, val.(int64)+1)
				log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s hlCount:%d", roomID, eventID, userID, updateType, val.(int64)+1)
			} else {
				tl.hlCount.Store(key, int64(1))
				log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s hlCount:%d", roomID, eventID, userID, updateType, 1)
			}
		}
	case "decrease":
		if val, ok := tl.readCount.Load(key); ok {
			count := val.(int64)
			if count > 0 {
				tl.readCount.Store(key, count-1)
				log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d", roomID, eventID, userID, updateType, count-1)
				if val1, ok := tl.hlCount.Load(key); ok {
					hlCount := val1.(int64)
					if hlCount >= count {
						tl.hlCount.Store(key, count-1)
						log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s hlCount:%d", roomID, eventID, userID, updateType, count-1)
					}
				}
			}

		} else {
			tl.UpdateRoomReadCountFromCache(roomID, userID)
			if val, ok := tl.readCount.Load(key); ok {
				count := val.(int64)
				if count > 0 {
					tl.readCount.Store(key, count-1)
					log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d", roomID, eventID, userID, updateType, count-1)
					if val1, ok := tl.hlCount.Load(key); ok {
						hlCount := val1.(int64)
						if hlCount >= count {
							tl.hlCount.Store(key, count-1)
							log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s hlCount:%d", roomID, eventID, userID, updateType, count-1)
						}
					}
				}
			} else {
				tl.readCount.Store(key, int64(0))
				tl.hlCount.Store(key, int64(0))
				log.Infof("UpdateRoomReadCount roomID:%s eventID:%s userID:%s updateType:%s readCount:%d hlCount:%d", roomID, eventID, userID, updateType, 0, 0)
			}
		}
	}

	upKey := UpdatedCountKey{
		RoomID: roomID,
		UserID: userID,
	}
	tl.updated.Store(key, &upKey)
}

func (tl *ReadCountRepo) flush() {
	log.Infof("ReadCountRepo start flush")
	tl.updated.Range(func(key, value interface{}) bool {
		tl.updated.Delete(key)

		upKey := value.(*UpdatedCountKey)
		readCount, hlCount := tl.GetRoomReadCount(upKey.RoomID, upKey.UserID)
		err := tl.cache.SetRoomUnreadCount(upKey.UserID, upKey.RoomID, readCount, hlCount)
		if err != nil {
			log.Errorf("ReadCountRepo write cache roomID %s userID %s err %v", upKey.RoomID, upKey.UserID, err)
		}

		return true
	})
	log.Infof("ReadCountRepo finished flush")
}
