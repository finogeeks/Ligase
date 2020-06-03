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

package federation

import (
	"context"
	"database/sql"
)

const joinedRoomsSchema = `
CREATE TABLE IF NOT EXISTS federation_joined_rooms (
    room_id TEXT NOT NULL,
	event_id TEXT NOT NULL,
	recv_offsets TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS federation_joined_rooms_event_id_idx
    ON federation_joined_rooms (event_id);

CREATE INDEX IF NOT EXISTS federation_joined_rooms_room_id_idx
    ON federation_joined_rooms (room_id)
`

const insertJoinedRoomsSQL = "" +
	"INSERT INTO federation_joined_rooms (room_id, event_id, recv_offsets)" +
	" VALUES ($1, $2, '')"

const updateJoinedRoomsRecvOffsetSQL = "" +
	"UPDATE federation_joined_rooms SET recv_offsets = $2 WHERE room_id = $1"

const selectJoinedRoomsSQL = "" +
	"SELECT event_id, recv_offsets FROM federation_joined_rooms WHERE room_id = $1"

type joinedRoomsStatements struct {
	insertJoinedRoomsStmt           *sql.Stmt
	updateJOinedRoomsRecvOffsetStmt *sql.Stmt
	selectJoinedRoomsStmt           *sql.Stmt
}

func (s *joinedRoomsStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(joinedRoomsSchema)
	if err != nil {
		return err
	}
	if s.insertJoinedRoomsStmt, err = db.Prepare(insertJoinedRoomsSQL); err != nil {
		return
	}
	if s.updateJOinedRoomsRecvOffsetStmt, err = db.Prepare(updateJoinedRoomsRecvOffsetSQL); err != nil {
		return
	}
	if s.selectJoinedRoomsStmt, err = db.Prepare(selectJoinedRoomsSQL); err != nil {
		return
	}
	return
}

func (s *joinedRoomsStatements) insertJoinedRooms(
	ctx context.Context,
	roomID, eventID string,
) error {
	_, err := s.insertJoinedRoomsStmt.ExecContext(ctx, roomID, eventID)
	return err
}

func (s *joinedRoomsStatements) updateJoinedRoomsRecvOffset(ctx context.Context, roomID, recvOffset string) error {
	_, err := s.updateJOinedRoomsRecvOffsetStmt.ExecContext(ctx, roomID, recvOffset)
	return err
}

func (s *joinedRoomsStatements) selectJoinedRooms(
	ctx context.Context, roomID string,
) (string, string, error) {
	var eventID string
	var recvOffsets string
	err := s.selectJoinedRoomsStmt.QueryRowContext(ctx, roomID).Scan(&eventID, &recvOffsets)
	return eventID, recvOffsets, err
}
