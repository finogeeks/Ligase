// Copyright 2018 Vector Creations Ltd
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

package encryptoapi

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
	common.Register("encryptoapi", NewDatabase)
}

// Database represents a presence database.
type Database struct {
	db                   *sql.DB
	topic                string
	underlying           string
	deviceKeyStatements  deviceKeyStatements
	oneTimeKeyStatements oneTimeKeyStatements
	alStatements         alStatements
	AsyncSave            bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase creates a new presence database
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	dataBase := new(Database)
	var err error
	common.CreateDatabase(driver, createAddr, "dendrite_encryptapi")

	if dataBase.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	dataBase.db.SetMaxOpenConns(30)
	dataBase.db.SetMaxIdleConns(30)
	dataBase.db.SetConnMaxLifetime(time.Minute * 3)
	if err = dataBase.deviceKeyStatements.prepare(dataBase); err != nil {
		return nil, err
	}
	if err = dataBase.oneTimeKeyStatements.prepare(dataBase); err != nil {
		return nil, err
	}
	if err = dataBase.alStatements.prepare(dataBase); err != nil {
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
	err := d.deviceKeyStatements.recoverDeviceKey()
	if err != nil {
		log.Errorf("deviceKeyStatements.recoverDeviceKey error %v", err)
	}

	err = d.oneTimeKeyStatements.recoverOneTimeKey()
	if err != nil {
		log.Errorf("oneTimeKeyStatements.recoverOneTimeKey error %v", err)
	}

	err = d.alStatements.recoverAls()
	if err != nil {
		log.Errorf("alStatements.recoverAls error %v", err)
	}

	log.Info("e2e db load finished")
}

func (d *Database) InsertDeviceKey(
	ctx context.Context,
	deviceID, userID, keyInfo, al, sig, identifier string,
) (err error) {
	return d.deviceKeyStatements.insertDeviceKey(ctx, deviceID, userID, keyInfo, al, sig, identifier)
}

func (d *Database) InsertOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, keyInfo, al, sig, identifier string,
) (err error) {
	return d.oneTimeKeyStatements.insertOneTimeKey(ctx, deviceID, userID, keyID, keyInfo, al, sig, identifier)
}

func (d *Database) OnInsertDeviceKey(
	ctx context.Context,
	deviceID, userID, keyInfo, al, sig, identifier string,
) (err error) {
	return d.deviceKeyStatements.onInsertDeviceKey(ctx, deviceID, userID, keyInfo, al, sig, identifier)
}

func (d *Database) OnInsertOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, keyInfo, al, sig, identifier string,
) (err error) {
	return d.oneTimeKeyStatements.onInsertOneTimeKey(ctx, deviceID, userID, keyID, keyInfo, al, sig, identifier)
}

func (d *Database) DeleteOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, algorithm string,
) (err error) {
	return d.oneTimeKeyStatements.deleteOneTimeKey(ctx, deviceID, userID, keyID, algorithm)
}

func (d *Database) OnDeleteOneTimeKey(
	ctx context.Context,
	deviceID, userID, keyID, algorithm string,
) (err error) {
	return d.oneTimeKeyStatements.onDeleteOneTimeKey(ctx, deviceID, userID, keyID, algorithm)
}

func (d *Database) DeleteMacOneTimeKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) (err error) {
	return d.oneTimeKeyStatements.deleteMacOneTimeKey(ctx, deviceID, userID, identifier)
}

func (d *Database) OnDeleteMacOneTimeKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) (err error) {
	return d.oneTimeKeyStatements.onDeleteMacOneTimeKey(ctx, deviceID, userID, identifier)
}

// InsertAl persist algorithms
func (d *Database) InsertAl(
	ctx context.Context, uid, device, als, identifier string,
) (err error) {
	return d.alStatements.insertAl(ctx, uid, device, als, identifier)
}

func (d *Database) OnInsertAl(
	ctx context.Context, uid, device, als, identifier string,
) (err error) {
	return d.alStatements.onInsertAl(ctx, uid, device, als, identifier)
}

func (d *Database) DeleteDeviceKeys(
	ctx context.Context, deviceID, userID string,
) error {
	err := d.alStatements.deleteAl(ctx, userID, deviceID)
	if err != nil {
		return err
	}

	err = d.deviceKeyStatements.deleteDeviceKey(ctx, deviceID, userID)
	if err != nil {
		return err
	}

	return d.oneTimeKeyStatements.deleteDeviceOneTimeKey(ctx, deviceID, userID)
}

func (d *Database) DeleteMacKeys(
	ctx context.Context, deviceID, userID, identifier string,
) error {
	err := d.alStatements.deleteMacAl(ctx, userID, deviceID, identifier)
	if err != nil {
		return err
	}

	err = d.deviceKeyStatements.deleteMacDeviceKey(ctx, deviceID, userID, identifier)
	if err != nil {
		return err
	}

	return d.oneTimeKeyStatements.deleteMacOneTimeKey(ctx, deviceID, userID, identifier)
}

func (d *Database) OnDeleteAl(
	ctx context.Context,
	userID, deviceID string,
) error {
	return d.alStatements.onDeleteAl(ctx, userID, deviceID)
}

func (d *Database) OnDeleteMacAl(
	ctx context.Context,
	userID, deviceID, identifier string,
) error {
	return d.alStatements.onDeleteMacAl(ctx, userID, deviceID, identifier)
}

func (d *Database) OnDeleteDeviceKey(
	ctx context.Context,
	deviceID, userID string,
) error {
	return d.deviceKeyStatements.onDeleteDeviceKey(ctx, deviceID, userID)
}

func (d *Database) OnDeleteMacDeviceKey(
	ctx context.Context,
	deviceID, userID, identifier string,
) error {
	return d.deviceKeyStatements.onDeleteMacDeviceKey(ctx, deviceID, userID, identifier)
}

func (d *Database) OnDeleteDeviceOneTimeKey(
	ctx context.Context,
	deviceID, userID string,
) error {
	return d.oneTimeKeyStatements.onDeleteDeviceOneTimeKey(ctx, deviceID, userID)
}

func (d *Database) DeleteDeviceOneTimeKey(
	ctx context.Context, deviceID, userID string,
) error {
	return d.oneTimeKeyStatements.deleteDeviceOneTimeKey(ctx, deviceID, userID)
}
