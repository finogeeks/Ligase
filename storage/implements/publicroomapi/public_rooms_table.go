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

package publicroomapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/publicroomstypes"

	"github.com/lib/pq"
)

var editableAttributes = []string{
	"aliases",
	"canonical_alias",
	"name",
	"topic",
	"world_readable",
	"guest_can_join",
	"avatar_url",
	"visibility",
}

const publicRoomsSchema = `
-- Stores all of the rooms with data needed to create the server's room directory
CREATE SEQUENCE IF NOT EXISTS public_room_api_seq;

CREATE TABLE IF NOT EXISTS publicroomsapi_public_rooms(
	-- The room's ID
	room_id TEXT NOT NULL PRIMARY KEY,
	-- sequence id
	seq_id BIGINT NOT NULL,
	-- Number of joined members in the room
	joined_members INTEGER NOT NULL,
	-- Aliases of the room (empty array if none)
	aliases TEXT[] NOT NULL,
	-- Canonical alias of the room (empty string if none)
	canonical_alias TEXT NOT NULL,
	-- Name of the room (empty string if none)
	name TEXT NOT NULL,
	-- Topic of the room (empty string if none)
	topic TEXT NOT NULL,
	-- Is the room world readable?
	world_readable BOOLEAN NOT NULL,
	-- Can guest join the room?
	guest_can_join BOOLEAN NOT NULL,
	-- URL of the room avatar (empty string if none)
	avatar_url TEXT NOT NULL,
	-- Visibility of the room: true means the room is publicly visible, false
	-- means the room is private
	visibility BOOLEAN NOT NULL
);
`

const insertNewRoomSQL = "" +
	"INSERT INTO publicroomsapi_public_rooms(room_id, seq_id, joined_members, aliases, canonical_alias, name, topic, world_readable, guest_can_join, avatar_url, visibility)" +
	" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING"

const incrementJoinedMembersInRoomSQL = "" +
	"UPDATE publicroomsapi_public_rooms" +
	" SET joined_members = joined_members + 1" +
	" WHERE room_id = $1"

const decrementJoinedMembersInRoomSQL = "" +
	"UPDATE publicroomsapi_public_rooms" +
	" SET joined_members = joined_members - 1" +
	" WHERE room_id = $1"

const updateRoomAttributeSQL = "" +
	"UPDATE publicroomsapi_public_rooms" +
	" SET %s = $1" +
	" WHERE room_id = $2"

const countPublicRoomsSQL = "" +
	"SELECT COUNT(*) FROM publicroomsapi_public_rooms" +
	" WHERE visibility = true"

const selectPublicRoomsSQL = "" +
	"SELECT room_id, joined_members, aliases, canonical_alias, name, topic, world_readable, guest_can_join, avatar_url" +
	" FROM publicroomsapi_public_rooms WHERE visibility = true" +
	" ORDER BY joined_members DESC" +
	" OFFSET $1"

const selectPublicRoomsWithLimitSQL = "" +
	"SELECT room_id, joined_members, aliases, canonical_alias, name, topic, world_readable, guest_can_join, avatar_url" +
	" FROM publicroomsapi_public_rooms WHERE visibility = true" +
	" ORDER BY joined_members DESC" +
	" OFFSET $1 LIMIT $2"

const selectPublicRoomsWithFilterSQL = "" +
	"SELECT room_id, joined_members, aliases, canonical_alias, name, topic, world_readable, guest_can_join, avatar_url" +
	" FROM publicroomsapi_public_rooms" +
	" WHERE visibility = true" +
	" AND (LOWER(name) LIKE LOWER($1)" +
	" OR LOWER(topic) LIKE LOWER($1)" +
	" OR LOWER(ARRAY_TO_STRING(aliases, ',')) LIKE LOWER($1))" +
	" ORDER BY joined_members DESC" +
	" OFFSET $2"

const selectPublicRoomsWithLimitAndFilterSQL = "" +
	"SELECT room_id, joined_members, aliases, canonical_alias, name, topic, world_readable, guest_can_join, avatar_url" +
	" FROM publicroomsapi_public_rooms" +
	" WHERE visibility = true" +
	" AND (LOWER(name) LIKE LOWER($1)" +
	" OR LOWER(topic) LIKE LOWER($1)" +
	" OR LOWER(ARRAY_TO_STRING(aliases, ',')) LIKE LOWER($1))" +
	" ORDER BY joined_members DESC" +
	" OFFSET $2 LIMIT $3"

