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

package configdb

import (
	"context"
	"database/sql"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common"
)

func init() {
	common.Register("server_conf", NewDatabase)
}

type Database struct {
	db         *sql.DB
	topic      string
	underlying string
	statements configDbStatements
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase creates a new device database
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	dataBase := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_serverconf")

	if dataBase.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}

	schemas := []string{dataBase.statements.getSchema()}
	for _, sqlStr := range schemas {
		_, err := dataBase.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = dataBase.statements.prepare(dataBase); err != nil {
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

func (d *Database) UpsertServerName(
	ctx context.Context,
	nid int64,
	serverName string,
) error {
	return d.statements.upsertServerName(ctx, nid, serverName)
}

func (d *Database) SelectServerNames(
	ctx context.Context,
) (results []string, err error) {
	return d.statements.selectServerNames(ctx)
}
