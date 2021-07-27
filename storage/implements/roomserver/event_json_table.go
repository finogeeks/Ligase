// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package roomserver

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common/encryption"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/lib/pq"
)

const eventJSONSchema = `
-- Stores the JSON for each event. This kept separate from the main events
-- table to keep the rows in the main events table small.
CREATE TABLE IF NOT EXISTS roomserver_event_json (
    -- Local numeric ID for the event.
    event_nid BIGINT NOT NULL PRIMARY KEY,
    -- The JSON for the event.
    -- Stored as TEXT because this should be valid UTF-8.
    -- Not stored as a JSONB because we always just pull the entire event
    -- so there is no point in postgres parsing it.
    -- Not stored as JSON because we already validate the JSON in the server
    -- so there is no point in postgres validating it.
    -- TODO: Should we be compressing the events with Snappy or DEFLATE?
    event_json TEXT NOT NULL
);

-- mirror table for debug, plaintext storage
CREATE TABLE IF NOT EXISTS roomserver_event_json_mirror (
    -- Local numeric ID for the event.
    event_nid BIGINT NOT NULL PRIMARY KEY,
    event_json TEXT NOT NULL
);
`

const insertEventJSONSQL = "" +
	"INSERT INTO roomserver_event_json (event_nid, event_json) VALUES ($1, $2)" +
	" ON CONFLICT DO NOTHING"

const insertEventJSONSQLMirror = "" +
	"INSERT INTO roomserver_event_json_mirror (event_nid, event_json) VALUES ($1, $2)" +
	" ON CONFLICT DO NOTHING"

// Bulk event JSON lookup by numeric event ID.
// Sort by the numeric event ID.
// This means that we can use binary search to lookup by numeric event ID.
const bulkSelectEventJSONSQL = "" +
	"SELECT t.event_nid, t.event_json, m.event_type_id FROM roomserver_event_json t inner join roomserver_events m on t.event_nid = m.event_nid " +
	" WHERE t.event_nid = ANY($1)" +
	" ORDER BY t.event_nid ASC"

const selectEventsCountSQL = "" +
	"SELECT count(1) FROM roomserver_event_json"

const selectRoomEventsSQL = "" +
	"select t.event_json, m.event_type_id from roomserver_event_json t inner join roomserver_events m on t.event_nid = m.event_nid where m.room_nid = $1 order by t.event_nid asc "

const selectRoomEventsWithLimitSQL = "" +
	"select t.event_nid, t.event_json, m.event_type_id from roomserver_event_json t inner join roomserver_events m on t.event_nid = m.event_nid where m.room_nid = $1 order by t.event_nid asc limit $2 offset $3"

// const selectMsgEventsSQL = "" +
// 	"SELECT t.event_nid, t.event_json FROM roomserver_event_json t inner join roomserver_events m on t.event_nid = m.event_nid WHERE m.event_type_id = 'm.room.message' AND t.event_json LIKE '%content%' ORDER BY t.event_nid ASC LIMIT $1 OFFSET $2"

const selectMsgEventsSQL = "" +
	"SELECT t.event_nid, t.event_json FROM roomserver_event_json t inner join roomserver_events m on t.event_nid = m.event_nid WHERE t.event_nid >= $1 AND m.event_type_id = 'm.room.message' AND t.event_json LIKE '%content%' ORDER BY t.event_nid ASC LIMIT $2"

const selectMsgEventsCountSQL = "" +
	"SELECT count(*), COALESCE(MIN(t.event_nid), 0) FROM roomserver_event_json t inner join roomserver_events m on t.event_nid = m.event_nid WHERE m.event_type_id = 'm.room.message' AND t.event_json LIKE '%content%'"

const updateMsgEventSQL = "" +
	"UPDATE roomserver_event_json SET event_json = $1 WHERE event_nid = $2"

const selectRoomEventByNIDSQL = "" +
	"SELECT event_json FROM roomserver_event_json WHERE event_nid = $1"

type eventJSONStatements struct {
	db                                 *Database
	insertEventJSONStmt                *sql.Stmt
	insertEventJSONStmtMirror          *sql.Stmt
	bulkSelectEventJSONStmt            *sql.Stmt
	selectMaxEventNIDStmt              *sql.Stmt
	selectEventsByLowAndUpperBoundStmt *sql.Stmt
	selectEventsCountStmt              *sql.Stmt
	selectRoomEventsStmt               *sql.Stmt
	selectRoomEventsWithLimitStmt      *sql.Stmt
	selectMsgEventsStmt                *sql.Stmt
	selectMsgEventsCountStmt           *sql.Stmt
	updateMsgEventStmt                 *sql.Stmt
	selectRoomEventByNIDStmt           *sql.Stmt
}

func (s *eventJSONStatements) getSchema() string {
	return eventJSONSchema
}

func (s *eventJSONStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	return statementList{
		{&s.insertEventJSONStmt, insertEventJSONSQL},
		{&s.insertEventJSONStmtMirror, insertEventJSONSQLMirror},
		{&s.bulkSelectEventJSONStmt, bulkSelectEventJSONSQL},
		{&s.selectEventsCountStmt, selectEventsCountSQL},
		{&s.selectRoomEventsStmt, selectRoomEventsSQL},
		{&s.selectRoomEventsWithLimitStmt, selectRoomEventsWithLimitSQL},
		{&s.selectMsgEventsStmt, selectMsgEventsSQL},
		{&s.selectMsgEventsCountStmt, selectMsgEventsCountSQL},
		{&s.updateMsgEventStmt, updateMsgEventSQL},
		{&s.selectRoomEventByNIDStmt, selectRoomEventByNIDSQL},
	}.prepare(db)
}

