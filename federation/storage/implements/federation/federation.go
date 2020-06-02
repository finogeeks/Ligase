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

package federation

import (
	"context"
	"database/sql"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/storage/model"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

func init() {
	common.Register("federation", NewDatabase)
}

// Database stores information needed by the federation sender
type Database struct {
	joinedRoomsStatements
	sendRecordStatements
	backfillRecordStatements
	missingEventsStatements
	db         *sql.DB
	topic      string
	underlying string
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase opens a new database
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	var result Database
	err := common.CreateDatabase(driver, createAddr, "dendrite_federation")
	if err != nil {
		return nil, err
	}

	if result.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	if err = result.prepare(); err != nil {
		return nil, err
	}
	result.db.SetMaxOpenConns(30)
	result.db.SetMaxIdleConns(30)
	result.db.SetConnMaxLifetime(time.Minute * 3)

	result.AsyncSave = useAsync
	result.topic = topic
	result.underlying = underlying
	return &result, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

func (d *Database) prepare() error {
	var err error

	if err = d.joinedRoomsStatements.prepare(d.db); err != nil {
		return err
	}
	if err = d.sendRecordStatements.prepare(d.db); err != nil {
		return err
	}
	if err = d.backfillRecordStatements.prepare(d.db); err != nil {
		return err
	}
	if err = d.missingEventsStatements.prepare(d.db); err != nil {
		return err
	}

	return nil
}

func (d *Database) SelectJoinedRooms(ctx context.Context, roomID string) (eventID, recvOffsets string, err error) {
	return d.selectJoinedRooms(ctx, roomID)
}

func (d *Database) UpdateJoinedRoomsRecvOffset(ctx context.Context, roomID string, recvOffsets string) error {
	return d.updateJoinedRoomsRecvOffset(ctx, roomID, recvOffsets)
}

func (d *Database) InsertJoinedRooms(ctx context.Context, roomID, eventID string) error {
	return d.insertJoinedRooms(ctx, roomID, eventID)
}

func (d *Database) SelectAllSendRecord(ctx context.Context) ([]string, []string, []string, []int32, []int32, []int64, int, error) {
	return d.selectAllSendRecord(ctx)
}

func (d *Database) SelectPendingSendRecord(ctx context.Context) ([]string, []string, []string, []int32, []int32, []int64, int, error) {
	return d.selectPendingSendRecord(ctx)
}

func (d *Database) SelectSendRecord(ctx context.Context, roomID, domain string) (eventID string, sendTimes, pendingSize int32, domainOffset int64, err error) {
	return d.selectSendRecord(ctx, roomID, domain)
}

func (d *Database) InsertSendRecord(ctx context.Context, roomID, domain string, domainOffset int64) error {
	return d.insertSendRecord(ctx, roomID, domain, domainOffset)
}

func (d *Database) UpdateSendRecordPendingSize(ctx context.Context, roomID, domain string, size int32, domainOffset int64) error {
	return d.updateSendRecordPendingSize(ctx, roomID, domain, size, domainOffset)
}

func (d *Database) UpdateSendRecordPendingSizeAndEventID(ctx context.Context, roomID, domain string, size int32, eventID string, domainOffset int64) error {
	return d.updateSendRecordPendingSizeAndEventID(ctx, roomID, domain, size, eventID, domainOffset)
}

func (d *Database) SelectAllBackfillRecord(ctx context.Context) ([]model.BackfillRecord, error) {
	return d.selectAllBackfillRecord(ctx)
}

func (d *Database) SelectBackfillRecord(ctx context.Context, roomID string) (model.BackfillRecord, error) {
	return d.selectBackfillRecord(ctx, roomID)
}

func (d *Database) InsertBackfillRecord(ctx context.Context, rec model.BackfillRecord) error {
	return d.insertBackfillRecord(ctx, rec)
}

func (d *Database) UpdateBackfillRecordDomainsInfo(ctx context.Context, roomID string, depth int64, finished bool, finishedDomains string, states string) error {
	return d.updateBackfillRecordDomainsInfo(ctx, roomID, depth, finished, finishedDomains, states)
}

func (d *Database) InsertMissingEvents(
	ctx context.Context,
	roomID, eventID string, amount int,
) error {
	return d.insertMissingEvents(ctx, roomID, eventID, amount)
}

func (d *Database) UpdateMissingEvents(ctx context.Context, roomID, eventID string, finished bool) error {
	return d.updateMissingEvents(ctx, roomID, eventID, finished)
}

func (d *Database) SelectMissingEvents(ctx context.Context) (roomIDs, eventIDs []string, amounts []int, err error) {
	return d.selectMissingEvents(ctx)
}