type publicRoomsStatements struct {
	db                                      *Database
	countPublicRoomsStmt                    *sql.Stmt
	insertNewRoomStmt                       *sql.Stmt
	incrementJoinedMembersInRoomStmt        *sql.Stmt
	decrementJoinedMembersInRoomStmt        *sql.Stmt
	selectPublicRoomsStmt                   *sql.Stmt
	selectPublicRoomsWithLimitStmt          *sql.Stmt
	selectPublicRoomsWithFilterStmt         *sql.Stmt
	selectPublicRoomsWithLimitAndFilterStmt *sql.Stmt
	updateRoomAttributeStmts                map[string]*sql.Stmt
}

func (s *publicRoomsStatements) getSchema() string {
	return publicRoomsSchema
}

// nolint: safesql
func (s *publicRoomsStatements) prepare(d *Database) (err error) {
	s.db = d

	stmts := statementList{
		{&s.countPublicRoomsStmt, countPublicRoomsSQL},
		{&s.selectPublicRoomsStmt, selectPublicRoomsSQL},
		{&s.selectPublicRoomsWithLimitStmt, selectPublicRoomsWithLimitSQL},
		{&s.selectPublicRoomsWithFilterStmt, selectPublicRoomsWithFilterSQL},
		{&s.selectPublicRoomsWithLimitAndFilterStmt, selectPublicRoomsWithLimitAndFilterSQL},
		{&s.insertNewRoomStmt, insertNewRoomSQL},
		{&s.incrementJoinedMembersInRoomStmt, incrementJoinedMembersInRoomSQL},
		{&s.decrementJoinedMembersInRoomStmt, decrementJoinedMembersInRoomSQL},
	}

	if err = stmts.prepare(d.db); err != nil {
		return
	}

	s.updateRoomAttributeStmts = make(map[string]*sql.Stmt)
	for _, editable := range editableAttributes {
		stmt := fmt.Sprintf(updateRoomAttributeSQL, editable)
		if s.updateRoomAttributeStmts[editable], err = d.db.Prepare(stmt); err != nil {
			return
		}
	}

	return
}

func (s *publicRoomsStatements) insertNewRoom(
	ctx context.Context,
	roomID string,
	joinedMembers int64,
	aliases []string,
	canonicalAlias,
	name,
	topic string,
	worldReadable,
	guestCanJoin bool,
	avatarUrl string,
	visibility bool,
) error {
	seqID, err := s.db.idg.Next()
	if err != nil {
		return err
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUBLICROOM_DB_EVENT
		update.Key = dbtypes.PublicRoomInsertKey
		update.PublicRoomDBEvents.PublicRoomInsert = &dbtypes.PublicRoomInsert{
			RoomID:         roomID,
			SeqID:          seqID,
			JoinedMembers:  joinedMembers,
			Aliases:        aliases,
			CanonicalAlias: canonicalAlias,
			Name:           name,
			Topic:          topic,
			WorldReadable:  worldReadable,
			GuestCanJoin:   guestCanJoin,
			AvatarUrl:      avatarUrl,
			Visibility:     visibility,
		}
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		return s.db.WriteDBEventWithTbl(&update, "publicroomsapi_public_rooms")
	}

	return s.onInsertNewRoom(ctx, roomID, seqID, joinedMembers, pq.StringArray(aliases), canonicalAlias, name, topic, worldReadable, guestCanJoin, avatarUrl, visibility)
}

func (s *publicRoomsStatements) onInsertNewRoom(
	ctx context.Context,
	roomID string,
	seqID,
	joinedMembers int64,
	aliases []string,
	canonicalAlias,
	name,
	topic string,
	worldReadable,
	guestCanJoin bool,
	avatarUrl string,
	visibility bool,
) error {
	_, err := s.insertNewRoomStmt.ExecContext(ctx, roomID, seqID, joinedMembers, pq.StringArray(aliases), canonicalAlias, name, topic, worldReadable, guestCanJoin, avatarUrl, visibility)
	return err
}

func (s *publicRoomsStatements) incrementJoinedMembersInRoom(
	ctx context.Context, roomID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUBLICROOM_DB_EVENT
		update.Key = dbtypes.PublicRoomIncrementJoinedKey
		update.PublicRoomDBEvents.PublicRoomJoined = &roomID
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		return s.db.WriteDBEventWithTbl(&update, "publicroomsapi_public_rooms")
	}

	return s.onIncrementJoinedMembersInRoom(ctx, roomID)
}

