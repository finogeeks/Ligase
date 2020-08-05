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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/lib/pq"
)

const currentRoomStateSchema = `
-- Stores the current room state for every room.
CREATE TABLE IF NOT EXISTS syncapi_current_room_state (
    -- The 'room_id' key for the state event.
    room_id TEXT NOT NULL,
    -- The state event ID
    event_id TEXT NOT NULL,
    -- The state event type e.g 'm.room.member'
    type TEXT NOT NULL,
    -- The state_key value for this state event e.g ''
    state_key TEXT NOT NULL,
    -- The JSON for the event. Stored as TEXT because this should be valid UTF-8.
    event_json TEXT NOT NULL,
    -- The 'content.membership' value if this event is an m.room.member event. For other
    -- events, this will be NULL.
    membership TEXT,
    -- The serial ID of the output_room_events table when this event became
    -- part of the current state of the room.
    added_at BIGINT,
    -- Clobber based on 3-uple of room_id, type and state_key
    CONSTRAINT syncapi_room_state_unique UNIQUE (room_id, type, state_key)
);
-- for event deletion
CREATE UNIQUE INDEX IF NOT EXISTS syncapi_rs_event_id_idx ON syncapi_current_room_state(event_id);
CREATE INDEX IF NOT EXISTS syncapi_current_rooom_id_idx ON syncapi_current_room_state(room_id);
-- for querying membership states of users
CREATE INDEX IF NOT EXISTS syncapi_membership_idx ON syncapi_current_room_state(type, state_key, membership) WHERE membership IS NOT NULL AND membership != 'leave';
`

const upsertRoomStateSQL = "" +
	"INSERT INTO syncapi_current_room_state (room_id, event_id, type, state_key, event_json, membership, added_at)" +
	" VALUES ($1, $2, $3, $4, $5, $6, $7)" +
	" ON CONFLICT ON CONSTRAINT syncapi_room_state_unique" +
	" DO UPDATE SET event_id = $2, event_json = $5, membership = $6, added_at = $7"

const selectRoomIDsWithMembershipSQL = "" +
	"SELECT room_id, added_at, event_id FROM syncapi_current_room_state WHERE type = 'm.room.member' AND state_key = $1 AND membership = ANY($2)"

const selectCurrentStateSQL = "" +
	"SELECT event_json, type, added_at FROM syncapi_current_room_state WHERE room_id = $1 order by added_at asc"

const updateMemberEventAvatarSQL = "" +
	"UPDATE syncapi_current_room_state SET event_json = replace(event_json, $1, $2) WHERE type = 'm.room.member' AND state_key = $3"

const selectRoomJoinedUsersSQL = "" +
	"SELECT DISTINCT state_key FROM syncapi_current_room_state where room_id = ANY($1) AND type = 'm.room.member' AND membership = 'join'"

//const selectRoomStateWithLimitSQL = "" +
// 	"SELECT event_id, event_json FROM syncapi_current_room_state WHERE type = 'm.room.message' AND event_json LIKE '%content%' ORDER BY id ASC LIMIT $1 OFFSET $2"

const selectRoomStateCountSQL = "" +
	"SELECT count(*) FROM syncapi_current_room_state WHERE type = 'm.room.message' AND event_json LIKE '%content%'"

const updateRoomStateSQL = "" +
	"UPDATE syncapi_current_room_state SET event_json = $1 WHERE event_id = $2"

const selectRoomStateByEventIDSQL = "" +
	"SELECT event_json FROM syncapi_current_room_state WHERE event_id = $1"

type currentRoomStateStatements struct {
	db                              *Database
	upsertRoomStateStmt             *sql.Stmt
	selectRoomIDsWithMembershipStmt *sql.Stmt
	selectCurrentStateStmt          *sql.Stmt
	updateMemberEventAvatarStmt     *sql.Stmt
	selectRoomJoinedUsersStmt       *sql.Stmt
	// selectRoomStateWithLimitStmt    *sql.Stmt
	selectRoomStateCountStmt     *sql.Stmt
	updateRoomStateStmt          *sql.Stmt
	selectRoomStateByEventIDStmt *sql.Stmt
}