//var event_json_cache = sync.Map{} //make(map[types.EventNID][]byte)

func (s *eventJSONStatements) insertEventJSON(
	ctx context.Context, eventNID int64, eventJSON []byte, eventType string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventJsonInsertKey
		update.SetUid(eventNID)
		update.RoomDBEvents.EventJsonInsert = &dbtypes.EventJsonInsert{
			EventNid:  eventNID,
			EventJson: eventJSON,
			EventType: eventType,
		}
		update.SetUid(eventNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_event_json")
		return nil
	}

	return s.insertEventJSONRaw(ctx, eventNID, eventJSON, eventType)
}

func (s *eventJSONStatements) insertEventJSONRaw(
	ctx context.Context, eventNID int64, eventJSON []byte, eventType string,
) (err error) {
	if encryption.CheckCrypto(eventType) {
		_, err = s.insertEventJSONStmt.ExecContext(ctx, eventNID, encryption.Encrypt(eventJSON))
		if encryption.CheckMirror(eventType) {
			_, err = s.insertEventJSONStmtMirror.ExecContext(ctx, eventNID, eventJSON)
		}
	} else {
		_, err = s.insertEventJSONStmt.ExecContext(ctx, eventNID, eventJSON)
	}

	return err
}

func (s *eventJSONStatements) bulkEvents(
	ctx context.Context, eventNIDs []int64,
) ([]*gomatrixserverlib.Event, []int64, error) {
	rows, err := s.bulkSelectEventJSONStmt.QueryContext(ctx, pq.Int64Array(eventNIDs))
	if err != nil {
		return nil, nil, err
	}

	var evs []*gomatrixserverlib.Event
	var ids []int64
	defer rows.Close()
	for rows.Next() {
		var eventNID int64
		var eventBytes []byte
		var eventType string
		var ev gomatrixserverlib.Event
		if err := rows.Scan(&eventNID, &eventBytes, &eventType); err != nil {
			return nil, nil, err
		}

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			ev, _ = gomatrixserverlib.NewEventFromUntrustedJSON(dec)
		} else {
			ev, _ = gomatrixserverlib.NewEventFromUntrustedJSON(eventBytes)
		}

		evs = append(evs, &ev)
		ids = append(ids, eventNID)
	}

	return evs, ids, nil
}

func (s *eventJSONStatements) selectEventsTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectEventsCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}

func (s *eventJSONStatements) getRoomEvents(ctx context.Context, roomNID int64) ([][]byte, error) {
	rows, err := s.selectRoomEventsStmt.QueryContext(ctx, roomNID)
	if err != nil {
		return nil, err
	}
	var result [][]byte
	defer rows.Close()
	for rows.Next() {
		var eventBytes []byte
		var eventType string
		if err := rows.Scan(&eventBytes, &eventType); err != nil {
			return nil, err
		}

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			result = append(result, dec)
		} else {
			result = append(result, eventBytes)
		}
	}
	return result, nil
}

func (s *eventJSONStatements) getRoomEventsWithLimit(ctx context.Context, roomNID int64, limit, offset int) ([]int64, [][]byte, error) {
	rows, err := s.selectRoomEventsWithLimitStmt.QueryContext(ctx, roomNID, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	var result [][]byte
	var nids []int64
	defer rows.Close()
	for rows.Next() {
		var eventBytes []byte
		var nid int64
		var eventType string
		if err := rows.Scan(&nid, &eventBytes, &eventType); err != nil {
			return nil, nil, err
		}

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			result = append(result, dec)
		} else {
			result = append(result, eventBytes)
		}

		nids = append(nids, nid)
	}
	return nids, result, nil
}

func (s *eventJSONStatements) selectMsgEventsMigration(ctx context.Context, limit, offset int64) ([]int64, [][]byte, error) {
	var rows *sql.Rows
	var err error

	rows, err = s.selectMsgEventsStmt.QueryContext(ctx, offset, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := [][]byte{}
	var ids []int64
	for rows.Next() {
		var eventBytes []byte
		var id int64
		if err = rows.Scan(&id, &eventBytes); err != nil {
			return nil, nil, err
		}

		ids = append(ids, id)
		result = append(result, eventBytes)
	}
	return ids, result, nil
}

func (s *eventJSONStatements) selectMsgEventsTotalMigration(
	ctx context.Context,
) (count int, minEventNID int64, err error) {
	err = s.selectMsgEventsCountStmt.QueryRowContext(ctx).Scan(&count, &minEventNID)
	return
}

func (s *eventJSONStatements) updateMsgEventMigration(
	ctx context.Context,
	id int64,
	EncryptedEventBytes []byte,
) error {
	_, err := s.updateMsgEventStmt.ExecContext(
		ctx, EncryptedEventBytes, id,
	)
	return err
}

func (s *eventJSONStatements) selectRoomEventByNID(ctx context.Context, eventNID int64) ([]byte, error) {
	rows, err := s.selectRoomEventByNIDStmt.QueryContext(ctx, eventNID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var eventBytes []byte
		if err := rows.Scan(&eventBytes); err != nil {
			return nil, err
		}
		return eventBytes, nil
	}
	return nil, nil
}
