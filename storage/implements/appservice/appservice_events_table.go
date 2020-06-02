// Copyright 2018 New Vector Ltd
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

package appservice

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/finogeeks/ligase/common/encryption"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

const appserviceEventsSchema = `
-- Stores events to be sent to application services
CREATE TABLE IF NOT EXISTS appservice_events (
	-- An auto-incrementing id unique to each event in the table
	id BIGSERIAL NOT NULL PRIMARY KEY,
	-- The ID of the application service the event will be sent to
	as_id TEXT NOT NULL,
	-- JSON representation of the event
	event_json TEXT NOT NULL,
	-- The ID of the transaction that this event is a part of
	txn_id BIGINT NOT NULL,
	-- The event type e.g 'm.room.message'
    type TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS appservice_events_as_id ON appservice_events(as_id);

-- mirror table for debug, plaintext storage
CREATE TABLE IF NOT EXISTS appservice_events_mirror (
	-- An auto-incrementing id unique to each event in the table
	id BIGSERIAL NOT NULL PRIMARY KEY,
	-- The ID of the application service the event will be sent to
	as_id TEXT NOT NULL,
	-- JSON representation of the event
	event_json TEXT NOT NULL,
	-- The ID of the transaction that this event is a part of
	txn_id BIGINT NOT NULL,
	-- The event type e.g 'm.room.message'
    type TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS appservice_events_as_id_mirror ON appservice_events_mirror(as_id);
`

const selectEventsByApplicationServiceIDSQL = "" +
	"SELECT id, event_json, txn_id, type " +
	"FROM appservice_events WHERE as_id = $1 ORDER BY txn_id DESC, id ASC"

const countEventsByApplicationServiceIDSQL = "" +
	"SELECT COUNT(id) FROM appservice_events WHERE as_id = $1"

const insertEventSQL = "" +
	"INSERT INTO appservice_events(as_id, event_json, txn_id, type) " +
	"VALUES ($1, $2, $3, $4)"

const insertEventSQLMirror = "" +
	"INSERT INTO appservice_events_mirror(as_id, event_json, txn_id, type) " +
	"VALUES ($1, $2, $3, $4)"

const updateTxnIDForEventsSQL = "" +
	"UPDATE appservice_events SET txn_id = $1 WHERE as_id = $2 AND id <= $3"

const deleteEventsBeforeAndIncludingIDSQL = "" +
	"DELETE FROM appservice_events WHERE as_id = $1 AND id <= $2"

const selectMsgEventsWithLimitSQL = "" +
	"SELECT id, event_json FROM appservice_events " +
	"WHERE type = 'm.room.message' AND event_json LIKE '%content%' AND id >= $1 ORDER BY id ASC LIMIT $2"

const selectMsgEventsCountSQL = "" +
	"SELECT count(*), COALESCE(MIN(id), 0) FROM appservice_events WHERE type = 'm.room.message' AND event_json LIKE '%content%'"

const updateMsgEventSQL = "" +
	"UPDATE appservice_events SET event_json = $1 WHERE id = $2"

const selectEncMsgEventsWithLimitSQL = "" +
	"SELECT id, event_json FROM appservice_events " +
	"WHERE event_json NOT LIKE '%{%' AND id > $1 ORDER BY id ASC LIMIT $2"

const selectEncMsgEventsCountSQL = "" +
	"SELECT count(*) FROM appservice_events WHERE event_json NOT LIKE '%{%'"

const (
	// A transaction ID number that no transaction should ever have. Used for
	// checking again the default value.
	invalidTxnID = -2
)

type eventsStatements struct {
	selectEventsByApplicationServiceIDStmt *sql.Stmt
	countEventsByApplicationServiceIDStmt  *sql.Stmt
	insertEventStmt                        *sql.Stmt
	insertEventStmtMirror                  *sql.Stmt
	updateTxnIDForEventsStmt               *sql.Stmt
	deleteEventsBeforeAndIncludingIDStmt   *sql.Stmt
	selectMsgEventsWithLimitStmt           *sql.Stmt
	selectMsgEventsCountStmt               *sql.Stmt
	updateMsgEventStmt                     *sql.Stmt
	selectEncMsgEventsWithLimitStmt        *sql.Stmt
	selectEncMsgEventsCountStmt            *sql.Stmt
}

