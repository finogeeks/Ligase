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

package rcs_server

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/plugins/message/external"

	"github.com/finogeeks/ligase/model/types"
)

const friendshipSchema = `
CREATE TABLE IF NOT EXISTS rcsserver_friendship (
    _id TEXT PRIMARY KEY DEFAULT '',
	room_id TEXT NOT NULL DEFAULT '',
	fcid TEXT NOT NULL DEFAULT '',
	to_fcid TEXT NOT NULL DEFAULT '',
	fcid_state TEXT NOT NULL DEFAULT '',
	to_fcid_state TEXT NOT NULL DEFAULT '',
	fcid_is_bot BOOLEAN NOT NULL DEFAULT FALSE,
	to_fcid_is_bot BOOLEAN NOT NULL DEFAULT FALSE,
	fcid_remark TEXT NOT NULL DEFAULT '',
	to_fcid_remark TEXT NOT NULL DEFAULT '',
	fcid_once_joined BOOLEAN NOT NULL DEFAULT FALSE,
	to_fcid_once_joined BOOLEAN NOT NULL DEFAULT FALSE,
	fcid_domain TEXT NOT NULL DEFAULT '',
	to_fcid_domain TEXT NOT NULL DEFAULT '',
	event_id TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS rcsserver_friendship_room_id_idx ON rcsserver_friendship(room_id);
CREATE INDEX IF NOT EXISTS rcsserver_friendship_fcid_idx ON rcsserver_friendship(fcid);
CREATE INDEX IF NOT EXISTS rcsserver_friendship_to_fcid_idx ON rcsserver_friendship(to_fcid);
CREATE INDEX IF NOT EXISTS rcsserver_friendship_fcid_state_idx ON rcsserver_friendship(fcid_state);
CREATE INDEX IF NOT EXISTS rcsserver_friendship_to_fcid_state_idx ON rcsserver_friendship(to_fcid_state);
CREATE INDEX IF NOT EXISTS rcsserver_friendship_fcid_is_bot_idx ON rcsserver_friendship(fcid_is_bot);
CREATE INDEX IF NOT EXISTS rcsserver_friendship_to_fcid_is_bot_idx ON rcsserver_friendship(to_fcid_is_bot);
`

const insertFriendshipSQL = "" +
	"INSERT INTO rcsserver_friendship" +
	" (_id, room_id, fcid, to_fcid, fcid_state, to_fcid_state, fcid_is_bot, to_fcid_is_bot, fcid_remark, to_fcid_remark," +
	" fcid_once_joined, to_fcid_once_joined, fcid_domain, to_fcid_domain, event_id)" +
	" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)"

const selectFriendshipByRoomIDSQL = "" +
	"SELECT * FROM rcsserver_friendship" +
	" WHERE room_id = $1"

const selectFriendshipByFcIDAndToFcIDSQL = "" +
	"SELECT * FROM rcsserver_friendship" +
	" WHERE fcid = $1 and to_fcid = $2"

const selectFriendshipsByFcIDOrToFcIDSQL = "" +
	"SELECT room_id, fcid, to_fcid, to_fcid_is_bot, to_fcid_remark FROM rcsserver_friendship" +
	" WHERE fcid = $1 AND fcid_state = 'join'" +
	" AND ((to_fcid_state = 'join' AND to_fcid_once_joined = FALSE) OR to_fcid_once_joined = TRUE)" +
	" UNION" +
	" SELECT room_id, to_fcid, fcid, fcid_is_bot, fcid_remark FROM rcsserver_friendship" +
	" WHERE to_fcid = $1 AND to_fcid_state = 'join'" +
	" AND ((fcid_state = 'join' AND fcid_once_joined = FALSE) OR fcid_once_joined = TRUE)"

