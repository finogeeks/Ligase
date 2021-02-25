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

package appservice

import (
	"context"
	"database/sql"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

func init() {
	common.Register("appservice", NewDatabase)
}

type Database struct {
	events     eventsStatements
	txnID      txnStatements
	db         *sql.DB
	topic      string
	underlying string

	qryDBGauge mon.LabeledGauge
}

// NewDatabase opens a new database
//base.Cfg.Database.ApplicationService
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	result := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_applicationservice")

	if result.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	result.db.SetMaxOpenConns(30)
	result.db.SetMaxIdleConns(30)
	result.db.SetConnMaxLifetime(time.Minute * 3)

	result.topic = topic
	result.underlying = underlying

	if err = result.prepare(); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

func (d *Database) prepare() error {
	if err := d.events.prepare(d.db); err != nil {
		return err
	}

	return d.txnID.prepare(d.db)
}

// StoreEvent takes in a gomatrixserverlib.ClientEvent and stores it in the database
// for a transaction worker to pull and later send to an application service.
func (d *Database) StoreEvent(
	ctx context.Context,
	appServiceID string,
	event *gomatrixserverlib.ClientEvent,
) error {
	return d.events.insertEvent(ctx, appServiceID, event)
}

// GetEventsWithAppServiceID returns a slice of events and their IDs intended to
// be sent to an application service given its ID.
func (d *Database) GetEventsWithAppServiceID(
	ctx context.Context,
	appServiceID string,
	limit int,
) (int, int, []gomatrixserverlib.ClientEvent, bool, error) {
	return d.events.selectEventsByApplicationServiceID(ctx, appServiceID, limit)
}

// CountEventsWithAppServiceID returns the number of events destined for an
// application service given its ID.
func (d *Database) CountEventsWithAppServiceID(
	ctx context.Context,
	appServiceID string,
) (int, error) {
	return d.events.countEventsByApplicationServiceID(ctx, appServiceID)
}

// UpdateTxnIDForEvents takes in an application service ID and a
// and stores them in the DB, unless the pair already exists, in
// which case it updates them.
func (d *Database) UpdateTxnIDForEvents(
	ctx context.Context,
	appserviceID string,
	maxID, txnID int,
) error {
	return d.events.updateTxnIDForEvents(ctx, appserviceID, maxID, txnID)
}

// RemoveEventsBeforeAndIncludingID removes all events from the database that
// are less than or equal to a given maximum ID. IDs here are implemented as a
// serial, thus this should always delete events in chronological order.
func (d *Database) RemoveEventsBeforeAndIncludingID(
	ctx context.Context,
	appserviceID string,
	eventTableID int,
) error {
	return d.events.deleteEventsBeforeAndIncludingID(ctx, appserviceID, eventTableID)
}

// GetLatestTxnID returns the latest available transaction id
func (d *Database) GetLatestTxnID(
	ctx context.Context,
) (int, error) {
	return d.txnID.selectTxnID(ctx)
}

func (d *Database) GetMsgEventsWithLimit(
	ctx context.Context, limit, offset int64,
) ([]int64, [][]byte, error) {
	return d.events.selectMsgEventsWithLimit(ctx, limit, offset)
}

func (d *Database) GetMsgEventsTotal(
	ctx context.Context,
) (count int, minID int64, err error) {
	return d.events.getMsgEventsTotal(ctx)
}

func (d *Database) UpdateMsgEvent(
	ctx context.Context, id int64, NewBytes []byte,
) error {
	return d.events.updateMsgEvent(ctx, id, NewBytes)
}

func (d *Database) GetEncMsgEventsWithLimit(
	ctx context.Context, limit, offset int64,
) ([]int64, [][]byte, error) {
	return d.events.selectEncMsgEventsWithLimit(ctx, limit, offset)
}

func (d *Database) GetEncMsgEventsTotal(
	ctx context.Context,
) (count int, err error) {
	return d.events.getEncMsgEventsTotal(ctx)
}
