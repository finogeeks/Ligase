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
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const devicesSchema = `
-- Stores data about devices.
CREATE TABLE IF NOT EXISTS device_devices (
    -- The device identifier. This only needs to uniquely identify a device for a given user, not globally.
    -- access_tokens will be clobbered based on the device ID for a user.
    device_id TEXT NOT NULL,
    -- The Matrix user ID for this device. This is preferable to storing the full user_id
    -- as it is smaller, makes it clearer that we only manage devices for our own users, and may make
    -- migration to different domain names easier.
    user_id TEXT NOT NULL,
    -- When this devices was first recognised on the network, as a unix timestamp (ms resolution).
    created_ts BIGINT NOT NULL,
    -- The display name, human friendlier than device_id and updatable
    display_name TEXT,
    -- The device is auto generated or a real one
	device_type TEXT NOT NULL,
	identifier TEXT NOT NULL DEFAULT '',
    last_active_ts BIGINT NOT NULL,
	CONSTRAINT device_devices_unique PRIMARY KEY (identifier, user_id),
	CONSTRAINT device_devices_unique UNIQUE (identifier, user_id)
);

-- Device IDs must be unique for a given user.
CREATE UNIQUE INDEX IF NOT EXISTS device_user_id_idx ON device_devices(user_id, identifier);
CREATE INDEX IF NOT EXISTS device_ts_idx ON device_devices(created_ts);
`

const upsertDeviceSQL = "" +
	"INSERT INTO device_devices(device_id, user_id, created_ts, display_name, device_type, identifier, last_active_ts) VALUES ($1, $2, $3, $4, $5, $6, $7)" +
	" ON CONFLICT ON CONSTRAINT device_devices_unique" +
	" DO UPDATE SET device_id = EXCLUDED.device_id, created_ts = EXCLUDED.created_ts, display_name = EXCLUDED.display_name, device_type = EXCLUDED.device_type, last_active_ts = EXCLUDED.last_active_ts"

const deleteDeviceSQL = "" +
	"DELETE FROM device_devices WHERE device_id = $1 AND user_id = $2 AND created_ts < $3"

const selectDeviceCountSQL = "" +
	"SELECT count(1) FROM device_devices"

const recoverDeviceSQL = "" +
	"SELECT device_id, user_id, last_active_ts, created_ts, display_name, device_type, identifier FROM device_devices WHERE device_type = 'actual' or device_type = 'bot' limit $1 offset $2"

const selectActiveDeviceSQL = "" +
	"SELECT device_id, identifier, user_id FROM device_devices ORDER BY created_ts ASC LIMIT $1 OFFSET $2"

const updateDeviceActiveTsSQL = "" +
	"UPDATE device_devices SET last_active_ts = $3 WHERE device_id = $1 AND user_id = $2"

const selectUnActiveDeviceSQL = "" +
	"SELECT device_id, identifier, user_id FROM device_devices WHERE last_active_ts < $1 AND identifier != 'super-gen' LIMIT $2 OFFSET $3"

const checkDeviceSQL = "" +
	"SELECT device_id, device_type, created_ts FROM device_devices WHERE identifier = $1 AND user_id = $2"

type devicesStatements struct {
	db                       *Database
	upsertDeviceStmt         *sql.Stmt
	deleteDeviceStmt         *sql.Stmt
	selectDeviceCountStmt    *sql.Stmt
	recoverDeviceStmt        *sql.Stmt
	selectActiveDeviceStmt   *sql.Stmt
	updateDeviceTsStmt       *sql.Stmt
	selectUnActiveDeviceStmt *sql.Stmt
	CheckDeviceStmt          *sql.Stmt
}

func (s *devicesStatements) getSchema() string {
	return devicesSchema
}

func (s *devicesStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.upsertDeviceStmt, err = d.db.Prepare(upsertDeviceSQL); err != nil {
		return
	}
	if s.deleteDeviceStmt, err = d.db.Prepare(deleteDeviceSQL); err != nil {
		return
	}
	if s.selectDeviceCountStmt, err = d.db.Prepare(selectDeviceCountSQL); err != nil {
		return
	}
	if s.recoverDeviceStmt, err = d.db.Prepare(recoverDeviceSQL); err != nil {
		return
	}
	if s.selectActiveDeviceStmt, err = d.db.Prepare(selectActiveDeviceSQL); err != nil {
		return
	}
	if s.updateDeviceTsStmt, err = d.db.Prepare(updateDeviceActiveTsSQL); err != nil {
		return
	}
	if s.selectUnActiveDeviceStmt, err = d.db.Prepare(selectUnActiveDeviceSQL); err != nil {
		return
	}
	if s.CheckDeviceStmt, err = d.db.Prepare(checkDeviceSQL); err != nil {
		return
	}
	return
}

func (s *devicesStatements) recoverDevice(ctx context.Context) error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverDeviceStmt.QueryContext(ctx, limit, offset)
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

