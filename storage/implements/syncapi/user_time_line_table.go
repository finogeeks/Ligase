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
	"github.com/finogeeks/ligase/model/types"
	"github.com/lib/pq"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const userTimeLineSchema = `
CREATE TABLE IF NOT EXISTS syncapi_user_time_line (
	  id BIGINT,
      user_id TEXT NOT NULL,
      room_id TEXT NOT NULL,
      event_nid BIGINT NOT NULL,
      event_offset BIGINT NOT NULL,
	  room_state TEXT NOT NULL,
	  ts BIGINT NOT NULL,
      CONSTRAINT syncapi_user_time_line_unique UNIQUE (id, user_id)
);
CREATE INDEX IF NOT EXISTS syncapi_user_time_line_user_idx ON syncapi_user_time_line(user_id);
CREATE INDEX IF NOT EXISTS syncapi_user_time_line_room_idx ON syncapi_user_time_line(room_id);
CREATE INDEX IF NOT EXISTS syncapi_user_time_line_evoffset_idx ON syncapi_user_time_line(user_id, event_offset);
CREATE INDEX IF NOT EXISTS syncapi_user_time_line_user_id_desc_null_last_idx ON syncapi_user_time_line(user_id, id DESC NULLS LAST)
`

const insertUserTimeLineSQL = "" +
	"INSERT INTO syncapi_user_time_line (id, room_id, event_nid, user_id, room_state, ts, event_offset) VALUES ($1, $2, $3, $4, $5, $6, $7)" +
	" ON CONFLICT DO NOTHING"

const selectHistoryUserTimeLineSQL = "" +
	"SELECT id, user_id, room_id, event_offset, room_state FROM syncapi_user_time_line WHERE user_id = $1 AND id > $2 order by id asc limit $3"

const selectHistorySQL = "" +
	"SELECT id, user_id, room_id, event_offset, room_state FROM syncapi_user_time_line WHERE user_id = $1 order by id desc NULLS LAST limit $2"

const selectUserTimeLineMinPosSQL = "" +
	"SELECT COALESCE(id, -1) AS min_pos FROM syncapi_user_time_line WHERE user_id = $1 ORDER BY id ASC NULLS FIRST LIMIT 1"

const selectOffsetSQL = "" +
	"SELECT id, user_id, room_id, event_offset, room_state FROM syncapi_user_time_line WHERE user_id = $1 AND event_offset = ANY($2)"

type userTimeLineStatements struct {
	db                           *Database
	insertUserTimeLineStmt       *sql.Stmt
	selectUserTimeLineStmt       *sql.Stmt
	selectUserTimeLineMinPosStmt *sql.Stmt
	selectHistoryStmt            *sql.Stmt
	selectOffsetStmt             *sql.Stmt
}

func (s *userTimeLineStatements) getSchema() string {
	return userTimeLineSchema
}

func (s *userTimeLineStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if s.insertUserTimeLineStmt, err = db.Prepare(insertUserTimeLineSQL); err != nil {
		return
	}
	if s.selectUserTimeLineStmt, err = db.Prepare(selectHistoryUserTimeLineSQL); err != nil {
		return
	}
	if s.selectUserTimeLineMinPosStmt, err = db.Prepare(selectUserTimeLineMinPosSQL); err != nil {
		return
	}
	if s.selectHistoryStmt, err = db.Prepare(selectHistorySQL); err != nil {
		return
	}
	if s.selectOffsetStmt, err = db.Prepare(selectOffsetSQL); err != nil {
		return
	}
	return
}

func (s *userTimeLineStatements) selectUserTimeLineEvents(
	ctx context.Context,
	userID string,
	id int64,
	limit int,
) (events []syncapitypes.UserTimeLineStream, err error) {
	rows, err := s.selectUserTimeLineStmt.QueryContext(ctx, userID, id, limit)
	if err != nil {
		log.Errorf("userTimeLineStatements.selectUserTimeLineEvents err: %v", err)
		return
	}

	var evs []syncapitypes.UserTimeLineStream
	defer rows.Close()
	for rows.Next() {
		var ev syncapitypes.UserTimeLineStream
		if err := rows.Scan(&ev.Offset, &ev.UserID, &ev.RoomID, &ev.RoomOffset, &ev.RoomState); err != nil {
			return nil, err
		}

		evs = append(evs, ev)
	}
	return evs, nil
}

