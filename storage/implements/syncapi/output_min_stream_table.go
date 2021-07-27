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

package syncapi

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const OutputMinStreamSchema = `
CREATE TABLE IF NOT EXISTS syncapi_output_min_stream (
	  id BIGINT,
      room_id TEXT NOT NULL,
	  CONSTRAINT syncapi_output_min_stream_unique UNIQUE (room_id)
);
CREATE INDEX IF NOT EXISTS syncapi_output_min_stream_idx ON syncapi_output_min_stream(room_id);
`

const insertOutputMinStreamSQL = "" +
	"INSERT INTO syncapi_output_min_stream (id, room_id) VALUES ($1, $2)" +
	" ON CONFLICT ON CONSTRAINT syncapi_output_min_stream_unique DO UPDATE SET id = EXCLUDED.id"

const selectOutputMinStreamSQL = "" +
	"SELECT id  FROM syncapi_output_min_stream WHERE room_id = $1"

type outputMinStreamStatements struct {
	db                        *Database
	insertOutputMinStreamStmt *sql.Stmt
	selectOutputMinStreamStmt *sql.Stmt
}

func (s *outputMinStreamStatements) getSchema() string {
	return OutputMinStreamSchema
}

func (s *outputMinStreamStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	_, err = db.Exec(OutputMinStreamSchema)
	if err != nil {
		return
	}
	if s.insertOutputMinStreamStmt, err = db.Prepare(insertOutputMinStreamSQL); err != nil {
		return
	}
	if s.selectOutputMinStreamStmt, err = db.Prepare(selectOutputMinStreamSQL); err != nil {
		return
	}
	return
}

func (s *outputMinStreamStatements) insertOutputMinStream(
	ctx context.Context,
	id int64, roomID string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncOutputMinStreamInsertKey
		update.SyncDBEvents.SyncOutputMinStreamInsert = &dbtypes.SyncOutputMinStreamInsert{
			ID:     id,
			RoomID: roomID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		s.db.WriteDBEventWithTbl(&update, "syncapi_output_min_stream")
		return nil
	} else {
		_, err = s.insertOutputMinStreamStmt.ExecContext(ctx, id, roomID)
		return
	}
}

func (s *outputMinStreamStatements) onInsertOutputMinStream(
	ctx context.Context,
	id int64, roomID string,
) (err error) {
	_, err = s.insertOutputMinStreamStmt.ExecContext(ctx, id, roomID)
	return
}

func (s *outputMinStreamStatements) selectOutputMinStream(
	ctx context.Context,
	roomID string,
) (int64, error) {
	rows, err := s.selectOutputMinStreamStmt.QueryContext(ctx, roomID)
	if err != nil {
		log.Errorf("outputMinStreamStatements.selectOutputMinStream err: %v", err)
		return -1, err
	}

	result := int64(-1)
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&result); err != nil {
			return -1, err
		}
	}
	return result, nil
}
