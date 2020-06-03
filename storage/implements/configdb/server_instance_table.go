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

package configdb

import (
	"context"
	"database/sql"
)

const serverInstanceSchema = `
CREATE TABLE IF NOT EXISTS server_instance (
	id BIGINT NOT NULL,
	name TEXT NOT NULL PRIMARY KEY,
	CONSTRAINT inst_unique UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS server_instance_id ON server_instance(name);
`

const upsertServerInstanceSQL = "" +
	"INSERT INTO server_instance (id,name)" +
	" VALUES ($1,$2)" +
	" ON CONFLICT ON CONSTRAINT inst_unique " +
	" DO UPDATE SET id = EXCLUDED.id, name = EXCLUDED.name"

const selectServerInstanceSQL = "" +
	"SELECT id FROM server_instance where name = $1 limit 1"

type serverInstanceStatements struct {
	db                       *Database
	upsertServerInstanceStmt *sql.Stmt
	selectServerInstanceStmt *sql.Stmt
}

func (s *serverInstanceStatements) getSchema() string {
	return serverInstanceSchema
}

func (s *serverInstanceStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.upsertServerInstanceStmt, err = d.db.Prepare(upsertServerInstanceSQL); err != nil {
		return
	}
	if s.selectServerInstanceStmt, err = d.db.Prepare(selectServerInstanceSQL); err != nil {
		return
	}
	return
}

func (s *serverInstanceStatements) upsertServerInstance(
	ctx context.Context,
	nid int64,
	serverName string,
) error {
	_, err := s.upsertServerInstanceStmt.ExecContext(
		ctx,
		nid,
		serverName,
	)
	return err
}

func (s *serverInstanceStatements) selectServerInstance(
	ctx context.Context,
	servername string,
) (instance int64, err error) {
	stmt := s.selectServerInstanceStmt
	err = stmt.QueryRowContext(ctx, servername).Scan(&instance)
	if err != nil {
		if err != sql.ErrNoRows {
			return 0, err
		} else {
			return 0, nil
		}
	}
	return instance, nil
}
