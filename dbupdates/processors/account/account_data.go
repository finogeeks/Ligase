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
	dbregistry.Register("account_data", NewDBAccountDataProcessor, NewCacheAccountDataProcessor)
}

type DBAccountDataProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.AccountsDatabase
}

func NewDBAccountDataProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBAccountDataProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBAccountDataProcessor) Start() {
	db, err := common.GetDBInstance("accounts", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to accounts db")
	}
	p.db = db.(model.AccountsDatabase)
}

func (p *DBAccountDataProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.AccountDataInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBAccountDataProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.AccountDataInsert
		err := p.db.OnInsertAccountData(ctx, msg.UserID, msg.RoomID, msg.Type, msg.Content)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.UserID, msg.RoomID, msg.Type, msg.Content)
		}
	}
	return nil
}

type CacheAccountDataProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheAccountDataProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheAccountDataProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheAccountDataProcessor) Start() {
}

func (p *CacheAccountDataProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.AccountDBEvents
	switch key {
	case dbtypes.AccountDataInsertKey:
		return p.onAccountDataInsert(ctx, data.AccountDataInsert)
	}
	return nil
}

func (p *CacheAccountDataProcessor) onAccountDataInsert(ctx context.Context, msg *dbtypes.AccountDataInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	if msg.RoomID != "" {
		roomAccountDatKey := fmt.Sprintf("%s:%s:%s:%s", "room_account_data", msg.UserID, msg.RoomID, msg.Type)
		err := conn.Send("hmset", roomAccountDatKey, "user_id", msg.UserID, "room_id", msg.RoomID, "type", msg.Type, "content", msg.Content)
		if err != nil {
			return err
		}
		err = conn.Send("hmset", fmt.Sprintf("%s:%s", "room_account_data_list", msg.UserID), roomAccountDatKey, roomAccountDatKey)
		if err != nil {
			return err
		}
	} else {
		accountDatKey := fmt.Sprintf("%s:%s:%s", "account_data", msg.UserID, msg.Type)
		err := conn.Send("hmset", accountDatKey, "user_id", msg.UserID, "type", msg.Type, "content", msg.Content)
		if err != nil {
			return err
		}
		err = conn.Send("hmset", fmt.Sprintf("%s:%s", "account_data_list", msg.UserID), accountDatKey, accountDatKey)
		if err != nil {
			return err
		}
	}

	return conn.Flush()
}
