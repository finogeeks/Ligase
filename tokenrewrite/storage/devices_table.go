// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package storage

import (
	"context"
	"database/sql"
)

const upsertDeviceSQL = "" +
	"INSERT INTO devices(user_id,device_id,display_name) VALUES ($1, $2, $3)" +
	" ON CONFLICT DO NOTHING"

type devicesStatements struct {
	upsetDeviceStmt *sql.Stmt
}

func (s *devicesStatements) prepare(db *sql.DB) (err error) {
	if s.upsetDeviceStmt, err = db.Prepare(upsertDeviceSQL); err != nil {
		return
	}
	return
}

func (s *devicesStatements) upsertDevice(
	ctx context.Context, userID, deviceID, displayName string,
) error {
	_, err := s.upsetDeviceStmt.ExecContext(ctx, userID, deviceID, displayName)
	return err
}
