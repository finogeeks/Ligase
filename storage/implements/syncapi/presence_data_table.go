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

const presencetDataStreamSchema = `
CREATE TABLE IF NOT EXISTS syncapi_presence_data_stream (
	id BIGINT,
    user_id TEXT NOT NULL,
	content TEXT NOT NULL,

    CONSTRAINT syncapi_presence_data_stream_unique UNIQUE (user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS syncapi_presence_data_stream_user_id ON syncapi_presence_data_stream(user_id);
CREATE INDEX IF NOT EXISTS syncapi_presence_data_stream_id_idx ON syncapi_presence_data_stream(id ASC);
`

const insertPresenceDataStreamSQL = "" +
	"INSERT INTO syncapi_presence_data_stream (id, user_id, content) VALUES ($1, $2, $3)" +
	" ON CONFLICT ON CONSTRAINT syncapi_presence_data_stream_unique" +
	" DO UPDATE SET id = EXCLUDED.id, content = EXCLUDED.content" +
	" RETURNING id"

const selectHistoryPresenceDataStreamSQL = "" +
	"SELECT id, user_id, content  FROM syncapi_presence_data_stream" +
	" ORDER BY id ASC limit $1 offset $2"

const selectUserPresenceDataStreamSQL = "" +
	"SELECT id, user_id, content  FROM syncapi_presence_data_stream WHERE user_id = any($1) ORDER BY id ASC"

type presenceDataStreamStatements struct {
	db                                  *Database
	insertPresenceDataStreamStmt        *sql.Stmt
	selectHistoryPresenceDataStreamStmt *sql.Stmt
	selectUserPresenceDataStreamStmt    *sql.Stmt
}

func (s *presenceDataStreamStatements) getSchema() string {
	return presencetDataStreamSchema
}

func (s *presenceDataStreamStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if s.insertPresenceDataStreamStmt, err = db.Prepare(insertPresenceDataStreamSQL); err != nil {
		return
	}
	if s.selectHistoryPresenceDataStreamStmt, err = db.Prepare(selectHistoryPresenceDataStreamSQL); err != nil {
		return
	}
	if s.selectUserPresenceDataStreamStmt, err = db.Prepare(selectUserPresenceDataStreamSQL); err != nil {
		return
	}
	return
}

func (s *presenceDataStreamStatements) insertPresenceDataStream(
	ctx context.Context, id int64,
	userID, content string,
) (pos int64, err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncPresenceInsertKey
		update.SyncDBEvents.SyncPresenceInsert = &dbtypes.SyncPresenceInsert{
			ID:      id,
			UserID:  userID,
			Content: content,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_presence_data_stream")
		return id, nil
	} else {
		err = s.insertPresenceDataStreamStmt.QueryRowContext(ctx, id, userID, content).Scan(&pos)
		return
	}
}

func (s *presenceDataStreamStatements) onInsertPresenceDataStream(
	ctx context.Context, id int64,
	userID, content string,
) (pos int64, err error) {
	err = s.insertPresenceDataStreamStmt.QueryRowContext(ctx, id, userID, content).Scan(&pos)
	return
}

func (s *presenceDataStreamStatements) selectHistoryStream(
	ctx context.Context, limit, offset int,
) (streams []types.PresenceStream, resultOffset []int64, err error) {
	rows, err := s.selectHistoryPresenceDataStreamStmt.QueryContext(ctx, limit, offset)
	if err != nil {
		log.Errorf("presenceDataStreamStatements.selectHistoryStream err: %v", err)
		return
	}

	streams = []types.PresenceStream{}
	resultOffset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.PresenceStream
		var streamPos int64
		if err := rows.Scan(&streamPos, &stream.UserID, &stream.Content); err != nil {
			return nil, nil, err
		}

		streams = append(streams, stream)
		resultOffset = append(resultOffset, streamPos)
	}
	return
}

func (s *presenceDataStreamStatements) selectUserPresenceStream(
	ctx context.Context, users []string,
) (streams []types.PresenceStream, resultOffset []int64, err error) {
	rows, err := s.selectUserPresenceDataStreamStmt.QueryContext(ctx, pq.Array(users))
	if err != nil {
		log.Errorf("presenceDataStreamStatements.selectUserPresenceStream err: %v", err)
		return
	}

	streams = []types.PresenceStream{}
	resultOffset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.PresenceStream
		var streamPos int64
		if err := rows.Scan(&streamPos, &stream.UserID, &stream.Content); err != nil {
			return nil, nil, err
		}

		streams = append(streams, stream)
		resultOffset = append(resultOffset, streamPos)
	}
	return
}
