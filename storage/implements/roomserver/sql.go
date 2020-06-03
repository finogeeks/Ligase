// Copyright 2017 Vector Creations Ltd
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

package roomserver

import (
	"database/sql"
)

type statements struct {
	roomStatements
	eventStatements
	eventJSONStatements
	stateSnapshotStatements
	roomAliasesStatements
	inviteStatements
	membershipStatements
	roomDomainsStatements
	settingsStatements
}

func (s *statements) prepare(db *sql.DB, d *Database) error {
	var err error

	for _, prepare := range []func(db *sql.DB, d *Database) error{
		s.roomStatements.prepare,
		s.eventStatements.prepare,
		s.eventJSONStatements.prepare,
		s.stateSnapshotStatements.prepare,
		s.roomAliasesStatements.prepare,
		s.inviteStatements.prepare,
		s.membershipStatements.prepare,
		s.roomDomainsStatements.prepare,
		s.settingsStatements.prepare,
	} {
		if err = prepare(db, d); err != nil {
			return err
		}
	}

	return nil
}
