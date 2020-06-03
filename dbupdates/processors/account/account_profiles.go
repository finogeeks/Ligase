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
	dbregistry.Register("account_profiles", NewDBAccountProfileProcessor, NewCacheAccountProfileProcessor)
}

type DBAccountProfileProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.AccountsDatabase
}

func NewDBAccountProfileProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBAccountProfileProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBAccountProfileProcessor) Start() {
	db, err := common.GetDBInstance("accounts", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to accounts db")
	}
	p.db = db.(model.AccountsDatabase)
}

func (p *DBAccountProfileProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.ProfileInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.ProfileInitKey:
		p.processInit(ctx, inputs)
	case dbtypes.DisplayNameInsertKey:
		p.processUpsertDisplayName(ctx, inputs)
	case dbtypes.AvatarInsertKey:
		p.processUpsertAvatarURL(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBAccountProfileProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.ProfileInsert
		err := p.db.OnUpsertProfile(ctx, msg.UserID, msg.DisplayName, msg.AvatarUrl)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.DisplayName, msg.AvatarUrl)
		}
	}
	return nil
}

func (p *DBAccountProfileProcessor) processInit(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.ProfileInsert
		err := p.db.OnInitProfile(ctx, msg.UserID, msg.DisplayName, msg.AvatarUrl)
		if err != nil {
			log.Error(p.name, "init err", err, msg.UserID, msg.DisplayName, msg.AvatarUrl)
		}
	}
	return nil
}

func (p *DBAccountProfileProcessor) processUpsertDisplayName(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.ProfileInsert
		err := p.db.OnUpsertDisplayName(ctx, msg.UserID, msg.DisplayName)
		if err != nil {
			log.Error(p.name, "upsert displayname err", err, msg.UserID, msg.DisplayName)
		}
	}
	return nil
}

func (p *DBAccountProfileProcessor) processUpsertAvatarURL(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.ProfileInsert
		err := p.db.OnUpsertAvatar(ctx, msg.UserID, msg.AvatarUrl)
		if err != nil {
			log.Error(p.name, "upsert avatarURL err", err, msg.UserID, msg.AvatarUrl)
		}
	}
	return nil
}

type CacheAccountProfileProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheAccountProfileProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheAccountProfileProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheAccountProfileProcessor) Start() {
}

func (p *CacheAccountProfileProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.AccountDBEvents
	switch key {
	case dbtypes.ProfileInsertKey:
		return p.onProfileInsert(ctx, data.ProfileInsert)
	case dbtypes.ProfileInitKey:
		return p.onProfileInit(ctx, data.ProfileInsert)
	}
	return nil
}

func (p *CacheAccountProfileProcessor) onProfileInsert(ctx context.Context, msg *dbtypes.ProfileInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", msg.UserID), "user_id", msg.UserID, "display_name", msg.DisplayName, "avatar_url", msg.AvatarUrl)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheAccountProfileProcessor) onProfileInit(ctx context.Context, msg *dbtypes.ProfileInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", msg.UserID), "user_id", msg.UserID)
	if err != nil {
		return err
	}

	return conn.Flush()
}
