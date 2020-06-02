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

	"github.com/finogeeks/ligase/skunkworks/log"
)

const sendRecordSchema = `
CREATE TABLE IF NOT EXISTS federation_send_record (
	room_id TEXT NOT NULL,
	domain TEXT NOT NULL,
	event_id TEXT NOT NULL,
	send_times int4 NOT NULL,
	pending_size int4 NOT NULL,
	domain_offset int4 NOT NULL DEFAULT 0,
	CONSTRAINT federation_send_record_unique UNIQUE (room_id, domain)
);

CREATE UNIQUE INDEX IF NOT EXISTS federation_send_record_room_id_idx
    ON federation_send_record (room_id, domain)
`

const insertSendRecordSQL = "" +
	"INSERT INTO federation_send_record (room_id, domain, event_id, send_times, pending_size, domain_offset)" +
	" VALUES ($1, $2, 0, 0, 0, 0)" +
	" ON CONFLICT ON CONSTRAINT federation_send_record_unique" +
	" DO NOTHING"

const selectAllSendRecordSQL = "" +
	"SELECT room_id, domain, event_id, send_times, pending_size, domain_offset FROM federation_send_record"

const selectPendingSendRecordSQL = "" +
	"SELECT room_id, domain, event_id, send_times, pending_size, domain_offset FROM federation_send_record WHERE pending_size > 0"

const selectSendRecordSQL = "" +
	"SELECT event_id, send_times, pending_size, domain_offset FROM federation_send_record WHERE room_id=$1 AND domain=$2"

const updateSendRecordPendingSizeSQL = "" +
	"INSERT INTO federation_send_record(room_id, domain, event_id, send_times, pending_size, domain_offset)" +
	" VALUES($1, $2, '', 0, $3, $4)" +
	" ON CONFLICT ON CONSTRAINT federation_send_record_unique" +
	" DO UPDATE SET pending_size = federation_send_record.pending_size + EXCLUDED.pending_size"
	//"UPDATE federation_send_record SET pending_size = pending_size + $3 WHERE room_id = $1 AND domain = $2"

const updateSendRecordPendingSizeAndEventIDSQL = "" +
	"UPDATE federation_send_record SET pending_size = pending_size + $3, event_id = $4, domain_offset = $5, send_times = send_times + 1 WHERE room_id = $1 AND domain = $2"

type sendRecordStatements struct {
	insertSendRecordStmt                      *sql.Stmt
	selectAllSendRecordStmt                   *sql.Stmt
	selectPendingSendRecordStmt               *sql.Stmt
	selectSendRecordStmt                      *sql.Stmt
	updateSendRecordPendingSizeStmt           *sql.Stmt
	updateSendRecordPendingSizeAndEventIDStmt *sql.Stmt
}

func (s *sendRecordStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(sendRecordSchema)
	if err != nil {
		return err
	}
	if s.insertSendRecordStmt, err = db.Prepare(insertSendRecordSQL); err != nil {
		return
	}
	if s.selectAllSendRecordStmt, err = db.Prepare(selectAllSendRecordSQL); err != nil {
		return
	}
	if s.selectPendingSendRecordStmt, err = db.Prepare(selectPendingSendRecordSQL); err != nil {
		return
	}
	if s.selectSendRecordStmt, err = db.Prepare(selectSendRecordSQL); err != nil {
		return
	}
	if s.updateSendRecordPendingSizeStmt, err = db.Prepare(updateSendRecordPendingSizeSQL); err != nil {
		return
	}
	if s.updateSendRecordPendingSizeAndEventIDStmt, err = db.Prepare(updateSendRecordPendingSizeAndEventIDSQL); err != nil {
		return
	}
	return
}

func (s *sendRecordStatements) insertSendRecord(
	ctx context.Context,
	roomID, domain string,
	domainOffset int64,
) error {
	_, err := s.insertSendRecordStmt.ExecContext(ctx, roomID, domain)
	return err
}

func (s *sendRecordStatements) selectAllSendRecord(
	ctx context.Context,
) ([]string, []string, []string, []int32, []int32, []int64, int, error) {
	rows, err := s.selectAllSendRecordStmt.QueryContext(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, 0, err
	}
	defer rows.Close()
	roomIDs := []string{}
	domains := []string{}
	eventIDs := []string{}
	sendTimeses := []int32{}
	pendingSizes := []int32{}
	domainOffsets := []int64{}
	var roomID string
	var domain string
	var eventID string
	var sendTimes int32
	var pendingSize int32
	var domainOffset int64
	total := 0
	for rows.Next() {
		e := rows.Scan(&roomID, &domain, &eventID, &sendTimes, &pendingSize, &domainOffset)
		if e != nil {
			log.Errorf("select send_record error %v", e)
			if err == nil {
				err = e
			}
			continue
		}
		roomIDs = append(roomIDs, roomID)
		domains = append(domains, domain)
		eventIDs = append(eventIDs, eventID)
		sendTimeses = append(sendTimeses, sendTimes)
		pendingSizes = append(pendingSizes, pendingSize)
		domainOffsets = append(domainOffsets, domainOffset)
		total++
	}
	return roomIDs, domains, eventIDs, sendTimeses, pendingSizes, domainOffsets, total, err
}

func (s *sendRecordStatements) selectPendingSendRecord(
	ctx context.Context,
) ([]string, []string, []string, []int32, []int32, []int64, int, error) {
	rows, err := s.selectPendingSendRecordStmt.QueryContext(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, 0, err
	}
	defer rows.Close()
	roomIDs := []string{}
	domains := []string{}
	eventIDs := []string{}
	sendTimeses := []int32{}
	pendingSizes := []int32{}
	domainOffsets := []int64{}
	var roomID string
	var domain string
	var eventID string
	var sendTimes int32
	var pendingSize int32
	var domainOffset int64
	total := 0
	for rows.Next() {
		e := rows.Scan(&roomID, &domain, &eventID, &sendTimes, &pendingSize, &domainOffset)
		if e != nil {
			log.Errorf("select send_record error %v", e)
			if err == nil {
				err = e
			}
			continue
		}
		roomIDs = append(roomIDs, roomID)
		domains = append(domains, domain)
		eventIDs = append(eventIDs, eventID)
		sendTimeses = append(sendTimeses, sendTimes)
		pendingSizes = append(pendingSizes, pendingSize)
		domainOffsets = append(domainOffsets, domainOffset)
		total++
	}
	return roomIDs, domains, eventIDs, sendTimeses, pendingSizes, domainOffsets, total, err
}

func (s *sendRecordStatements) selectSendRecord(ctx context.Context, roomID, domain string) (eventID string, sendTimes, pendingSize int32, domainOffset int64, err error) {
	err = s.selectSendRecordStmt.QueryRowContext(ctx, roomID, domain).Scan(&eventID, &sendTimes, &pendingSize, &domainOffset)
	return
}

func (s *sendRecordStatements) updateSendRecordPendingSize(
	ctx context.Context,
	roomID, domain string,
	pendingSize int32,
	domainOffset int64,
) error {
	_, err := s.updateSendRecordPendingSizeStmt.ExecContext(ctx, roomID, domain, pendingSize, domainOffset)
	return err
}

func (s *sendRecordStatements) updateSendRecordPendingSizeAndEventID(
	ctx context.Context,
	roomID, domain string,
	pendingSize int32,
	eventID string,
	domainOffset int64,
) error {
	_, err := s.updateSendRecordPendingSizeAndEventIDStmt.ExecContext(ctx, roomID, domain, pendingSize, eventID, domainOffset)
	return err
}
