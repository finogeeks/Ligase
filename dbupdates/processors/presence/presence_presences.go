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
	dbregistry.Register("presence_presences", NewDBPresencePresencesProcessor, NewCachePresencePresencesProcessor)
}

type DBPresencePresencesProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.PresenceDatabase
}

func NewDBPresencePresencesProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBPresencePresencesProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBPresencePresencesProcessor) Start() {
	db, err := common.GetDBInstance("presence", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to presence db")
	}
	p.db = db.(model.PresenceDatabase)
}

func (p *DBPresencePresencesProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.PresencesInsertKey:
		p.processUpsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBPresencePresencesProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PresenceDBEvents.PresencesInsert
		err := p.db.OnUpsertPresences(ctx, msg.UserID, msg.Status, msg.StatusMsg, msg.ExtStatusMsg)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.Status, msg.StatusMsg, msg.ExtStatusMsg)
		}
	}
	return nil
}

type CachePresencePresencesProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCachePresencePresencesProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CachePresencePresencesProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CachePresencePresencesProcessor) Start() {
}

func (p *CachePresencePresencesProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.PresenceDBEvents
	switch key {
	case dbtypes.PresencesInsertKey:
		return p.onPresencesInsert(ctx, data.PresencesInsert)
	}
	return nil
}

func (p *CachePresencePresencesProcessor) onPresencesInsert(ctx context.Context, msg *dbtypes.PresencesInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	presencesKey := fmt.Sprintf("%s:%s", "presences", msg.UserID)
	err := conn.Send("hmset", presencesKey, "user_id", msg.UserID, "status", msg.Status, "status_msg", msg.StatusMsg, "ext_status_msg", msg.ExtStatusMsg)
	if err != nil {
		return err
	}

	return conn.Flush()
}
