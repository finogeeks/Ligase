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

package federation

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/federation/storage/model"
)

const backfillRecordSchema = `
CREATE TABLE IF NOT EXISTS federation_backfill_record (
    room_id TEXT NOT NULL,
    domain TEXT NOT NULL,
    states TEXT NOT NULL,
    finished BOOLEAN NOT NULL,
    finishedDomains TEXT NOT NULL,
    depth BIGINT NOT NULL DEFAULT 0,
	CONSTRAINT federation_backfill_record_unique UNIQUE (room_id)
);

-- CREATE UNIQUE INDEX IF NOT EXISTS federation_backfill_record_room_id_idx
--	ON federation_backfill_record (room_id)
`

const insertBackfillRecordSQL = "" +
	"INSERT INTO federation_backfill_record(room_id, domain, states, depth, finished, finishedDomains)" +
	" VALUES($1, $2, $3, $4, $5, $6)"

const selectAllBackfillRecordSQL = "" +
	"SELECT room_id, domain, states, depth, finished, finishedDomains FROM federation_backfill_record"

const selectBackfillRecordSQL = "" +
	"SELECT room_id, domain, states, depth, finished, finishedDomains FROM federation_backfill_record WHERE room_id = $1"

const updateBackfillRecordDomainInfoSQL = "" +
	"UPDATE federation_backfill_record SET depth=$2, finished=$3, finishedDomains=$4, states=$5 WHERE room_id=$1"

type backfillRecordStatements struct {
	insertBackfillRecordStmt            *sql.Stmt
	selectAllBackfillRecordStmt         *sql.Stmt
	selectBackfillRecordStmt            *sql.Stmt
	updateBackfillRecordDomainsInfoStmt *sql.Stmt
}

func (s *backfillRecordStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(backfillRecordSchema)
	if err != nil {
		return err
	}
	if s.insertBackfillRecordStmt, err = db.Prepare(insertBackfillRecordSQL); err != nil {
		return
	}
	if s.selectAllBackfillRecordStmt, err = db.Prepare(selectAllBackfillRecordSQL); err != nil {
		return
	}
	if s.selectBackfillRecordStmt, err = db.Prepare(selectBackfillRecordSQL); err != nil {
		return
	}
	if s.updateBackfillRecordDomainsInfoStmt, err = db.Prepare(updateBackfillRecordDomainInfoSQL); err != nil {
		return
	}
	return
}

func (s *backfillRecordStatements) insertBackfillRecord(ctx context.Context, rec model.BackfillRecord) error {
	_, err := s.insertBackfillRecordStmt.ExecContext(ctx, rec.RoomID, rec.Domain, rec.States, rec.Depth, rec.Finished, rec.FinishedDomains)
	return err
}

func (s *backfillRecordStatements) selectAllBackfillRecord(ctx context.Context) ([]model.BackfillRecord, error) {
	rows, err := s.selectAllBackfillRecordStmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	recs := []model.BackfillRecord{}
	for rows.Next() {
		rec := model.BackfillRecord{}
		err = rows.Scan(
			&rec.RoomID,
			&rec.Domain,
			&rec.States,
			&rec.Depth,
			&rec.Finished,
			&rec.FinishedDomains,
		)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

func (s *backfillRecordStatements) selectBackfillRecord(ctx context.Context, roomID string) (model.BackfillRecord, error) {
	var rec model.BackfillRecord
	err := s.selectBackfillRecordStmt.QueryRowContext(ctx, roomID).Scan(
		&rec.RoomID,
		&rec.Domain,
		&rec.States,
		&rec.Depth,
		&rec.Finished,
		&rec.FinishedDomains,
	)
	if err != nil {
		return rec, err
	}
	return rec, nil
}

func (s *backfillRecordStatements) updateBackfillRecordDomainsInfo(ctx context.Context, roomID string, depth int64, finished bool, finishedDomains string, states string) error {
	_, err := s.updateBackfillRecordDomainsInfoStmt.ExecContext(ctx, roomID, depth, finished, finishedDomains, states)
	return err
}
