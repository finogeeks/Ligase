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

package syncapi

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/encryption"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/lib/pq"
)

const outputRoomEventsSchema = `
-- This sequence is shared between all the tables generated from kafka logs.
--CREATE SEQUENCE IF NOT EXISTS syncapi_stream_id;

-- Stores output room events received from the roomserver.
CREATE TABLE IF NOT EXISTS syncapi_output_room_events (
    -- An incrementing ID which denotes the position in the log that this event resides at.
    -- NB: 'serial' makes no guarantees to increment by 1 every time, only that it increments.
    --     This isn't a problem for us since we just want to order by this field.
	-- id BIGINT PRIMARY KEY DEFAULT nextval('syncapi_stream_id'),
	id BIGINT PRIMARY KEY,
    -- The event ID for the event
    event_id TEXT NOT NULL,
    domain_offset BIGINT NOT NULL default  -1,
    origin_server_ts BIGINT NOT NULL  default  -1,
    domain TEXT NOT NULL DEFAULT '',
    -- The state event type e.g 'm.room.member'
    type TEXT NOT NULL,
    -- The 'room_id' key for the event.
    room_id TEXT NOT NULL,
    -- The JSON for the event. Stored as TEXT because this should be valid UTF-8.
    event_json TEXT NOT NULL,
    -- A list of event IDs which represent a delta of added/removed room state. This can be NULL
    -- if there is no delta.
    add_state_ids TEXT[],
    remove_state_ids TEXT[],
    device_id TEXT,  -- The local device that sent the event, if any
    transaction_id TEXT,  -- The transaction id used to send the event, if any
    depth BIGINT NOT NULL default -1,
    CONSTRAINT syncapi_output_room_events_unique UNIQUE (event_id, room_id)
);
-- for event selection
CREATE INDEX IF NOT EXISTS syncapi_output_room_visibility ON syncapi_output_room_events (type,room_id,device_id);
CREATE INDEX IF NOT EXISTS syncapi_roomid_id_desc on syncapi_output_room_events(room_id, id desc);
CREATE INDEX IF NOT EXISTS syncapi_load_room_history ON syncapi_output_room_events (id,room_id);

-- mirror table for debug, plaintext storage
CREATE TABLE IF NOT EXISTS syncapi_output_room_events_mirror (
    -- An incrementing ID which denotes the position in the log that this event resides at.
    -- NB: 'serial' makes no guarantees to increment by 1 every time, only that it increments.
    --     This isn't a problem for us since we just want to order by this field.
	-- id BIGINT PRIMARY KEY DEFAULT nextval('syncapi_stream_id'),
	id BIGINT PRIMARY KEY,
    -- The event ID for the event
    event_id TEXT NOT NULL,
    domain_offset BIGINT NOT NULL default  -1,
    origin_server_ts BIGINT NOT NULL  default  -1,
    domain TEXT NOT NULL DEFAULT '',
    -- The state event type e.g 'm.room.member'
    type TEXT NOT NULL,
    -- The 'room_id' key for the event.
    room_id TEXT NOT NULL,
    -- The JSON for the event. Stored as TEXT because this should be valid UTF-8.
    event_json TEXT NOT NULL,
    -- A list of event IDs which represent a delta of added/removed room state. This can be NULL
    -- if there is no delta.
    add_state_ids TEXT[],
    remove_state_ids TEXT[],
    device_id TEXT,  -- The local device that sent the event, if any
    transaction_id TEXT,  -- The transaction id used to send the event, if any
    depth BIGINT NOT NULL default -1,
    CONSTRAINT syncapi_output_room_events_unique_mirror UNIQUE (event_id, room_id)
);
-- for event selection
--CREATE UNIQUE INDEX IF NOT EXISTS syncapi_event_id_idx_mirror ON syncapi_output_room_events_mirror(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS syncapi_event_id_uni_idx_mirror ON syncapi_output_room_events_mirror(id);
CREATE INDEX IF NOT EXISTS syncapi_output_event_id_idx_mirror ON syncapi_output_room_events_mirror(event_id);
CREATE INDEX  IF NOT EXISTS syncapi_user_history_mirror  ON syncapi_output_room_events_mirror (type,room_id);
CREATE INDEX IF NOT EXISTS syncapi_output_room_visibility_mirror  ON syncapi_output_room_events_mirror (type,room_id,device_id) WHERE device_id IS NOT NULL;
CREATE INDEX  IF NOT EXISTS syncapi_user_recent_mirror ON syncapi_output_room_events_mirror (room_id);
CREATE INDEX  IF NOT EXISTS syncapi_load_room_history_mirror ON syncapi_output_room_events_mirror (id,room_id);
`

