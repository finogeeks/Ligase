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
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const userInfoSchema = `
-- Stores data about accounts user information.
CREATE TABLE IF NOT EXISTS account_user_info (
	user_id TEXT NOT NULL PRIMARY KEY,
    user_name TEXT,
    job_number TEXT,
    mobile TEXT,
    landline TEXT,
	email TEXT,
	is_deleted TEXT NOT NULL DEFAULT 0,
	state int4,
    CONSTRAINT account_user_info_unique UNIQUE (user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS account_user_info_user_id_idx ON account_user_info(user_id);
`

const upsertUserInfoSQL = "" +
	"INSERT INTO account_user_info(user_id, user_name, job_number, mobile, landline, email, is_deleted, state) VALUES ($1, $2, $3, $4, $5, $6, 0, $7)" +
	" ON CONFLICT ON CONSTRAINT account_user_info_unique" +
	" DO UPDATE SET user_name = EXCLUDED.user_name, job_number = EXCLUDED.job_number, mobile = EXCLUDED.mobile, landline = EXCLUDED.landline, email = EXCLUDED.email, is_deleted = 0, state = EXCLUDED.state"

const initUserInfoSQL = "" +
	"INSERT INTO account_user_info(user_id, user_name, job_number, mobile, landline, email, state) VALUES ($1, $2, $3, $4, $5, $6, $7)" +
	" ON CONFLICT ON CONSTRAINT account_user_info_unique DO NOTHING"

const recoverUserInfoSQL = "" +
	"SELECT user_id, COALESCE(user_name,'') as user_name, COALESCE(job_number,'') as job_number, COALESCE(mobile,'') as mobile, COALESCE(landline,'') as landline, COALESCE(email,'') as email, COALESCE(state, 0) as state FROM account_user_info limit $1 offset $2"

const selectAllUserInfoSQL = "" +
	"SELECT user_id, COALESCE(user_name,'') as user_name, COALESCE(job_number,'') as job_number, COALESCE(mobile,'') as mobile, COALESCE(landline,'') as landline, COALESCE(email,'') as email FROM account_user_info"

const deleteUserInfoSQL = "" +
	"UPDATE account_user_info SET is_deleted = $1 WHERE user_id = $2"

type userInfoStatements struct {
	db                    *Database
	upsertUserInfoStmt    *sql.Stmt
	initUserInfoStmt      *sql.Stmt
	recoverUserInfoStmt   *sql.Stmt
	selectAllUserInfoStmt *sql.Stmt
	deleteUserInfoStmt    *sql.Stmt
}

func (s *userInfoStatements) getSchema() string {
	return userInfoSchema
}

func (s *userInfoStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.upsertUserInfoStmt, err = d.db.Prepare(upsertUserInfoSQL); err != nil {
		return
	}
	if s.initUserInfoStmt, err = d.db.Prepare(initUserInfoSQL); err != nil {
		return
	}
	if s.recoverUserInfoStmt, err = d.db.Prepare(recoverUserInfoSQL); err != nil {
		return
	}
	if s.selectAllUserInfoStmt, err = d.db.Prepare(selectAllUserInfoSQL); err != nil {
		return
	}
	if s.deleteUserInfoStmt, err = d.db.Prepare(deleteUserInfoSQL); err != nil {
		return
	}
	return
}

func (s *userInfoStatements) recoverUserInfo(ctx context.Context) error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverUserInfoStmt.QueryContext(ctx, limit, offset)
		if err != nil {
			return err
		}
		offset = offset + limit
		exists, err = s.processRecover(ctx, rows)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *userInfoStatements) processRecover(ctx context.Context, rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var ui dbtypes.UserInfoInsert
		if err1 := rows.Scan(&ui.UserID, &ui.UserName, &ui.JobNumber, &ui.Mobile, &ui.Landline, &ui.Email, &ui.State); err1 != nil {
			log.Errorf("load user_info error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.UserInfoInsertKey
		update.IsRecovery = true
		update.AccountDBEvents.UserInfoInsert = &ui
		update.SetUid(int64(common.CalcStringHashCode64(ui.UserID)))
		err2 := s.db.WriteDBEventWithTbl(ctx, &update, "account_user_info")
		if err2 != nil {
			log.Errorf("update user_info cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *userInfoStatements) upsertUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.UserInfoInsertKey
		update.AccountDBEvents.UserInfoInsert = &dbtypes.UserInfoInsert{
			UserID:    userID,
			UserName:  userName,
			JobNumber: jobNumber,
			Mobile:    mobile,
			Landline:  landline,
			Email:     email,
			State:     state,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "account_user_info")
	}

	_, err := s.upsertUserInfoStmt.ExecContext(ctx, userID, userName, jobNumber, mobile, landline, email, state)
	return err
}

func (s *userInfoStatements) initUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.UserInfoInitKey
		update.AccountDBEvents.UserInfoInsert = &dbtypes.UserInfoInsert{
			UserID:    userID,
			UserName:  userName,
			JobNumber: jobNumber,
			Mobile:    mobile,
			Landline:  landline,
			Email:     email,
			State:     state,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "account_user_info")
	}

	_, err := s.initUserInfoStmt.ExecContext(ctx, userID, userName, jobNumber, mobile, landline, email, state)
	return err
}

func (s *userInfoStatements) onUpsertUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	_, err := s.upsertUserInfoStmt.ExecContext(ctx, userID, userName, jobNumber, mobile, landline, email, state)
	return err
}

func (s *userInfoStatements) onInitUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	_, err := s.initUserInfoStmt.ExecContext(ctx, userID, userName, jobNumber, mobile, landline, email, state)
	return err
}

func (s *userInfoStatements) getAllUserInfo() ([]authtypes.UserInfo, error) {
	rows, err := s.selectAllUserInfoStmt.QueryContext(context.TODO())
	if err != nil {
		return nil, err
	}

	var userInfoList []authtypes.UserInfo
	defer rows.Close()

	for rows.Next() {
		var ui authtypes.UserInfo
		if err := rows.Scan(&ui.UserID, &ui.UserName, &ui.JobNumber, &ui.Mobile, &ui.Landline, &ui.Email, &ui.State); err != nil {
			continue
		}
		userInfoList = append(userInfoList, ui)
	}

	return userInfoList, nil
}

func (s *userInfoStatements) deleteUserInfo(
	ctx context.Context, userID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.UserInfoDeleteKey
		update.AccountDBEvents.UserInfoDelete = &dbtypes.UserInfoDelete{
			UserID: userID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "account_user_info")
	}

	isDeleted := 1
	_, err := s.deleteUserInfoStmt.ExecContext(ctx, isDeleted, userID)
	return err
}

func (s *userInfoStatements) onDeleteUserInfo(
	ctx context.Context, userID string,
) error {
	isDeleted := 1
	_, err := s.deleteUserInfoStmt.ExecContext(ctx, isDeleted, userID)
	return err
}
