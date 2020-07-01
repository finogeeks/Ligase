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

package content

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

func init() {
	common.Register("content", NewDatabase)
}

type Database struct {
	mediaDownloadStatements
	db         *sql.DB
	topic      string
	underlying string
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase opens a new database
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	var result Database
	err := common.CreateDatabase(driver, createAddr, "dendrite_content")
	if err != nil {
		return nil, err
	}

	if result.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	if err = result.prepare(); err != nil {
		return nil, err
	}

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

	if err = d.mediaDownloadStatements.prepare(d.db); err != nil {
		return err
	}

	return nil
}

func (d *Database) InsertMediaDownload(
	ctx context.Context,
	roomID, eventID, event string,
) error {
	return d.insertMediaDownload(ctx, roomID, eventID, event)
}

func (d *Database) UpdateMediaDownload(ctx context.Context, roomID, eventID string, finished bool) error {
	return d.updateMediaDownload(ctx, roomID, eventID, finished)
}

func (d *Database) SelectMediaDownload(ctx context.Context) (roomIDs, eventIDs, events []string, err error) {
	return d.selectMediaDownload(ctx)
}