const insertEventSQL = "" +
	"INSERT INTO syncapi_output_room_events (" +
	"id, room_id, event_id, event_json, add_state_ids, remove_state_ids, device_id, transaction_id, type, domain_offset, depth, domain, origin_server_ts" +
	") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) ON CONFLICT DO NOTHING"

const insertEventSQLMirror = "" +
	"INSERT INTO syncapi_output_room_events_mirror (" +
	"id, room_id, event_id, event_json, add_state_ids, remove_state_ids, device_id, transaction_id, type, domain_offset, depth, domain, origin_server_ts" +
	") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) ON CONFLICT DO NOTHING"

const selectEventsSQL = "" +
	"SELECT id, event_json, device_id, transaction_id, type FROM syncapi_output_room_events WHERE event_id = ANY($1)"

const selectEventForwardSQL = "" +
	"SELECT id, event_json, origin_server_ts, type FROM syncapi_output_room_events" +
	" WHERE room_id = $1 AND id >= $2" +
	" ORDER BY id ASC, origin_server_ts ASC, depth ASC, domain ASC LIMIT $3"

const selectEventBackSQL = "" +
	"SELECT id, event_json, origin_server_ts, type FROM syncapi_output_room_events" +
	" WHERE room_id = $1 AND id <= $2" +
	" ORDER BY id DESC, origin_server_ts ASC, depth ASC, domain ASC LIMIT $3"

const selectEventRangeForwardSQL = "" +
	"SELECT id, event_json, origin_server_ts, type FROM syncapi_output_room_events" +
	" WHERE room_id = $1 AND id >= $2 AND id <= $3" +
	" ORDER BY id ASC, origin_server_ts ASC, depth ASC, domain ASC"

const selectEventRangeBackSQL = "" +
	"SELECT id, event_json, origin_server_ts, type FROM syncapi_output_room_events" +
	" WHERE room_id = $1 AND id <= $2 AND id >= $3" +
	" ORDER BY id DESC, origin_server_ts ASC, depth ASC, domain ASC"

const updateEventSQL = "" +
	"UPDATE syncapi_output_room_events SET event_json = $1 WHERE event_id = $2 and room_id= $3"

const updateEventMirrorSQL = "" +
	"UPDATE syncapi_output_room_events_mirror SET event_json = $1 WHERE event_id = $2 and room_id= $3"

const selectHistoryEvents = "" +
	"SELECT id, event_json, type FROM syncapi_output_room_events" +
	" WHERE room_id = $1 AND id > 0" +
	" ORDER BY id DESC LIMIT $2"

const selectRoomStateStreamSQL = "" +
	"SELECT event_json, id, type FROM syncapi_output_room_events WHERE room_id = $1 AND type=any('{\"m.room.create\", \"m.room.member\", \"m.room.power_levels\", \"m.room.join_rules\", \"m.room.history_visibility\", \"m.room.visibility\",\"m.room.name\", \"m.room.topic\", \"m.room.desc\", \"m.room.pinned_events\",\"m.room.aliases\", \"m.room.canonical_alias\", \"m.room.avatar\", \"m.room.encryption\"}') AND id >= $2 ORDER BY id ASC"

const selectRoomLatestStreamsSQL = "" +
	"SELECT max(id), room_id FROM syncapi_output_room_events WHERE room_id = ANY($1) group by room_id"

const selectTypeEventForwardSQL = "" +
	"SELECT id, event_json, type FROM syncapi_output_room_events" +
	" WHERE type = any($1) AND room_id = $2 AND id > 0 ORDER BY id ASC"

const updateSyncMemberEventAvatarSQL = "" +
	"UPDATE syncapi_output_room_events set event_json = replace(event_json, $1, $2) where type = 'm.room.member' AND event_json like $3"

// const selectSyncMemberEventAvatarSQL = "" +
//	"SELECT id, event_json FROM syncapi_output_room_events where type = 'm.room.member' AND event_json like $3"

const selectAllSyncRoomsSQL = "" +
	"SELECT distinct room_id from syncapi_output_room_events"

const selectRoomDomainMaxOffsetSQL = "" +
	"SELECT max(domain_offset), domain FROM syncapi_output_room_events where room_id = $1 group by domain"

const updateSyncEventSQL = "" +
	"UPDATE syncapi_output_room_events set domain_offset = $1, origin_server_ts = $2, domain = $3 WHERE room_id = $4 AND event_id = $5"

const selectAllSyncEventsSQL = "" +
	"SELECT event_json, type FROM syncapi_output_room_events ORDER BY id ASC LIMIT $1 OFFSET $2"
const selectSyncEventsRangeSQL = "" +
	"SELECT event_json, type FROM syncapi_output_room_events WHERE origin_server_ts >= $1 AND origin_server_ts <= $2 ORDER BY id ASC LIMIT $3 OFFSET $4"
