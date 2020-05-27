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
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/lib/pq"
)

func init() {
	dbregistry.Register("syncapi_send_to_device", NewDBSyncapiSendToDeviceProcessor, nil)
}

type DBSyncapiSendToDeviceProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.SyncAPIDatabase
}

func NewDBSyncapiSendToDeviceProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBSyncapiSendToDeviceProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBSyncapiSendToDeviceProcessor) Start() {
	db, err := common.GetDBInstance("syncapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to syncapi db")
	}
	p.db = db.(model.SyncAPIDatabase)
}

func (p *DBSyncapiSendToDeviceProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.SyncStdEventInertKey: true,
	}
}

func (p *DBSyncapiSendToDeviceProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.SyncStdEventInertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.SyncStdEventDeleteKey:
		p.processDelete(ctx, inputs)
	case dbtypes.SyncMacStdEventDeleteKey:
		p.processMacDelete(ctx, inputs)
	case dbtypes.SyncDeviceStdEventDeleteKey:
		p.processDeviceDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBSyncapiSendToDeviceProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		common.WithTransaction(p.db.GetDB(), func(txn *sql.Tx) error {
			stmt, err := txn.Prepare(pq.CopyIn("syncapi_send_to_device", "id", "sender", "event_type", "target_user_id", "target_device_id", "event_json", "identifier"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt.Close()

			for _, v := range inputs {
				msg := v.Event.SyncDBEvents.SyncStdEventInsert
				_, err = stmt.ExecContext(ctx, msg.ID, msg.StdEvent.Sender, msg.StdEvent.EventTyp, msg.TargetUID, msg.TargetDevice, string(msg.StdEvent.Event), msg.Identifier)
				if err != nil {
					log.Errorf("bulk insert one error %v", err)
					return err
				}
			}
			_, err = stmt.ExecContext(ctx)
			if err != nil {
				log.Warnf("bulk insert error %v", err)
				return err
			}

			success = true
			log.Debugf("bulk insert %s success, len %d", p.name, len(inputs))
			return nil
		})
	}
	if !success {
		if len(inputs) > 1 {
			log.Warnf("not use bulk instert, user normal stmt instead len %d", len(inputs))
		}
		for _, v := range inputs {
			msg := v.Event.SyncDBEvents.SyncStdEventInsert
			_, err := p.db.OnInsertStdMessage(ctx, msg.ID, msg.StdEvent, msg.TargetUID, msg.TargetDevice, msg.Identifier)
			if err != nil {
				log.Error(p.name, "insert err", err, msg.ID, msg.StdEvent, msg.TargetUID, msg.TargetDevice, msg.Identifier)
			}
		}
	}
	return nil
}

func (p *DBSyncapiSendToDeviceProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncStdEventDelete
		err := p.db.OnDeleteStdMessage(ctx, msg.ID, msg.TargetUID, msg.TargetDevice)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.ID, msg.TargetUID, msg.TargetDevice)
		}
	}
	return nil
}

func (p *DBSyncapiSendToDeviceProcessor) processMacDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncMacStdEventDelete
		err := p.db.OnDeleteMacStdMessage(ctx, msg.Identifier, msg.TargetUID, msg.TargetDevice)
		if err != nil {
			log.Error(p.name, "mac delete err", err, msg.Identifier, msg.TargetUID, msg.TargetDevice)
		}
	}
	return nil
}

func (p *DBSyncapiSendToDeviceProcessor) processDeviceDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncStdEventDelete
		err := p.db.OnDeleteDeviceStdMessage(ctx, msg.TargetUID, msg.TargetDevice)
		if err != nil {
			log.Error(p.name, "device delete err", err, msg.TargetUID, msg.TargetDevice)
		}
	}
	return nil
}
