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
	dbregistry.Register("push_rules", NewDBPushRulesProcessor, NewCachePushRulesProcessor)
}

type DBPushRulesProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.PushAPIDatabase
}

func NewDBPushRulesProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBPushRulesProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBPushRulesProcessor) Start() {
	db, err := common.GetDBInstance("pushapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to pushapi db")
	}
	p.db = db.(model.PushAPIDatabase)
}

func (p *DBPushRulesProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.PushRuleUpsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.PushRuleDeleteKey:
		p.processDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBPushRulesProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PushRuleInert
		err := p.db.OnAddPushRule(ctx, msg.UserID, msg.RuleID, msg.PriorityClass, msg.Priority, msg.Conditions, msg.Actions)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.RuleID, msg.PriorityClass, msg.Priority, msg.Conditions, msg.Actions)
		}
	}
	return nil
}

func (p *DBPushRulesProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PushRuleDelete
		err := p.db.OnDeletePushRule(ctx, msg.UserID, msg.RuleID)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.RuleID)
		}
	}
	return nil
}

type CachePushRulesProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCachePushRulesProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CachePushRulesProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CachePushRulesProcessor) Start() {
}

func (p *CachePushRulesProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.PushDBEvents
	switch key {
	case dbtypes.PushRuleUpsertKey:
		return p.onPushRuleUpsert(ctx, data.PushRuleInert)
	case dbtypes.PushRuleDeleteKey:
		return p.onPushRuleDelete(ctx, data.PushRuleDelete)
	}
	return nil
}

func (p *CachePushRulesProcessor) onPushRuleUpsert(ctx context.Context, msg *dbtypes.PushRuleInert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule", msg.UserID, msg.RuleID)

	err := conn.Send("hmset", ruleKey, "user_name", msg.UserID, "rule_id", msg.RuleID, "priority_class", msg.PriorityClass,
		"priority", msg.Priority, "conditions", msg.Conditions, "actions", msg.Actions)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "push_rule_user_list", msg.UserID), ruleKey, ruleKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CachePushRulesProcessor) onPushRuleDelete(ctx context.Context, msg *dbtypes.PushRuleDelete) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule", msg.UserID, msg.RuleID)

	err := conn.Send("del", ruleKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "push_rule_user_list", msg.UserID), ruleKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}