const selectSyncEventsLowerSQL = "" +
	"SELECT event_json, type FROM syncapi_output_room_events WHERE origin_server_ts >= $1 ORDER BY id ASC LIMIT $2 OFFSET $3"
const selectSyncEventsUpperSQL = "" +
	"SELECT event_json, type FROM syncapi_output_room_events WHERE origin_server_ts <= $1 ORDER BY id ASC LIMIT $2 OFFSET $3"

const selectSyncEventByDepthSQL = "" +
	"SELECT event_id, id FROM syncapi_output_room_events WHERE room_id = $1 AND depth = $2"

//const selectSyncMsgEventsSQL = "" +
//	"SELECT s.id, s.event_id, s.event_json FROM syncapi_output_room_events s inner join " +
//	"(SELECT id FROM syncapi_output_room_events WHERE type = 'm.room.message' AND event_json LIKE '%content%' ORDER BY id ASC LIMIT $1 OFFSET $2) n on s.id=n.id"

const selectSyncMsgEventsSQL = "" +
	"SELECT id, event_id, event_json FROM syncapi_output_room_events WHERE type = 'm.room.message' AND event_json LIKE '%content%' AND id >= $1 ORDER BY id ASC LIMIT $2"

const selectSyncMsgEventsCountSQL = "" +
	"SELECT count(*), COALESCE(MIN(id), 0) FROM syncapi_output_room_events WHERE type = 'm.room.message' AND event_json LIKE '%content%'"

const updateSyncMsgEventSQL = "" +
	"UPDATE syncapi_output_room_events SET event_json = $1 WHERE id = $2"

const selectEventRawSQL = "" +
	"SELECT id, event_json FROM syncapi_output_room_events WHERE event_id = $1"

const selectEventsByRoomIDSQL = "" +
	"SELECT id, event_id, event_json FROM syncapi_output_room_events WHERE room_id = $1"

const selectEventsByEventsSQL = "" +
	"SELECT id, event_id, room_id FROM syncapi_output_room_events WHERE event_id = ANY($1)"

type outputRoomEventsStatements struct {
	db                          *Database
	insertEventStmt             *sql.Stmt
	insertEventStmtMirror       *sql.Stmt
	selectEventsStmt            *sql.Stmt
	selectEventBackStmt         *sql.Stmt
	selectEventForwardStmt      *sql.Stmt
	selectEventRangeBackStmt    *sql.Stmt
	selectEventRangeForwardStmt *sql.Stmt
	updateEventStmt             *sql.Stmt
	updateEventMirrorStmt       *sql.Stmt
	selectHistoryEventStmt      *sql.Stmt
	selectRoomStateStreamStmt   *sql.Stmt
	selectRoomLatestStreamsStmt *sql.Stmt
	selectTypeEventForwardStmt  *sql.Stmt
	updateMemberEventAvatarStmt *sql.Stmt
	// selectMemberEventAvatarStmt   *sql.Stmt
	selectAllSyncRoomsStmt        *sql.Stmt
	selectRoomDomainMaxOffsetStmt *sql.Stmt
	updateSyncEventStmt           *sql.Stmt
	selectAllSyncEventsStmt       *sql.Stmt
	selectSyncEventsRangeStmt     *sql.Stmt
	selectSyncEventsLowerStmt     *sql.Stmt
	selectSyncEventsUpperStmt     *sql.Stmt
	selectSyncEventByDepthStmt    *sql.Stmt
	selectSyncMsgEventsStmt       *sql.Stmt
	selectSyncMsgEventsCountStmt  *sql.Stmt
	updateSyncMsgEventStmt        *sql.Stmt
	selectEventRawStmt            *sql.Stmt
	selectEventsByRoomIDStmt      *sql.Stmt
	selectEventsByEventsStmt 	  *sql.Stmt
}

func (s *outputRoomEventsStatements) getSchema() string {
	return outputRoomEventsSchema
}

