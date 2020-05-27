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

	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type BackfillJobFinishItem struct {
	EventID      string
	StartEventID string
	StartOffset  int64
	Finished     bool
}

type BackfillRepo struct {
	db    model.FederationDatabase
	cache service.Cache
}

func NewBackfillRepo(
	db model.FederationDatabase,
	cache service.Cache,
) *BackfillRepo {
	ret := &BackfillRepo{
		db:    db,
		cache: cache,
	}
	return ret
}

func (r *BackfillRepo) LoadHistory() error {
	records, err := r.db.SelectAllBackfillRecord(context.TODO())
	if err != nil {
		return err
	}
	for i := 0; i < len(records); i++ {
		rec := &records[i]
		if rec.Finished {
			// 节约内存
			rec.States = ""
		}
		r.cache.StoreFedBackfillRec(rec.RoomID, rec.Depth, rec.Finished, rec.FinishedDomains, rec.States)
	}
	return nil
}

func (r *BackfillRepo) GenerateFakePartition(ctx context.Context) int32 {
	p, err := r.cache.GetFakePartition("fedbackfill")
	if err != nil {
		log.Errorf("SendRecRepo generation fake partition error %v", err)
		return 0
	}
	return p
}

func (r *BackfillRepo) AssignRoomPartition(ctx context.Context, roomID string, partition int32) (*model.BackfillRecord, bool) {
	err := r.cache.AssignFedBackfillRecPartition(roomID, partition)
	if err != nil {
		return nil, false
	}
	recItem := r.getRec(ctx, roomID)
	if recItem == nil {
		r.cache.UnassignFedBackfillRecPartition(roomID)
		return nil, false
	}
	return recItem, true
}

func (r *BackfillRepo) UnassignRoomPartition(ctx context.Context, roomID string) error {
	err := r.cache.UnassignFedBackfillRecPartition(roomID)
	return err
}

func (r *BackfillRepo) ExpireRoomPartition(ctx context.Context, roomID string) error {
	err := r.cache.ExpireFedBackfillRecPartition(roomID)
	return err
}

func (r *BackfillRepo) getRec(ctx context.Context, roomID string) *model.BackfillRecord {
	depth, finished, finishedDomains, states, err := r.cache.GetFedBackfillRec(roomID)
	if err != nil {
		return nil
	}
	return &model.BackfillRecord{
		RoomID:          roomID,
		Depth:           depth,
		Finished:        finished,
		FinishedDomains: finishedDomains,
		States:          states,
	}
}

func (r *BackfillRepo) GetUnfinishedRooms() ([]string, error) {
	roomIDs, err := r.cache.QryFedBackfillRooms()
	if err != nil {
		log.Errorf("get backfill unfinished room error %v", err)
		return nil, err
	}
	return roomIDs, nil
}

func (r *BackfillRepo) IsBackfillFinished(ctx context.Context, roomID string) (isFinished, hasRec bool, err error) {
	rec := r.getRec(ctx, roomID)
	if rec != nil {
		var rec model.BackfillRecord
		rec, err = r.db.SelectBackfillRecord(ctx, roomID)
		if err != nil {
			hasRec = strings.Contains(err.Error(), "no rows in result set")
			return
		}
		loaded, err := r.cache.StoreFedBackfillRec(roomID, rec.Depth, rec.Finished, rec.FinishedDomains, rec.States)
		if err != nil {
			return false, true, err
		}
		if loaded {
			r.cache.AssignFedBackfillRecPartition(roomID, -1) // TODO:
		}

	}
	return rec.Finished, true, nil
}

func (r *BackfillRepo) InsertBackfillRecord(ctx context.Context, rec model.BackfillRecord) (bool, error) {
	loaded, err := r.cache.StoreFedBackfillRec(rec.RoomID, rec.Depth, rec.Finished, rec.FinishedDomains, rec.States)
	if err != nil {
		return false, err
	}
	if loaded {
		return false, nil
	}
	err = r.db.InsertBackfillRecord(ctx, rec)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *BackfillRepo) UpdateRecord(ctx context.Context, roomID string, depth int64, finished bool, finishedDomains string, states string) error {
	err := r.db.UpdateBackfillRecordDomainsInfo(ctx, roomID, 0, true, finishedDomains, states)
	if err != nil {
		return err
	}
	_, err = r.cache.UpdateFedBackfillRec(roomID, depth, finished, finishedDomains, states)
	if err != nil {
		return err
	}
	return nil
}

func (r *BackfillRepo) GetFinishedDomains(ctx context.Context, roomID string) (map[string]*BackfillJobFinishItem, bool) {
	rec := r.getRec(ctx, roomID)
	if rec == nil {
		return nil, false
	}
	var finished map[string]*BackfillJobFinishItem
	json.Unmarshal([]byte(rec.FinishedDomains), &finished)
	return finished, true
}
