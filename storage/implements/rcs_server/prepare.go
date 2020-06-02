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

package rcs_server

import "database/sql"

// a statementList is a list of SQL statements to prepare and a pointer to where to store the resulting prepared statement.
type statementList []struct {
	statement **sql.Stmt
	sql       string
}

// prepare the SQL for each statement in the list and assign the result to the prepared statement.
// nolint: safesql
func (s statementList) prepare(db *sql.DB) (err error) {
	for _, statement := range s {
		if *statement.statement, err = db.Prepare(statement.sql); err != nil {
			return
		}
	}
	return
}