func (s *outputRoomEventsStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	_, err = db.Exec(outputRoomEventsSchema)
	if err != nil {
		return
	}
	if s.insertEventStmt, err = db.Prepare(insertEventSQL); err != nil {
		return
	}
	if s.insertEventStmtMirror, err = db.Prepare(insertEventSQLMirror); err != nil {
		return
	}
	if s.selectEventsStmt, err = db.Prepare(selectEventsSQL); err != nil {
		return
	}
	if s.selectEventBackStmt, err = db.Prepare(selectEventBackSQL); err != nil {
		return
	}
	if s.selectEventForwardStmt, err = db.Prepare(selectEventForwardSQL); err != nil {
		return
	}
	if s.selectEventRangeBackStmt, err = db.Prepare(selectEventRangeBackSQL); err != nil {
		return
	}
	if s.selectEventRangeForwardStmt, err = db.Prepare(selectEventRangeForwardSQL); err != nil {
		return
	}
	if s.updateEventStmt, err = db.Prepare(updateEventSQL); err != nil {
		return
	}
	if s.updateEventMirrorStmt, err = db.Prepare(updateEventMirrorSQL); err != nil {
		return
	}
	if s.selectHistoryEventStmt, err = db.Prepare(selectHistoryEvents); err != nil {
		return
	}
	if s.selectRoomStateStreamStmt, err = db.Prepare(selectRoomStateStreamSQL); err != nil {
		return
	}
	if s.selectRoomLatestStreamsStmt, err = db.Prepare(selectRoomLatestStreamsSQL); err != nil {
		return
	}
	if s.selectTypeEventForwardStmt, err = db.Prepare(selectTypeEventForwardSQL); err != nil {
		return
	}
	if s.updateMemberEventAvatarStmt, err = db.Prepare(updateSyncMemberEventAvatarSQL); err != nil {
		return
	}
	// if s.selectMemberEventAvatarStmt, err = db.Prepare(selectSyncMemberEventAvatarSQL); err != nil {
	//	return
	// }
	if s.selectAllSyncRoomsStmt, err = db.Prepare(selectAllSyncRoomsSQL); err != nil {
		return
	}
	if s.selectRoomDomainMaxOffsetStmt, err = db.Prepare(selectRoomDomainMaxOffsetSQL); err != nil {
		return
	}
	if s.updateSyncEventStmt, err = db.Prepare(updateSyncEventSQL); err != nil {
		return
	}
	if s.selectAllSyncEventsStmt, err = db.Prepare(selectAllSyncEventsSQL); err != nil {
		return
	}
	if s.selectSyncEventsRangeStmt, err = db.Prepare(selectSyncEventsRangeSQL); err != nil {
		return
	}
	if s.selectSyncEventsLowerStmt, err = db.Prepare(selectSyncEventsLowerSQL); err != nil {
		return
	}
	if s.selectSyncEventsUpperStmt, err = db.Prepare(selectSyncEventsUpperSQL); err != nil {
		return
	}
	if s.selectSyncEventByDepthStmt, err = db.Prepare(selectSyncEventByDepthSQL); err != nil {
		return
	}
	if s.selectSyncMsgEventsStmt, err = db.Prepare(selectSyncMsgEventsSQL); err != nil {
		return
	}
	if s.selectSyncMsgEventsCountStmt, err = db.Prepare(selectSyncMsgEventsCountSQL); err != nil {
		return
	}
	if s.updateSyncMsgEventStmt, err = db.Prepare(updateSyncMsgEventSQL); err != nil {
		return
	}
	if s.selectEventRawStmt, err = db.Prepare(selectEventRawSQL); err != nil {
		return
	}
	if s.selectEventsByRoomIDStmt, err = db.Prepare(selectEventsByRoomIDSQL); err != nil {
		return
	}
	if s.selectEventsByEventsStmt, err = db.Prepare(selectEventsByEventsSQL); err != nil {
		return
	}
	return
}

func (s *outputRoomEventsStatements) selectTypeEventForward(
	ctx context.Context,
	typ []string, roomID string,
) (events []gomatrixserverlib.ClientEvent, offsets []int64, err error) {
	rows, err := s.selectTypeEventForwardStmt.QueryContext(ctx, pq.StringArray(typ), roomID)
	if err != nil {
		log.Errorf("outputRoomEventsStatements.selectTypeEventForward err: %v", err)
		return
	}

	var evs []gomatrixserverlib.ClientEvent
	var ids []int64

	defer rows.Close()
	for rows.Next() {
		var (
			streamPos  int64
			eventBytes []byte
			eventType  string
		)
		if err := rows.Scan(&streamPos, &eventBytes, &eventType); err != nil {
			return nil, nil, err
		}

		var ev gomatrixserverlib.ClientEvent

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			err = json.Unmarshal(dec, &ev)
		} else {
			err = json.Unmarshal(eventBytes, &ev)
		}
		if err != nil {
			return nil, nil, err
		}

		evs = append(evs, ev)
		ids = append(ids, streamPos)
	}
	return evs, ids, nil
}

