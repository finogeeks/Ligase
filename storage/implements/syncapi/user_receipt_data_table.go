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
	log "github.com/finogeeks/ligase/skunkworks/log"
)

const userReceiptDataSchema = `
-- Stores the types of account data that a user set has globally and in each room
-- and the stream ID when that type was last updated.
CREATE TABLE IF NOT EXISTS syncapi_user_receipt_data (
	evt_offset BIGINT,
    room_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	content TEXT NOT NULL,
 
    CONSTRAINT syncapi_user_receipt_data_unique UNIQUE (user_id, room_id)
);
`

const insertUserReceiptDataSQL = "" +
	"INSERT INTO syncapi_user_receipt_data (evt_offset, room_id, user_id, content) VALUES ($1, $2, $3, $4)" +
	" ON CONFLICT ON CONSTRAINT syncapi_user_receipt_data_unique" +
	" DO UPDATE SET content = EXCLUDED.content, evt_offset = EXCLUDED.evt_offset"

const selectHistoryUserReceiptDataSQL = "" +
	"SELECT evt_offset, content FROM syncapi_user_receipt_data WHERE user_id = $1 and room_id = $2"

type userReceiptDataStatements struct {
	db                               *Database
	insertUserReceiptDataStmt        *sql.Stmt
	selectHistoryUserReceiptDataStmt *sql.Stmt
}

func (s *userReceiptDataStatements) getSchema() string {
	return userReceiptDataSchema
}

func (s *userReceiptDataStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if s.insertUserReceiptDataStmt, err = db.Prepare(insertUserReceiptDataSQL); err != nil {
		return
	}
	if s.selectHistoryUserReceiptDataStmt, err = db.Prepare(selectHistoryUserReceiptDataSQL); err != nil {
		return
	}
	return
}

func (s *userReceiptDataStatements) insertUserReceiptData(
	ctx context.Context, roomID, userID, content string, evtOffset int64,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncUserReceiptInsertKey
		update.SyncDBEvents.SyncUserReceiptInsert = &dbtypes.SyncUserReceiptInsert{
			UserID:    userID,
			RoomID:    roomID,
			Content:   content,
			EvtOffset: evtOffset,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		s.db.WriteDBEventWithTbl(&update, "syncapi_user_receipt_data")
		return nil
	} else {
		_, err = s.insertUserReceiptDataStmt.ExecContext(ctx, evtOffset, roomID, userID, content)
		return
	}
}

func (s *userReceiptDataStatements) onInsertUserReceiptData(
	ctx context.Context, roomID, userID, content string, evtOffset int64,
) (pos int64, err error) {
	_, err = s.insertUserReceiptDataStmt.ExecContext(ctx, evtOffset, roomID, userID, content)
	return
}

func (s *userReceiptDataStatements) selectHistoryStream(
	ctx context.Context, roomID, userID string,
) (evtOffset int64, content []byte, err error) {
	rows, err := s.selectHistoryUserReceiptDataStmt.QueryContext(ctx, userID, roomID)
	if err != nil {
		log.Errorf("userReceiptDataStatements.selectHistoryStream err: %v", err)
		return
	}

	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&evtOffset, &content); err != nil {
			return int64(0), nil, err
		}
	}
	return
}
