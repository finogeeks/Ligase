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
	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/lib/pq"
)

const KeyChangeSchema = `

--CREATE SEQUENCE IF NOT EXISTS syncapi_stream_id;

-- Stores the types of data that a user upload his device key
-- stream ID between fromPos and toPos.
CREATE TABLE IF NOT EXISTS syncapi_key_change_stream (
	-- stream pos of sync sequence
	--id BIGINT PRIMARY KEY DEFAULT nextval('syncapi_stream_id'),
	id BIGINT PRIMARY KEY,
    changed_user_id TEXT NOT NULL,
	CONSTRAINT syncapi_key_change_tream_unique UNIQUE (changed_user_id)
);

CREATE INDEX IF NOT EXISTS syncapi_key_change_user_id_idx ON syncapi_key_change_stream(changed_user_id);
`

const insertKeyStreamSQL = "" +
	"INSERT INTO syncapi_key_change_stream (id, changed_user_id) VALUES ($1, $2)" +
	" ON CONFLICT ON CONSTRAINT syncapi_key_change_tream_unique" +
	" DO UPDATE SET id = EXCLUDED.id"

const selectHistoryKeyStreamSQL = "" +
	"SELECT id, changed_user_id  FROM syncapi_key_change_stream WHERE changed_user_id = any($1) ORDER BY id ASC"

type keyChangeStatements struct {
	db                         *Database
	insertKeyStreamStmt        *sql.Stmt
	selectHistoryKeyStreamStmt *sql.Stmt
}

func (s *keyChangeStatements) getSchema() string {
	return KeyChangeSchema
}

func (s *keyChangeStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	_, err = db.Exec(KeyChangeSchema)
	if err != nil {
		return
	}
	if s.insertKeyStreamStmt, err = db.Prepare(insertKeyStreamSQL); err != nil {
		return
	}
	if s.selectHistoryKeyStreamStmt, err = db.Prepare(selectHistoryKeyStreamSQL); err != nil {
		return
	}
	return
}

func (s *keyChangeStatements) insertKeyStream(
	ctx context.Context,
	id int64, changedUserID string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncKeyStreamInsertKey
		update.SyncDBEvents.SyncKeyStreamInsert = &dbtypes.SyncKeyStreamInsert{
			ID:            id,
			ChangedUserID: changedUserID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(changedUserID)))
		s.db.WriteDBEventWithTbl(&update, "syncapi_key_change_stream")
		return nil
	} else {
		_, err = s.insertKeyStreamStmt.ExecContext(ctx, id, changedUserID)
		return
	}
}

func (s *keyChangeStatements) onInsertKeyStream(
	ctx context.Context,
	id int64, changedUserID string,
) (err error) {
	_, err = s.insertKeyStreamStmt.ExecContext(ctx, id, changedUserID)
	return
}

func (s *keyChangeStatements) selectHistoryStream(
	ctx context.Context,
	users []string,
) (streams []types.KeyChangeStream, offset []int64, err error) {
	rows, err := s.selectHistoryKeyStreamStmt.QueryContext(ctx, pq.Array(users))
	if err != nil {
		log.Errorf("keyChangeStatements.selectHistoryStream err: %v", err)
		return
	}

	streams = []types.KeyChangeStream{}
	offset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.KeyChangeStream
		var streamPos int64
		if err := rows.Scan(&streamPos, &stream.ChangedUserID); err != nil {
			return nil, nil, err
		}
		streams = append(streams, stream)
		offset = append(offset, streamPos)
	}
	return
}
