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

const pushRulesEnableSchema = `
-- storage data for push_rules_enable
CREATE TABLE IF NOT EXISTS push_rules_enable (
	user_name TEXT NOT NULL,
	rule_id TEXT NOT NULL,
	enabled SMALLINT,

	CONSTRAINT push_rules_enable_unique UNIQUE (user_name, rule_id)
);

CREATE INDEX IF NOT EXISTS push_rules_enable_user_name ON push_rules_enable(user_name);
`

const insertPushRuleEnableSQL = "" +
	"INSERT INTO push_rules_enable(user_name, rule_id, enabled) VALUES ($1, $2, $3)" +
	" ON CONFLICT ON CONSTRAINT push_rules_enable_unique" +
	" DO UPDATE SET enabled = EXCLUDED.enabled"

const selectPushRuleEnableCountSQL = "" +
	"SELECT count(1) FROM push_rules_enable"

const recoverPushRuleEnableSQL = "" +
	"SELECT user_name, rule_id, enabled FROM push_rules_enable limit $1 offset $2"

type pushRulesEnableStatements struct {
	db                            *DataBase
	insertPushRuleEnableStmt      *sql.Stmt
	selectPushRuleEnableCountStmt *sql.Stmt
	recoverPushRuleEnableStmt     *sql.Stmt
}

func (s *pushRulesEnableStatements) getSchema() string {
	return pushRulesEnableSchema
}

func (s *pushRulesEnableStatements) prepare(d *DataBase) (err error) {
	s.db = d
	if s.insertPushRuleEnableStmt, err = d.db.Prepare(insertPushRuleEnableSQL); err != nil {
		return
	}
	if s.selectPushRuleEnableCountStmt, err = d.db.Prepare(selectPushRuleEnableCountSQL); err != nil {
		return
	}
	if s.recoverPushRuleEnableStmt, err = d.db.Prepare(recoverPushRuleEnableSQL); err != nil {
		return
	}
	return
}

func (s *pushRulesEnableStatements) loadPushRuleEnable(ctx context.Context) ([]pushapitypes.PushRuleEnable, error) {
	offset := 0
	limit := 1000
	result := []pushapitypes.PushRuleEnable{}
	for {
		ruleEnables := []pushapitypes.PushRuleEnable{}
		rows, err := s.recoverPushRuleEnableStmt.QueryContext(ctx, limit, offset)
		if err != nil {
			log.Errorf("load push rule enable exec recoverPushRuleEnableStmt err:%v", err)
			return nil, err
		}
		for rows.Next() {
			var pushRuleEnable pushapitypes.PushRuleEnable
			if err := rows.Scan(&pushRuleEnable.UserID, &pushRuleEnable.RuleID, &pushRuleEnable.Enabled); err != nil {
				log.Errorf("load push rule enable scan rows error:%v", err)
				return nil, err
			} else {
				ruleEnables = append(ruleEnables, pushRuleEnable)
			}
		}
		result = append(result, ruleEnables...)
		if len(ruleEnables) < limit {
			break
		} else {
			offset = offset + limit
		}
	}
	return result, nil
}

func (s *pushRulesEnableStatements) recoverPushRuleEnable() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverPushRuleEnableStmt.QueryContext(context.TODO(), limit, offset)
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

func (s *pushRulesEnableStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var pushRuleEnableInsert dbtypes.PushRuleEnableInsert
		if err1 := rows.Scan(&pushRuleEnableInsert.UserID, &pushRuleEnableInsert.RuleID, &pushRuleEnableInsert.Enabled); err1 != nil {
			log.Errorf("load pushRulesEnable error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PushRuleEnableUpsetKey
		update.IsRecovery = true
		update.PushDBEvents.PushRuleEnableInsert = &pushRuleEnableInsert
		update.SetUid(int64(common.CalcStringHashCode64(pushRuleEnableInsert.UserID)))
		err2 := s.db.WriteDBEventWithTbl(&update, "push_rules_enable")
		if err2 != nil {
			log.Errorf("update pushRulesEnable cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *pushRulesEnableStatements) insertPushRuleEnable(
	ctx context.Context, userID, ruleID string, enable int,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PushRuleEnableUpsetKey
		update.PushDBEvents.PushRuleEnableInsert = &dbtypes.PushRuleEnableInsert{
			UserID:  userID,
			RuleID:  ruleID,
			Enabled: enable,
		}
		update.SetUid(int64(common.CalcStringHashCode64(userID)))
		return s.db.WriteDBEventWithTbl(&update, "push_rules_enable")
	}

	return s.onInsertPushRuleEnable(ctx, userID, ruleID, enable)
}

func (s *pushRulesEnableStatements) onInsertPushRuleEnable(
	ctx context.Context, userID, ruleID string, enable int,
) (err error) {
	stmt := s.insertPushRuleEnableStmt
	_, err = stmt.ExecContext(ctx, userID, ruleID, enable)
	return
}

func (s *pushRulesEnableStatements) selectPushRulesEnableTotal(
	ctx context.Context,
) (count int, err error) {
	err = s.selectPushRuleEnableCountStmt.QueryRowContext(ctx).Scan(&count)
	return
}
