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

package federation

import (
	"context"
	"database/sql"
)

const missingEventsSchema = `
CREATE TABLE IF NOT EXISTS federation_missing_events (
    room_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
	amount int4 NOT NULL,
	finished bool NOT NULL,
    CONSTRAINT federation_missing_events_unique UNIQUE (room_id, event_id)
);
`

const insertMissingEventsSQL = "" +
	"INSERT INTO federation_missing_events (room_id, event_id, amount, finished)" +
	" VALUES ($1, $2, $3, FALSE)"

const updateMissingEventsSQL = "" +
	"UPDATE federation_missing_events SET finished = $3 WHERE room_id = $1 AND event_id = $2"

const selectMissingEventsSQL = "" +
	"SELECT room_id, event_id, amount FROM federation_missing_events WHERE finished = FALSE"

type missingEventsStatements struct {
	insertMissingEventsStmt *sql.Stmt
	updateMissingEventsStmt *sql.Stmt
	selectMissingEventsStmt *sql.Stmt
}

func (s *missingEventsStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(missingEventsSchema)
	if err != nil {
		return err
	}
	if s.insertMissingEventsStmt, err = db.Prepare(insertMissingEventsSQL); err != nil {
		return
	}
	if s.updateMissingEventsStmt, err = db.Prepare(updateMissingEventsSQL); err != nil {
		return
	}
	if s.selectMissingEventsStmt, err = db.Prepare(selectMissingEventsSQL); err != nil {
		return
	}
	return
}

func (s *missingEventsStatements) insertMissingEvents(
	ctx context.Context,
	roomID, eventID string, amount int,
) error {
	_, err := s.insertMissingEventsStmt.ExecContext(ctx, roomID, eventID, amount)
	return err
}

func (s *missingEventsStatements) updateMissingEvents(ctx context.Context, roomID, eventID string, finished bool) error {
	_, err := s.updateMissingEventsStmt.ExecContext(ctx, roomID, eventID, finished)
	return err
}

func (s *missingEventsStatements) selectMissingEvents(
	ctx context.Context,
) (roomIDs, eventIDs []string, amounts []int, err error) {
	var roomID string
	var eventID string
	var amount int
	rows, err := s.selectMissingEventsStmt.QueryContext(ctx)
	if err != nil {
		return
	}
	for rows.Next() {
		err = rows.Scan(&roomID, &eventID, &amount)
		if err != nil {
			return
		}
		roomIDs = append(roomIDs, roomID)
		eventIDs = append(eventIDs, eventID)
		amounts = append(amounts, amount)
	}
	return
}