func (s *eventsStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(appserviceEventsSchema)
	if err != nil {
		return
	}

	if s.selectEventsByApplicationServiceIDStmt, err = db.Prepare(selectEventsByApplicationServiceIDSQL); err != nil {
		return
	}
	if s.countEventsByApplicationServiceIDStmt, err = db.Prepare(countEventsByApplicationServiceIDSQL); err != nil {
		return
	}
	if s.insertEventStmt, err = db.Prepare(insertEventSQL); err != nil {
		return
	}
	if s.insertEventStmtMirror, err = db.Prepare(insertEventSQLMirror); err != nil {
		return
	}
	if s.updateTxnIDForEventsStmt, err = db.Prepare(updateTxnIDForEventsSQL); err != nil {
		return
	}
	if s.deleteEventsBeforeAndIncludingIDStmt, err = db.Prepare(deleteEventsBeforeAndIncludingIDSQL); err != nil {
		return
	}
	if s.selectMsgEventsWithLimitStmt, err = db.Prepare(selectMsgEventsWithLimitSQL); err != nil {
		return
	}
	if s.selectMsgEventsCountStmt, err = db.Prepare(selectMsgEventsCountSQL); err != nil {
		return
	}
	if s.updateMsgEventStmt, err = db.Prepare(updateMsgEventSQL); err != nil {
		return
	}
	if s.selectEncMsgEventsWithLimitStmt, err = db.Prepare(selectEncMsgEventsWithLimitSQL); err != nil {
		return
	}
	if s.selectEncMsgEventsCountStmt, err = db.Prepare(selectEncMsgEventsCountSQL); err != nil {
		return
	}

	return
}

// selectEventsByApplicationServiceID takes in an application service ID and
// returns a slice of events that need to be sent to that application service,
// as well as an int later used to remove these same events from the database
// once successfully sent to an application service.
func (s *eventsStatements) selectEventsByApplicationServiceID(
	ctx context.Context,
	applicationServiceID string,
	limit int,
) (
	txnID, maxID int,
	events []gomatrixserverlib.ClientEvent,
	eventsRemaining bool,
	err error,
) {
	// Retrieve events from the database. Unsuccessfully sent events first
	eventRows, err := s.selectEventsByApplicationServiceIDStmt.QueryContext(ctx, applicationServiceID)
	if err != nil {
		return
	}
	defer func() {
		err = eventRows.Close()
		if err != nil {
			log.Fatalw("appservice unable to select new events to send", log.KeysAndValues{"appservice", applicationServiceID, "error", err})
		}
	}()
	events, maxID, txnID, eventsRemaining, err = retrieveEvents(eventRows, limit)
	if err != nil {
		log.Fatalw("retrieveEvents error", log.KeysAndValues{"appservice", applicationServiceID, "error", err})
		return
	}

	return
}

func retrieveEvents(eventRows *sql.Rows, limit int) (events []gomatrixserverlib.ClientEvent, maxID, txnID int, eventsRemaining bool, err error) {

	// Iterate through each row and store event contents
	// If txn_id changes dramatically, we've switched from collecting old events to
	// new ones. Send back those events first.
	lastTxnID := invalidTxnID
	for eventsProcessed := 0; eventRows.Next(); {
		var event gomatrixserverlib.ClientEvent
		var eventJSON []byte
		var id int
		var eventType string
		err = eventRows.Scan(
			&id,
			&eventJSON,
			&txnID,
			&eventType,
		)
		if err != nil {
			log.Warnw("eventRows sacn error", log.KeysAndValues{"error", err})
			return nil, 0, 0, false, err
		}

		if encryption.CheckCrypto(eventType) {
			eventJSON = encryption.Decrypt(eventJSON)
		}
		if err = json.Unmarshal(eventJSON, &event); err != nil {
			log.Warnw("Unmarshal json error", log.KeysAndValues{"error", err})
			return nil, 0, 0, false, err
		}

		// If txnID has changed on this event from the previous event, then we've
		// reached the end of a transaction's events. Return only those events.
		if lastTxnID > invalidTxnID && lastTxnID != txnID {
			return events, maxID, lastTxnID, true, nil
		}
		lastTxnID = txnID

		// Limit events that aren't part of an old transaction
		if txnID == -1 {
			// Return if we've hit the limit
			if eventsProcessed++; eventsProcessed > limit {
				return events, maxID, lastTxnID, true, nil
			}
		}

		if id > maxID {
			maxID = id
		}

		events = append(events, event)
	}

	return
}