const selectFriendshipsByFcIDOrToFcIDWithBotSQL = "" +
	"SELECT room_id, fcid, to_fcid, to_fcid_is_bot, to_fcid_remark FROM rcsserver_friendship" +
	" WHERE fcid = $1 AND fcid_state = 'join' AND to_fcid_is_bot = TRUE" +
	" AND ((to_fcid_state = 'join' AND to_fcid_once_joined = FALSE) OR to_fcid_once_joined = TRUE)" +
	" UNION" +
	" SELECT room_id, to_fcid, fcid, fcid_is_bot, fcid_remark FROM rcsserver_friendship" +
	" WHERE to_fcid = $1 AND to_fcid_state = 'join' AND fcid_is_bot = TRUE" +
	" AND ((fcid_state = 'join' AND fcid_once_joined = FALSE) OR fcid_once_joined = TRUE)"

const updateFriendshipByRoomIDSQL = "" +
	"UPDATE rcsserver_friendship" +
	" SET _id = $1, fcid = $3, to_fcid = $4, fcid_state = $5, to_fcid_state = $6, fcid_is_bot = $7, to_fcid_is_bot = $8," +
	" fcid_remark = $9, to_fcid_remark = $10, fcid_once_joined = $11, to_fcid_once_joined = $12, fcid_domain = $13, to_fcid_domain = $14, event_id = $15" +
	" WHERE room_id = $2"

const selectFriendshipByFcIDOrToFcIDSQL = "" +
	"SELECT room_id, fcid, to_fcid, fcid_state, to_fcid_state FROM rcsserver_friendship" +
	" WHERE (fcid = $1 AND to_fcid = $2)" +
	" OR (to_fcid = $1 AND fcid = $2)" +
	" ORDER BY room_id"

const deleteFriendshipByRoomIDSQL = "" +
	"DELETE FROM rcsserver_friendship" +
	" WHERE room_id = $1"

type friendshipStatements struct {
	db                                         *Database
	insertFriendshipStmt                       *sql.Stmt
	selectFriendshipByRoomIDStmt               *sql.Stmt
	selectFriendshipByFcIDAndToFcIDStmt        *sql.Stmt
	selectFriendshipsByFcIDOrToFcIDStmt        *sql.Stmt
	selectFriendshipsByFcIDOrToFcIDWithBotStmt *sql.Stmt
	selectFriendshipByFcIDOrToFcIDStmt         *sql.Stmt
	updateFriendshipByRoomIDStmt               *sql.Stmt
	deleteFriendshipByRoomIDStmt               *sql.Stmt
}

func (s *friendshipStatements) getSchema() string {
	return friendshipSchema
}

func (s *friendshipStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	_, err = db.Exec(friendshipSchema)
	if err != nil {
		return
	}

	return statementList{
		{&s.insertFriendshipStmt, insertFriendshipSQL},
		{&s.selectFriendshipByRoomIDStmt, selectFriendshipByRoomIDSQL},
		{&s.selectFriendshipByFcIDAndToFcIDStmt, selectFriendshipByFcIDAndToFcIDSQL},
		{&s.selectFriendshipsByFcIDOrToFcIDStmt, selectFriendshipsByFcIDOrToFcIDSQL},
		{&s.selectFriendshipsByFcIDOrToFcIDWithBotStmt, selectFriendshipsByFcIDOrToFcIDWithBotSQL},
		{&s.selectFriendshipByFcIDOrToFcIDStmt, selectFriendshipByFcIDOrToFcIDSQL},
		{&s.updateFriendshipByRoomIDStmt, updateFriendshipByRoomIDSQL},
		{&s.deleteFriendshipByRoomIDStmt, deleteFriendshipByRoomIDSQL},
	}.prepare(db)
}

func (s *friendshipStatements) insertFriendship(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string,
) error {
	_, err := s.insertFriendshipStmt.ExecContext(
		ctx, ID, roomID, fcID, toFcID, fcIDState, toFcIDState,
		fcIDIsBot, toFcIDIsBot, fcIDRemark, toFcIDRemark,
		fcIDOnceJoined, toFcIDOnceJoined, fcIDDomain, toFcIDDomain, eventID)
	return err
}

