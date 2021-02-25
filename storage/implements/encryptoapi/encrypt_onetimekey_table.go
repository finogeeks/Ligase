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

const oneTimeKeySchema = `
-- The media_repository table holds metadata for each media file stored and accessible to the local server,
-- the actual file is stored separately.
CREATE TABLE IF NOT EXISTS encrypt_onetime_key (
    device_id TEXT 				NOT NULL,
    user_id TEXT 				NOT NULL,
    key_id TEXT 				NOT NULL,
    key_info TEXT 				NOT NULL,
    algorithm TEXT 				NOT NULL,
	signature TEXT 				NOT NULL,
	identifier TEXT  NOT NULL DEFAULT '',
	CONSTRAINT encrypt_onetime_key_unique UNIQUE (device_id, user_id, key_id, algorithm)
);

CREATE INDEX IF NOT EXISTS encrypt_onetime_key_user_id ON encrypt_onetime_key(user_id);
CREATE INDEX IF NOT EXISTS encrypt_onetime_key_device_id ON encrypt_onetime_key(device_id);
`
const insertOneTimeKeySQL = `
INSERT INTO encrypt_onetime_key (device_id, user_id, key_id, key_info, algorithm, signature, identifier)
VALUES ($1, $2, $3, $4, $5, $6, $7) on conflict ON CONSTRAINT encrypt_onetime_key_unique 
DO UPDATE SET key_info = EXCLUDED.key_info, signature = EXCLUDED.signature, identifier = EXCLUDED.identifier
`
const deleteOneTimeKeySQL = `
DELETE FROM encrypt_onetime_key 
WHERE user_id = $1 AND device_id = $2 AND algorithm = $3 AND key_id = $4
`

const recoverOneTimeKeySQL = `
SELECT device_id, user_id, key_id, key_info, algorithm, signature, identifier FROM encrypt_onetime_key limit $1 offset $2
`

const deleteDeviceOneTimeKeySQL = `
DELETE FROM encrypt_onetime_key WHERE user_id = $1 AND device_id = $2
`

const deleteMacOneTimeKeySQL = `
DELETE FROM encrypt_onetime_key WHERE user_id = $1 AND identifier = $2 AND device_id != $3
`

type oneTimeKeyStatements struct {
	db                         *Database
	insertOneTimeKeyStmt       *sql.Stmt
	deleteOneTimeKeyStmt       *sql.Stmt
	recoverOneTimeKeyStmt      *sql.Stmt
	deleteDeviceOneTimeKeyStmt *sql.Stmt
	deleteMacOneTimeKeyStmt    *sql.Stmt
}

func (s *oneTimeKeyStatements) prepare(d *Database) (err error) {
	s.db = d
	_, err = d.db.Exec(oneTimeKeySchema)
	if err != nil {
		return
	}
	if s.insertOneTimeKeyStmt, err = d.db.Prepare(insertOneTimeKeySQL); err != nil {
		return
	}
	if s.deleteOneTimeKeyStmt, err = d.db.Prepare(deleteOneTimeKeySQL); err != nil {
		return
	}
	if s.recoverOneTimeKeyStmt, err = d.db.Prepare(recoverOneTimeKeySQL); err != nil {
		return
	}
	if s.deleteDeviceOneTimeKeyStmt, err = d.db.Prepare(deleteDeviceOneTimeKeySQL); err != nil {
		return
	}
	if s.deleteMacOneTimeKeyStmt, err = d.db.Prepare(deleteMacOneTimeKeySQL); err != nil {
		return
	}
	return
}