func (s *outputRoomEventsStatements) selectRoomLastOffsets(
	ctx context.Context,
	roomIDs []string,
) (map[string]int64, error) {
	rows, err := s.selectRoomLatestStreamsStmt.QueryContext(ctx, pq.StringArray(roomIDs))
	if err != nil {
		log.Errorf("outputRoomEventsStatements.selectRoomLastOffsets err: %v", err)
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

func (s *outputRoomEventsStatements) selectHistoryEvents(
	ctx context.Context,
	roomID string,
	limit int,
) (events []gomatrixserverlib.ClientEvent, offset []int64, err error) {
	rows, err := s.selectHistoryEventStmt.QueryContext(ctx, roomID, limit)
	if err != nil {
		log.Errorf("outputRoomEventsStatements.selectHistoryEvents err: %v", err)
		return
	}

	var evs []gomatrixserverlib.ClientEvent
	var pos []int64
	defer rows.Close()
	for rows.Next() {
		var (
			streamPos  int64
			eventBytes []byte
			eventType  string
		)
		if err := rows.Scan(&streamPos, &eventBytes, &eventType); err != nil {
			return nil, nil, err
		}

		// decrypt messages
		if encryption.CheckCrypto(eventType) {
			eventBytes = encryption.Decrypt(eventBytes)
		}

		var ev gomatrixserverlib.ClientEvent
		err = json.Unmarshal(eventBytes, &ev)
		if err != nil {
			log.Errorf("outputRoomEvents selectHistoryEvents json unmarsharl faild, id: %d, room: %s, type: %s, err: %v", streamPos, roomID, eventType, err)
			return nil, nil, err
		}

		evs = append(evs, ev)
		pos = append(pos, streamPos)
	}

	return evs, pos, nil
}

// InsertEvent into the output_room_events table. addState and removeState are an optional list of state event IDs. Returns the position
// of the inserted event.
func (s *outputRoomEventsStatements) insertEvent(
	ctx context.Context,
	id int64, event *gomatrixserverlib.ClientEvent, addState, removeState []string,
	transactionID *roomservertypes.TransactionID,
	domainOffset, depth int64, domain string,
	originTs int64,
) (err error) {
	var deviceID, txnID string
	if transactionID != nil {
		localPart, _, _ := gomatrixserverlib.SplitID('@', transactionID.DeviceID)
		deviceID = localPart
		txnID = transactionID.TransactionID
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncEventInsertKey
		update.SyncDBEvents.SyncEventInsert = &dbtypes.SyncEventInsert{
			Pos:          id,
			RoomId:       event.RoomID,
			EventId:      event.EventID,
			EventJson:    eventBytes,
			Add:          pq.StringArray(addState),
			Remove:       pq.StringArray(removeState),
			Device:       deviceID,
			TxnId:        txnID,
			Type:         event.Type,
			DomainOffset: domainOffset,
			Depth:        depth,
			Domain:       domain,
			OriginTs:     originTs,
		}
		update.SetUid(int64(common.CalcStringHashCode64(event.RoomID)))
		s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_output_room_events")
	} else {
		if encryption.CheckCrypto(event.Type) {
			_, err = s.insertEventStmt.ExecContext(ctx,
				id, event.RoomID, event.EventID,
				encryption.Encrypt(eventBytes), pq.StringArray(addState), pq.StringArray(removeState),
				deviceID, txnID, event.Type, domainOffset, depth, domain, originTs,
			)
			if encryption.CheckMirror(event.Type) {
				_, err = s.insertEventStmtMirror.ExecContext(ctx,
					id, event.RoomID, event.EventID,
					eventBytes, pq.StringArray(addState), pq.StringArray(removeState),
					deviceID, txnID, event.Type, domainOffset, depth, domain, originTs,
				)
			}
		} else {
			_, err = s.insertEventStmt.ExecContext(ctx,
				id, event.RoomID, event.EventID,
				eventBytes, pq.StringArray(addState), pq.StringArray(removeState),
				deviceID, txnID, event.Type, domainOffset, depth, domain, originTs,
			)
		}
	}
	return
}

func (s *outputRoomEventsStatements) insertEventRaw(
	ctx context.Context,
	id int64, roomId, eventId string, json []byte, addState, removeState []string,
	device, txnID, eventType string, domainOffset, depth int64, domain string,
	originTs int64,
) (err error) {
	if encryption.CheckCrypto(eventType) {
		_, err = s.insertEventStmt.ExecContext(ctx,
			id, roomId, eventId,
			encryption.Encrypt(json), pq.StringArray(addState), pq.StringArray(removeState),
			device, txnID, eventType, domainOffset, depth, domain, originTs,
		)
		if encryption.CheckMirror(eventType) {
			_, err = s.insertEventStmtMirror.ExecContext(ctx,
				id, roomId, eventId,
				json, pq.StringArray(addState), pq.StringArray(removeState),
				device, txnID, eventType, domainOffset, depth, domain, originTs,
			)
		}
	} else {
		_, err = s.insertEventStmt.ExecContext(ctx,
			id, roomId, eventId,
			json, pq.StringArray(addState), pq.StringArray(removeState),
			device, txnID, eventType, domainOffset, depth, domain, originTs,
		)
	}
	if err != nil {
		return err
	}

	if depth > 1 {
		var eventID string
		var offset int64
		err = s.selectSyncEventByDepthStmt.QueryRowContext(ctx, roomId, depth-1).Scan(&eventID, &offset)
		if err != nil || offset == 0 {
			log.Errorf("EventLoss syncapi roomID: %s, depth: %d, nextEventID: %s", roomId, depth-1, eventID)
		}
	}
	return nil
}

func (s *outputRoomEventsStatements) selectEventsByDir(
	ctx context.Context,
	roomID string, dir string, from int64, limit int,
) ([]gomatrixserverlib.ClientEvent, []int64, []int64, error, int64, int64) {
	var rows *sql.Rows
	var err error
	var endPos, endTs int64

	if dir == "b" {
		rows, err = s.selectEventBackStmt.Query(roomID, from, limit)
		// endPos = 0
		// endTs = 0
	} else {
		rows, err = s.selectEventForwardStmt.Query(roomID, from, limit)
	}
	if err != nil {
		log.Errorf("failed to selectEventsByDir, dir: %s, roomID: %s, from: %d, limit: %d,", dir, roomID, from, limit)
		return nil, nil, nil, err, -1, -1
	}
	defer rows.Close() // nolint: errcheck

	var evs []gomatrixserverlib.ClientEvent
	var offsets []int64
	var tss []int64
	for rows.Next() {
		var (
			streamPos  int64
			ts         int64
			eventBytes []byte
			eventType  string
		)
		if err = rows.Scan(&streamPos, &eventBytes, &ts, &eventType); err != nil {
			return nil, nil, nil, err, endPos, endTs
		}

		endTs = ts
		endPos = streamPos

		var ev gomatrixserverlib.ClientEvent

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			err = json.Unmarshal(dec, &ev)
		} else {
			err = json.Unmarshal(eventBytes, &ev)
		}
		if err != nil {
			return nil, nil, nil, err, endPos, endTs
		}

		evs = append(evs, ev)
		offsets = append(offsets, streamPos)
		tss = append(tss, ts)
	}

	return evs, offsets, tss, nil, endPos, endTs
}

func (s *outputRoomEventsStatements) selectEventsByDirRange(
	ctx context.Context,
	roomID string, dir string, from, to int64,
) ([]gomatrixserverlib.ClientEvent, []int64, []int64, error, int64, int64) {
	var rows *sql.Rows
	var err error
	var endPos, endTs int64

	if dir == "b" {
		rows, err = s.selectEventRangeBackStmt.Query(roomID, from, to)
		// endPos = 0
		// endTs = 0
	} else {
		rows, err = s.selectEventRangeForwardStmt.Query(roomID, from, to)
	}
	if err != nil {
		log.Errorf("failed to selectEventsByDirRange, dir: %s, roomID: %s, from: %d, to: %d,", dir, roomID, from, to)
		return nil, nil, nil, err, -1, -1
	}
	defer rows.Close() // nolint: errcheck

	var evs []gomatrixserverlib.ClientEvent
	var offsets []int64
	var tss []int64
	for rows.Next() {
		var (
			streamPos  int64
			ts         int64
			eventBytes []byte
			eventType  string
		)
		if err = rows.Scan(&streamPos, &eventBytes, &ts, &eventType); err != nil {
			return nil, nil, nil, err, endPos, endTs
		}

		endTs = ts
		endPos = streamPos

		var ev gomatrixserverlib.ClientEvent

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			err = json.Unmarshal(dec, &ev)
		} else {
			err = json.Unmarshal(eventBytes, &ev)
		}
		if err != nil {
			return nil, nil, nil, err, endPos, endTs
		}

		evs = append(evs, ev)
		offsets = append(offsets, streamPos)
		tss = append(tss, ts)
	}

	return evs, offsets, tss, nil, endPos, endTs
}

