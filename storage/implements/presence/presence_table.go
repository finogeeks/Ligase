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

package presence

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const presenceSchema = `
-- Stores data about presence.
CREATE TABLE IF NOT EXISTS presence_presences (
    user_id TEXT NOT NULL,
	status TEXT,
    status_msg TEXT,
	ext_status_msg TEXT,
	CONSTRAINT presence_presences_unique UNIQUE (user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS presences_user_id_idx ON presence_presences(user_id);
`

const upsertPresencesSQL = "" +
	"INSERT INTO presence_presences(user_id, status, status_msg, ext_status_msg) VALUES ($1, $2, $3, $4)" +
	" ON CONFLICT ON CONSTRAINT presence_presences_unique" +
	" DO UPDATE SET status = EXCLUDED.status, status_msg = EXCLUDED.status_msg"

const recoverPresencesSQL = "" +
	"SELECT user_id, status, status_msg, ext_status_msg FROM presence_presences limit $1 offset $2"

type presencesStatements struct {
	db                   *Database
	upsertPresencesStmt  *sql.Stmt
	recoverPresencesStmt *sql.Stmt
}

func (s *presencesStatements) getSchema() string {
	return presenceSchema
}

func (s *presencesStatements) prepare(d *Database) (err error) {
	s.db = d
	if s.upsertPresencesStmt, err = d.db.Prepare(upsertPresencesSQL); err != nil {
		return
	}
	if s.recoverPresencesStmt, err = d.db.Prepare(recoverPresencesSQL); err != nil {
		return
	}
	return
}

func (s *presencesStatements) recoverPresences() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverPresencesStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *presencesStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var presencesInsert dbtypes.PresencesInsert
		if err1 := rows.Scan(&presencesInsert.UserID, &presencesInsert.Status, &presencesInsert.StatusMsg, &presencesInsert.ExtStatusMsg); err1 != nil {
			log.Errorf("load presence error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PRESENCE_DB_EVENT
		update.Key = dbtypes.PresencesInsertKey
		update.IsRecovery = true
		update.PresenceDBEvents.PresencesInsert = &presencesInsert
		update.SetUid(int64(common.CalcStringHashCode64(presencesInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "presence_presences")
		if err2 != nil {
			log.Errorf("update presence cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *presencesStatements) upsertPresences(
	ctx context.Context, userID, status, statusMsg, extStatusMsg string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PRESENCE_DB_EVENT
		update.Key = dbtypes.PresencesInsertKey
		update.PresenceDBEvents.PresencesInsert = &dbtypes.PresencesInsert{
			UserID:       userID,
			Status:       status,
			StatusMsg:    statusMsg,
			ExtStatusMsg: extStatusMsg,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "presence_presences")
	} else {
		_, err := s.upsertPresencesStmt.ExecContext(ctx, userID, status, statusMsg, extStatusMsg)
		return err
	}
}

func (s *presencesStatements) onUpsertPresences(
	ctx context.Context, userID, status, statusMsg, extStatusMsg string,
) error {
	_, err := s.upsertPresencesStmt.ExecContext(ctx, userID, status, statusMsg, extStatusMsg)
	return err
}