func (s *friendshipStatements) selectFriendshipByRoomID(
	ctx context.Context, roomID string,
) (*types.RCSFriendship, error) {
	var f types.RCSFriendship
	err := s.selectFriendshipByRoomIDStmt.QueryRowContext(ctx, roomID).Scan(
		&f.ID, &f.RoomID, &f.FcID, &f.ToFcID, &f.FcIDState, &f.ToFcIDState,
		&f.FcIDIsBot, &f.ToFcIDIsBot, &f.FcIDRemark, &f.ToFcIDRemark,
		&f.FcIDOnceJoined, &f.ToFcIDOnceJoined, &f.FcIDDomain, &f.ToFcIDDomain, &f.EventID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *friendshipStatements) selectFriendshipByFcIDAndToFcID(
	ctx context.Context, fcID, toFcID string,
) (*types.RCSFriendship, error) {
	var f types.RCSFriendship
	err := s.selectFriendshipByFcIDAndToFcIDStmt.QueryRowContext(ctx, fcID, toFcID).Scan(
		&f.ID, &f.RoomID, &f.FcID, &f.ToFcID, &f.FcIDState, &f.ToFcIDState,
		&f.FcIDIsBot, &f.ToFcIDIsBot, &f.FcIDRemark, &f.ToFcIDRemark,
		&f.FcIDOnceJoined, &f.ToFcIDOnceJoined, &f.FcIDDomain, &f.ToFcIDDomain, &f.EventID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *friendshipStatements) selectFriendshipByFcIDOrToFcID(
	ctx context.Context, fcID, toFcID string,
) (*external.GetFriendshipResponse, error) {
	var f external.GetFriendshipResponse
	err := s.selectFriendshipByFcIDOrToFcIDStmt.QueryRowContext(ctx, fcID, toFcID).Scan(
		&f.RoomID, &f.FcID, &f.ToFcID, &f.FcIDState, &f.ToFcIDState)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *friendshipStatements) selectFriendshipsByFcIDOrToFcID(
	ctx context.Context, userID string,
) ([]external.Friendship, error) {
	rows, err := s.selectFriendshipsByFcIDOrToFcIDStmt.QueryContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	var resps []external.Friendship
	for rows.Next() {
		var resp external.Friendship
		if err = rows.Scan(&resp.RoomID, &resp.FcID, &resp.ToFcID, &resp.IsBot, &resp.Remark); err != nil {
			return nil, err
		}
		resps = append(resps, resp)
	}
	return resps, nil
}

func (s *friendshipStatements) selectFriendshipsByFcIDOrToFcIDWithBot(
	ctx context.Context, userID string,
) ([]external.Friendship, error) {
	rows, err := s.selectFriendshipsByFcIDOrToFcIDWithBotStmt.QueryContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	var resps []external.Friendship
	for rows.Next() {
		var resp external.Friendship
		if err = rows.Scan(&resp.RoomID, &resp.FcID, &resp.ToFcID, &resp.IsBot, &resp.Remark); err != nil {
			return nil, err
		}
		resps = append(resps, resp)
	}
	return resps, nil
}

func (s *friendshipStatements) updateFriendshipByRoomID(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string,
) error {
	_, err := s.updateFriendshipByRoomIDStmt.ExecContext(
		ctx, ID, roomID, fcID, toFcID, fcIDState, toFcIDState,
		fcIDIsBot, toFcIDIsBot, fcIDRemark, toFcIDRemark,
		fcIDOnceJoined, toFcIDOnceJoined, fcIDDomain, toFcIDDomain, eventID,
	)
	return err
}

func (s *friendshipStatements) deleteFriendshipByRoomID(
	ctx context.Context, roomID string,
) error {
	_, err := s.deleteFriendshipByRoomIDStmt.ExecContext(
		ctx, roomID,
	)
	return err
}
