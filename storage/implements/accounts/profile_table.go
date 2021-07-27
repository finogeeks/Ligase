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

const profilesSchema = `
-- Stores data about accounts profiles.
CREATE TABLE IF NOT EXISTS account_profiles (
    -- The Matrix user ID  for this account
    user_id TEXT NOT NULL PRIMARY KEY,
    -- The display name for this account
    display_name TEXT,
    -- The URL of the avatar for this account
    avatar_url TEXT,
	CONSTRAINT account_profiles_unique UNIQUE (user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS account_profiles_user_id_idx ON account_profiles(user_id);
`

const upsertProfileSQL = "" +
	"INSERT INTO account_profiles(user_id, display_name, avatar_url) VALUES ($1, $2, $3)" +
	" ON CONFLICT ON CONSTRAINT account_profiles_unique" +
	" DO UPDATE SET display_name = EXCLUDED.display_name, avatar_url = EXCLUDED.avatar_url"

const initProfileSQL = "" +
	"INSERT INTO account_profiles(user_id, display_name, avatar_url) VALUES ($1, $2, $3)" +
	" ON CONFLICT ON CONSTRAINT account_profiles_unique DO NOTHING"

const upsertDisplayNameSQL = "" +
	"INSERT INTO account_profiles(user_id, display_name) VALUES ($1, $2)" +
	" ON CONFLICT ON CONSTRAINT account_profiles_unique" +
	" DO UPDATE SET display_name = EXCLUDED.display_name"

const upsertAvatarSQL = "" +
	"INSERT INTO account_profiles(user_id, avatar_url) VALUES ($1, $2)" +
	" ON CONFLICT ON CONSTRAINT account_profiles_unique" +
	" DO UPDATE SET avatar_url = EXCLUDED.avatar_url"

const recoverProfileSQL = "" +
	"SELECT user_id, COALESCE(display_name,'') as display_name, COALESCE(avatar_url,'') as avatar_url FROM account_profiles limit $1 offset $2"

const selectAllProfileSQL = "" +
	"SELECT user_id, COALESCE(display_name,'') as display_name, COALESCE(avatar_url,'') as avatar_url FROM account_profiles"

const selectProfileSQL = "" +
	"SELECT COALESCE(display_name,'') AS display_name, COALESCE(avatar_url,'') AS avatar_url FROM account_profiles WHERE user_id = $1"

type profilesStatements struct {
	db                    *Database
	upsertProfileStmt     *sql.Stmt
	initProfilesStmt      *sql.Stmt
	recoverProfileStmt    *sql.Stmt
	upsertDisplayNameStmt *sql.Stmt
	upsertAvatarStmt      *sql.Stmt
	selectAllProfileStmt  *sql.Stmt
	selectProfileStmt     *sql.Stmt
}

func (s *profilesStatements) getSchema() string {
	return profilesSchema
}

func (s *profilesStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.upsertProfileStmt, err = d.db.Prepare(upsertProfileSQL); err != nil {
		return
	}
	if s.initProfilesStmt, err = d.db.Prepare(initProfileSQL); err != nil {
		return
	}
	if s.recoverProfileStmt, err = d.db.Prepare(recoverProfileSQL); err != nil {
		return
	}
	if s.upsertDisplayNameStmt, err = d.db.Prepare(upsertDisplayNameSQL); err != nil {
		return
	}
	if s.upsertAvatarStmt, err = d.db.Prepare(upsertAvatarSQL); err != nil {
		return
	}
	if s.selectAllProfileStmt, err = d.db.Prepare(selectAllProfileSQL); err != nil {
		return
	}
	if s.selectProfileStmt, err = d.db.Prepare(selectProfileSQL); err != nil {
		return
	}
	return
}

