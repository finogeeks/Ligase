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

	//"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
)

const membershipSchema = `
-- The membership table is used to coordinate updates between the invite table
-- and the room state tables.
-- This table is updated in one of 3 ways:
--   1) The membership of a user changes within the current state of the room.
--   2) An invite is received outside of a room over federation.
--   3) An invite is rejected outside of a room over federation.
CREATE TABLE IF NOT EXISTS roomserver_membership (
	room_nid BIGINT NOT NULL,
	-- Numeric state key ID for the user ID this state is for.
	target_nid BIGINT NOT NULL DEFAULT 0, 
	-- The numeric ID of the forget event.
	target_forget_nid BIGINT NOT NULL,
	-- Numeric state key ID for the user ID who changed the state.
	-- This may be 0 since it is not always possible to identify the user that
	-- changed the state.
	sender_nid BIGINT NOT NULL DEFAULT 0,
	-- The state the user is in within this room.
	-- Default value is "membershipStateLeaveOrBan"
	membership_nid BIGINT NOT NULL DEFAULT 1,
	-- The numeric ID of the membership event.
	-- It refers to the join membership event if the membership_nid is join (3),
	-- and to the leave/ban membership event if the membership_nid is leave or
	-- ban (1).
	-- If the membership_nid is invite (2) and the user has been in the room
	-- before, it will refer to the previous leave/ban membership event, and will
	-- be equals to 0 (its default) if the user never joined the room before.
	-- This NID is updated if the join event gets updated (e.g. profile update),
	-- or if the user leaves/joins the room.
	event_nid BIGINT NOT NULL DEFAULT 0,
	target_id text NOT NULL DEFAULT '',
    target_forget_id text NOT NULL DEFAULT '',
	sender_id text NOT NULL DEFAULT '',
	room_id text NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 0,
	UNIQUE (room_nid, target_id)
);
`

// Insert a row in to membership table so that it can be locked by the
// SELECT FOR UPDATE
const insertMembershipSQL = "" +
	"INSERT INTO roomserver_membership (room_nid, target_id, target_forget_nid, room_id, membership_nid, event_nid)" +
	" VALUES ($1, $2, -1, $3, $4, $5)" +
	" ON CONFLICT DO NOTHING"

const selectMembershipForUpdateSQL = "" +
	"SELECT membership_nid FROM roomserver_membership" +
	" WHERE room_nid = $1 AND target_id = $2"

const selectMembershipFromRoomAndTargetSQL = "" +
	"SELECT membership_nid, event_nid, target_forget_nid, sender_nid FROM roomserver_membership" +
	" WHERE room_nid = $1 AND target_nid = $2"

const selectMembershipFromRoomSQL = "" +
	"SELECT membership_nid, event_nid, target_forget_nid, sender_nid, target_nid FROM roomserver_membership" +
	" WHERE room_nid = $1"

const selectMembershipsFromRoomAndMembershipSQL = "" +
	"SELECT event_nid FROM roomserver_membership" +
	" WHERE room_nid = $1 AND membership_nid = $2"

const selectMembershipsFromRoomSQL = "" +
	"SELECT event_nid FROM roomserver_membership" +
	" WHERE room_nid = $1"

const updateMembershipSQL = "" +
	"UPDATE roomserver_membership SET sender_id = $3, membership_nid = $4, event_nid = $5, version = $6" +
	" WHERE room_nid = $1 AND target_id = $2 AND version < $6"

const updateMembershipForgetNIDSQL = "" +
	"UPDATE roomserver_membership SET target_forget_nid = $3" +
	" WHERE room_nid = $1 AND target_id = $2 AND target_forget_nid = -1"

const selectMembershipsFromTargetSQL = "" +
	"SELECT room_id, membership_nid FROM roomserver_membership" +
	" WHERE target_id = $1"

type memshipItem struct {
	room_nid          roomservertypes.RoomNID
	target_nid        roomservertypes.EventStateKeyNID
	target_forget_nid roomservertypes.EventNID
	sender_nid        roomservertypes.EventStateKeyNID
	membership_nid    roomservertypes.MembershipState
	event_nid         roomservertypes.EventNID
}

type membershipStatements struct {
	db                                         *Database
	insertMembershipStmt                       *sql.Stmt
	selectMembershipForUpdateStmt              *sql.Stmt
	selectMembershipFromRoomAndTargetStmt      *sql.Stmt
	selectMembershipsFromRoomAndMembershipStmt *sql.Stmt
	selectMembershipsFromRoomStmt              *sql.Stmt
	updateMembershipStmt                       *sql.Stmt
	updateMembershipForgetNIDStmt              *sql.Stmt
	selectMembershipFromRoomStmt               *sql.Stmt
	selectMembershipsFromTargetStmt            *sql.Stmt
}

