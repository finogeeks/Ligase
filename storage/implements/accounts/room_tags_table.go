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

package accounts

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const roomTagsSchema = `
-- Stores data about room tags.
CREATE TABLE IF NOT EXISTS room_tags (
    user_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    tag     TEXT NOT NULL,  -- The name of the tag.
    content TEXT NOT NULL,  -- The JSON content of the tag.
	PRIMARY KEY (user_id, room_id, tag)
);

CREATE INDEX IF NOT EXISTS room_tags_user_id ON room_tags(user_id);
`

const insertRoomTagsSQL = `
	INSERT INTO room_tags(user_id, room_id, tag, content) VALUES ($1, $2, $3, $4)
	ON CONFLICT (user_id, room_id, tag) DO UPDATE SET content = EXCLUDED.content
`

const deleteRoomTagsSQL = "" +
	"DELETE FROM room_tags WHERE user_id = $1 and room_id = $2 and tag = $3"

const selectRoomTagsCountSQL = "" +
	"SELECT count(1) FROM room_tags"

const recoverRoomTagsSQL = "" +
	"SELECT user_id, room_id, tag, content FROM room_tags limit $1 offset $2"

type roomTagsStatements struct {
	db                      *Database
	insertRoomTagsStmt      *sql.Stmt
	deleteRoomTagsStmt      *sql.Stmt
	selectRoomTagsCountStmt *sql.Stmt
	recoverRoomTagsStmt     *sql.Stmt
}

func (s *roomTagsStatements) getSchema() string {
	return roomTagsSchema
}

func (s *roomTagsStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.insertRoomTagsStmt, err = d.db.Prepare(insertRoomTagsSQL); err != nil {
		return
	}
	if s.deleteRoomTagsStmt, err = d.db.Prepare(deleteRoomTagsSQL); err != nil {
		return
	}
	if s.selectRoomTagsCountStmt, err = d.db.Prepare(selectRoomTagsCountSQL); err != nil {
		return
	}
	if s.recoverRoomTagsStmt, err = d.db.Prepare(recoverRoomTagsSQL); err != nil {
		return
	}
	return
}

func (s *roomTagsStatements) recoverRoomTag() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverRoomTagsStmt.QueryContext(context.TODO(), limit, offset)
		if err != nil {
			return err
		}
		offset = offset + limit
		exists, err = s.processRecover(rows)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *roomTagsStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var roomTagInsert dbtypes.RoomTagInsert
		if err1 := rows.Scan(&roomTagInsert.UserID, &roomTagInsert.RoomID, &roomTagInsert.Tag, &roomTagInsert.Content); err1 != nil {
			log.Errorf("load tag error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.RoomTagInsertKey
		update.IsRecovery = true
		update.AccountDBEvents.RoomTagInsert = &roomTagInsert
		update.SetUid(int64(common.CalcStringHashCode64(roomTagInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "room_tags")
		if err2 != nil {
			log.Errorf("update tag cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *roomTagsStatements) insertRoomTag(
	ctx context.Context, userId, roomID, tag string, content []byte,
) (err error) {
	stmt := s.insertRoomTagsStmt
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.RoomTagInsertKey
		update.AccountDBEvents.RoomTagInsert = &dbtypes.RoomTagInsert{
			UserID:  userId,
			RoomID:  roomID,
			Tag:     tag,
			Content: content,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userId)))
		return s.db.WriteDBEventWithTbl(&update, "room_tags")
	} else {
		_, err = stmt.ExecContext(ctx, userId, roomID, tag, content)
		return
	}
}

func (s *roomTagsStatements) onInsertRoomTag(
	ctx context.Context, userId, roomID, tag string, content []byte,
) (err error) {
	stmt := s.insertRoomTagsStmt
	_, err = stmt.ExecContext(ctx, userId, roomID, tag, content)
	return
}

func (s *roomTagsStatements) deleteRoomTag(
	ctx context.Context, userId, roomID, tag string,
) (err error) {
	stmt := s.deleteRoomTagsStmt
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.RoomTagDeleteKey
		update.AccountDBEvents.RoomTagDelete = &dbtypes.RoomTagDelete{
			UserID: userId,
			RoomID: roomID,
			Tag:    tag,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userId)))
		return s.db.WriteDBEventWithTbl(&update, "room_tags")
	} else {
		_, err = stmt.ExecContext(ctx, userId, roomID, tag)
		return
	}
}

func (s *roomTagsStatements) onDeleteRoomTag(
	ctx context.Context, userId, roomID, tag string,
) (err error) {
	stmt := s.deleteRoomTagsStmt
	_, err = stmt.ExecContext(ctx, userId, roomID, tag)
	return
}

func (s *roomTagsStatements) selectRoomTagsTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectRoomTagsCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}
