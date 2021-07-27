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

package syncapi

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

// we treat send to device as abbrev as STD in the context below.

const sendToDeviceSchema = `
CREATE TABLE IF NOT EXISTS syncapi_send_to_device (
	--id BIGINT PRIMARY KEY DEFAULT nextval('syncapi_stream_id'),
	id BIGINT PRIMARY KEY,
	sender TEXT NOT NULL,
	event_type TEXT NOT NULL,
	target_device_id TEXT NOT NULL,
	target_user_id TEXT NOT NULL,	
	event_json TEXT NOT NULL,
	identifier TEXT  NOT NULL DEFAULT '',
CONSTRAINT syncapi_send_to_device_unique UNIQUE (id, target_device_id, target_user_id)
);

CREATE INDEX IF NOT EXISTS syncapi_send_to_device_user_id_device_id_id_desc_idx ON syncapi_send_to_device(target_user_id,target_device_id, id desc nulls last);
`

const insertSTDSQL = "" +
	"INSERT INTO syncapi_send_to_device (" +
	" id, sender, event_type, target_user_id, target_device_id,event_json, identifier" +
	") VALUES ($1, $2, $3, $4, $5, $6, $7)" +
	" RETURNING id"

const selectHistorySTDEventsStreamSQL = "" +
	"SELECT id, sender, event_type, event_json FROM syncapi_send_to_device" +
	" WHERE target_user_id = $1 AND target_device_id = $2" +
	" ORDER BY id DESC LIMIT $3"

const selectHistorySTDEventsStreamAfterSQL = "" +
	"SELECT id, sender, event_type, event_json FROM syncapi_send_to_device" +
	" WHERE target_user_id = $1 AND target_device_id = $2 AND id > $3" +
	" ORDER BY id LIMIT $4"

const deleteSTDSQL = "" +
	"DELETE FROM syncapi_send_to_device WHERE target_user_id = $1 AND target_device_id = $2 AND id <= $3"

const deleteDeviceSTDSQL = "" +
	"DELETE FROM syncapi_send_to_device WHERE target_user_id = $1 AND target_device_id = $2"

const deleteMacSTDSQL = "" +
	"DELETE FROM syncapi_send_to_device WHERE target_user_id = $1 AND identifier = $2 AND target_device_id != $3"

type stdEventsStatements struct {
	db                                    *Database
	insertStdEventStmt                    *sql.Stmt
	selectHistorySTDEventsStreamStmt      *sql.Stmt
	selectHistorySTDEventsStreamAfterStmt *sql.Stmt
	deleteStdEventStmt                    *sql.Stmt
	deleteDeviceStdEventStmt              *sql.Stmt
	deleteMacStdEventStmt                 *sql.Stmt
}

func (s *stdEventsStatements) getSchema() string {
	return sendToDeviceSchema
}

func (s *stdEventsStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	_, err = db.Exec(sendToDeviceSchema)
	if err != nil {
		return
	}
	if s.insertStdEventStmt, err = db.Prepare(insertSTDSQL); err != nil {
		return
	}
	if s.selectHistorySTDEventsStreamStmt, err = db.Prepare(selectHistorySTDEventsStreamSQL); err != nil {
		return
	}
	if s.selectHistorySTDEventsStreamAfterStmt, err = db.Prepare(selectHistorySTDEventsStreamAfterSQL); err != nil {
		return
	}
	if s.deleteStdEventStmt, err = db.Prepare(deleteSTDSQL); err != nil {
		return
	}
	if s.deleteDeviceStdEventStmt, err = db.Prepare(deleteDeviceSTDSQL); err != nil {
		return
	}
	if s.deleteMacStdEventStmt, err = db.Prepare(deleteMacSTDSQL); err != nil {
		return
	}
	return
}

func (s *stdEventsStatements) insertStdEvent(
	ctx context.Context, id int64, stdEvent syncapitypes.StdHolder,
	targetUID, targetDevice, identifier string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncStdEventInertKey
		update.SyncDBEvents.SyncStdEventInsert = &dbtypes.SyncStdEventInsert{
			ID:           id,
			StdEvent:     stdEvent,
			TargetUID:    targetUID,
			TargetDevice: targetDevice,
			Identifier:   identifier,
		}
		update.SetUid(int64(common.CalcStringHashCode64(targetUID)))
		s.db.WriteDBEventWithTbl(&update, "syncapi_send_to_device")
		return nil
	} else {
		_, err = s.insertStdEventStmt.ExecContext(
			ctx,
			id,
			stdEvent.Sender,
			stdEvent.EventTyp,
			targetUID,
			targetDevice,
			stdEvent.Event,
			identifier,
		)
		return err
	}
}