// Events returns the events for the given event IDs. Returns an error if any one of the event IDs given are missing
// from the database.
func (s *outputRoomEventsStatements) selectEvents(
	ctx context.Context, txn *sql.Tx, eventIDs []string,
) ([]streamEvent, error) {
	stmt := common.TxStmt(txn, s.selectEventsStmt)
	rows, err := stmt.QueryContext(ctx, pq.StringArray(eventIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint: errcheck
	return rowsToStreamEvents(rows)
}

func (s *outputRoomEventsStatements) updateEvent(
	ctx context.Context,
	eventJson []byte,
	eventID string,
	RoomID string,
	eventType string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncEventUpdateContentKey
		update.SyncDBEvents.SyncEventUpdateContent = &dbtypes.SyncEventUpdateContent{
			EventID:   eventID,
			RoomID:    RoomID,
			Content:   string(eventJson),
			EventType: eventType,
		}
		update.SetUid(int64(common.CalcStringHashCode64(RoomID)))
		s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_output_room_events")
	} else {
		err = s.updateEventRaw(ctx, eventJson, eventID, RoomID, eventType)
	}

	return
}

func (s *outputRoomEventsStatements) updateEventRaw(
	ctx context.Context,
	eventJson []byte,
	eventID string,
	RoomID string,
	eventType string,
) error {
	var err error
	if encryption.CheckCrypto(eventType) {
		_, err = s.updateEventStmt.ExecContext(
			ctx, encryption.Encrypt(eventJson), eventID, RoomID,
		)
		if encryption.CheckMirror(eventType) {
			_, err = s.updateEventMirrorStmt.ExecContext(
				ctx, eventJson, eventID, RoomID,
			)
		}
	} else {
		_, err = s.updateEventStmt.ExecContext(
			ctx, eventJson, eventID, RoomID,
		)
	}

	return err
}

func rowsToStreamEvents(rows *sql.Rows) ([]streamEvent, error) {
	var result []streamEvent
	for rows.Next() {
		var (
			streamPos     int64
			eventBytes    []byte
			deviceID      *string
			txnID         *string
			transactionID *roomservertypes.TransactionID
			eventType     string
			err           error
		)
		if err = rows.Scan(&streamPos, &eventBytes, &deviceID, &txnID, &eventType); err != nil {
			return nil, err
		}

		var ev gomatrixserverlib.ClientEvent

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			err = json.Unmarshal(dec, &ev)
		} else {
			err = json.Unmarshal(eventBytes, &ev)
		}
		if err != nil {
			return nil, err
		}

		if deviceID != nil && txnID != nil {
			transactionID = &roomservertypes.TransactionID{
				DeviceID:      *deviceID,
				TransactionID: *txnID,
			}
		}

		result = append(result, streamEvent{
			ClientEvent:    ev,
			StreamPosition: syncapitypes.StreamPosition(streamPos),
			TransactionID:  transactionID,
		})
	}
	return result, nil
}

