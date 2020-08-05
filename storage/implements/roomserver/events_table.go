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

	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/lib/pq"
)

const eventsSchema = `
-- The events table holds metadata for each event, the actual JSON is stored
-- separately to keep the size of the rows small.
CREATE TABLE IF NOT EXISTS roomserver_events (
    -- Local numeric ID for the event.
    event_nid BIGINT PRIMARY KEY,
    -- Local numeric ID for the room the event is in.
    -- This is never 0.
    room_nid BIGINT NOT NULL,
    -- Local numeric ID for the type of the event.
    -- This is never 0.
    event_type_nid BIGINT NOT NULL DEFAULT 0,
    -- Local numeric ID for the state_key of the event
    -- This is 0 if the event is not a state event.
    event_state_key_nid BIGINT NOT NULL DEFAULT 0,
    -- Whether the event has been written to the output log.
    sent_to_output BOOLEAN NOT NULL DEFAULT FALSE,
    -- Local numeric ID for the state at the event.
    -- This is 0 if we don't know the state at the event.
    -- If the state is not 0 then this event is part of the contiguous
    -- part of the event graph
    -- Since many different events can have the same state we store the
    -- state into a separate state table and refer to it by numeric ID.
    state_snapshot_nid BIGINT NOT NULL DEFAULT 0,
    -- Depth of the event in the event graph.
    depth BIGINT NOT NULL,
    -- The textual event id.
    -- Used to lookup the numeric ID when processing requests.
    -- Needed for state resolution.
    -- An event may only appear in this table once.
    event_id TEXT NOT NULL CONSTRAINT roomserver_event_id_unique UNIQUE,
    -- The sha256 reference hash for the event.
    -- Needed for setting reference hashes when sending new events.
    reference_sha256 BYTEA NOT NULL,
    -- A list of numeric IDs for events that can authenticate this event.
	auth_event_nids BIGINT[],
	-- The string event ID taken from the prev_events key of an event.
	previous_event_id TEXT,
	offsets BIGINT NOT NULL DEFAULT 0,
    -- The SHA256 reference hash taken from the prev_events key of an event.
	previous_reference_sha256 BYTEA,
	event_type_id text NOT NULL DEFAULT '',
	event_state_key_id text NOT NULL DEFAULT '',
	domain TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS roomserver_events_room_nid_depth_idx
	ON roomserver_events (room_nid, depth);`

const insertEventSQL = "" +
	"INSERT INTO roomserver_events (room_nid, event_type_id, event_state_key_id, event_id, reference_sha256, auth_event_nids, depth, event_nid, state_snapshot_nid, previous_event_id, previous_reference_sha256, offsets, domain, sent_to_output)" +
	" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, TRUE)" +
	" ON CONFLICT ON CONSTRAINT roomserver_event_id_unique" +
	" DO NOTHING" +
	" RETURNING state_snapshot_nid"

const selectEventNIDSQL = "SELECT event_nid FROM roomserver_events WHERE event_id = $1"

const bulkSelectEventNIDSQL = "SELECT event_id, event_nid FROM roomserver_events WHERE event_id = ANY($1)"

const selectRoomStateNIDSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and event_type_id=any('{\"m.room.create\", \"m.room.member\", \"m.room.power_levels\", \"m.room.join_rules\", \"m.room.third_party_invite\", \"m.room.history_visibility\", \"m.room.visibility\",\"m.room.name\", \"m.room.topic\", \"m.room.desc\", \"m.room.pinned_events\",\"m.room.aliases\", \"m.room.canonical_alias\"}') order by event_nid asc"

//backfill
//const selectRoomBackfillNIDSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and domain = $2 and offsets< $3 and not (event_type_id=any('{\"m.room.create\", \"m.room.member\", \"m.room.power_levels\", \"m.room.join_rules\", \"m.room.third_party_invite\", \"m.room.history_visibility\", \"m.room.visibility\",\"m.room.name\", \"m.room.topic\", \"m.room.desc\", \"m.room.pinned_events\",\"m.room.aliases\", \"m.room.canonical_alias\"}')) order by event_nid desc limit $3"
//const selectRoomBackfillNIDUnLimitedSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and domain = $2 and not (event_type_id=any('{\"m.room.create\", \"m.room.member\", \"m.room.power_levels\", \"m.room.join_rules\", \"m.room.third_party_invite\", \"m.room.history_visibility\", \"m.room.visibility\",\"m.room.name\", \"m.room.topic\", \"m.room.desc\", \"m.room.pinned_events\",\"m.room.aliases\", \"m.room.canonical_alias\"}')) order by event_nid desc limit 500"
const selectRoomBackfillNIDSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and domain = $2 and event_nid < $3 order by event_nid desc limit $4"
const selectRoomBackfillNIDUnLimitedSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and domain = $2 and event_nid < $3 order by event_nid desc"
const selectRoomBackfillForwardNIDSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and domain = $2 and event_nid > $3 order by event_nid asc limit $4"
const selectRoomBackfillForwardNIDUnLimitedSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 and domain = $2 and event_nid > $3 order by event_nid asc"
const selectEventNidForBackfillSQL = `SELECT event_nid FROM roomserver_events WHERE room_nid = $1 AND event_type_id = 'm.room.member' AND event_state_key_id like $2 ORDER BY event_nid LIMIT 1`

