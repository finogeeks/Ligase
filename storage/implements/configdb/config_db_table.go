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

const serverNameSchema = `
CREATE TABLE IF NOT EXISTS server_name (
	id BIGINT NOT NULL,
	name TEXT NOT NULL PRIMARY KEY,
	CONSTRAINT name_unique UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS server_name_id ON server_name(name);
`

const upsertServerNameSQL = "" +
	"INSERT INTO server_name (id,name)" +
	" VALUES ($1,$2)" +
	" ON CONFLICT ON CONSTRAINT name_unique " +
	" DO UPDATE SET id = EXCLUDED.id, name = EXCLUDED.name"

const selectServerNameSQL = "" +
	"SELECT name FROM server_name"

type configDbStatements struct {
	db                   *Database
	upsertServerNameStmt *sql.Stmt
	selectServerNameStmt *sql.Stmt
}

func (s *configDbStatements) getSchema() string {
	return serverNameSchema
}

func (s *configDbStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.upsertServerNameStmt, err = d.db.Prepare(upsertServerNameSQL); err != nil {
		return
	}
	if s.selectServerNameStmt, err = d.db.Prepare(selectServerNameSQL); err != nil {
		return
	}
	return
}

func (s *configDbStatements) upsertServerName(
	ctx context.Context,
	nid int64,
	serverName string,
) error {
	_, err := s.upsertServerNameStmt.ExecContext(
		ctx,
		nid,
		serverName,
	)
	return err
}

func (s *configDbStatements) selectServerNames(
	ctx context.Context,
) (results []string, err error) {
	stmt := s.selectServerNameStmt
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var serverName string
		if err = rows.Scan(&serverName); err != nil {
			return nil, err
		}
		results = append(results, serverName)
	}
	return
}
