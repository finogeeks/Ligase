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
	dbregistry.Register("room_tags", NewDBAccountRoomTagProcessor, NewCacheAccountRoomTagsProcessor)
}

type DBAccountRoomTagProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.AccountsDatabase
}

func NewDBAccountRoomTagProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBAccountRoomTagProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBAccountRoomTagProcessor) Start() {
	db, err := common.GetDBInstance("accounts", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to accounts db")
	}
	p.db = db.(model.AccountsDatabase)
}

func (p *DBAccountRoomTagProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.RoomTagInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.RoomTagDeleteKey:
		p.processDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBAccountRoomTagProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.RoomTagInsert
		err := p.db.OnInsertRoomTag(ctx, msg.UserID, msg.RoomID, msg.Tag, msg.Content)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.RoomID, msg.Tag, msg.Content)
		}
	}
	return nil
}

func (p *DBAccountRoomTagProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.RoomTagDelete
		err := p.db.OnDeleteRoomTag(ctx, msg.UserID, msg.RoomID, msg.Tag)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.UserID)
		}
	}
	return nil
}

type CacheAccountRoomTagsProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheAccountRoomTagsProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheAccountRoomTagsProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheAccountRoomTagsProcessor) Start() {
}

func (p *CacheAccountRoomTagsProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.AccountDBEvents
	switch key {
	case dbtypes.RoomTagInsertKey:
		return p.onRoomTagInsert(ctx, data.RoomTagInsert)
	case dbtypes.RoomTagDeleteKey:
		return p.onRoomTagDelete(ctx, data.RoomTagDelete)
	}
	return nil
}

func (p *CacheAccountRoomTagsProcessor) onRoomTagInsert(ctx context.Context, msg *dbtypes.RoomTagInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	tagKey := fmt.Sprintf("%s:%s:%s:%s", "room_tags", msg.UserID, msg.RoomID, msg.Tag)

	err := conn.Send("hmset", tagKey, "user_id", msg.UserID, "room_id", msg.RoomID, "tag", msg.Tag, "content", msg.Content)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "user_tags_list", msg.UserID), tagKey, tagKey)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "room_tags_list", msg.UserID, msg.RoomID), tagKey, tagKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheAccountRoomTagsProcessor) onRoomTagDelete(ctx context.Context, msg *dbtypes.RoomTagDelete) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	tagKey := fmt.Sprintf("%s:%s:%s:%s", "room_tags", msg.UserID, msg.RoomID, msg.Tag)

	err := conn.Send("del", tagKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "user_tags_list", msg.UserID), tagKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "room_tags_list", msg.UserID, msg.RoomID), tagKey)
	if err != nil {
		return err
	}

	return nil
}