func (s *currentRoomStateStatements) getSchema() string {
	return currentRoomStateSchema
}

func (s *currentRoomStateStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	if s.upsertRoomStateStmt, err = db.Prepare(upsertRoomStateSQL); err != nil {
		return
	}
	if s.selectRoomIDsWithMembershipStmt, err = db.Prepare(selectRoomIDsWithMembershipSQL); err != nil {
		return
	}
	if s.selectCurrentStateStmt, err = db.Prepare(selectCurrentStateSQL); err != nil {
		return
	}
	if s.updateMemberEventAvatarStmt, err = db.Prepare(updateMemberEventAvatarSQL); err != nil {
		return
	}
	if s.selectRoomJoinedUsersStmt, err = db.Prepare(selectRoomJoinedUsersSQL); err != nil {
		return
	}
	//if s.selectRoomStateWithLimitStmt, err = db.Prepare(selectRoomStateWithLimitSQL); err != nil {
	//	return
	//}
	if s.selectRoomStateCountStmt, err = db.Prepare(selectRoomStateCountSQL); err != nil {
		return
	}
	if s.updateRoomStateStmt, err = db.Prepare(updateRoomStateSQL); err != nil {
		return
	}
	if s.selectRoomStateByEventIDStmt, err = db.Prepare(selectRoomStateByEventIDSQL); err != nil {
		return
	}
	return
}

