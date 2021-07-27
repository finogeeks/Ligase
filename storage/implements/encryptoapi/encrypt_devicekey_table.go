// Copyright 2018 Vector Creations Ltd
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

package encryptoapi

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const deviceKeySchema = `
-- The media_repository table holds metadata for each media file stored and accessible to the local server,
-- the actual file is stored separately.
CREATE TABLE IF NOT EXISTS encrypt_device_key (
    device_id TEXT 				NOT NULL,
    user_id TEXT 				NOT NULL,
    key_info TEXT 				NOT NULL,
    algorithm TEXT 				NOT NULL,
	signature TEXT 				NOT NULL,
	identifier TEXT  NOT NULL DEFAULT '',
	CONSTRAINT encrypt_device_key_unique UNIQUE (device_id, user_id, algorithm)
);

CREATE INDEX IF NOT EXISTS encrypt_device_key_user_id ON encrypt_device_key(user_id);
CREATE INDEX IF NOT EXISTS encrypt_device_key_device_id ON encrypt_device_key(device_id);
`
const insertDeviceKeySQL = `
INSERT INTO encrypt_device_key (device_id, user_id, key_info, algorithm, signature, identifier)
VALUES ($1, $2, $3, $4, $5, $6) on conflict ON CONSTRAINT encrypt_device_key_unique 
DO UPDATE SET key_info = EXCLUDED.key_info, signature = EXCLUDED.signature
`

const recoverDeviceKeysSQL = `
SELECT device_id, user_id, key_info, algorithm, signature, identifier FROM encrypt_device_key limit $1 offset $2
`

const deleteDeviceKeySQL = `
DELETE FROM encrypt_device_key WHERE device_id = $1 AND user_id = $2
`

const deleteMacDeviceKeySQL = `
DELETE FROM encrypt_device_key WHERE  user_id = $1 AND identifier = $2 AND device_id != $3
`

type deviceKeyStatements struct {
	db                     *Database
	insertDeviceKeyStmt    *sql.Stmt
	recoverDeviceKeyStmt   *sql.Stmt
	deleteDeviceKeyStmt    *sql.Stmt
	deleteMacDeviceKeyStmt *sql.Stmt
}

func (s *deviceKeyStatements) prepare(d *Database) (err error) {
	s.db = d
	_, err = d.db.Exec(deviceKeySchema)
	if err != nil {
		return
	}
	if s.insertDeviceKeyStmt, err = d.db.Prepare(insertDeviceKeySQL); err != nil {
		return
	}
	if s.recoverDeviceKeyStmt, err = d.db.Prepare(recoverDeviceKeysSQL); err != nil {
		return
	}
	if s.deleteDeviceKeyStmt, err = d.db.Prepare(deleteDeviceKeySQL); err != nil {
		return
	}
	if s.deleteMacDeviceKeyStmt, err = d.db.Prepare(deleteMacDeviceKeySQL); err != nil {
		return
	}
	return
}

func (s *deviceKeyStatements) recoverDeviceKey() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverDeviceKeyStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *deviceKeyStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var keyInsert dbtypes.KeyInsert
		if err1 := rows.Scan(&keyInsert.DeviceID, &keyInsert.UserID, &keyInsert.KeyInfo, &keyInsert.Algorithm, &keyInsert.Signature, &keyInsert.Identifier); err1 != nil {
			log.Errorf("load deviceKey error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.DeviceKeyInsertKey
		update.IsRecovery = true
		update.E2EDBEvents.KeyInsert = &keyInsert
		update.SetUid(int64(common.CalcStringHashCode64(keyInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "encrypt_device_key")
		if err2 != nil {
			log.Errorf("update deviceKey cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

// insert keys
func (s *deviceKeyStatements) insertDeviceKey(
	ctx context.Context,
	deviceID, userID, keyInfo, algorithm, signature, identifier string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.DeviceKeyInsertKey
		update.E2EDBEvents.KeyInsert = &dbtypes.KeyInsert{
			DeviceID:   deviceID,
			UserID:     userID,
			KeyInfo:    keyInfo,
			Algorithm:  algorithm,
			Signature:  signature,
			Identifier: identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_device_key")
	} else {
		stmt := s.insertDeviceKeyStmt
		_, err := stmt.ExecContext(ctx, deviceID, userID, keyInfo, algorithm, signature, identifier)
		return err
	}
}

func (s *deviceKeyStatements) onInsertDeviceKey(
	ctx context.Context,
	deviceID, userID, keyInfo, algorithm, signature, identifier string,
) error {
	stmt := s.insertDeviceKeyStmt
	_, err := stmt.ExecContext(ctx, deviceID, userID, keyInfo, algorithm, signature, identifier)
	return err
}

func (s *deviceKeyStatements) deleteDeviceKey(
	ctx context.Context,
	deviceID, userID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.DeviceKeyDeleteKey
		update.E2EDBEvents.DeviceKeyDelete = &dbtypes.DeviceKeyDelete{
			DeviceID: deviceID,
			UserID:   userID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_device_key")
	} else {
		stmt := s.deleteDeviceKeyStmt
		_, err := stmt.ExecContext(ctx, deviceID, userID)
		return err
	}
}

func (s *deviceKeyStatements) onDeleteDeviceKey(
	ctx context.Context,
	deviceID, userID string,
) error {
	stmt := s.deleteDeviceKeyStmt
	_, err := stmt.ExecContext(ctx, deviceID, userID)
	return err
}

func (s *deviceKeyStatements) deleteMacDeviceKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.MacDeviceKeyDeleteKey
		update.E2EDBEvents.MacKeyDelete = &dbtypes.MacKeyDelete{
			DeviceID:   deviceID,
			UserID:     userID,
			Identifier: identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_device_key")
	} else {
		stmt := s.deleteMacDeviceKeyStmt
		_, err := stmt.ExecContext(ctx, userID, identifier, deviceID)
		return err
	}
}

func (s *deviceKeyStatements) onDeleteMacDeviceKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) error {
	stmt := s.deleteMacDeviceKeyStmt
	_, err := stmt.ExecContext(ctx, userID, identifier, deviceID)
	return err
}
