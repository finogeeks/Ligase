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

const upsertUserSQL = "" +
	"INSERT INTO users(name) VALUES ($1)" +
	" ON CONFLICT DO NOTHING"

type usersStatements struct {
	upsetUserStmt *sql.Stmt
}

func (s *usersStatements) prepare(db *sql.DB) (err error) {
	if s.upsetUserStmt, err = db.Prepare(upsertUserSQL); err != nil {
		return
	}
	return
}

func (s *usersStatements) upsertUser(
	ctx context.Context, userID string,
) error {
	_, err := s.upsetUserStmt.ExecContext(ctx, userID)
	return err
}