func (s *currentRoomStateStatements) selectRoomJoinedUsers(
	ctx context.Context,
	roomIDs []string, // nolint: unparam
) ([]string, error) {
	rows, err := s.selectRoomJoinedUsersStmt.QueryContext(ctx, pq.Array(roomIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint: errcheck

	var result []string
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, nil
}

// SelectRoomIDsWithMembership returns the list of room IDs which have the given user in the given membership state.
func (s *currentRoomStateStatements) selectRoomIDsWithMembership(
	ctx context.Context,
	userID string,
	memberships []string, // nolint: unparam
) ([]string, []int64, []string, error) {
	rows, err := s.selectRoomIDsWithMembershipStmt.QueryContext(ctx, userID, pq.Array(memberships))
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	var result []string
	var offsets []int64
	var events []string
	for rows.Next() {
		var roomID string
		var addAt int64
		var eventID string
		if err := rows.Scan(&roomID, &addAt, &eventID); err != nil {
			return nil, nil, nil, err
		}
		result = append(result, roomID)
		offsets = append(offsets, addAt)
		events = append(events, eventID)
	}
	return result, offsets, events, nil
}

// CurrentState returns all the current state events for the given room.
func (s *currentRoomStateStatements) selectCurrentState(
	ctx context.Context, roomID string,
) ([]gomatrixserverlib.ClientEvent, []int64, error) {
	rows, err := s.selectCurrentStateStmt.QueryContext(ctx, roomID)
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
		if err := rows.Scan(&eventBytes, &eventType, &offset); err != nil {
			return nil, nil, err
		}

		var ev gomatrixserverlib.ClientEvent

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

func (s *currentRoomStateStatements) upsertRoomState(
	ctx context.Context,
	event gomatrixserverlib.ClientEvent, membership *string, addedAt int64,
) error {
	stateKey := ""
	member := ""
	if event.StateKey != nil {
		stateKey = *event.StateKey
	}
	if membership != nil {
		member = *membership
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncRoomStateUpdateKey
		update.SyncDBEvents.SyncRoomStateUpdate = &dbtypes.SyncRoomStateUpdate{
			RoomId:        event.RoomID,
			EventId:       event.EventID,
			Type:          event.Type,
			EventJson:     eventBytes,
			EventStateKey: stateKey,
			Membership:    member,
			AddPos:        addedAt,
		}
		update.SetUid(int64(common.CalcStringHashCode64(event.RoomID)))
		s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_current_room_state")
		return nil
	}

	if encryption.CheckCrypto(event.Type) {
		_, err = s.upsertRoomStateStmt.ExecContext(
			ctx,
			event.RoomID,
			event.EventID,
			event.Type,
			stateKey,
			encryption.Encrypt(eventBytes),
			member,
			addedAt,
		)
	} else {
		_, err = s.upsertRoomStateStmt.ExecContext(
			ctx,
			event.RoomID,
			event.EventID,
			event.Type,
			stateKey,
			eventBytes,
			member,
			addedAt,
		)
	}
	return err
}

func (s *currentRoomStateStatements) upsertRoomStateRaw(
	ctx context.Context,
	roomId, eventId string, json []byte,
	eventType, stateKey string,
	membership string, addedAt int64,
) error {
	var err error
	if encryption.CheckCrypto(eventType) {
		_, err = s.upsertRoomStateStmt.ExecContext(
			ctx, roomId, eventId, eventType,
			stateKey, encryption.Encrypt(json), membership, addedAt,
		)
	} else {
		_, err = s.upsertRoomStateStmt.ExecContext(
			ctx, roomId, eventId, eventType,
			stateKey, json, membership, addedAt,
		)
	}

	return err
}

func (s *currentRoomStateStatements) upsertRoomStateRaw2(
	ctx context.Context,
	roomId, eventId string, json []byte,
	eventType, stateKey string,
	membership string, addedAt int64,
) error {
	var update dbtypes.DBEvent
	update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
	update.Key = dbtypes.SyncRoomStateUpdateKey

	update.SyncDBEvents.SyncRoomStateUpdate = &dbtypes.SyncRoomStateUpdate{
		RoomId:        roomId,
		EventId:       eventId,
		Type:          eventType,
		EventJson:     json,
		EventStateKey: stateKey,
		Membership:    membership,
		AddPos:        addedAt,
	}
	update.SetUid(int64(common.CalcStringHashCode64(roomId)))
	s.db.WriteDBEventWithTbl(ctx, &update, "syncapi_current_room_state")
	return nil
}

func (s *currentRoomStateStatements) updateMemberEventAvatar(
	ctx context.Context,
	userID, oldAvatar, newAvatar string,
) error {
	_, err := s.updateMemberEventAvatarStmt.ExecContext(
		ctx, oldAvatar, newAvatar, userID,
	)
	return err
}

func (s *currentRoomStateStatements) selectRoomStateWithLimit(
	ctx context.Context, limit, offset int64,
) ([]string, [][]byte, error) {
	return nil, nil, nil

	/*
		rows, err := s.selectRoomStateWithLimitStmt.QueryContext(ctx, limit, offset)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close() // nolint: errcheck

		result := [][]byte{}
		eventIDs := []string{}
		for rows.Next() {
			var eventBytes []byte
			var eventID string
			if err := rows.Scan(&eventID, &eventBytes); err != nil {
				return nil, nil, err
			}

			result = append(result, eventBytes)
			eventIDs = append(eventIDs, eventID)
		}
		return eventIDs, result, nil
	*/
}

func (s *currentRoomStateStatements) selectRoomStateTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectRoomStateCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}

func (s *currentRoomStateStatements) updateRoomStateWithEventID(
	ctx context.Context,
	eventID string,
	eventBytes []byte,
) error {
	_, err := s.updateRoomStateStmt.ExecContext(
		ctx, eventBytes, eventID,
	)
	return err
}

func (s *currentRoomStateStatements) selectRoomStateByEventID(
	ctx context.Context, eventID string,
) (eventBytes []byte, err error) {
	rows, err := s.selectRoomStateByEventIDStmt.QueryContext(ctx, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // nolint: errcheck

	for rows.Next() {
		if err = rows.Scan(&eventBytes); err != nil {
			return nil, err
		}
		return eventBytes, nil
	}
	return eventBytes, nil
}
