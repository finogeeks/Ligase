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
	"github.com/finogeeks/ligase/common/encryption"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/dbupdates/processors/sqlutil"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/lib/pq"
)

func init() {
	dbregistry.Register("syncapi_output_room_events", NewDBSyncapiEventProcessor, nil)
}

type DBSyncapiEventProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.SyncAPIDatabase
}

func NewDBSyncapiEventProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBSyncapiEventProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBSyncapiEventProcessor) Start() {
	db, err := common.GetDBInstance("syncapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to syncapi db")
	}
	p.db = db.(model.SyncAPIDatabase)
}

func (p *DBSyncapiEventProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.SyncEventInsertKey: true,
	}
}

func (p *DBSyncapiEventProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.SyncEventInsertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.SyncEventUpdateKey:
		p.processUpdate(ctx, inputs)
	case dbtypes.SyncEventUpdateContentKey:
		p.processUpdateContent(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBSyncapiEventProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		sqlutil.WithTransaction(p.db.GetDB(), func(txn0, txn1 *sql.Tx) error {
			stmt0, err := txn0.Prepare(pq.CopyIn("syncapi_output_room_events", "id", "room_id",
				"event_id", "event_json", "add_state_ids", "remove_state_ids", "device_id",
				"transaction_id", "type", "domain_offset", "depth", "domain", "origin_server_ts"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt0.Close()

			stmt1, err := txn1.Prepare(pq.CopyIn("syncapi_output_room_events_mirror", "id",
				"room_id", "event_id", "event_json", "add_state_ids", "remove_state_ids", "device_id",
				"transaction_id", "type", "domain_offset", "depth", "domain", "origin_server_ts"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt1.Close()

			for _, v := range inputs {
				msg := v.Event.SyncDBEvents.SyncEventInsert
				if encryption.CheckCrypto(msg.Type) {
					_, err = stmt0.ExecContext(ctx, msg.Pos, msg.RoomId, msg.EventId, string(encryption.Encrypt(msg.EventJson)),
						pq.StringArray(msg.Add), pq.StringArray(msg.Remove), msg.Device, msg.TxnId,
						msg.Type, msg.DomainOffset, msg.Depth, msg.Domain, msg.OriginTs)
					if err != nil {
						log.Errorf("bulk insert one error %v", err)
						return err
					}
					if encryption.CheckMirror(msg.Type) {
						_, err = stmt1.ExecContext(ctx, msg.Pos, msg.RoomId, msg.EventId, string(msg.EventJson),
							pq.StringArray(msg.Add), pq.StringArray(msg.Remove), msg.Device, msg.TxnId,
							msg.Type, msg.DomainOffset, msg.Depth, msg.Domain, msg.OriginTs)
						if err != nil {
							log.Errorf("bulk insert one error %v", err)
							return err
						}
					}
				} else {
					_, err = stmt0.ExecContext(ctx, msg.Pos, msg.RoomId, msg.EventId, string(msg.EventJson),
						pq.StringArray(msg.Add), pq.StringArray(msg.Remove), msg.Device, msg.TxnId,
						msg.Type, msg.DomainOffset, msg.Depth, msg.Domain, msg.OriginTs)
					if err != nil {
						log.Errorf("bulk insert one error %v", err)
						return err
					}
				}
			}
			_, err = stmt0.ExecContext(ctx)
			if err != nil {
				log.Warnf("bulk insert error %v", err)
				return err
			}
			_, err = stmt1.ExecContext(ctx)
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
			msg := v.Event.SyncDBEvents.SyncEventInsert
			err := p.db.InsertEventRaw(ctx, msg.Pos, msg.RoomId, msg.EventId,
				msg.EventJson, msg.Add, msg.Remove, msg.Device, msg.TxnId,
				msg.Type, msg.DomainOffset, msg.Depth, msg.Domain, msg.OriginTs)
			if err != nil {
				log.Error(p.name, "insert err", err, msg.Pos, msg.RoomId, msg.EventId,
					msg.EventJson, msg.Add, msg.Remove, msg.Device, msg.TxnId,
					msg.Type, msg.DomainOffset, msg.Depth, msg.Domain, msg.OriginTs)
			}
		}
	}
	return nil
}

func (p *DBSyncapiEventProcessor) processUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncEventUpdate
		err := p.db.OnUpdateSyncEvent(ctx, msg.DomainOffset, msg.OriginTs, msg.Domain, msg.RoomId, msg.EventId)
		if err != nil {
			log.Errorf("update syncapi roomEvent err", err, msg.DomainOffset, msg.OriginTs, msg.Domain, msg.RoomId, msg.EventId)
		}
	}
	return nil
}

func (p *DBSyncapiEventProcessor) processUpdateContent(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncEventUpdateContent
		err := p.db.OnUpdateEvent(ctx, msg.EventID, msg.RoomID, []byte(msg.Content), msg.EventType)
		if err != nil {
			log.Errorf("update syncapi roomEventContent err", err, msg.EventID, msg.RoomID, msg.Content, msg.EventType)
		}
	}
	return nil
}
