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

	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const settingsSchema = `
CREATE TABLE IF NOT EXISTS public.roomserver_settings 
(
  setting_key TEXT NOT NULL,
  val TEXT NOT NULL,
  CONSTRAINT roomserver_setting_unique UNIQUE (setting_key)
);
`

const upsertSettingsSQL = "" +
	"INSERT INTO roomserver_settings (setting_key, val) VALUES ($1, $2)" +
	" ON CONFLICT ON CONSTRAINT roomserver_setting_unique" +
	" DO UPDATE SET val = $2"

const selectSettingsSQL = "" +
	"SELECT setting_key, val FROM roomserver_settings"

const recoverSettingsSQL = "" +
	"SELECT setting_key, val FROM roomserver_settings"

type settingsStatements struct {
	db                  *Database
	usertSettingStmt    *sql.Stmt
	selectSettingsStmt  *sql.Stmt
	recoverSettingsStmt *sql.Stmt
}

func (s *settingsStatements) getSchema() string {
	return settingsSchema
}

func (s *settingsStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if prepare_with_create {
		_, err = db.Exec(settingsSchema)
		if err != nil {
			return
		}
	}

	return statementList{
		{&s.usertSettingStmt, upsertSettingsSQL},
		{&s.selectSettingsStmt, selectSettingsSQL},
		{&s.recoverSettingsStmt, recoverSettingsSQL},
	}.prepare(db)
}

func (s *settingsStatements) insertSetting(
	ctx context.Context, settingKey string, val string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.SettingUpsertKey
		update.RoomDBEvents.SettingsInsert = &dbtypes.SettingsInsert{
			SettingKey: settingKey,
			Val:        val,
		}
		s.db.WriteDBEventWithTbl(&update, "roomserver_settings")
		return nil
	}

	return s.insertSettingRaw(ctx, settingKey, val)
}

func (s *settingsStatements) insertSettingRaw(
	ctx context.Context, settingKey string, val string,
) error {
	_, err := s.usertSettingStmt.ExecContext(ctx, settingKey, val)
	return err
}

func (s *settingsStatements) selectSettings(
	ctx context.Context,
) ([]string, []int64, error) {
	rows, err := s.selectSettingsStmt.QueryContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() // nolint: errcheck
	var settingKeys []string
	var vals []int64
	var settingKey string
	var val int64
	for rows.Next() {
		if err = rows.Scan(&settingKey, &val); err != nil {
			return nil, nil, err
		}
		settingKeys = append(settingKeys, settingKey)
		vals = append(vals, val)
	}
	return settingKeys, vals, err
}

func (s *settingsStatements) recoverSettings() error {
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverSettingsStmt.QueryContext(context.TODO())
		if err != nil {
			return err
		}
		exists, err = s.processRecover(rows)
		if err != nil {
			return err
		}
		break
	}

	return nil
}

func (s *settingsStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var settingsInsert dbtypes.SettingsInsert
		if err1 := rows.Scan(&settingsInsert.SettingKey, &settingsInsert.Val); err1 != nil {
			log.Errorf("load setting error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.SettingUpsertKey
		update.IsRecovery = true
		update.RoomDBEvents.SettingsInsert = &settingsInsert
		err2 := s.db.WriteDBEventWithTbl(&update, "roomserver_settings")
		if err2 != nil {
			log.Errorf("update setting cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}
