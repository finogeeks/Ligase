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
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

func init() {
	common.Register("presence", NewDatabase)
}

type Database struct {
	db         *sql.DB
	topic      string
	underlying string
	presence   presencesStatements
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	dataBase := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_presence")

	if dataBase.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	dataBase.db.SetMaxOpenConns(30)
	dataBase.db.SetMaxIdleConns(30)
	dataBase.db.SetConnMaxLifetime(time.Minute * 3)

	schemas := []string{dataBase.presence.getSchema()}
	for _, sqlStr := range schemas {
		_, err := dataBase.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = dataBase.presence.prepare(dataBase); err != nil {
		return nil, err
	}

	dataBase.AsyncSave = useAsync
	dataBase.topic = topic
	dataBase.underlying = underlying

	return dataBase, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

// WriteOutputEvents implements OutputRoomEventWriter
func (d *Database) WriteDBEvent(update *dbtypes.DBEvent) error {
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic,
		&core.TransportPubMsg{
			Keys: []byte(update.GetEventKey()),
			Obj:  update,
		})
}

func (d *Database) WriteDBEventWithTbl(update *dbtypes.DBEvent, tbl string) error {
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic+"_"+tbl,
		&core.TransportPubMsg{
			Keys: []byte(update.GetEventKey()),
			Obj:  update,
		})
}

func (d *Database) RecoverCache() {
	err := d.presence.recoverPresences()
	if err != nil {
		log.Errorf("presence.recoverPresences error %v", err)
	}

	log.Info("presence db load finished")
}

func (d *Database) UpsertPresences(
	ctx context.Context, userID, status, statusMsg, extStatusMsg string,
) error {
	return d.presence.upsertPresences(ctx, userID, status, statusMsg, extStatusMsg)
}

func (d *Database) OnUpsertPresences(
	ctx context.Context, userID, status, statusMsg, extStatusMsg string,
) error {
	return d.presence.onUpsertPresences(ctx, userID, status, statusMsg, extStatusMsg)
}
