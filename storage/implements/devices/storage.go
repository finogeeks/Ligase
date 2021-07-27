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

package devices

import (
	"context"
	"database/sql"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	common.Register("devices", NewDatabase)
}

// Database represents a device database.
type Database struct {
	db         *sql.DB
	topic      string
	underlying string
	devices    devicesStatements
	migDevices migDevicesStatements
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase creates a new device database
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	dataBase := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_device")

	if dataBase.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	dataBase.db.SetMaxOpenConns(30)
	dataBase.db.SetMaxIdleConns(30)
	dataBase.db.SetConnMaxLifetime(time.Minute * 3)

	schemas := []string{dataBase.devices.getSchema(), dataBase.migDevices.getSchema()}
	for _, sqlStr := range schemas {
		_, err := dataBase.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = dataBase.devices.prepare(dataBase); err != nil {
		return nil, err
	}

	if err = dataBase.migDevices.prepare(dataBase); err != nil {
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
	err := d.devices.recoverDevice()
	if err != nil {
		log.Errorf("devices.recoverDevice error %v", err)
	}

	err = d.migDevices.recoverMigDevice()
	if err != nil {
		log.Errorf("devices.recoverMigDevice error %v", err)
	}

	log.Info("device db load finished")
}

// CreateDevice makes a new device associated with the given user ID.
// If there is already a device with the same device ID for this user, that access token will be revoked
// and replaced with the given accessToken. If the given accessToken is already in use for another device,
// an error will be returned.
// If no device ID is given one is generated.
// Returns the device on success.
func (d *Database) CreateDevice(
	ctx context.Context, userID, deviceID, deviceType string,
	displayName *string, isHuman bool, identifier *string, specifiedTime int64,
) (dev *authtypes.Device, returnErr error) {
	if displayName == nil {
		name := ""
		displayName = &name
	}

	if identifier == nil {
		idf := ""
		identifier = &idf
	}

	d.devices.upsertDevice(ctx, deviceID, userID, displayName, deviceType, *identifier, specifiedTime)

	dev = &authtypes.Device{}
	dev.UserID = userID
	dev.DisplayName = *displayName
	dev.ID = deviceID
	dev.DeviceType = deviceType
	dev.Identifier = *identifier

	return dev, nil
}

func (d *Database) InsertDevice(
	ctx context.Context, userID string, deviceID *string,
	displayName *string, deviceType, identifier string,
) error {
	return d.devices.upsertDevice(ctx, *deviceID, userID, displayName, deviceType, identifier, -1)
}

func (d *Database) OnInsertDevice(
	ctx context.Context, userID string, deviceID *string,
	displayName *string, createdTs, lastActiveTs int64, deviceType, identifier string,
) error {
	return d.devices.onUpsertDevice(ctx, *deviceID, userID, displayName, createdTs, lastActiveTs, deviceType, identifier)
}

func (d *Database) OnInsertMigDevice(
	ctx context.Context, access_token, mig_access_token, deviceID, userID string,
) error {
	return d.migDevices.onUpsertMigDevice(ctx, access_token, mig_access_token, deviceID, userID)
}

// RemoveDevice revokes a device by deleting the entry in the database
// matching with the given device ID and user ID
// If the device doesn't exist, it will not return an error
// If something went wrong during the deletion, it will return the SQL error
func (d *Database) RemoveDevice(
	ctx context.Context, deviceID, userID string, createTs int64,
) error {
	return d.devices.deleteDevice(ctx, deviceID, userID, createTs)
}

func (d *Database) OnDeleteDevice(
	ctx context.Context, deviceID, userID string, createTs int64,
) error {
	return d.devices.onDeleteDevice(ctx, deviceID, userID, createTs)
}

func (d *Database) GetDeviceTotal(
	ctx context.Context,
) (int, error) {
	return d.devices.selectDeviceTotal(ctx)
}

func (d *Database) CreateMigDevice(
	ctx context.Context, userID, deviceID, token, migToken string,
) error {
	return d.migDevices.insertMigDevice(ctx, token, migToken, deviceID, userID)
}

func (d *Database) RemoveMigDevice(
	ctx context.Context, deviceID, userID string,
) error {
	return d.migDevices.deleteMigDevice(ctx, deviceID, userID)
}

func (d *Database) RemoveAllUserMigDevices(
	ctx context.Context, userID string,
) error {
	return d.migDevices.deleteUserMigDevices(ctx, userID)
}

func (d *Database) UpdateDeviceActiveTs(
	ctx context.Context, deviceID, userID string, lastActiveTs int64,
) error {
	return d.devices.updateDeviceActiveTs(ctx, deviceID, userID, lastActiveTs)
}

func (d *Database) OnUpdateDeviceActiveTs(
	ctx context.Context, deviceID, userID string, lastActiveTs int64,
) error {
	return d.devices.onUpdateDeviceActiveTs(ctx, deviceID, userID, lastActiveTs)
}

func (d *Database) SelectUnActiveDevice(
	ctx context.Context, lastActiveTs int64, limit, offset int,
) ([]string, []string, []string, int, error) {
	return d.devices.selectUnActiveDevices(ctx, lastActiveTs, limit, offset)
}

func (d *Database) CheckDevice(
	ctx context.Context, identifier, userID string,
) (string, string, int64, error) {
	return d.devices.checkDevice(ctx, identifier, userID)
}

func (d *Database) LoadSimpleFilterData(f *filter.SimpleFilter) bool {
	offset := 0
	finish := false
	limit := 1000
	ctx := context.Background()
	for {
		if finish {
			return true
		}
		log.Infof("LoadSimpleFilterData limit:%d @offset:%d", limit, offset)
		devids, _, uids, total, err := d.devices.selectActiveDevices(ctx, limit, offset)
		if err != nil {
			log.Errorf("LoadSimpleFilterData from db with err %v", err)
			return false
		}
		for idx := range devids {
			f.Insert(uids[idx], devids[idx])
		}
		if total < limit {
			finish = true
		} else {
			offset = offset + limit
		}
	}
	return true
}

func (d *Database) LoadFilterData(key string, f *filter.Filter) bool {
	/*offset := 0
	finish := false
	limit := 500
	ctx := context.Background()

	if key == "device" {
		for {
			if finish {
				return true
			}

			log.Infof("LoadFilterData %s limit:%d @offset:%d", key, limit, offset)
			devids, dids, uids, total, err := d.devices.selectActiveDevices(ctx, limit, offset)
			if err != nil {
				log.Errorf("LoadFilterData %s with err %v", key, err)
				return false
			}
			for idx := range dids {
				key := uids[idx] + ":" + dids[idx]
				filterKey := uids[idx] + ":" + devids[idx]
				f.Insert([]byte(key))
				f.Insert([]byte(filterKey))
			}

			if total < limit {
				finish = true
			} else {
				offset = offset + limit
			}
		}
	}*/

	return true
}