const selectRoomEventsByDomainOffsetSQL = "SELECT event_nid FROM roomserver_events WHERE room_nid = $1 AND domain = $2 AND offsets >= $3 order by offsets limit $4"

const selectEventStateSnapshotNIDSQL = "SELECT state_snapshot_nid FROM roomserver_events WHERE event_id = $1"

const selectRoomStateNIDByStateBlockNIDSQL = "SELECT event_nid, event_type_id, event_state_key_id, domain FROM roomserver_events WHERE room_nid = $1 AND event_nid <= $2 AND event_type_id=ANY('{\"m.room.create\", \"m.room.member\", \"m.room.power_levels\", \"m.room.join_rules\", \"m.room.third_party_invite\", \"m.room.history_visibility\", \"m.room.visibility\",\"m.room.name\", \"m.room.topic\", \"m.room.desc\", \"m.room.pinned_events\",\"m.room.aliases\", \"m.room.canonical_alias\"}') ORDER BY event_nid ASC"

//fix db
const getLastConfirmEventSQL = "select event_nid, state_snapshot_nid, depth from roomserver_events where room_nid = $1 and state_snapshot_nid != 0 and sent_to_output=true order by event_nid desc limit 1"

const getLastEventSQL = "select state_snapshot_nid from roomserver_events where room_nid = $1 order by event_nid desc limit 1"

//migration
const selectLatestEventIDSQL = "SELECT t.event_id FROM roomserver_events t LEFT JOIN roomserver_rooms m on t.room_nid = m.room_nid WHERE m.room_id = $1 ORDER BY t.event_nid DESC LIMIT 1"

const selectRoomEventCountSQL = "SELECT count(*) FROM roomserver_events t LEFT JOIN roomserver_rooms m on t.room_nid = m.room_nid WHERE m.room_id = $1"

const selectEventCountSQL = "SELECT count(*) FROM roomserver_events"

const selectCorruptRoomsSQL = "select DISTINCT room_nid from roomserver_events where state_snapshot_nid =0"

const updateRoomEventSQL = "update roomserver_events set offsets = $1, domain = $2, depth = $3 where room_nid = $4 and event_nid = $5"

const selectRoomEventByDepthSQL = "SELECT event_nid, event_id FROM roomserver_events WHERE room_nid = $1 AND depth = $2"

const selectRoomMaxDomainOffsetSQL = "SELECT t.domain, t.m, m.event_id FROM(SELECT MAX(offsets) AS m, domain FROM roomserver_events WHERE room_nid=$1 GROUP BY domain) t LEFT JOIN roomserver_events m ON room_nid=$1 AND t.domain=m.domain AND t.m=m.offsets"

type eventStatements struct {
	db                                         *Database
	insertEventStmt                            *sql.Stmt
	selectEventNIDStmt                         *sql.Stmt
	bulkSelectEventNIDStmt                     *sql.Stmt
	selectLatestEventIDStmt                    *sql.Stmt
	selectRoomEventCountStmt                   *sql.Stmt
	selectEventCountStmt                       *sql.Stmt
	getLastConfirmEventStmt                    *sql.Stmt
	getLastEventStmt                           *sql.Stmt
	selectRoomStateNIDStmt                     *sql.Stmt
	updatePreviousEventStmt                    *sql.Stmt
	selectCorruptRoomsStmt                     *sql.Stmt
	selectRoomBackFillNIDStmt                  *sql.Stmt
	selectRoomBackFillNIDSUnLimitedStmt        *sql.Stmt
	selectRoomBackFillForwardNIDStmt           *sql.Stmt
	selectRoomBackFillForwardNIDSUnLimitedStmt *sql.Stmt
	selectEventNidForBackfillStmt              *sql.Stmt
	selectRoomEventsByDomainOffsetStmt         *sql.Stmt
	selectEventStateSnapshotNIDStmt            *sql.Stmt
	selectRoomStateNIDByStateBlockNIDStmt      *sql.Stmt
	updateRoomEventStmt                        *sql.Stmt
	selectRoomEventByDepthStmt                 *sql.Stmt
	selectRoomMaxDomainOffsetStmt              *sql.Stmt
}

