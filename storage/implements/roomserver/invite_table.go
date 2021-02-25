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
)

const inviteSchema = `
CREATE TABLE IF NOT EXISTS roomserver_invites (
	-- The string ID of the invite event itself.
	-- We can't use a numeric event ID here because we don't always have
	-- enough information to store an invite in the event table.
	-- In particular we don't always have a chain of auth_events for invites
	-- received over federation.
	invite_event_id TEXT PRIMARY KEY,
	-- The numeric ID of the room the invite m.room.member event is in.
	room_nid BIGINT NOT NULL,
	-- The numeric ID for the state key of the invite m.room.member event.
	-- This tells us who the invite is for.
	-- This is used to query the active invites for a user.
	target_nid BIGINT NOT NULL DEFAULT 0,
	-- The numeric ID for the sender of the invite m.room.member event.
	-- This tells us who sent the invite.
	-- This is used to work out which matrix server we should talk to when
	-- we try to join the room.
	sender_nid BIGINT NOT NULL DEFAULT 0,
	-- This is used to track whether the invite is still active.
	-- This is set implicitly when processing new join and leave events and
	-- explicitly when rejecting events over federation.
	retired BOOLEAN NOT NULL DEFAULT FALSE,
	-- The invite event JSON.
	invite_event_json TEXT NOT NULL,
	target_id text NOT NULL DEFAULT '',
	sender_id text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS roomserver_invites_active_idx ON roomserver_invites (target_id, room_nid)
	WHERE NOT retired;
`
const insertInviteEventSQL = "" +
	"INSERT INTO roomserver_invites (invite_event_id, room_nid, target_id," +
	" sender_id, invite_event_json) VALUES ($1, $2, $3, $4, $5)" +
	" ON CONFLICT DO NOTHING"

// Retire every active invite for a user in a room.
// Ideally we'd know which invite events were retired by a given update so we
// wouldn't need to remove every active invite.
// However the matrix protocol doesn't give us a way to reliably identify the
// invites that were retired, so we are forced to retire all of them.
const updateInviteRetiredSQL = "" +
	"UPDATE roomserver_invites SET retired = TRUE" +
	" WHERE room_nid = $1 AND target_id = $2 AND NOT retired" +
	" RETURNING invite_event_id"

type inviteStatements struct {
	db                      *Database
	insertInviteEventStmt   *sql.Stmt
	updateInviteRetiredStmt *sql.Stmt
}

type inviteItem struct {
	invite_event_id string
	sender_nid      int64
}

func (s *inviteStatements) getSchema() string {
	return inviteSchema
}

func (s *inviteStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	return statementList{
		{&s.insertInviteEventStmt, insertInviteEventSQL},
		{&s.updateInviteRetiredStmt, updateInviteRetiredSQL},
	}.prepare(db)
}

func (s *inviteStatements) insertInviteEvent(
	ctx context.Context,
	inviteEventID string, roomNID int64,
	targetUser, senderUser string,
	inviteEventJSON []byte,
) (bool, error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventInviteInsertKey
		update.RoomDBEvents.EventInviteInsert = &dbtypes.EventInviteInsert{
			RoomNid: roomNID,
			EventId: inviteEventID,
			Target:  targetUser,
			Sender:  senderUser,
			Content: inviteEventJSON,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_invites")
		return true, nil
	}

	return true, s.insertInviteEventRaw(ctx, inviteEventID, roomNID, targetUser, senderUser, inviteEventJSON)
}

func (s *inviteStatements) insertInviteEventRaw(
	ctx context.Context,
	inviteEventID string, roomNID int64, targetUser, senderUser string,
	inviteEventJSON []byte,
) error {
	result, err := s.insertInviteEventStmt.ExecContext(
		ctx, inviteEventID, roomNID, targetUser, senderUser, inviteEventJSON,
	)
	if err != nil {
		return err
	}
	_, err = result.RowsAffected()
	if err != nil {
		return err
	}
	return nil
}

func (s *inviteStatements) updateInviteRetired(
	ctx context.Context,
	roomNID int64, targetUser string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventInviteUpdateKey
		update.RoomDBEvents.EventInviteUpdate = &dbtypes.EventInviteUpdate{
			RoomNid: roomNID,
			Target:  targetUser,
		}
		update.SetUid(roomNID)
		s.db.WriteDBEventWithTbl(&update, "roomserver_invites")

		return nil
	}

	return s.updateInviteRetiredRaw(ctx, roomNID, targetUser)
}

func (s *inviteStatements) updateInviteRetiredRaw(
	ctx context.Context, roomNID int64, targetUser string,
) error {
	_, err := s.updateInviteRetiredStmt.ExecContext(ctx, roomNID, targetUser)

	return err
}
