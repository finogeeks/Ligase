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

const receiptDataStreamSchema = `
-- This sequence is shared between all the tables generated from kafka logs.
-- CREATE SEQUENCE IF NOT EXISTS syncapi_stream_id;

-- Stores the types of account data that a user set has globally and in each room
-- and the stream ID when that type was last updated.
CREATE TABLE IF NOT EXISTS syncapi_receipt_data_stream (
	id BIGINT PRIMARY KEY,
	evt_offset BIGINT,
    room_id TEXT NOT NULL,
	content TEXT NOT NULL,
 
    CONSTRAINT syncapi_receipt_data_stream_unique UNIQUE (room_id, id)
);

`

const insertReceiptDataStreamSQL = "" +
	"INSERT INTO syncapi_receipt_data_stream (id, evt_offset, room_id, content) VALUES ($1, $2, $3, $4)" +
	" ON CONFLICT DO NOTHING"

const selectHistoryReceiptDataStreamSQL = "" +
	"SELECT id, evt_offset, room_id, content  FROM syncapi_receipt_data_stream WHERE room_id = $1" +
	" ORDER BY id ASC"

const deleteLatestReceiptDataStreamSQL = "" +
	"DELETE FROM syncapi_receipt_data_stream WHERE room_id = $1 AND evt_offset < $2"

const selectRoomReceiptLatestStreamsSQL = "" +
	"SELECT max(id), room_id FROM syncapi_receipt_data_stream WHERE room_id = ANY($1) group by room_id"

const selectUserMaxReceiptPosSQL = "" +
	"SELECT COALESCE(MAX(id), 0) from syncapi_receipt_data_stream where room_id  = any(select room_id from syncapi_current_room_state where type = 'm.room.member' and state_key=$1 and membership = 'join')"

type receiptDataStreamStatements struct {
	db                                 *Database
	insertReceiptDataStreamStmt        *sql.Stmt
	selectHistoryReceiptDataStreamStmt *sql.Stmt
	deleteLatestReceiptDataStreamStmt  *sql.Stmt
	selectRoomLatestStreamsStmt        *sql.Stmt
	selectUserMaxReceiptPosStmt        *sql.Stmt
}

func (s *receiptDataStreamStatements) getSchema() string {
	return receiptDataStreamSchema
}

func (s *receiptDataStreamStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if s.insertReceiptDataStreamStmt, err = db.Prepare(insertReceiptDataStreamSQL); err != nil {
		return
	}
	if s.selectHistoryReceiptDataStreamStmt, err = db.Prepare(selectHistoryReceiptDataStreamSQL); err != nil {
		return
	}
	if s.deleteLatestReceiptDataStreamStmt, err = db.Prepare(deleteLatestReceiptDataStreamSQL); err != nil {
		return
	}
	if s.selectRoomLatestStreamsStmt, err = db.Prepare(selectRoomReceiptLatestStreamsSQL); err != nil {
		return
	}
	if s.selectUserMaxReceiptPosStmt, err = db.Prepare(selectUserMaxReceiptPosSQL); err != nil {
		return
	}
	return
}

func (s *receiptDataStreamStatements) insertReceiptDataStream(
	ctx context.Context, id, evtOffset int64,
	roomID, content string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncReceiptInsertKey
		update.SyncDBEvents.SyncReceiptInsert = &dbtypes.SyncReceiptInsert{
			ID:        id,
			EvtOffset: evtOffset,
			RoomID:    roomID,
			Content:   content,
		}
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		s.db.WriteDBEventWithTbl(&update, "syncapi_receipt_data_stream")
		return nil
	} else {
		s.deleteLatestReceiptDataStreamStmt.ExecContext(ctx, roomID, evtOffset)
		_, err = s.insertReceiptDataStreamStmt.ExecContext(ctx, id, evtOffset, roomID, content)
		return
	}
}

func (s *receiptDataStreamStatements) onInsertReceiptDataStream(
	ctx context.Context, id, evtOffset int64,
	roomID, content string,
) (err error) {
	s.deleteLatestReceiptDataStreamStmt.ExecContext(ctx, roomID, evtOffset)
	_, err = s.insertReceiptDataStreamStmt.ExecContext(ctx, id, evtOffset, roomID, content)
	return
}

func (s *receiptDataStreamStatements) selectRoomLastOffsets(
	ctx context.Context,
	roomIDs []string,
) (map[string]int64, error) {
	rows, err := s.selectRoomLatestStreamsStmt.QueryContext(ctx, pq.StringArray(roomIDs))
	if err != nil {
		log.Errorf("receiptDataStreamStatements.selectRoomLastOffsets err: %v", err)
		return nil, err
	}
	defer rows.Close()

	var offset int64
	var roomID string
	result := make(map[string]int64)
	for rows.Next() {
		if err = rows.Scan(
			&offset, &roomID,
		); err != nil {
			return nil, err
		}

		result[roomID] = offset
	}
	return result, nil
}

func (s *receiptDataStreamStatements) selectHistoryStream(
	ctx context.Context, roomID string,
) (streams []types.ReceiptStream, offset []int64, err error) {
	rows, err := s.selectHistoryReceiptDataStreamStmt.QueryContext(ctx, roomID)
	if err != nil {
		log.Errorf("receiptDataStreamStatements.selectHistoryStream err: %v", err)
		return
	}
	streams = []types.ReceiptStream{}
	offset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.ReceiptStream
		var streamPos int64
		if err := rows.Scan(&streamPos, &stream.ReceiptOffset, &stream.RoomID, &stream.Content); err != nil {
			return nil, nil, err
		}

		streams = append(streams, stream)
		offset = append(offset, streamPos)
	}
	return
}

func (s *receiptDataStreamStatements) selectUserMaxPos(
	ctx context.Context,
	userID string,
) (int64, error) {
	rows, err := s.selectUserMaxReceiptPosStmt.QueryContext(ctx, userID)
	if err != nil {
		log.Errorf("receiptDataStreamStatements.selectUserMaxPos err: %v\n", err)
		return -1, err
	}

	defer rows.Close()
	for rows.Next() {
		var streamPos int64
		if err := rows.Scan(&streamPos); err != nil {
			return -1, err
		} else {
			return streamPos, nil
		}
	}
	return -1, err
}