func (s *stdEventsStatements) onInsertStdEvent(
	ctx context.Context, id int64, stdEvent syncapitypes.StdHolder,
	targetUID, targetDevice, identifier string,
) (streamPos int64, err error) {
	err = s.insertStdEventStmt.QueryRowContext(
		ctx,
		id,
		stdEvent.Sender,
		stdEvent.EventTyp,
		targetUID,
		targetDevice,
		stdEvent.Event,
		identifier,
	).Scan(&streamPos)
	return
}

func (s *stdEventsStatements) selectHistoryStream(
	ctx context.Context,
	targetUserID,
	targetDeviceID string,
	limit int64,
) (streams []types.StdEvent, offset []int64, err error) {
	rows, err := s.selectHistorySTDEventsStreamStmt.QueryContext(ctx, targetUserID, targetDeviceID, limit)
	if err != nil {
		log.Errorf("stdEventsStatements.selectHistoryStream err: %v", err)
		return
	}
	streams = []types.StdEvent{}
	offset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.StdEvent
		var streamPos int64
		var eventJSON []byte
		if err := rows.Scan(&streamPos, &stream.Sender, &stream.Type, &eventJSON); err != nil {
			return nil, nil, err
		}
		err := json.Unmarshal(eventJSON, &stream.Content)
		if err != nil {
			log.Errorf("stdEventsStatements.selectHistoryStream err: %v", err)
			continue
		}

		streams = append(streams, stream)
		offset = append(offset, streamPos)
	}
	return
}

func (s *stdEventsStatements) selectHistoryStreamAfter(
	ctx context.Context,
	targetUserID,
	targetDeviceID string,
	afterOffset int64,
	limit int64,
) (streams []types.StdEvent, offset []int64, err error) {
	rows, err := s.selectHistorySTDEventsStreamAfterStmt.QueryContext(ctx, targetUserID, targetDeviceID, afterOffset, limit)
	if err != nil {
		log.Errorf("stdEventsStatements.selectHistoryStreamAfter err: %v", err)
		return
	}
	streams = []types.StdEvent{}
	offset = []int64{}
	defer rows.Close()
	for rows.Next() {
		var stream types.StdEvent
		var streamPos int64
		var eventJSON []byte
		if err := rows.Scan(&streamPos, &stream.Sender, &stream.Type, &eventJSON); err != nil {
			return nil, nil, err
		}
		err := json.Unmarshal(eventJSON, &stream.Content)
		if err != nil {
			log.Errorf("stdEventsStatements.selectHistoryStreamAfter err: %v", err)
			continue
		}

		streams = append(streams, stream)
		offset = append(offset, streamPos)
	}
	return
}

func (s *stdEventsStatements) deleteStdEvent(
	ctx context.Context, id int64,
	userID, deviceID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncStdEventDeleteKey
		update.SyncDBEvents.SyncStdEventDelete = &dbtypes.SyncStdEventDelete{
			ID:           id,
			TargetDevice: deviceID,
			TargetUID:    userID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "syncapi_send_to_device")
	} else {
		stmt := s.deleteStdEventStmt
		_, err := stmt.ExecContext(ctx, userID, deviceID, id)
		return err
	}
}

func (s *stdEventsStatements) onDeleteStdEvent(
	ctx context.Context, id int64,
	userID, deviceID string,
) error {
	stmt := s.deleteStdEventStmt
	_, err := stmt.ExecContext(ctx, userID, deviceID, id)
	return err
}

func (s *stdEventsStatements) deleteMacStdEvent(
	ctx context.Context, identifier,
	userID, deviceID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncMacStdEventDeleteKey
		update.SyncDBEvents.SyncMacStdEventDelete = &dbtypes.SyncMacStdEventDelete{
			Identifier:   identifier,
			TargetDevice: deviceID,
			TargetUID:    userID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "syncapi_send_to_device")
	} else {
		stmt := s.deleteMacStdEventStmt
		_, err := stmt.ExecContext(ctx, userID, identifier, deviceID)
		return err
	}
}

func (s *stdEventsStatements) onDeleteMacStdEvent(
	ctx context.Context, identifier,
	userID, deviceID string,
) error {
	stmt := s.deleteMacStdEventStmt
	_, err := stmt.ExecContext(ctx, userID, identifier, deviceID)
	return err
}

func (s *stdEventsStatements) deleteDeviceStdEvent(
	ctx context.Context,
	userID, deviceID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_SYNC_DB_EVENT
		update.Key = dbtypes.SyncDeviceStdEventDeleteKey
		update.SyncDBEvents.SyncStdEventDelete = &dbtypes.SyncStdEventDelete{
			TargetDevice: deviceID,
			TargetUID:    userID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "syncapi_send_to_device")
	} else {
		stmt := s.deleteDeviceStdEventStmt
		_, err := stmt.ExecContext(ctx, userID, deviceID)
		return err
	}
}

func (s *stdEventsStatements) onDeleteDeviceStdEvent(
	ctx context.Context,
	userID, deviceID string,
) error {
	stmt := s.deleteDeviceStdEventStmt
	_, err := stmt.ExecContext(ctx, userID, deviceID)
	return err
}
