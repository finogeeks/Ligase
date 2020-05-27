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

const upsertTokenSQL = "" +
	"INSERT INTO access_tokens(id, user_id, device_id, token) VALUES ($1, $2, $3, $4)" +
	" ON CONFLICT DO NOTHING"

type tokensStatements struct {
	upsetTokenStmt *sql.Stmt
}

func (s *tokensStatements) prepare(db *sql.DB) (err error) {
	if s.upsetTokenStmt, err = db.Prepare(upsertTokenSQL); err != nil {
		return
	}
	return
}

func (s *tokensStatements) upsertToken(
	ctx context.Context, id int64, userID, deviceID, token string,
) error {
	_, err := s.upsetTokenStmt.ExecContext(ctx, id, userID, deviceID, token)
	return err
}