// countEventsByApplicationServiceID inserts an event mapped to its corresponding application service
// IDs into the db.
func (s *eventsStatements) countEventsByApplicationServiceID(
	ctx context.Context,
	appServiceID string,
) (int, error) {
	var count int
	err := s.countEventsByApplicationServiceIDStmt.QueryRowContext(ctx, appServiceID).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	return count, nil
}

// insertEvent inserts an event mapped to its corresponding application service
// IDs into the db.
func (s *eventsStatements) insertEvent(
	ctx context.Context,
	appServiceID string,
	event *gomatrixserverlib.ClientEvent,
) (err error) {
	// Convert event to JSON before inserting
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if encryption.CheckCrypto(event.Type) {
		if encryption.CheckMirror(event.Type) {
			_, err = s.insertEventStmtMirror.ExecContext(
				ctx,
				appServiceID,
				eventJSON,
				-1, // No transaction ID yet
				event.Type,
			)
		}

		eventJSON = encryption.Encrypt(eventJSON)
	}
	_, err = s.insertEventStmt.ExecContext(
		ctx,
		appServiceID,
		eventJSON,
		-1, // No transaction ID yet
		event.Type,
	)

	return
}

// updateTxnIDForEvents sets the transactionID for a collection of events. Done
// before sending them to an AppService. Referenced before sending to make sure
// we aren't constructing multiple transactions with the same events.
func (s *eventsStatements) updateTxnIDForEvents(
	ctx context.Context,
	appserviceID string,
	maxID, txnID int,
) (err error) {
	_, err = s.updateTxnIDForEventsStmt.ExecContext(ctx, txnID, appserviceID, maxID)
	return
}

// deleteEventsBeforeAndIncludingID removes events matching given IDs from the database.
func (s *eventsStatements) deleteEventsBeforeAndIncludingID(
	ctx context.Context,
	appserviceID string,
	eventTableID int,
) (err error) {
	_, err = s.deleteEventsBeforeAndIncludingIDStmt.ExecContext(ctx, appserviceID, eventTableID)
	return
}

func (s *eventsStatements) selectMsgEventsWithLimit(ctx context.Context, limit, offset int64) ([]int64, [][]byte, error) {
	var rows *sql.Rows
	var err error

	rows, err = s.selectMsgEventsWithLimitStmt.QueryContext(ctx, offset, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := [][]byte{}
	var ids []int64
	for rows.Next() {
		var eventBytes []byte
		var id int64
		if err = rows.Scan(&id, &eventBytes); err != nil {
			return nil, nil, err
		}

		// get raw, don't decrypt

		ids = append(ids, id)
		result = append(result, eventBytes)
	}
	return ids, result, nil
}

func (s *eventsStatements) getMsgEventsTotal(
	ctx context.Context,
) (count int, minID int64, err error) {
	err = s.selectMsgEventsCountStmt.QueryRowContext(ctx).Scan(&count, &minID)
	return
}

func (s *eventsStatements) updateMsgEvent(
	ctx context.Context,
	id int64,
	NewBytes []byte,
) error {
	_, err := s.updateMsgEventStmt.ExecContext(
		ctx, NewBytes, id,
	)
	return err
}

func (s *eventsStatements) getEncMsgEventsTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectEncMsgEventsCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}

func (s *eventsStatements) selectEncMsgEventsWithLimit(ctx context.Context, limit, offset int64) ([]int64, [][]byte, error) {
	var rows *sql.Rows
	var err error

	rows, err = s.selectEncMsgEventsWithLimitStmt.QueryContext(ctx, offset, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() // nolint: errcheck

	result := [][]byte{}
	var ids []int64
	for rows.Next() {
		var eventBytes []byte
		var id int64
		if err = rows.Scan(&id, &eventBytes); err != nil {
			return nil, nil, err
		}

		// get raw, don't decrypt

		ids = append(ids, id)
		result = append(result, eventBytes)
	}
	return ids, result, nil
}
