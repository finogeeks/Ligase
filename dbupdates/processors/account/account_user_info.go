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
	dbregistry.Register("account_user_info", NewDBAccountUserInfoProcessor, NewCacheAccountUserInfoProcessor)
}

type DBAccountUserInfoProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.AccountsDatabase
}

func NewDBAccountUserInfoProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBAccountUserInfoProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBAccountUserInfoProcessor) Start() {
	db, err := common.GetDBInstance("accounts", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to accounts db")
	}
	p.db = db.(model.AccountsDatabase)
}

func (p *DBAccountUserInfoProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.UserInfoInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.UserInfoInitKey:
		p.processInit(ctx, inputs)
	case dbtypes.UserInfoDeleteKey:
		p.processDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBAccountUserInfoProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.UserInfoInsert
		err := p.db.OnUpsertUserInfo(ctx, msg.UserID, msg.UserName, msg.JobNumber, msg.Mobile, msg.Landline, msg.Email, msg.State)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.UserName, msg.JobNumber, msg.Mobile, msg.Landline, msg.Email)
		}
	}
	return nil
}

func (p *DBAccountUserInfoProcessor) processInit(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.UserInfoInsert
		err := p.db.OnInitUserInfo(ctx, msg.UserID, msg.UserName, msg.JobNumber, msg.Mobile, msg.Landline, msg.Email, msg.State)
		if err != nil {
			log.Error(p.name, "init err", err, msg.UserID, msg.UserName, msg.JobNumber, msg.Mobile, msg.Landline, msg.Email)
		}
	}
	return nil
}

func (p *DBAccountUserInfoProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.AccountDBEvents.UserInfoDelete
		err := p.db.OnDeleteUserInfo(ctx, msg.UserID)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.UserID)
		}
	}
	return nil
}

type CacheAccountUserInfoProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheAccountUserInfoProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheAccountRoomTagsProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheAccountUserInfoProcessor) Start() {
}

func (p *CacheAccountUserInfoProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.AccountDBEvents
	switch key {
	case dbtypes.UserInfoInsertKey:
		return p.onUserInfoInsert(ctx, data.UserInfoInsert)
	case dbtypes.UserInfoInitKey:
		return p.onUserInfoInit(ctx, data.UserInfoInsert)
	case dbtypes.UserInfoDeleteKey:
		return p.onUserInfoDelete(ctx, data.UserInfoDelete)
	}
	return nil
}

func (p *CacheAccountUserInfoProcessor) onUserInfoInsert(ctx context.Context, msg *dbtypes.UserInfoInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "user_info", msg.UserID), "user_id", msg.UserID, "user_name", msg.UserName, "job_number", msg.JobNumber, "mobile", msg.Mobile, "landline", msg.Landline, "email", msg.Email)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheAccountUserInfoProcessor) onUserInfoInit(ctx context.Context, msg *dbtypes.UserInfoInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "user_info", msg.UserID), "user_id", msg.UserID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheAccountUserInfoProcessor) onUserInfoDelete(ctx context.Context, msg *dbtypes.UserInfoDelete) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	UserInfokey := fmt.Sprintf("%s:%s", "user_info", msg.UserID)

	err := conn.Send("del", UserInfokey)
	if err != nil {
		return err
	}

	return nil
}
