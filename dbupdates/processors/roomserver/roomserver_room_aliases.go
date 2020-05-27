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
	dbregistry.Register("roomserver_room_aliases", NewDBRoomserverRoomAliasesProcessor, nil)
}

type DBRoomserverRoomAliasesProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverRoomAliasesProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverRoomAliasesProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverRoomAliasesProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverRoomAliasesProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.AliasInsertKey: true,
	}
}

func (p *DBRoomserverRoomAliasesProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.AliasInsertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.AliasDeleteKey:
		p.processDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverRoomAliasesProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		common.WithTransaction(p.db.GetDB(), func(txn *sql.Tx) error {
			stmt, err := txn.Prepare(pq.CopyIn("roomserver_room_aliases", "alias", "room_id"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt.Close()

			for _, v := range inputs {
				msg := v.Event.RoomDBEvents.AliaseInsert
				_, err = stmt.ExecContext(ctx, msg.Alias, msg.RoomID)
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
			msg := v.Event.RoomDBEvents.AliaseInsert
			err := p.db.AliaseInsertRaw(ctx, msg.Alias, msg.RoomID)
			if err != nil {
				log.Error(p.name, "insert err", err, msg.Alias, msg.RoomID)
			}
		}
	}
	return nil
}

func (p *DBRoomserverRoomAliasesProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.AliaseDelete
		err := p.db.AliaseDeleteRaw(ctx, msg.Alias)
		if err != nil {
			log.Errorf(p.name, "delete err", err, msg.Alias)
		}
	}
	return nil
}