func (s *profilesStatements) recoverProfile() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverProfileStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *profilesStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var profileInsert dbtypes.ProfileInsert
		if err1 := rows.Scan(&profileInsert.UserID, &profileInsert.DisplayName, &profileInsert.AvatarUrl); err1 != nil {
			log.Errorf("load profile error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.ProfileInsertKey
		update.IsRecovery = true
		update.AccountDBEvents.ProfileInsert = &profileInsert
		update.SetUid(int64(common.CalcStringHashCode64(profileInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "account_profiles")
		if err2 != nil {
			log.Errorf("update profile cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *profilesStatements) upsertProfile(
	ctx context.Context, userID, displayName, avatarURL string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.ProfileInsertKey
		update.AccountDBEvents.ProfileInsert = &dbtypes.ProfileInsert{
			UserID:      userID,
			DisplayName: displayName,
			AvatarUrl:   avatarURL,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "account_profiles")
	}

	return s.upsertProfileSync(ctx, userID, displayName, avatarURL)
}
func (s *profilesStatements) upsertProfileSync(
	ctx context.Context, userID, displayName string, avatarURL string,
) error {
	_, err := s.upsertProfileStmt.ExecContext(ctx, userID, displayName, avatarURL)
	return err
}

func (s *profilesStatements) initProfile(
	ctx context.Context, userID, displayName, avatarURL string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.ProfileInitKey
		update.AccountDBEvents.ProfileInsert = &dbtypes.ProfileInsert{
			UserID:      userID,
			DisplayName: displayName,
			AvatarUrl:   avatarURL,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "account_profiles")
	}

	_, err := s.initProfilesStmt.ExecContext(ctx, userID, displayName, avatarURL)
	return err
}

func (s *profilesStatements) upsertDisplayName(
	ctx context.Context, userID, displayName string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.DisplayNameInsertKey
		update.AccountDBEvents.ProfileInsert = &dbtypes.ProfileInsert{
			UserID:      userID,
			DisplayName: displayName,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "account_profiles")
	}

	return s.upsertDisplayNameSync(ctx, userID, displayName)
}
func (s *profilesStatements) upsertDisplayNameSync(
	ctx context.Context, userID, displayName string,
) error {
	_, err := s.upsertDisplayNameStmt.ExecContext(ctx, userID, displayName)
	return err
}

func (s *profilesStatements) upsertAvatar(
	ctx context.Context, userID, avatarURL string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ACCOUNT_DB_EVENT
		update.Key = dbtypes.AvatarInsertKey
		update.AccountDBEvents.ProfileInsert = &dbtypes.ProfileInsert{
			UserID:    userID,
			AvatarUrl: avatarURL,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "account_profiles")
	}

	return s.upsertAvatarSync(ctx, userID, avatarURL)
}
func (s *profilesStatements) upsertAvatarSync(
	ctx context.Context, userID, avatarURL string,
) error {
	_, err := s.upsertAvatarStmt.ExecContext(ctx, userID, avatarURL)
	return err
}

func (s *profilesStatements) onUpsertProfile(
	ctx context.Context, userID, displayName, avatarURL string,
) error {
	_, err := s.upsertProfileStmt.ExecContext(ctx, userID, displayName, avatarURL)
	return err
}

func (s *profilesStatements) onInitProfile(
	ctx context.Context, userID, displayName, avatarURL string,
) error {
	_, err := s.initProfilesStmt.ExecContext(ctx, userID, displayName, avatarURL)
	return err
}

func (s *profilesStatements) onUpsertDisplayName(
	ctx context.Context, userID, displayName string,
) error {
	_, err := s.upsertDisplayNameStmt.ExecContext(ctx, userID, displayName)
	return err
}

func (s *profilesStatements) onUpsertAvatar(
	ctx context.Context, userID, avatarURL string,
) error {
	_, err := s.upsertAvatarStmt.ExecContext(ctx, userID, avatarURL)
	return err
}

func (s *profilesStatements) getAllProfile() ([]authtypes.Profile, error) {
	rows, err := s.selectAllProfileStmt.QueryContext(context.TODO())
	if err != nil {
		return nil, err
	}

	var profiles []authtypes.Profile
	defer rows.Close()

	for rows.Next() {
		var profile authtypes.Profile
		if err := rows.Scan(&profile.UserID, &profile.DisplayName, &profile.AvatarURL); err != nil {
			continue
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func (s *profilesStatements) getProfile(ctx context.Context, userID string) (profile authtypes.Profile, err error) {
	err = s.selectProfileStmt.QueryRowContext(ctx, userID).Scan(&profile.DisplayName, &profile.AvatarURL)
	return
}