func (s *eventStatements) getSchema() string {
	return eventsSchema
}

func (s *eventStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	return statementList{
		{&s.insertEventStmt, insertEventSQL},
		{&s.selectEventNIDStmt, selectEventNIDSQL},
		{&s.bulkSelectEventNIDStmt, bulkSelectEventNIDSQL},
		{&s.selectLatestEventIDStmt, selectLatestEventIDSQL},
		{&s.selectRoomEventCountStmt, selectRoomEventCountSQL},
		{&s.selectEventCountStmt, selectEventCountSQL},
		{&s.getLastConfirmEventStmt, getLastConfirmEventSQL},
		{&s.getLastEventStmt, getLastEventSQL},
		{&s.selectRoomStateNIDStmt, selectRoomStateNIDSQL},
		{&s.selectCorruptRoomsStmt, selectCorruptRoomsSQL},
		{&s.selectRoomBackFillNIDStmt, selectRoomBackfillNIDSQL},
		{&s.selectRoomBackFillNIDSUnLimitedStmt, selectRoomBackfillNIDUnLimitedSQL},
		{&s.selectRoomBackFillForwardNIDStmt, selectRoomBackfillForwardNIDSQL},
		{&s.selectRoomBackFillForwardNIDSUnLimitedStmt, selectRoomBackfillForwardNIDUnLimitedSQL},
		{&s.selectRoomBackFillForwardNIDSUnLimitedStmt, selectRoomBackfillForwardNIDUnLimitedSQL},
		{&s.selectEventNidForBackfillStmt, selectEventNidForBackfillSQL},
		{&s.selectRoomEventsByDomainOffsetStmt, selectRoomEventsByDomainOffsetSQL},
		{&s.selectEventStateSnapshotNIDStmt, selectEventStateSnapshotNIDSQL},
		{&s.selectRoomStateNIDByStateBlockNIDStmt, selectRoomStateNIDByStateBlockNIDSQL},
		{&s.updateRoomEventStmt, updateRoomEventSQL},
		{&s.selectRoomEventByDepthStmt, selectRoomEventByDepthSQL},
		{&s.selectRoomMaxDomainOffsetStmt, selectRoomMaxDomainOffsetSQL},
	}.prepare(db)
}

func (s *eventStatements) getCorruptRooms(ctx context.Context) ([]int64, error) {
	rows, err := s.selectCorruptRoomsStmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var rnid int64
		if err = rows.Scan(
			&rnid,
		); err != nil {
			return nil, err
		}

		res = append(res, rnid)
	}
	return res, nil
}

func (s *eventStatements) getLastConfirmEvent(ctx context.Context, roomNID int64) (int64, int64, int64, error) {
	rows, err := s.getLastConfirmEventStmt.QueryContext(ctx, roomNID)
	if err != nil {
		return -1, -1, -1, err
	}
	defer rows.Close()

	for rows.Next() {
		var ev_nid int64
		var snap_nid int64
		var depth int64
		if err = rows.Scan(
			&ev_nid,
			&snap_nid,
			&depth,
		); err != nil {
			return -1, -1, -1, err
		}
		return ev_nid, snap_nid, depth, nil
	}

	return -1, -1, -1, nil
}

func (s *eventStatements) getLastEvent(ctx context.Context, roomNID int64) (int64, error) {
	rows, err := s.getLastEventStmt.QueryContext(ctx, roomNID)
	if err != nil {
		return -1, err
	}
	defer rows.Close()

	for rows.Next() {
		var snap_nid int64
		if err = rows.Scan(
			&snap_nid,
		); err != nil {
			return -1, err
		}
		return snap_nid, nil
	}

	return -1, nil
}

func (s *eventStatements) getRoomsStateEvNID(ctx context.Context, roomNID int64) ([]int64, error) {
	rows, err := s.selectRoomStateNIDStmt.QueryContext(ctx, roomNID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var nid int64
		if err = rows.Scan(
			&nid,
		); err != nil {
			return nil, err
		}
		res = append(res, nid)
	}

	return res, nil
}

