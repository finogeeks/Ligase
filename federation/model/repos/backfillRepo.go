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

	"github.com/finogeeks/ligase/federation/storage/model"
)

type BackfillJobFinishItem struct {
	EventID      string
	StartEventID string
	StartOffset  int64
	Finished     bool
}

type BackfillRepo struct {
	db model.FederationDatabase

	repo sync.Map
}

func NewBackfillRepo(
	db model.FederationDatabase,
) *BackfillRepo {
	ret := &BackfillRepo{
		db: db,
	}
	return ret
}

func (r *BackfillRepo) InitBackfillRec(roomID string, rec *model.BackfillRecord) bool {
	_, ok := r.repo.LoadOrStore(roomID, rec)
	return !ok
}

func (r *BackfillRepo) DeleteBackfillRec(roomID string) {
	r.repo.Delete(roomID)
}

func (r *BackfillRepo) GetUnfinishedRecs() []*model.BackfillRecord {
	ret := []*model.BackfillRecord{}
	r.repo.Range(func(k, v interface{}) bool {
		rec := v.(*model.BackfillRecord)
		if !rec.Finished {
			ret = append(ret, rec)
		}
		return true
	})
	return ret
}

func (r *BackfillRepo) SetBackfillFinished(roomID string) {
	v, ok := r.repo.Load(roomID)
	if !ok {
		return
	}
	rec := v.(*model.BackfillRecord)
	rec.Finished = true
	// 节约内存
	rec.States = ""
}

func (r *BackfillRepo) IsBackfillFinished(roomID string) (isFinished, hasRec bool, err error) {
	v, ok := r.repo.Load(roomID)
	if !ok {
		var rec model.BackfillRecord
		rec, err = r.db.SelectBackfillRecord(context.TODO(), roomID)
		if err != nil {
			hasRec = strings.Contains(err.Error(), "no rows in result set")
			return
		}
		v, _ = r.repo.LoadOrStore(roomID, &rec)
	}
	return v.(*model.BackfillRecord).Finished, true, nil
}

func (r *BackfillRepo) SetFinishedDomains(roomID, finished string) {
	v, ok := r.repo.Load(roomID)
	if !ok {
		return
	}
	v.(*model.BackfillRecord).FinishedDomains = finished
}

func (r *BackfillRepo) SetState(roomID, state string) {
	v, ok := r.repo.Load(roomID)
	if !ok {
		return
	}
	v.(*model.BackfillRecord).States = state
}

func (r *BackfillRepo) GetFinishedDomains(roomID string) (map[string]*BackfillJobFinishItem, bool) {
	var finished map[string]*BackfillJobFinishItem
	v, ok := r.repo.Load(roomID)
	if !ok {
		return nil, false
	}
	json.Unmarshal([]byte(v.(*model.BackfillRecord).FinishedDomains), &finished)
	return finished, true
}