func (s *oneTimeKeyStatements) recoverOneTimeKey() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverOneTimeKeyStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *oneTimeKeyStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var keyInsert dbtypes.KeyInsert
		if err1 := rows.Scan(&keyInsert.DeviceID, &keyInsert.UserID, &keyInsert.KeyID, &keyInsert.KeyInfo, &keyInsert.Algorithm, &keyInsert.Signature, &keyInsert.Identifier); err1 != nil {
			log.Errorf("load onetimeKey error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.OneTimeKeyInsertKey
		update.IsRecovery = true
		update.E2EDBEvents.KeyInsert = &keyInsert
		update.SetUid(int64(common.CalcStringHashCode64(keyInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "encrypt_onetime_key")
		if err2 != nil {
			log.Errorf("update onetimeKey cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

// insert keys
func (s *oneTimeKeyStatements) insertOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, keyInfo, algorithm, signature, identifier string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.OneTimeKeyInsertKey
		update.E2EDBEvents.KeyInsert = &dbtypes.KeyInsert{
			DeviceID:   deviceID,
			UserID:     userID,
			KeyID:      keyID,
			KeyInfo:    keyInfo,
			Algorithm:  algorithm,
			Signature:  signature,
			Identifier: identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_onetime_key")
	} else {
		stmt := s.insertOneTimeKeyStmt
		_, err := stmt.ExecContext(ctx, deviceID, userID, keyID, keyInfo, algorithm, signature, identifier)
		return err
	}
}

func (s *oneTimeKeyStatements) onInsertOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, keyInfo, algorithm, signature, identifier string,
) error {
	stmt := s.insertOneTimeKeyStmt
	_, err := stmt.ExecContext(ctx, deviceID, userID, keyID, keyInfo, algorithm, signature, identifier)
	return err
}

func (s *oneTimeKeyStatements) deleteOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, algorithm string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.OneTimeKeyDeleteKey
		update.E2EDBEvents.KeyDelete = &dbtypes.KeyDelete{
			DeviceID:  deviceID,
			UserID:    userID,
			KeyID:     keyID,
			Algorithm: algorithm,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_onetime_key")
	} else {
		stmt := s.deleteOneTimeKeyStmt
		_, err := stmt.ExecContext(ctx, userID, deviceID, algorithm, keyID)
		return err
	}
}

func (s *oneTimeKeyStatements) onDeleteOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, algorithm string,
) error {
	stmt := s.deleteOneTimeKeyStmt
	_, err := stmt.ExecContext(ctx, userID, deviceID, algorithm, keyID)
	return err
}

func (s *oneTimeKeyStatements) deleteMacOneTimeKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.MacOneTimeKeyDeleteKey
		update.E2EDBEvents.MacKeyDelete = &dbtypes.MacKeyDelete{
			DeviceID:   deviceID,
			UserID:     userID,
			Identifier: identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_onetime_key")
	} else {
		stmt := s.deleteMacOneTimeKeyStmt
		_, err := stmt.ExecContext(ctx, userID, identifier, deviceID)
		return err
	}
}

func (s *oneTimeKeyStatements) onDeleteMacOneTimeKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) error {
	stmt := s.deleteMacOneTimeKeyStmt
	_, err := stmt.ExecContext(ctx, userID, identifier, deviceID)
	return err
}

func (s *oneTimeKeyStatements) deleteDeviceOneTimeKey(
	ctx context.Context,
	deviceID, userID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.DeviceOneTimeKeyDeleteKey
		update.E2EDBEvents.DeviceKeyDelete = &dbtypes.DeviceKeyDelete{
			DeviceID: deviceID,
			UserID:   userID,
		}
		return s.db.WriteDBEventWithTbl(&update, "encrypt_onetime_key")
	} else {
		stmt := s.deleteDeviceOneTimeKeyStmt
		_, err := stmt.ExecContext(ctx, userID, deviceID)
		return err
	}
}

func (s *oneTimeKeyStatements) onDeleteDeviceOneTimeKey(
	ctx context.Context,
	deviceID, userID string,
) error {
	stmt := s.deleteDeviceOneTimeKeyStmt
	_, err := stmt.ExecContext(ctx, userID, deviceID)
	return err
}
