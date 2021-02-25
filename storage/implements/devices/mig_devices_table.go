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

package devices

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const migDevicesSchema = `
-- Stores data about devices.
CREATE TABLE IF NOT EXISTS mig_device_devices (
	-- so we can distinguish which device is making a given request.
	access_token TEXT NOT NULL PRIMARY KEY,
	device_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
	mig_access_token TEXT NOT NULL 
);

CREATE UNIQUE INDEX IF NOT EXISTS mig_device_user_id_idx ON mig_device_devices(user_id, device_id);
`

const insertMigDeviceSQL = "" +
	"INSERT INTO mig_device_devices(access_token, mig_access_token, device_id, user_id) VALUES ($1, $2, $3, $4)"

const deleteMigDeviceSQL = "" +
	"DELETE FROM mig_device_devices WHERE device_id = $1 AND user_id = $2"

const deleteMigDevicesByUserIDSQL = "" +
	"DELETE FROM mig_device_devices WHERE user_id = $1"

const recoverMigDeviceSQL = "" +
	"SELECT access_token, mig_access_token FROM mig_device_devices limit $1 offset $2"

type migDevicesStatements struct {
	db                        *Database
	insertMigDeviceStmt       *sql.Stmt
	deleteMigDeviceStmt       *sql.Stmt
	deleteDevicesByUserIDStmt *sql.Stmt
	recoverMigDeviceStmt      *sql.Stmt
}

func (s *migDevicesStatements) getSchema() string {
	return migDevicesSchema
}

func (s *migDevicesStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.insertMigDeviceStmt, err = d.db.Prepare(insertMigDeviceSQL); err != nil {
		return
	}
	if s.deleteMigDeviceStmt, err = d.db.Prepare(deleteMigDeviceSQL); err != nil {
		return
	}
	if s.deleteDevicesByUserIDStmt, err = d.db.Prepare(deleteMigDevicesByUserIDSQL); err != nil {
		return
	}
	if s.recoverMigDeviceStmt, err = d.db.Prepare(recoverMigDeviceSQL); err != nil {
		return
	}
	return
}

func (s *migDevicesStatements) recoverMigDevice() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverMigDeviceStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *migDevicesStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var deviceInsert dbtypes.MigDeviceInsert
		if err1 := rows.Scan(&deviceInsert.AccessToken, &deviceInsert.MigAccessToken); err1 != nil {
			log.Errorf("load migDevice error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_DEVICE_DB_EVENT
		update.Key = dbtypes.MigDeviceInsertKey
		update.IsRecovery = true
		update.DeviceDBEvents.MigDeviceInsert = &dbtypes.MigDeviceInsert{
			AccessToken:    deviceInsert.AccessToken,
			MigAccessToken: deviceInsert.MigAccessToken,
		}
		update.SetUid(int64(common.CalcStringHashCode64(deviceInsert.AccessToken)))
		err2 := s.db.WriteDBEventWithTbl(&update, "mig_device_devices")
		if err2 != nil {
			log.Errorf("update migDevice cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *migDevicesStatements) insertMigDevice(
	ctx context.Context, access_token, mig_access_token, deviceID, userID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_DEVICE_DB_EVENT
		update.Key = dbtypes.MigDeviceInsertKey
		update.DeviceDBEvents.MigDeviceInsert = &dbtypes.MigDeviceInsert{
			DeviceID:       deviceID,
			UserID:         userID,
			AccessToken:    access_token,
			MigAccessToken: mig_access_token,
		}
		update.SetUid(int64(common.CalcStringHashCode64(access_token)))
		return s.db.WriteDBEventWithTbl(&update, "mig_device_devices")
	} else {
		_, err := s.insertMigDeviceStmt.ExecContext(ctx, access_token, mig_access_token, deviceID, userID)
		return err
	}
}

func (s *migDevicesStatements) onUpsertMigDevice(
	ctx context.Context, access_token, mig_access_token, deviceID, userID string,
) error {
	_, err := s.insertMigDeviceStmt.ExecContext(ctx, access_token, mig_access_token, deviceID, userID)
	return err
}

func (s *migDevicesStatements) deleteMigDevice(
	ctx context.Context, deviceID, userID string,
) error {
	_, err := s.deleteMigDeviceStmt.ExecContext(ctx, deviceID, userID)
	return err
}

func (s *migDevicesStatements) deleteUserMigDevices(
	ctx context.Context, userID string,
) error {
	_, err := s.deleteDevicesByUserIDStmt.ExecContext(ctx, userID)
	return err
}
