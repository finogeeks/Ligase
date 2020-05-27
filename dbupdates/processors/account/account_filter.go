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
	dbregistry.Register("account_filter", NewDBAccountFilterProcessor, NewCacheAccountFilterProcessor)
}

type DBAccountFilterProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.AccountsDatabase
}

func NewDBAccountFilterProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBAccountFilterProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBAccountFilterProcessor) Start() {
	db, err := common.GetDBInstance("accounts", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to accounts db")
	}
	p.db = db.(model.AccountsDatabase)
}

func (p *DBAccountFilterProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.FilterInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBAccountFilterProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.FilterInsert
		err := p.db.OnInsertFilter(ctx, msg.Filter, msg.FilterID, msg.UserID)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.Filter, msg.FilterID, msg.UserID)
		}
	}
	return nil
}

type CacheAccountFilterProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheAccountFilterProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheAccountFilterProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheAccountFilterProcessor) Start() {
}

func (p *CacheAccountFilterProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.AccountDBEvents
	switch key {
	case dbtypes.FilterInsertKey:
		return p.onFilterInsert(ctx, data.FilterInsert)
	}
	return nil
}

func (p *CacheAccountFilterProcessor) onFilterInsert(ctx context.Context, msg *dbtypes.FilterInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "filter", msg.UserID, msg.FilterID), "user_id", msg.UserID, "id", msg.FilterID, "filter", msg.Filter)
	if err != nil {
		return err
	}

	/*filterHash := fn.GetStringHash(msg.Filter)
	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "filter_content", msg.UserID, filterHash), "user_id", msg.UserID, "id", msg.FilterID, "filter", msg.Filter)
	if err != nil {
		return err
	}*/

	return conn.Flush()
}