// CurrentState returns all the current state events for the given room.
func (s *outputRoomEventsStatements) selectRoomStateStream(
	ctx context.Context, roomID string, pos int64,
) ([]gomatrixserverlib.ClientEvent, []int64, error) {
	rows, err := s.selectRoomStateStreamStmt.QueryContext(ctx, roomID, pos)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := []gomatrixserverlib.ClientEvent{}
	offsets := []int64{}
	for rows.Next() {
		var eventBytes []byte
		var offset int64
		var eventType string
		if err := rows.Scan(&eventBytes, &offset, &eventType); err != nil {
			return nil, nil, err
		}

		var ev gomatrixserverlib.ClientEvent

		// decrypt message
		if encryption.CheckCrypto(eventType) {
			dec := encryption.Decrypt(eventBytes)
			err = json.Unmarshal(dec, &ev)
		} else {
			err = json.Unmarshal(eventBytes, &ev)
		}
		if err != nil {
			return nil, nil, err
		}

		result = append(result, ev)
		offsets = append(offsets, offset)
	}
	return result, offsets, nil
}

func (s *outputRoomEventsStatements) updateMemberEventAvatar(
	ctx context.Context,
	userID, oldAvatar, newAvatar string,
) error {
	pattern := "%" + userID + "%"

	// FIXME
	// decrypt event json, update avtar and then update one by one
	// rows, err := s.selectRoomDomainMaxOffsetStmt.ExecContext(ctx, pattern)
	// if err != nil {
	// 	return err
	// }
	//defer rows.Close() // nolint: errcheck

	_, err := s.updateMemberEventAvatarStmt.ExecContext(
		ctx, oldAvatar, newAvatar, pattern,
	)
	return err
}

func (s *outputRoomEventsStatements) selectAllSyncRooms() ([]string, error) {
	rows, err := s.selectAllSyncRoomsStmt.QueryContext(context.TODO())
	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := []string{}
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err != nil {
			return nil, err
		}

		result = append(result, roomID)
	}
	return result, nil
}

func (s *outputRoomEventsStatements) selectDomainMaxOffset(
	ctx context.Context,
	roomID string,
) (domains []string, offset []int64, err error) {
	rows, err := s.selectRoomDomainMaxOffsetStmt.QueryContext(ctx, roomID)
	if err != nil {
		log.Errorf("outputRoomEventsStatements.selectDomainMaxOffset err: %v", err)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var (
			streamPos int64
			domain    string
		)
		if err := rows.Scan(&streamPos, &domain); err != nil {
			return nil, nil, err
		}

		domains = append(domains, domain)
		offset = append(offset, streamPos)
	}
	return domains, offset, nil
}

func (s *outputRoomEventsStatements) UpdateSyncEvent(
	ctx context.Context,
	domainOffset,
	originTs int64,
	domain,
	roomID,
	eventID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncEventUpdateKey
		update.SyncDBEvents.SyncEventUpdate = &dbtypes.SyncEventUpdate{
			RoomId:       roomID,
			EventId:      eventID,
			DomainOffset: domainOffset,
			Domain:       domain,
			OriginTs:     originTs,
		}
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_output_room_events")
	} else {
		return s.onUpdateSyncEvent(ctx, domainOffset, originTs, domain, roomID, eventID)
	}
}