func (s *membershipStatements) getSchema() string {
	return membershipSchema
}

func (s *membershipStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	return statementList{
		{&s.insertMembershipStmt, insertMembershipSQL},
		{&s.selectMembershipForUpdateStmt, selectMembershipForUpdateSQL},
		{&s.selectMembershipFromRoomAndTargetStmt, selectMembershipFromRoomAndTargetSQL},
		{&s.selectMembershipsFromRoomAndMembershipStmt, selectMembershipsFromRoomAndMembershipSQL},
		{&s.selectMembershipsFromRoomStmt, selectMembershipsFromRoomSQL},
		{&s.updateMembershipStmt, updateMembershipSQL},
		{&s.updateMembershipForgetNIDStmt, updateMembershipForgetNIDSQL},
		{&s.selectMembershipFromRoomStmt, selectMembershipFromRoomSQL},
		{&s.selectMembershipsFromTargetStmt, selectMembershipsFromTargetSQL},
	}.prepare(db)
}

func (s *membershipStatements) insertMembership(
	ctx context.Context,
	roomNID int64, targetUserID, roomID string,
	membership_nid, event_nid int64,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventMembershipInsertKey
		update.RoomDBEvents.EventMembershipInsert = &dbtypes.EventMembershipInsert{
			RoomNID:       roomNID,
			Target:        targetUserID,
			RoomID:        roomID,
			MembershipNID: membership_nid,
			EventNID:      event_nid,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_membership")
		return nil
	}

	return s.membershipInsertdRaw(ctx, roomNID, targetUserID, roomID, membership_nid, event_nid)
}

func (s *membershipStatements) membershipInsertdRaw(
	ctx context.Context,
	roomNid int64, target, roomID string, membership_nid, event_nid int64,
) error {
	_, err := s.insertMembershipStmt.ExecContext(ctx, roomNid, target, roomID, membership_nid, event_nid)
	return err
}

func (s *membershipStatements) selectMembershipForUpdate(
	ctx context.Context,
	txn *sql.Tx, roomNID roomservertypes.RoomNID, targetUserID string,
) (membership roomservertypes.MembershipState, err error) {
	err = s.selectMembershipForUpdateStmt.QueryRowContext(ctx, roomNID, targetUserID).Scan(&membership)
	return
}

func (s *membershipStatements) updateMembership(
	ctx context.Context,
	roomNID int64, targetUserID string,
	senderUserID string, membership roomservertypes.MembershipState,
	eventNID int64,
) error {
	version, _ := s.db.idg.Next()

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventMembershipUpdateKey
		update.RoomDBEvents.EventMembershipUpdate = &dbtypes.EventMembershipUpdate{
			RoomID:     roomNID,
			Target:     targetUserID,
			EventNID:   eventNID,
			Sender:     senderUserID,
			Membership: int64(membership),
			Version:    version,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_membership")
		return nil
	}

	return s.membershipUpdateRaw(ctx, roomNID, targetUserID, senderUserID, int64(membership), eventNID, version)
}

func (s *membershipStatements) membershipUpdateRaw(
	ctx context.Context,
	roomNid int64, target, sender string, membership, eventID, version int64,
) error {
	_, err := s.updateMembershipStmt.ExecContext(
		ctx, roomNid, target, sender, membership, eventID, version,
	)
	return err
}

func (s *membershipStatements) updateMembershipForgetNID(
	ctx context.Context,
	roomNID int64, targetUser string,
	eventNID int64,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventMembershipForgetUpdateKey
		update.RoomDBEvents.EventMembershipForgetUpdate = &dbtypes.EventMembershipForgetUpdate{
			RoomID:   roomNID,
			Target:   targetUser,
			ForgetID: eventNID,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_membership")
		return nil
	}

	return s.membershipForgetUpdateRaw(ctx, roomNID, targetUser, eventNID)
}

func (s *membershipStatements) membershipForgetUpdateRaw(
	ctx context.Context,
	roomNid int64, target string, forgetNid int64,
) error {
	_, err := s.updateMembershipForgetNIDStmt.ExecContext(
		ctx, roomNid, target, forgetNid,
	)

	return err
}

func (s *membershipStatements) selectMembershipsFromTarget(
	ctx context.Context, targetUser string,
) (roomMap map[string]int64, err error) {
	res := make(map[string]int64)
	rows, err := s.selectMembershipsFromTargetStmt.QueryContext(ctx, targetUser)
	if err != nil {
		return res, err
	}

	for rows.Next() {
		var roomId string
		var membershipNid int64
		if err = rows.Scan(&roomId, &membershipNid); err != nil {
			return
		}
		res[roomId] = membershipNid
	}
	return res, nil
}