func (s *eventStatements) insertEvent(
	ctx context.Context,
	roomNID int64,
	eventType string,
	eventStateKey string,
	eventID string,
	eventNID int64,
	referenceSHA256 []byte,
	depth int64,
	stateNID int64,
	refId string,
	refHash []byte,
	offset int64,
	domain string,
) (int64, int64, error) {
	if eventNID == 0 {
		nid, err := s.db.idg.Next()
		if err != nil {
			return 0, 0, err
		}
		eventNID = nid
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventInsertKey
		update.RoomDBEvents.EventInsert = &dbtypes.EventInsert{
			RoomNid:       roomNID,
			EventType:     eventType,
			EventStateKey: eventStateKey,
			EventId:       eventID,
			RefSha:        referenceSHA256,
			Depth:         depth,
			EventNid:      eventNID,
			StateSnapNid:  stateNID,
			RefEventId:    refId,
			Sha:           refHash,
			Offset:        offset,
			Domain:        domain,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(ctx, &update, "roomserver_events")
		return eventNID, 0, nil
	}

	err := s.insertEventRaw(ctx, eventNID, roomNID, eventType,
		eventStateKey, eventID, referenceSHA256, []int64{},
		depth, stateNID, refId, refHash, offset, domain)
	if err != nil {
		return 0, 0, err
	}
	return eventNID, 0, nil
}

func (s *eventStatements) insertEventRaw(
	ctx context.Context,
	eventNID int64,
	roomNID int64,
	eventType string,
	eventStateKey string,
	eventID string,
	referenceSHA256 []byte,
	authEventNIDs []int64,
	depth int64,
	stateNID int64,
	refId string,
	refHash []byte,
	offset int64,
	domain string,
) error {
	_, err := s.insertEventStmt.ExecContext(
		ctx, roomNID, eventType, eventStateKey,
		eventID, referenceSHA256, pq.Int64Array(authEventNIDs), depth,
		eventNID, stateNID, refId, refHash, offset, domain,
	)
	if err != nil {
		return err
	}
	if depth > 1 {
		var lastEventNID int64
		var lastEventID string
		err = s.selectRoomEventByDepthStmt.QueryRowContext(ctx, roomNID, depth-1).Scan(&lastEventNID, &lastEventID)
		if err != nil || lastEventNID == 0 {
			log.Errorf("EventLoss roomNid: %d, depth: %d, nextEventID: %s", roomNID, depth-1, eventID)
		}
	}

	return nil
}

func (s *eventStatements) updateRoomEvent(
	ctx context.Context,
	eventNID,
	roomNID,
	depth,
	domainOffset int64,
	domain string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.RoomEventUpdateKey
		update.RoomDBEvents.RoomEventUpdate = &dbtypes.RoomEventUpdate{
			RoomNid:  roomNID,
			Depth:    depth,
			EventNid: eventNID,
			Offset:   domainOffset,
			Domain:   domain,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(ctx, &update, "roomserver_events")
		return nil
	}
	return s.onUpdateRoomEvent(ctx, eventNID, roomNID, depth, domainOffset, domain)
}

func (s *eventStatements) onUpdateRoomEvent(
	ctx context.Context,
	eventNID,
	roomNID,
	depth,
	domainOffset int64,
	domain string,
) error {
	_, err := s.updateRoomEventStmt.ExecContext(ctx, domainOffset, domain, depth, roomNID, eventNID)
	return err
}

func (s *eventStatements) loadEventNIDByID(ctx context.Context, eventID string) (int64, error) {
	var eventNID int64
	err := s.selectEventNIDStmt.QueryRowContext(ctx, eventID).Scan(&eventNID)

	return eventNID, err
}

func (s *eventStatements) bulkLoadEventNIDByID(ctx context.Context, eventIDs []string) (map[string]int64, error) {
	rows, err := s.bulkSelectEventNIDStmt.QueryContext(ctx, pq.StringArray(eventIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventNID int64
	var eventID string
	result := make(map[string]int64)
	for rows.Next() {
		if err = rows.Scan(
			&eventID, &eventNID,
		); err != nil {
			return nil, err
		}

		result[eventID] = eventNID
	}
	return result, err
}

func (s *eventStatements) selectLatestEventID(ctx context.Context, roomId string) (string, error) {
	var result string
	stmt := s.selectLatestEventIDStmt
	err := stmt.QueryRowContext(ctx, roomId).Scan(&result)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (s *eventStatements) selectRoomEventCount(ctx context.Context, roomId string) (int, error) {
	var result int
	stmt := s.selectRoomEventCountStmt
	err := stmt.QueryRowContext(ctx, roomId).Scan(&result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *eventStatements) selectEventCount(ctx context.Context) (int, error) {
	var result int
	stmt := s.selectEventCountStmt
	err := stmt.QueryRowContext(ctx).Scan(&result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *eventStatements) selectBackFillEvNID(ctx context.Context, roomNID int64, domain string, eventNid int64, limit int) ([]int64, error) {
	rows, err := s.selectRoomBackFillNIDStmt.QueryContext(ctx, roomNID, domain, eventNid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var nid int64
		if err = rows.Scan(
			&nid,
		); err != nil {
			return nil, err
		}
		res = append(res, nid)
	}

	return res, nil
}

func (s *eventStatements) selectBackFillEvNIDUnLimited(ctx context.Context, roomNID int64, domain string, eventNid int64) ([]int64, error) {
	rows, err := s.selectRoomBackFillNIDSUnLimitedStmt.QueryContext(ctx, roomNID, domain, eventNid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var nid int64
		if err = rows.Scan(
			&nid,
		); err != nil {
			return nil, err
		}
		res = append(res, nid)
	}

	return res, nil
}

func (s *eventStatements) selectBackFillEvFowradNID(ctx context.Context, roomNID int64, domain string, eventNid int64, limit int) ([]int64, error) {
	rows, err := s.selectRoomBackFillForwardNIDStmt.QueryContext(ctx, roomNID, domain, eventNid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var nid int64
		if err = rows.Scan(
			&nid,
		); err != nil {
			return nil, err
		}
		res = append(res, nid)
	}

	return res, nil
}

func (s *eventStatements) selectBackFillEvForwardNIDUnLimited(ctx context.Context, roomNID int64, domain string, eventNid int64) ([]int64, error) {
	rows, err := s.selectRoomBackFillForwardNIDSUnLimitedStmt.QueryContext(ctx, roomNID, domain, eventNid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []int64
	for rows.Next() {
		var nid int64
		if err = rows.Scan(
			&nid,
		); err != nil {
			return nil, err
		}
		res = append(res, nid)
	}

	return res, nil
}

func (s *eventStatements) selectEventNidForBackfill(ctx context.Context, roomNID int64, domain string) (int64, error) {
	rows, err := s.selectEventNidForBackfillStmt.QueryContext(ctx, roomNID, "%"+domain)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var eventNID = int64(0)
	for rows.Next() {
		err = rows.Scan(&eventNID)
		if err != nil {
			return 0, err
		}
	}
	return eventNID, nil
}

func (s *eventStatements) selectRoomEventsByDomainOffset(ctx context.Context, roomNID int64, domain string, domainOffset int64, limit int) ([]int64, error) {
	rows, err := s.selectRoomEventsByDomainOffsetStmt.QueryContext(ctx, roomNID, domain, domainOffset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []int64
	for rows.Next() {
		var nid int64
		if err = rows.Scan(&nid); err != nil {
			return nil, err
		}
		res = append(res, nid)
	}
	return res, nil
}

func (s *eventStatements) selectRoomMaxDomainOffset(ctx context.Context, roomNID int64) (domains, eventIDs []string, offsets []int64, err error) {
	rows, err := s.selectRoomMaxDomainOffsetStmt.QueryContext(ctx, roomNID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var (
			domain  string
			eventID string
			offset  int64
		)
		if err = rows.Scan(&domain, &offset, &eventID); err != nil {
			return
		}
		domains = append(domains, domain)
		offsets = append(offsets, offset)
		eventIDs = append(eventIDs, eventID)
	}
	return
}

func (s *eventStatements) selectEventStateSnapshotNID(ctx context.Context, eventID string) (int64, error) {
	var stateSnapshotNID int64
	err := s.selectEventStateSnapshotNIDStmt.QueryRowContext(ctx, eventID).Scan(&stateSnapshotNID)
	if err != nil {
		return 0, err
	}
	return stateSnapshotNID, nil
}

func (s *eventStatements) selectRoomStateNIDByStateBlockNID(ctx context.Context, roomNID int64, stateBlockNID int64) ([]int64, []string, []string, []string, error) {
	rows, err := s.selectRoomStateNIDByStateBlockNIDStmt.QueryContext(ctx, roomNID, stateBlockNID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	eventNIDs := []int64{}
	eventTypes := []string{}
	stateKeys := []string{}
	domains := []string{}
	for rows.Next() {
		var eventNID int64
		var eventType string
		var stateKey string
		var domain string
		err = rows.Scan(&eventNID, &eventType, &stateKey, &domain)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		eventNIDs = append(eventNIDs, eventNID)
		eventTypes = append(eventTypes, eventType)
		stateKeys = append(stateKeys, stateKey)
		domains = append(domains, domain)
	}
	return eventNIDs, eventTypes, stateKeys, domains, nil
}