func (s *outputRoomEventsStatements) onUpdateSyncEvent(
	ctx context.Context,
	domainOffset,
	originTs int64,
	domain,
	roomID,
	eventID string,
) error {
	_, err := s.updateSyncEventStmt.ExecContext(ctx, domainOffset, originTs, domain, roomID, eventID)
	return err
}

func (s *outputRoomEventsStatements) selectSyncEvents(ctx context.Context, start, end int64, limit, offset int64) ([][]byte, error) {
	var rows *sql.Rows
	var err error

	if start < 0 {
		if end < 0 {
			rows, err = s.selectAllSyncEventsStmt.QueryContext(ctx, limit, offset)
		} else {
			rows, err = s.selectSyncEventsUpperStmt.QueryContext(ctx, end, limit, offset)
		}
	} else {
		if end < 0 {
			rows, err = s.selectSyncEventsLowerStmt.QueryContext(ctx, start, limit, offset)
		} else {
			rows, err = s.selectSyncEventsRangeStmt.QueryContext(ctx, end, limit, offset)
		}
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := [][]byte{}
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

		result = append(result, eventBytes)
	}
	return result, nil
}

func (s *outputRoomEventsStatements) selectSyncMsgEventsMigration(ctx context.Context, limit, offset int64) ([]int64, []string, [][]byte, error) {
	var rows *sql.Rows
	var err error

	// rows, err = s.selectSyncMsgEventsStmt.QueryContext(ctx, limit, offset)
	rows, err = s.selectSyncMsgEventsStmt.QueryContext(ctx, offset, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := [][]byte{}
	var ids []int64
	var eventIDs []string
	for rows.Next() {
		var eventBytes []byte
		var id int64
		var eventID string
		if err = rows.Scan(&id, &eventID, &eventBytes); err != nil {
			return nil, nil, nil, err
		}

		ids = append(ids, id)
		eventIDs = append(eventIDs, eventID)
		result = append(result, eventBytes)
	}
	return ids, eventIDs, result, nil
}

func (s *outputRoomEventsStatements) selectSyncMsgEventsTotalMigration(
	ctx context.Context,
) (count int, minID int64, err error) {
	err = s.selectSyncMsgEventsCountStmt.QueryRowContext(ctx).Scan(&count, &minID)
	return
}

func (s *outputRoomEventsStatements) updateSyncMsgEventMigration(
	ctx context.Context,
	id int64,
	EncryptedEventBytes []byte,
) error {
	_, err := s.updateSyncMsgEventStmt.ExecContext(
		ctx, EncryptedEventBytes, id,
	)
	return err
}

func (s *outputRoomEventsStatements) selectEventRaw(
	ctx context.Context, eventID string,
) (id int64, eventBytes []byte, err error) {
	rows, err := s.selectEventRawStmt.QueryContext(ctx, eventID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close() // nolint: errcheck

	for rows.Next() {
		if err = rows.Scan(&id, &eventBytes); err != nil {
			return 0, nil, err
		}
		return id, eventBytes, nil
	}
	return id, eventBytes, nil
}

func (s *outputRoomEventsStatements) selectEventsByRoomIDMigration(
	ctx context.Context, roomID string,
) ([]int64, []string, [][]byte, error) {
	var rows *sql.Rows
	var err error

	rows, err = s.selectEventsByRoomIDStmt.QueryContext(ctx, roomID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := [][]byte{}
	var ids []int64
	var eventIDs []string
	for rows.Next() {
		var eventBytes []byte
		var id int64
		var eventID string
		if err = rows.Scan(&id, &eventID, &eventBytes); err != nil {
			return nil, nil, nil, err
		}

		ids = append(ids, id)
		eventIDs = append(eventIDs, eventID)
		result = append(result, eventBytes)
	}
	return ids, eventIDs, result, nil
}

func (s *outputRoomEventsStatements) selectEventsByEvents(
	ctx context.Context, events []string,
) ([]int64, []string, []string, error) {
	var rows *sql.Rows
	var err error

	rows, err = s.selectEventsByEventsStmt.QueryContext(ctx,pq.Array(events))
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	var ids []int64
	var eventIDs []string
	var roomIDs []string
	for rows.Next() {
		var id int64
		var eventID string
		var roomID string
		if err = rows.Scan(&id, &eventID, &roomID); err != nil {
			return nil, nil, nil, err
		}

		ids = append(ids, id)
		eventIDs = append(eventIDs, eventID)
		roomIDs = append(roomIDs, roomID)
	}
	return ids, eventIDs, roomIDs, nil
}
