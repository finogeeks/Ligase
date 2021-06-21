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

package pushapi

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/model/pushapitypes"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const pushrulesSchema = `
-- storage data for push_rules
CREATE TABLE IF NOT EXISTS push_rules (
	user_name TEXT NOT NULL,
	rule_id TEXT NOT NULL,
	priority_class integer NOT NULL,
	priority integer NOT NULL DEFAULT 0,
	conditions TEXT NOT NULL,
	actions TEXT NOT NULL,

	CONSTRAINT push_rules_unique UNIQUE (user_name, rule_id)
);

CREATE INDEX IF NOT EXISTS push_rules_user_name ON push_rules(user_name);
`

const insertPushRuleSQL = "" +
	"INSERT INTO push_rules(user_name, rule_id, priority_class, priority, conditions, actions) VALUES ($1, $2, $3, $4, $5, $6)" +
	" ON CONFLICT ON CONSTRAINT push_rules_unique" +
	" DO UPDATE SET priority_class = EXCLUDED.priority_class, priority = EXCLUDED.priority, conditions = EXCLUDED.conditions, actions = EXCLUDED.actions"

const deletePushRuleSQL = "" +
	"DELETE FROM push_rules WHERE user_name = $1 and rule_id = $2"

const selectPushRulesCountSQL = "" +
	"SELECT count(1) FROM push_rules"

const recoverPushRuleSQL = "" +
	"SELECT user_name, rule_id, priority_class, priority, conditions, actions FROM push_rules limit $1 offset $2"

type pushRulesStatements struct {
	db                       *DataBase
	insertPushRuleStmt       *sql.Stmt
	deletePushRuleStmt       *sql.Stmt
	selectPushRulesCountStmt *sql.Stmt
	recoverPushRuleStmt      *sql.Stmt
}

func (s *pushRulesStatements) getSchema() string {
	return pushrulesSchema
}

func (s *pushRulesStatements) prepare(d *DataBase) (err error) {
	s.db = d
	if s.insertPushRuleStmt, err = d.db.Prepare(insertPushRuleSQL); err != nil {
		return
	}
	if s.deletePushRuleStmt, err = d.db.Prepare(deletePushRuleSQL); err != nil {
		return
	}
	if s.selectPushRulesCountStmt, err = d.db.Prepare(selectPushRulesCountSQL); err != nil {
		return
	}
	if s.recoverPushRuleStmt, err = d.db.Prepare(recoverPushRuleSQL); err != nil {
		return
	}
	return
}

func (s *pushRulesStatements) loadPushRule(ctx context.Context) ([]pushapitypes.PushRuleData, error) {
	offset := 0
	limit := 1000
	result := []pushapitypes.PushRuleData{}
	for {
		rules := []pushapitypes.PushRuleData{}
		rows, err := s.recoverPushRuleStmt.QueryContext(ctx, limit, offset)
		if err != nil {
			log.Errorf("load push rule exec recoverPushRuleStmt err:%v", err)
			return nil, err
		}
		for rows.Next() {
			var pushRule pushapitypes.PushRuleData
			if err := rows.Scan(&pushRule.UserName, &pushRule.RuleId, &pushRule.PriorityClass, &pushRule.Priority, &pushRule.Conditions, &pushRule.Actions); err != nil {
				log.Errorf("load push rule scan rows error:%v", err)
				return nil, err
			} else {
				rules = append(rules, pushRule)
			}
		}
		result = append(result, rules...)
		if len(rules) < limit {
			break
		} else {
			offset = offset + limit
		}
	}
	return result, nil
}

func (s *pushRulesStatements) recoverPushRule() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverPushRuleStmt.QueryContext(context.TODO(), limit, offset)
		if err != nil {
			return err
		}
		offset = offset + limit
		exists, err = s.processRecover(rows)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *pushRulesStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var pushRuleInsert dbtypes.PushRuleInert
		if err1 := rows.Scan(&pushRuleInsert.UserID, &pushRuleInsert.RuleID, &pushRuleInsert.PriorityClass, &pushRuleInsert.Priority, &pushRuleInsert.Conditions, &pushRuleInsert.Actions); err1 != nil {
			log.Errorf("load pushRules error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PushRuleUpsertKey
		update.IsRecovery = true
		update.PushDBEvents.PushRuleInert = &pushRuleInsert
		update.SetUid(int64(common.CalcStringHashCode64(pushRuleInsert.UserID)))
		err2 := s.db.WriteDBEvent(&update)
		if err2 != nil {
			log.Errorf("update pushRules cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *pushRulesStatements) deletePushRule(
	ctx context.Context, userID, ruleID string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PushRuleDeleteKey
		update.PushDBEvents.PushRuleDelete = &dbtypes.PushRuleDelete{
			UserID: userID,
			RuleID: ruleID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEvent(&update)
	}

	return s.onDeletePushRule(ctx, userID, ruleID)
}

func (s *pushRulesStatements) onDeletePushRule(
	ctx context.Context, userID, ruleID string,
) (err error) {
	stmt := s.deletePushRuleStmt
	_, err = stmt.ExecContext(ctx, userID, ruleID)
	return
}

func (s *pushRulesStatements) insertPushRule(
	ctx context.Context, userID, ruleID string, priorityClass, priority int, conditions, actions []byte,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PushRuleUpsertKey
		update.PushDBEvents.PushRuleInert = &dbtypes.PushRuleInert{
			UserID:        userID,
			RuleID:        ruleID,
			PriorityClass: priorityClass,
			Priority:      priority,
			Conditions:    conditions,
			Actions:       actions,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEvent(&update)
	}

	return s.onInsertPushRule(ctx, userID, ruleID, priorityClass, priority, conditions, actions)
}

func (s *pushRulesStatements) onInsertPushRule(
	ctx context.Context, userID, ruleID string, priorityClass, priority int, conditions, actions []byte,
) (err error) {
	stmt := s.insertPushRuleStmt
	_, err = stmt.ExecContext(ctx, userID, ruleID, priorityClass, priority, conditions, actions)
	return
}

func (s *pushRulesStatements) selectPushRulesTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectPushRulesCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}
