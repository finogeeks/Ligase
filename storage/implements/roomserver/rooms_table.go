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
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/lib/pq"
)

const roomsSchema = `
CREATE TABLE IF NOT EXISTS roomserver_rooms (
    -- Local numeric ID for the room.
    room_nid BIGINT PRIMARY KEY ,
    -- Textual ID for the room.
    room_id TEXT NOT NULL CONSTRAINT roomserver_room_id_unique UNIQUE,
    -- The most recent events in the room that aren't referenced by another event.
    -- This list may empty if the server hasn't joined the room yet.
    -- (The server will be in that state while it stores the events for the initial state of the room)
    latest_event_nids BIGINT[] DEFAULT '{}'::BIGINT[],
    -- The last event written to the output log for this room.
    last_event_sent_nid BIGINT NOT NULL DEFAULT 0,
    -- The state of the room after the current set of latest events.
    -- This will be 0 if there are no latest events in the room.
	state_snapshot_nid BIGINT NOT NULL DEFAULT 0,
	depth bigint NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 0
);
`

// Same as insertEventTypeNIDSQL
const insertRoomNIDSQL = "" +
	"INSERT INTO roomserver_rooms (room_nid, room_id) VALUES ($1, $2)" +
	" ON CONFLICT ON CONSTRAINT roomserver_room_id_unique DO NOTHING"

const selectRoomExistsSQL = "" +
	"SELECT count(1) FROM roomserver_rooms WHERE room_id = $1"

const updateLatestEventNIDsSQL = "" +
	"UPDATE roomserver_rooms SET latest_event_nids = $2, last_event_sent_nid = $3, state_snapshot_nid = $4, version = $5, depth = $6 WHERE room_nid = $1 AND version < $5"

const selectRoomInfoSQL = "" +
	"SELECT room_nid, state_snapshot_nid, depth FROM roomserver_rooms WHERE room_id = $1"

const selectAllRoomsSQL = "select room_id, room_nid from roomserver_rooms limit $1 offset $2"

const updateRoomDepthSQL = "UPDATE roomserver_rooms Set depth = $1 where room_nid = $2"

type roomStatements struct {
	db                        *Database
	insertRoomNIDStmt         *sql.Stmt
	selectRoomExistsStmt      *sql.Stmt
	updateLatestEventNIDsStmt *sql.Stmt
	selectRoomInfoStmt        *sql.Stmt
	selectAllRoomsStmt        *sql.Stmt
	updateRoomDepthStmt       *sql.Stmt
}

func (s *roomStatements) getSchema() string {
	return roomsSchema
}

func (s *roomStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if prepare_with_create {
		_, err = db.Exec(roomsSchema)
		if err != nil {
			return
		}
	}

	return statementList{
		{&s.insertRoomNIDStmt, insertRoomNIDSQL},
		{&s.selectRoomExistsStmt, selectRoomExistsSQL},
		{&s.updateLatestEventNIDsStmt, updateLatestEventNIDsSQL},
		{&s.selectRoomInfoStmt, selectRoomInfoSQL},
		{&s.selectAllRoomsStmt, selectAllRoomsSQL},
		{&s.updateRoomDepthStmt, updateRoomDepthSQL},
	}.prepare(db)
}

func (s *roomStatements) insertRoomNID(
	ctx context.Context, roomID string,
) (int64, error) {
	id, err := s.db.idg.Next()
	if err != nil {
		return 0, err
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventRoomInsertKey
		update.RoomDBEvents.EventRoomInsert = &dbtypes.EventRoomInsert{
			RoomNid: id,
			RoomId:  roomID,
		}
		update.SetUid(id)
		s.db.WriteDBEventWithTbl(&update, "roomserver_rooms")
		return id, nil
	}

	return id, s.insertRoomNIDRaw(ctx, id, roomID)
}

func (s *roomStatements) insertRoomNIDRaw(
	ctx context.Context, roomNID int64, roomID string,
) error {
	_, err := s.insertRoomNIDStmt.ExecContext(ctx, roomNID, roomID)
	return err
}

func (s *roomStatements) selectRoomInfo(
	ctx context.Context, roomID string,
) (int64, int64, int64, error) {
	var roomNID int64
	var snapNID int64
	var depth int64
	err := s.selectRoomInfoStmt.QueryRowContext(ctx, roomID).Scan(&roomNID, &snapNID, &depth)

	return roomNID, snapNID, depth, err
}

func (s *roomStatements) updateLatestEventNIDs(
	ctx context.Context,
	roomNID int64,
	lastEventSentNID, stateSnapshotNID, depth int64,
) error {
	version, _ := s.db.idg.Next()

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventRoomUpdateKey
		update.RoomDBEvents.EventRoomUpdate = &dbtypes.EventRoomUpdate{
			LastEventSentNid: lastEventSentNID,
			StateSnapNid:     stateSnapshotNID,
			RoomNid:          roomNID,
			Version:          version,
			Depth:            depth,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_rooms")
		return nil
	}

	return s.updateLatestEventNIDsRaw(ctx, roomNID, []int64{lastEventSentNID}, lastEventSentNID, stateSnapshotNID, version, depth)
}

func (s *roomStatements) updateLatestEventNIDsRaw(
	ctx context.Context,
	roomNID int64,
	eventNIDs []int64,
	lastEventSentNID int64,
	stateSnapshotNID int64,
	version int64,
	depth int64,
) error {
	_, err := s.updateLatestEventNIDsStmt.ExecContext(
		ctx,
		roomNID,
		pq.Int64Array(eventNIDs),
		lastEventSentNID,
		stateSnapshotNID,
		version,
		depth,
	)
	return err
}

func (s *roomStatements) selectRoomExists(
	ctx context.Context, roomId string,
) (exists bool, err error) {
	stmt := s.selectRoomExistsStmt
	rows, err := stmt.QueryContext(ctx, roomId)
	if err != nil {
		return
	}
	defer rows.Close() // nolint: errcheck
	var count int
	for rows.Next() {
		if err = rows.Scan(&count); err != nil {
			return
		}
	}
	if count > 0 {
		exists = true
	}
	return
}

func (s *roomStatements) getAllRooms(ctx context.Context, limit, offset int) ([]roomservertypes.RoomNIDs, error) {
	stmt := s.selectAllRoomsStmt
	rows, err := stmt.QueryContext(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint: errcheck
	var rooms []roomservertypes.RoomNIDs
	rooms = []roomservertypes.RoomNIDs{}
	for rows.Next() {
		var room roomservertypes.RoomNIDs
		if err = rows.Scan(&room.RoomID, &room.RoomNID); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func (s *roomStatements) updateRoomDepth(ctx context.Context, depth, roomNid int64) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.RoomDepthUpdateKey
		update.RoomDBEvents.RoomDepthUpdate = &dbtypes.RoomDepthUpdate{
			RoomNid: roomNid,
			Depth:   depth,
		}
		update.SetUid(roomNid)
		s.db.WriteDBEventWithTbl(&update, "roomserver_rooms")
		return nil
	}
	return s.onUpdateRoomDepth(ctx, depth, roomNid)
}

func (s *roomStatements) onUpdateRoomDepth(ctx context.Context, depth, roomNid int64) error {
	_, err := s.updateRoomDepthStmt.ExecContext(ctx, depth, roomNid)
	return err
}
