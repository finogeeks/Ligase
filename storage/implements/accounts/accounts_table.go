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
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

const accountsSchema = `
-- Stores data about accounts.
CREATE TABLE IF NOT EXISTS account_accounts (
    -- The Matrix user ID for this account
    user_id TEXT NOT NULL PRIMARY KEY,
    -- When this account was first created, as a unix timestamp (ms resolution).
    created_ts BIGINT NOT NULL,
    -- The password hash for this account. Can be NULL if this is a passwordless account.
    password_hash TEXT,
    -- Identifies which Application Service this account belongs to, if any.
    app_service_id TEXT
    -- TODO:
    -- is_guest, is_admin, upgraded_ts, devices, any email reset stuff?
);

CREATE UNIQUE INDEX IF NOT EXISTS account_accounts_user_id ON account_accounts(user_id);
`

const insertAccountSQL = "" +
	"INSERT INTO account_accounts(user_id, created_ts, password_hash, app_service_id) VALUES ($1, $2, $3, $4)" +
	"ON CONFLICT (user_id) DO NOTHING"

const selectAccountsCountSQL = "" +
	"SELECT count(1) FROM account_accounts"

const selectAccountSQL = "" +
	"SELECT user_id FROM account_accounts WHERE user_id = $1"

type accountsStatements struct {
	db                      *Database
	insertAccountStmt       *sql.Stmt
	selectAccountsCountStmt *sql.Stmt
	selectAccountStmt       *sql.Stmt
}

func (s *accountsStatements) getSchema() string {
	return accountsSchema
}

func (s *accountsStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.insertAccountStmt, err = d.db.Prepare(insertAccountSQL); err != nil {
		return
	}
	if s.selectAccountsCountStmt, err = d.db.Prepare(selectAccountsCountSQL); err != nil {
		return
	}
	if s.selectAccountStmt, err = d.db.Prepare(selectAccountSQL); err != nil {
		return
	}
	return
}

func (s *accountsStatements) selectAccount(ctx context.Context, userID string) (*authtypes.Account, error) {
	rows, err := s.selectAccountStmt.QueryContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	var account authtypes.Account
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&account.UserID); err != nil {
			return nil, err
		}
	}

	return &account, nil
}

// insertAccount creates a new account. 'hash' should be the password hash for this account. If it is missing,
// this account will be passwordless. Returns an error if this account already exists. Returns the account
// on success.
func (s *accountsStatements) insertAccount(
	ctx context.Context, userID, hash, appServiceID string,
) (*authtypes.Account, error) {
	createdTimeMS := time.Now().UnixNano() / 1000000
	stmt := s.insertAccountStmt

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.AccountInsertKey
		update.AccountDBEvents.AccountInsert = &dbtypes.AccountInsert{
			UserID:       userID,
			CreatedTs:    createdTimeMS,
			PassWordHash: hash,
			AppServiceID: appServiceID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		err := s.db.WriteDBEventWithTbl(&update, "account_accounts")
		if err != nil {
			return nil, err
		}
	} else {
		_, err := stmt.ExecContext(ctx, userID, createdTimeMS, hash, appServiceID)
		if err != nil {
			return nil, err
		}
	}

	domain, _ := common.DomainFromID(userID)
	return &authtypes.Account{
		UserID:       userID,
		ServerName:   gomatrixserverlib.ServerName(domain),
		AppServiceID: appServiceID,
	}, nil
}

func (s *accountsStatements) onInsertAccount(
	ctx context.Context, userID, hash, appServiceID string, createdTs int64,
) error {
	stmt := s.insertAccountStmt
	_, err := stmt.ExecContext(ctx, userID, createdTs, hash, appServiceID)
	return err
}

func (s *accountsStatements) selectAccountsTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectAccountsCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}