func (s *userTimeLineStatements) selectUserTimeLineMinPos(
	ctx context.Context,
	userID string,
) (int64, error) {
	rows, err := s.selectUserTimeLineMinPosStmt.QueryContext(ctx, userID)
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

func (s *userTimeLineStatements) insertUserTimeLine(
	ctx context.Context,
	id int64,
	roomID string,
	evtNID int64,
	userID,
	roomState string,
	ts,
	eventOffset int64,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncUserTimeLineInsertKey
		update.SyncDBEvents.SyncUserTimeLineInsert = &dbtypes.SyncUserTimeLineInsert{
			ID:          id,
			RoomID:      roomID,
			EventNID:    evtNID,
			UserID:      userID,
			RoomState:   roomState,
			Ts:          ts,
			EventOffset: eventOffset,
		}
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_user_time_line")
		return nil
	} else {
		_, err = s.insertUserTimeLineStmt.ExecContext(ctx, id, roomID, evtNID, userID, roomState, ts, eventOffset)
		return
	}
}

func (s *userTimeLineStatements) onInsertUserTimeLine(
	ctx context.Context,
	id int64,
	roomID string,
	evtNID int64,
	userID,
	roomState string,
	ts,
	eventOffset int64,
) (err error) {
	_, err = s.insertUserTimeLineStmt.ExecContext(ctx, id, roomID, evtNID, userID, roomState, ts, eventOffset)
	return
}

func (s *userTimeLineStatements) selectUserTimeLineHistory(
	ctx context.Context,
	userID string,
	limit int,
) (events []syncapitypes.UserTimeLineStream, err error) {
	rows, err := s.selectHistoryStmt.QueryContext(ctx, userID, limit)
	if err != nil {
		log.Errorf("userTimeLineStatements.selectUserTimeLineHistory err: %v", err)
		return
	}

	var evs []syncapitypes.UserTimeLineStream
	defer rows.Close()
	for rows.Next() {
		var ev syncapitypes.UserTimeLineStream
		if err := rows.Scan(&ev.Offset, &ev.UserID, &ev.RoomID, &ev.RoomOffset, &ev.RoomState); err != nil {
			return nil, err
		}

		evs = append(evs, ev)
	}
	return evs, nil
}

func (s *userTimeLineStatements) selectUserTimeLineOffset(
	ctx context.Context,
	userID string,
	roomOffsets []int64,
) (events []syncapitypes.UserTimeLineStream, err error) {
	bs := time.Now().UnixNano() / 1000000
	rows, err := s.selectOffsetStmt.QueryContext(ctx, userID, pq.Array(roomOffsets))
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("load db failed userTimeLineStatements.selectUserTimeLineOffset user:%s roomOffsets len:%d spend:%d err: %v", userID, len(roomOffsets), spend, err)
		return
	}
	if spend > types.DB_EXCEED_TIME {
		log.Errorf("load db exceed %d ms userTimeLineStatements.selectUserTimeLineOffset user %s roomOffsets len %d", userID, len(roomOffsets), spend)
	} else {
		log.Infof("load db succ userTimeLineStatements.selectUserTimeLineOffset user %s roomOffsets len %d spend:%d ms", userID, len(roomOffsets), spend)
	}
	var evs []syncapitypes.UserTimeLineStream
	defer rows.Close()
	for rows.Next() {
		var ev syncapitypes.UserTimeLineStream
		if err := rows.Scan(&ev.Offset, &ev.UserID, &ev.RoomID, &ev.RoomOffset, &ev.RoomState); err != nil {
			return nil, err
		}

		evs = append(evs, ev)
	}
	return evs, nil
}
