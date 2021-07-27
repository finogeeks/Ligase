// Copyright 2017 Jan Christian Grünhage
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

const filterSchema = `
-- Stores data about filters
CREATE TABLE IF NOT EXISTS account_filter (
	-- The filter
	filter TEXT NOT NULL,
	-- The ID
	id TEXT NOT NULL,
	-- The user id of the Matrix user ID associated to this filter
	user_id TEXT NOT NULL,

	PRIMARY KEY(id, user_id)
);

CREATE INDEX IF NOT EXISTS account_filter_user_Id ON account_filter(user_id);
`

const insertFilterSQL = "" +
	"INSERT INTO account_filter (filter, id, user_id) VALUES ($1, $2, $3)" +
	" ON CONFLICT (id, user_id) DO UPDATE SET filter = EXCLUDED.filter"

const recoverFilterSQL = "" +
	"SELECT filter, id, user_id FROM  account_filter limit $1 offset $2"

type filterStatements struct {
	db                *Database
	insertFilterStmt  *sql.Stmt
	recoverFilterStmt *sql.Stmt
}

func (s *filterStatements) getSchema() string {
	return filterSchema
}

func (s *filterStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.insertFilterStmt, err = d.db.Prepare(insertFilterSQL); err != nil {
		return
	}
	if s.recoverFilterStmt, err = d.db.Prepare(recoverFilterSQL); err != nil {
		return
	}
	return
}

func (s *filterStatements) recoverFilter() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverFilterStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *filterStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var filterInsert dbtypes.FilterInsert
		if err1 := rows.Scan(&filterInsert.Filter, &filterInsert.FilterID, &filterInsert.UserID); err1 != nil {
			log.Errorf("load filter error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.FilterInsertKey
		update.IsRecovery = true
		update.AccountDBEvents.FilterInsert = &filterInsert
		update.SetUid(int64(common.CalcStringHashCode64(filterInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "account_filter")
		if err2 != nil {
			log.Errorf("update filter cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *filterStatements) insertFilter(
	ctx context.Context, filter, filterID, userID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.FilterInsertKey
		update.AccountDBEvents.FilterInsert = &dbtypes.FilterInsert{
			UserID:   userID,
			Filter:   filter,
			FilterID: filterID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "account_filter")
	} else {
		stmt := s.insertFilterStmt
		_, err := stmt.ExecContext(ctx, filter, filterID, userID)
		return err
	}
}

func (s *filterStatements) onInsertFilter(
	ctx context.Context, filter, filterID, userID string,
) error {
	stmt := s.insertFilterStmt
	_, err := stmt.ExecContext(ctx, filter, filterID, userID)
	return err
}
