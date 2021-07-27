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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
)

const roomAliasesSchema = `
-- Stores room aliases and room IDs they refer to
CREATE TABLE IF NOT EXISTS roomserver_room_aliases (
    -- Alias of the room
    alias TEXT NOT NULL PRIMARY KEY,
    -- Room ID the alias refers to
    room_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS roomserver_room_id_idx ON roomserver_room_aliases(room_id);
`

const insertRoomAliasSQL = "" +
	"INSERT INTO roomserver_room_aliases (alias, room_id) VALUES ($1, $2) ON CONFLICT DO NOTHING"

const selectRoomIDFromAliasSQL = "" +
	"SELECT room_id FROM roomserver_room_aliases WHERE alias = $1"

const selectAliasesFromRoomIDSQL = "" +
	"SELECT alias FROM roomserver_room_aliases WHERE room_id = $1"

const deleteRoomAliasSQL = "" +
	"DELETE FROM roomserver_room_aliases WHERE alias = $1"

const selectAllAliasesSQL = "" +
	"SELECT alias FROM roomserver_room_aliases LIMIT $1 OFFSET $2"

type roomAliasesStatements struct {
	db                          *Database
	insertRoomAliasStmt         *sql.Stmt
	selectRoomIDFromAliasStmt   *sql.Stmt
	selectAliasesFromRoomIDStmt *sql.Stmt
	deleteRoomAliasStmt         *sql.Stmt
	selectAllAliasesStmt        *sql.Stmt
}

func (s *roomAliasesStatements) getSchema() string {
	return roomAliasesSchema
}

func (s *roomAliasesStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	return statementList{
		{&s.insertRoomAliasStmt, insertRoomAliasSQL},
		{&s.selectRoomIDFromAliasStmt, selectRoomIDFromAliasSQL},
		{&s.selectAliasesFromRoomIDStmt, selectAliasesFromRoomIDSQL},
		{&s.deleteRoomAliasStmt, deleteRoomAliasSQL},
		{&s.selectAllAliasesStmt, selectAllAliasesSQL},
	}.prepare(db)
}

func (s *roomAliasesStatements) insertRoomAlias(
	ctx context.Context, alias string, roomID string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.AliasInsertKey
		update.RoomDBEvents.AliaseInsert = &dbtypes.AliaseInsert{
			Alias:  alias,
			RoomID: roomID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(alias)))
		s.db.WriteDBEventWithTbl(&update, "roomserver_room_aliases")
		return nil
	}

	return s.insertRoomAliasRaw(ctx, alias, roomID)
}

func (s *roomAliasesStatements) insertRoomAliasRaw(
	ctx context.Context, alias string, roomID string,
) (err error) {
	_, err = s.insertRoomAliasStmt.ExecContext(ctx, alias, roomID)
	return
}

func (s *roomAliasesStatements) selectRoomIDFromAlias(
	ctx context.Context, alias string,
) (roomID string, err error) {
	err = s.selectRoomIDFromAliasStmt.QueryRowContext(ctx, alias).Scan(&roomID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return
}

func (s *roomAliasesStatements) selectAliasesFromRoomID(
	ctx context.Context, roomID string,
) (aliases []string, err error) {
	aliases = []string{}
	rows, err := s.selectAliasesFromRoomIDStmt.QueryContext(ctx, roomID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var alias string
		if err = rows.Scan(&alias); err != nil {
			return
		}

		aliases = append(aliases, alias)
	}

	return
}

func (s *roomAliasesStatements) deleteRoomAlias(
	ctx context.Context, alias string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.AliasDeleteKey
		update.RoomDBEvents.AliaseDelete = &dbtypes.AliaseDelete{
			Alias: alias,
		}
		update.SetUid(int64(common.CalcStringHashCode64(alias)))
		s.db.WriteDBEventWithTbl(&update, "roomserver_room_aliases")
		return nil
	}

	return s.deleteRoomAliasRaw(ctx, alias)
}

func (s *roomAliasesStatements) deleteRoomAliasRaw(
	ctx context.Context, alias string,
) (err error) {
	_, err = s.deleteRoomAliasStmt.ExecContext(ctx, alias)
	return
}

func (s *roomAliasesStatements) selectAllAliases(
	ctx context.Context, limit, offset int,
) ([]string, int, error) {
	total := 0
	rows, err := s.selectAllAliasesStmt.QueryContext(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	aliases := []string{}
	for rows.Next() {
		var alias string
		if err = rows.Scan(&alias); err != nil {
			return nil, 0, err
		}

		total = total + 1
		aliases = append(aliases, alias)
	}
	return aliases, total, nil
}
