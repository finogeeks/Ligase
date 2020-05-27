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
	dbregistry.Register("push_rules_enable", NewDBPushRulesEnableProcessor, NewCachePushRulesEnableProcessor)
}

type DBPushRulesEnableProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.PushAPIDatabase
}

func NewDBPushRulesEnableProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBPushRulesEnableProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBPushRulesEnableProcessor) Start() {
	db, err := common.GetDBInstance("pushapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to pushapi db")
	}
	p.db = db.(model.PushAPIDatabase)
}

func (p *DBPushRulesEnableProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.PushRuleEnableUpsetKey:
		p.processUpsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBPushRulesEnableProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PushRuleEnableInsert
		err := p.db.OnAddPushRuleEnable(ctx, msg.UserID, msg.RuleID, msg.Enabled)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.RuleID, msg.Enabled)
		}
	}
	return nil
}

type CachePushRulesEnableProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCachePushRulesEnableProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CachePushRulesEnableProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CachePushRulesEnableProcessor) Start() {
}

func (p *CachePushRulesEnableProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.PushDBEvents
	switch key {
	case dbtypes.PushRuleEnableUpsetKey:
		return p.onPushRuleEnableUpset(ctx, data.PushRuleEnableInsert)
	}
	return nil
}

func (p *CachePushRulesEnableProcessor) onPushRuleEnableUpset(ctx context.Context, msg *dbtypes.PushRuleEnableInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule_enable", msg.UserID, msg.RuleID)

	err := conn.Send("hmset", ruleKey, "user_name", msg.UserID, "rule_id", msg.RuleID, "enabled", msg.Enabled)
	if err != nil {
		return err
	}

	return conn.Flush()
}
