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
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type JoinedRoomFinishedEventOffset struct {
	EventID string
	Offset  int64
}

type JoinRoomsData struct {
	RoomID         string
	EventID        string
	RecvOffsets    string
	RecvOffsetsMap map[string]*JoinedRoomFinishedEventOffset
	HasJoined      bool
	lastActiveTime int64
}

type JoinRoomsRepo struct {
	db model.FederationDatabase

	repo sync.Map
}

func NewJoinRoomsRepo(
	db model.FederationDatabase,
) *JoinRoomsRepo {
	ret := &JoinRoomsRepo{
		db: db,
	}
	ret.startTimer()
	return ret
}

func (r *JoinRoomsRepo) startTimer() {
	go func() {
		ticker := time.NewTicker(time.Minute * 5)
		for {
			<-ticker.C
			log.Infof("JoinRoomsRepo check release")

			deletes := []string{}
			now := time.Now().Unix()
			r.repo.Range(func(k, v interface{}) bool {
				if v.(*JoinRoomsData).lastActiveTime+300 < now {
					deletes = append(deletes, k.(string))
				}
				return true
			})
			for _, v := range deletes {
				val, ok := r.repo.Load(v)
				if !ok {
					continue
				}
				if val.(*JoinRoomsData).lastActiveTime+300 < now {
					r.repo.Delete(v)
				}
			}
		}
	}()
}

func (r *JoinRoomsRepo) GetData(ctx context.Context, roomID string) (*JoinRoomsData, error) {
	var data *JoinRoomsData
	if val, ok := r.repo.Load(roomID); !ok {
		eventID, recvOffsets, err := r.db.SelectJoinedRooms(ctx, roomID)
		if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
			return nil, err
		}
		if eventID == "" {
			val, _ = r.repo.LoadOrStore(roomID, &JoinRoomsData{
				RoomID:         roomID,
				RecvOffsetsMap: map[string]*JoinedRoomFinishedEventOffset{},
			})
		} else {
			var recvOffsetsMap map[string]*JoinedRoomFinishedEventOffset
			json.Unmarshal([]byte(recvOffsets), &recvOffsetsMap)
			if recvOffsetsMap == nil {
				recvOffsetsMap = map[string]*JoinedRoomFinishedEventOffset{}
			}
			val, _ = r.repo.LoadOrStore(roomID, &JoinRoomsData{
				RoomID:         roomID,
				EventID:        eventID,
				RecvOffsets:    recvOffsets,
				RecvOffsetsMap: recvOffsetsMap,
				HasJoined:      true,
			})
		}
		data = val.(*JoinRoomsData)
	} else {
		data = val.(*JoinRoomsData)
	}
	data.lastActiveTime = time.Now().Unix()
	return data, nil
}

func (r *JoinRoomsRepo) AddData(ctx context.Context, data *JoinRoomsData) error {
	r.repo.Store(data.RoomID, data)
	data.lastActiveTime = time.Now().Unix()
	return r.db.InsertJoinedRooms(ctx, data.RoomID, data.EventID)
}

func (r *JoinRoomsRepo) UpdateData(ctx context.Context, data *JoinRoomsData) error {
	r.repo.Store(data.RoomID, data)
	data.lastActiveTime = time.Now().Unix()
	recvOffsets, _ := json.Marshal(data.RecvOffsetsMap)
	data.RecvOffsets = string(recvOffsets)
	return r.db.UpdateJoinedRoomsRecvOffset(ctx, data.RoomID, data.RecvOffsets)
}
