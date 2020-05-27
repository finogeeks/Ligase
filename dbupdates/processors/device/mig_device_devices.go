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

package processors

import (
	"context"
	"fmt"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	dbregistry.Register("mig_device_devices", NewDBDeviceMigDevicesProcessor, NewCacheDeviceMigDevicesProcessor)
}

type DBDeviceMigDevicesProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.DeviceDatabase
}

func NewDBDeviceMigDevicesProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBDeviceMigDevicesProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBDeviceMigDevicesProcessor) Start() {
	db, err := common.GetDBInstance("devices", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to devices db")
	}
	p.db = db.(model.DeviceDatabase)
}

func (p *DBDeviceMigDevicesProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.MigDeviceInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBDeviceMigDevicesProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.DeviceDBEvents.MigDeviceInsert
		err := p.db.OnInsertMigDevice(ctx, msg.AccessToken, msg.MigAccessToken, msg.DeviceID, msg.UserID)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.AccessToken, msg.MigAccessToken, msg.DeviceID, msg.UserID)
		}
	}
	return nil
}

type CacheDeviceMigDevicesProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheDeviceMigDevicesProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheDeviceMigDevicesProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheDeviceMigDevicesProcessor) Start() {
}

func (p *CacheDeviceMigDevicesProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.DeviceDBEvents
	switch key {
	case dbtypes.MigDeviceInsertKey:
		return p.onMigDeviceInsert(ctx, data.MigDeviceInsert)
	}
	return nil
}

func (p *CacheDeviceMigDevicesProcessor) onMigDeviceInsert(ctx context.Context, msg *dbtypes.MigDeviceInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("set", fmt.Sprintf("%s:%s", "tokens", msg.AccessToken), msg.MigAccessToken)
	if err != nil {
		return err
	}

	return conn.Flush()
}
