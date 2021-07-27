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

package accounts

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const accountDataSchema = `
-- Stores data about accounts data.
CREATE TABLE IF NOT EXISTS account_data (
    -- The Matrix user ID for this account
    user_id TEXT NOT NULL,
    -- The room ID for this data (empty string if not specific to a room)
    room_id TEXT NOT NULL,
    -- The account data type
    type TEXT NOT NULL,
    -- The account data content
    content TEXT NOT NULL,

    CONSTRAINT account_data_unique UNIQUE (user_id, room_id, type)
);

CREATE INDEX IF NOT EXISTS account_data_user_id ON account_data(user_id, room_id);
`

const insertAccountDataSQL = `
	INSERT INTO account_data(user_id, room_id, type, content) VALUES($1, $2, $3, $4)
	ON CONFLICT ON CONSTRAINT account_data_unique DO UPDATE SET content = EXCLUDED.content
`

const selectAccountDataCountSQL = "" +
	"SELECT count(1) FROM account_data"

const recoverAccountDataSQL = "" +
	"SELECT user_id, room_id, type, content from account_data limit $1 offset $2"

type accountDataStatements struct {
	db                         *Database
	insertAccountDataStmt      *sql.Stmt
	selectAccountDataCountStmt *sql.Stmt
	recoverAccountDataStmt     *sql.Stmt
}

func (s *accountDataStatements) getSchema() string {
	return accountDataSchema
}

func (s *accountDataStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.insertAccountDataStmt, err = d.db.Prepare(insertAccountDataSQL); err != nil {
		return
	}
	if s.selectAccountDataCountStmt, err = d.db.Prepare(selectAccountDataCountSQL); err != nil {
		return
	}
	if s.recoverAccountDataStmt, err = d.db.Prepare(recoverAccountDataSQL); err != nil {
		return
	}
	return
}

func (s *accountDataStatements) recoverAccountData() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverAccountDataStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *accountDataStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var accountDataInsert dbtypes.AccountDataInsert
		if err1 := rows.Scan(&accountDataInsert.UserID, &accountDataInsert.RoomID, &accountDataInsert.Type, &accountDataInsert.Content); err1 != nil {
			log.Errorf("load account data error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.AccountDataInsertKey
		update.IsRecovery = true
		update.AccountDBEvents.AccountDataInsert = &accountDataInsert
		update.SetUid(int64(common.CalcStringHashCode64(accountDataInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "account_data")
		if err2 != nil {
			log.Errorf("update account data cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *accountDataStatements) insertAccountData(
	ctx context.Context, userID, roomID, dataType, content string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.AccountDataInsertKey
		update.AccountDBEvents.AccountDataInsert = &dbtypes.AccountDataInsert{
			UserID:  userID,
			RoomID:  roomID,
			Type:    dataType,
			Content: content,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "account_data")
	} else {
		_, err := s.insertAccountDataStmt.ExecContext(ctx, userID, roomID, dataType, content)
		return err
	}
}

func (s *accountDataStatements) onInsertAccountData(
	ctx context.Context, userID, roomID, dataType, content string,
) (err error) {
	_, err = s.insertAccountDataStmt.ExecContext(ctx, userID, roomID, dataType, content)
	return
}

func (s *accountDataStatements) selectAccountDataTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectAccountDataCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}
