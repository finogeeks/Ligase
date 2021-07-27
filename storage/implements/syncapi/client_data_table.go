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
)

const clientDataStreamSchema = `
-- This sequence is shared between all the tables generated from kafka logs.
-- CREATE SEQUENCE IF NOT EXISTS syncapi_stream_id;

-- Stores the types of account data that a user set has globally and in each room
-- and the stream ID when that type was last updated.
CREATE TABLE IF NOT EXISTS syncapi_client_data_stream (
    -- An incrementing ID which denotes the position in the log that this event resides at.
	-- id BIGINT PRIMARY KEY DEFAULT nextval('syncapi_stream_id'),
	id BIGINT PRIMARY KEY,
    -- ID of the user the data belongs to
    user_id TEXT NOT NULL, 
    room_id TEXT,
    -- Type of the data
    data_type TEXT,
	stream_type TEXT NOT NULL,
 
    CONSTRAINT syncapi_client_data_stream_unique UNIQUE (user_id, room_id, data_type, stream_type)
);

`

const insertClientDataStreamSQL = "" +
	"INSERT INTO syncapi_client_data_stream (id, user_id, room_id, data_type, stream_type) VALUES ($1, $2, $3, $4, $5)" +
	" ON CONFLICT ON CONSTRAINT syncapi_client_data_stream_unique" +
	" DO UPDATE SET id = EXCLUDED.id" +
	" RETURNING id"

const selectHistoryClientDataStreamSQL = "" +
	"SELECT id, user_id, room_id, data_type, stream_type  FROM syncapi_client_data_stream" +
	" WHERE user_id = $1 AND stream_type != ''" +
	" ORDER BY id DESC LIMIT $2"

type clientDataStreamStatements struct {
	db                                *Database
	insertClientDataStreamStmt        *sql.Stmt
	selectHistoryClientDataStreamStmt *sql.Stmt
}

func (s *clientDataStreamStatements) getSchema() string {
	return clientDataStreamSchema
}

func (s *clientDataStreamStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if s.insertClientDataStreamStmt, err = db.Prepare(insertClientDataStreamSQL); err != nil {
		return
	}
	if s.selectHistoryClientDataStreamStmt, err = db.Prepare(selectHistoryClientDataStreamSQL); err != nil {
		return
	}
	return
}

func (s *clientDataStreamStatements) insertClientDataStream(
	ctx context.Context, id int64,
	userID, roomID, dataType, streamType string,
) (pos int64, err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncClientDataInsertKey
		update.SyncDBEvents.SyncClientDataInsert = &dbtypes.SyncClientDataInsert{
			ID:         id,
			UserID:     userID,
			RoomID:     roomID,
			DataType:   dataType,
			StreamType: streamType,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		s.db.WriteDBEventWithTbl(&update, "syncapi_client_data_stream")
		return id, nil
	} else {
		err = s.insertClientDataStreamStmt.QueryRowContext(ctx, id, userID, roomID, dataType, streamType).Scan(&pos)
		return
	}
}

func (s *clientDataStreamStatements) onInsertClientDataStream(
	ctx context.Context, id int64,
	userID, roomID, dataType, streamType string,
) (pos int64, err error) {
	err = s.insertClientDataStreamStmt.QueryRowContext(ctx, id, userID, roomID, dataType, streamType).Scan(&pos)
	return
}

func (s *clientDataStreamStatements) selectHistoryStream(
	ctx context.Context,
	userID string,
	limit int,
) (streams []types.ActDataStreamUpdate, offset []int64, err error) {
	rows, err := s.selectHistoryClientDataStreamStmt.QueryContext(ctx, userID, limit)
	if err != nil {
		log.Errorf("clientDataStreamStatements.selectHistoryStream err: %v", err)
		return
	}
	streams = []types.ActDataStreamUpdate{}
	offset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.ActDataStreamUpdate
		var streamPos int64
		if err := rows.Scan(&streamPos, &stream.UserID, &stream.RoomID, &stream.DataType, &stream.StreamType); err != nil {
			return nil, nil, err
		}

		streams = append(streams, stream)
		offset = append(offset, streamPos)
	}
	return
}