func (s *devicesStatements) processRecover(ctx context.Context, rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var deviceInsert dbtypes.DeviceInsert
		if err1 := rows.Scan(&deviceInsert.DeviceID, &deviceInsert.UserID, &deviceInsert.LastActiveTs, &deviceInsert.CreatedTs, &deviceInsert.DisplayName, &deviceInsert.DeviceType, &deviceInsert.Identifier); err1 != nil {
			log.Errorf("load device error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_DEVICE_DB_EVENT
		update.Key = dbtypes.DeviceRecoverKey
		update.IsRecovery = true
		update.DeviceDBEvents.DeviceInsert = &deviceInsert
		update.SetUid(int64(common.CalcStringHashCode64(deviceInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(ctx, &update, "device_devices")
		if err2 != nil {
			log.Errorf("update device cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

// insertDevice creates a new device. Returns an error if any device with the same access token already exists.
// Returns an error if the user already has a device with the given device ID.
// Returns the device on success.
func (s *devicesStatements) upsertDevice(
	ctx context.Context, deviceID, userID string, displayName *string, deviceType, identifier string, specifiedTime int64,
) error {
	createdTimeMS := time.Now().UnixNano() / 1000000
	if specifiedTime > 0 {
		createdTimeMS = specifiedTime
	}

	if displayName == nil {
		name := ""
		displayName = &name
	}
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_DEVICE_DB_EVENT
		update.Key = dbtypes.DeviceInsertKey
		update.DeviceDBEvents.DeviceInsert = &dbtypes.DeviceInsert{
			DeviceID:     deviceID,
			UserID:       userID,
			DisplayName:  *displayName,
			CreatedTs:    createdTimeMS,
			DeviceType:   deviceType,
			Identifier:   identifier,
			LastActiveTs: createdTimeMS,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "device_devices")
	} else {
		_, err := s.upsertDeviceStmt.ExecContext(ctx, deviceID, userID, createdTimeMS, displayName, deviceType, identifier, createdTimeMS)
		return err
	}
}

func (s *devicesStatements) onUpsertDevice(
	ctx context.Context, id, userID string, displayName *string, createdTs, lastActiveTs int64, deviceType, identifier string,
) error {
	if displayName == nil {
		name := ""
		displayName = &name
	}

	_, err := s.upsertDeviceStmt.ExecContext(ctx, id, userID, createdTs, displayName, deviceType, identifier, lastActiveTs)
	return err
}

func (s *devicesStatements) deleteDevice(
	ctx context.Context, deviceID, userID string, createTs int64,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_DEVICE_DB_EVENT
		update.Key = dbtypes.DeviceDeleteKey
		update.DeviceDBEvents.DeviceDelete = &dbtypes.DeviceDelete{
			DeviceID: deviceID,
			UserID:   userID,
			CreateTs: createTs,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "device_devices")
	} else {
		_, err := s.deleteDeviceStmt.ExecContext(ctx, deviceID, userID, createTs)
		return err
	}
}

func (s *devicesStatements) onDeleteDevice(
	ctx context.Context, deviceID, userID string, createTs int64,
) error {
	_, err := s.deleteDeviceStmt.ExecContext(ctx, deviceID, userID, createTs)
	return err
}

func (s *devicesStatements) selectDeviceTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectDeviceCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}

func (s *devicesStatements) selectActiveDevices(
	ctx context.Context, limit, offset int,
) ([]string, []string, []string, int, error) {
	total := 0
	rows, err := s.selectActiveDeviceStmt.QueryContext(ctx, limit, offset)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	defer rows.Close()
	devids := []string{}
	dids := []string{}
	uids := []string{}
	for rows.Next() {
		var devid string
		var did string
		var uid string
		if err = rows.Scan(&devid, &did, &uid); err != nil {
			return nil, nil, nil, 0, err
		}

		total = total + 1
		devids = append(devids, devid)
		dids = append(dids, did)
		uids = append(uids, uid)
	}
	return devids, dids, uids, total, nil
}

func (s *devicesStatements) updateDeviceActiveTs(
	ctx context.Context, deviceID, userID string, lastActiveTs int64,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_DEVICE_DB_EVENT
		update.Key = dbtypes.DeviceUpdateTsKey
		update.DeviceDBEvents.DeviceUpdateTs = &dbtypes.DeviceUpdateTs{
			DeviceID:     deviceID,
			UserID:       userID,
			LastActiveTs: lastActiveTs,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(ctx, &update, "device_devices")
	} else {
		_, err := s.updateDeviceTsStmt.ExecContext(ctx, deviceID, userID, lastActiveTs)
		return err
	}
}

func (s *devicesStatements) onUpdateDeviceActiveTs(
	ctx context.Context, deviceID, userID string, lastActiveTs int64,
) error {
	_, err := s.updateDeviceTsStmt.ExecContext(ctx, deviceID, userID, lastActiveTs)
	return err
}

func (s *devicesStatements) selectUnActiveDevices(
	ctx context.Context, lastActiveTs int64, limit, offset int,
) ([]string, []string, []string, int, error) {
	total := 0
	rows, err := s.selectUnActiveDeviceStmt.QueryContext(ctx, lastActiveTs, limit, offset)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	defer rows.Close()
	devids := []string{}
	dids := []string{}
	uids := []string{}
	for rows.Next() {
		var devid string
		var did string
		var uid string
		if err = rows.Scan(&devid, &did, &uid); err != nil {
			return nil, nil, nil, 0, err
		}

		total = total + 1
		devids = append(devids, devid)
		dids = append(dids, did)
		uids = append(uids, uid)
	}
	return devids, dids, uids, total, nil
}

func (s *devicesStatements) checkDevice(
	ctx context.Context,
	identifier, userID string,
) (deviceID, deviceType string, ts int64, err error) {
	deviceID = ""
	deviceType = ""

	err = s.CheckDeviceStmt.QueryRowContext(ctx, identifier, userID).Scan(&deviceID, &deviceType, &ts)
	return
}