func (s *publicRoomsStatements) onIncrementJoinedMembersInRoom(
	ctx context.Context, roomID string,
) error {
	_, err := s.incrementJoinedMembersInRoomStmt.ExecContext(ctx, roomID)
	return err
}

func (s *publicRoomsStatements) decrementJoinedMembersInRoom(
	ctx context.Context, roomID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUBLICROOM_DB_EVENT
		update.Key = dbtypes.PublicRoomDecrementJoinedKey
		update.PublicRoomDBEvents.PublicRoomJoined = &roomID
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		return s.db.WriteDBEventWithTbl(&update, "publicroomsapi_public_rooms")
	}

	return s.onDecrementJoinedMembersInRoom(ctx, roomID)
}

func (s *publicRoomsStatements) onDecrementJoinedMembersInRoom(
	ctx context.Context, roomID string,
) error {
	_, err := s.decrementJoinedMembersInRoomStmt.ExecContext(ctx, roomID)
	return err
}

func (s *publicRoomsStatements) updateRoomAttribute(
	ctx context.Context, attrName string, attrValue interface{}, roomID string,
) error {
	_, isEditable := s.updateRoomAttributeStmts[attrName]
	if !isEditable {
		return errors.New("Cannot edit " + attrName)
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUBLICROOM_DB_EVENT
		update.Key = dbtypes.PublicRoomUpdateKey
		update.PublicRoomDBEvents.PublicRoomUpdate = &dbtypes.PublicRoomUpdate{
			RoomID:    roomID,
			AttrName:  attrName,
			AttrValue: attrValue,
		}
		update.SetUid(int64(common.CalcStringHashCode64(roomID)))
		return s.db.WriteDBEventWithTbl(&update, "publicroomsapi_public_rooms")
	}

	return s.onUpdateRoomAttribute(ctx, attrName, attrValue, roomID)
}

func (s *publicRoomsStatements) onUpdateRoomAttribute(
	ctx context.Context, attrName string, attrValue interface{}, roomID string,
) error {
	stmt := s.updateRoomAttributeStmts[attrName]

	switch attrName {
	case "canonical_alias", "name", "topic", "avatar_url":
		value := attrValue.(string)
		_, err := stmt.ExecContext(ctx, value, roomID)
		return err
	case "world_readable", "guest_can_join", "visibility":
		value := attrValue.(bool)
		_, err := stmt.ExecContext(ctx, value, roomID)
		return err
	case "aliases":
		value := attrValue.([]interface{})
		aliases := make([]string, len(value))
		for i := 0; i < len(aliases); i++ {
			aliases[i] = value[i].(string)
		}
		_, err := stmt.ExecContext(ctx, pq.StringArray(aliases), roomID)
		return err
	default:
		return nil
	}
}

func (s *publicRoomsStatements) countPublicRooms(ctx context.Context) (nb int64, err error) {
	err = s.countPublicRoomsStmt.QueryRowContext(ctx).Scan(&nb)
	return
}

func (s *publicRoomsStatements) selectPublicRooms(
	ctx context.Context, offset int64, limit int64, filter string,
) ([]publicroomstypes.PublicRoom, error) {
	var rows *sql.Rows
	var err error

	if len(filter) > 0 {
		pattern := "%" + filter + "%"
		if limit == 0 {
			rows, err = s.selectPublicRoomsWithFilterStmt.QueryContext(
				ctx, pattern, offset,
			)
		} else {
			rows, err = s.selectPublicRoomsWithLimitAndFilterStmt.QueryContext(
				ctx, pattern, offset, limit,
			)
		}
	} else {
		if limit == 0 {
			rows, err = s.selectPublicRoomsStmt.QueryContext(ctx, offset)
		} else {
			rows, err = s.selectPublicRoomsWithLimitStmt.QueryContext(
				ctx, offset, limit,
			)
		}
	}

	if err != nil {
		return []publicroomstypes.PublicRoom{}, nil
	}

	var rooms []publicroomstypes.PublicRoom
	rooms = []publicroomstypes.PublicRoom{}
	for rows.Next() {
		var r publicroomstypes.PublicRoom
		var aliases pq.StringArray

		err = rows.Scan(
			&r.RoomID, &r.NumJoinedMembers, &aliases, &r.CanonicalAlias,
			&r.Name, &r.Topic, &r.WorldReadable, &r.GuestCanJoin, &r.AvatarURL,
		)
		if err != nil {
			return rooms, err
		}

		r.Aliases = aliases

		rooms = append(rooms, r)
	}

	return rooms, nil
}
