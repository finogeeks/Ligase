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
	log "github.com/finogeeks/ligase/skunkworks/log"
)

const algorithmSchema = `
-- The media_repository table holds metadata for each media file stored and accessible to the local server,
-- the actual file is stored separately.
CREATE TABLE IF NOT EXISTS encrypt_algorithm (
    device_id TEXT 				NOT NULL,
    user_id TEXT 				NOT NULL,
	algorithms TEXT 			NOT NULL,
	identifier TEXT NOT NULL DEFAULT '',
	CONSTRAINT encrypt_algorithm_unique UNIQUE (device_id, user_id),
    PRIMARY KEY(device_id, user_id)
);

CREATE INDEX IF NOT EXISTS encrypt_algorithm_user_id ON encrypt_algorithm(user_id);
CREATE INDEX IF NOT EXISTS encrypt_algorithm_device_id ON encrypt_algorithm(device_id);
`
const insertAlSQL = `
INSERT INTO encrypt_algorithm (device_id, user_id, algorithms, identifier) 
VALUES ($1, $2, $3, $4) on conflict (device_id, user_id) do UPDATE SET algorithms = EXCLUDED.algorithms, identifier = EXCLUDED.identifier
`

const recoverAlsSQL = `
SELECT device_id, user_id, algorithms, identifier FROM encrypt_algorithm limit $1 offset $2
`

const deleteAlSQL = `
DELETE FROM encrypt_algorithm WHERE device_id = $1 AND user_id = $2
`

const deleteMacAlSQL = `
DELETE FROM encrypt_algorithm WHERE user_id = $1 AND identifier = $2 AND device_id != $3
`

type alStatements struct {
	db              *Database
	insertAlStmt    *sql.Stmt
	recoverAlsStmt  *sql.Stmt
	deleteAlStmt    *sql.Stmt
	deleteMacAlStmt *sql.Stmt
}

func (s *alStatements) prepare(d *Database) (err error) {
	s.db = d
	_, err = d.db.Exec(algorithmSchema)
	if err != nil {
		return
	}
	if s.insertAlStmt, err = d.db.Prepare(insertAlSQL); err != nil {
		return
	}
	if s.recoverAlsStmt, err = d.db.Prepare(recoverAlsSQL); err != nil {
		return
	}
	if s.deleteAlStmt, err = d.db.Prepare(deleteAlSQL); err != nil {
		return
	}
	if s.deleteMacAlStmt, err = d.db.Prepare(deleteMacAlSQL); err != nil {
		return
	}
	return
}

func (s *alStatements) recoverAls() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverAlsStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *alStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var alsInsert dbtypes.AlInsert
		if err1 := rows.Scan(&alsInsert.DeviceID, &alsInsert.UserID, &alsInsert.Algorithm, &alsInsert.Identifier); err1 != nil {
			log.Errorf("load algorithm error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}

		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.AlInsertKey
		update.IsRecovery = true
		update.E2EDBEvents.AlInsert = &alsInsert
		update.SetUid(int64(common.CalcStringHashCode64(alsInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "encrypt_algorithm")
		if err2 != nil {
			log.Errorf("update algorithm cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

// persist algorithms
func (s *alStatements) insertAl(
	ctx context.Context,
	userID, deviceID, algorithms, identifier string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.AlInsertKey
		update.E2EDBEvents.AlInsert = &dbtypes.AlInsert{
			DeviceID:   deviceID,
			UserID:     userID,
			Algorithm:  algorithms,
			Identifier: identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_algorithm")
	} else {
		stmt := s.insertAlStmt
		_, err := stmt.ExecContext(ctx, deviceID, userID, algorithms, identifier)
		return err
	}
}

func (s *alStatements) onInsertAl(
	ctx context.Context,
	userID, deviceID, algorithms, identifier string,
) error {
	stmt := s.insertAlStmt
	_, err := stmt.ExecContext(ctx, deviceID, userID, algorithms, identifier)
	return err
}

func (s *alStatements) deleteAl(
	ctx context.Context,
	userID, deviceID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.DeviceAlDeleteKey
		update.E2EDBEvents.DeviceKeyDelete = &dbtypes.DeviceKeyDelete{
			DeviceID: deviceID,
			UserID:   userID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_algorithm")
	} else {
		stmt := s.deleteAlStmt
		_, err := stmt.ExecContext(ctx, deviceID, userID)
		return err
	}
}

func (s *alStatements) onDeleteAl(
	ctx context.Context,
	userID, deviceID string,
) error {
	stmt := s.deleteAlStmt
	_, err := stmt.ExecContext(ctx, deviceID, userID)
	return err
}

func (s *alStatements) deleteMacAl(
	ctx context.Context,
	userID, deviceID, identifier string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_E2E_DB_EVENT
		update.Key = dbtypes.MacDeviceAlDeleteKey
		update.E2EDBEvents.MacKeyDelete = &dbtypes.MacKeyDelete{
			DeviceID:   deviceID,
			UserID:     userID,
			Identifier: identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "encrypt_algorithm")
	} else {
		stmt := s.deleteMacAlStmt
		_, err := stmt.ExecContext(ctx, deviceID, userID, identifier)
		return err
	}
}

func (s *alStatements) onDeleteMacAl(
	ctx context.Context,
	userID, deviceID, identifier string,
) error {
	stmt := s.deleteMacAlStmt
	_, err := stmt.ExecContext(ctx, deviceID, userID, identifier)
	return err
}
