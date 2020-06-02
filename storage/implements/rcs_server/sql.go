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

type statements struct {
	friendshipStatements
}

func (s *statements) prepare(db *sql.DB, d *Database) error {
	var err error

	for _, prepare := range []func(db *sql.DB, d *Database) error{
		s.friendshipStatements.prepare,
	} {
		if err = prepare(db, d); err != nil {
			return err
		}
	}

	return nil
}
