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
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

func init() {
	common.Register("pushapi", NewDatabase)
}

type DataBase struct {
	db              *sql.DB
	topic           string
	underlying      string
	pushers         pushersStatements
	pushRules       pushRulesStatements
	pushRulesEnable pushRulesEnableStatements
	AsyncSave       bool

	qryDBGauge mon.LabeledGauge
}

func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	d := new(DataBase)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_pushapi")

	if d.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	d.db.SetMaxOpenConns(30)
	d.db.SetMaxIdleConns(30)
	d.db.SetConnMaxLifetime(time.Minute * 3)

	schemas := []string{d.pushers.getSchema(), d.pushRules.getSchema(), d.pushRulesEnable.getSchema()}
	for _, sqlStr := range schemas {
		_, err := d.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = d.pushers.prepare(d); err != nil {
		return nil, err
	}
	if err = d.pushRules.prepare(d); err != nil {
		return nil, err
	}
	if err = d.pushRulesEnable.prepare(d); err != nil {
		return nil, err
	}

	d.topic = topic
	d.underlying = underlying
	d.AsyncSave = useAsync
	return d, nil
}

func (d *DataBase) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

// WriteOutputEvents implements OutputRoomEventWriter
func (d *DataBase) WriteDBEvent(update *dbtypes.DBEvent) error {
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic,
		&core.TransportPubMsg{
			Keys: []byte(update.GetEventKey()),
			Obj:  update,
		})
}

func (d *DataBase) WriteDBEventWithTbl(update *dbtypes.DBEvent, tbl string) error {
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic+"_"+tbl,
		&core.TransportPubMsg{
			Keys: []byte(update.GetEventKey()),
			Obj:  update,
		})
}

func (d *DataBase) RecoverCache() {
	err := d.pushers.recoverPusher()
	if err != nil {
		log.Errorf("pushers.recoverPusher error %v", err)
	}

	err = d.pushRules.recoverPushRule()
	if err != nil {
		log.Errorf("pushRules.recoverPushRule error %v", err)
	}

	err = d.pushRulesEnable.recoverPushRuleEnable()
	if err != nil {
		log.Errorf("pushRulesEnable.recoverPushRuleEnable error %v", err)
	}

	log.Info("push api db load finished")
}

func (d *DataBase) AddPushRuleEnable(
	ctx context.Context, userID, ruleID string, enable int,
) error {
	return d.pushRulesEnable.insertPushRuleEnable(ctx, userID, ruleID, enable)
}

func (d *DataBase) OnAddPushRuleEnable(
	ctx context.Context, userID, ruleID string, enable int,
) error {
	return d.pushRulesEnable.onInsertPushRuleEnable(ctx, userID, ruleID, enable)
}

func (d *DataBase) DeletePushRule(
	ctx context.Context, userID, ruleID string,
) error {
	return d.pushRules.deletePushRule(ctx, userID, ruleID)
}

func (d *DataBase) OnDeletePushRule(
	ctx context.Context, userID, ruleID string,
) error {
	return d.pushRules.onDeletePushRule(ctx, userID, ruleID)
}

func (d *DataBase) AddPushRule(
	ctx context.Context, userID, ruleID string, priorityClass, priority int, conditions, actions []byte,
) error {
	return d.pushRules.insertPushRule(ctx, userID, ruleID, priorityClass, priority, conditions, actions)
}

func (d *DataBase) OnAddPushRule(
	ctx context.Context, userID, ruleID string, priorityClass, priority int, conditions, actions []byte,
) error {
	return d.pushRules.onInsertPushRule(ctx, userID, ruleID, priorityClass, priority, conditions, actions)
}

func (d *DataBase) AddPusher(
	ctx context.Context, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey string, pushKeyTs int64, lang string, data []byte, deviceID string,
) error {
	return d.pushers.insertPusher(ctx, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey, pushKeyTs, lang, data, deviceID)
}

func (d *DataBase) OnAddPusher(
	ctx context.Context, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey string, pushKeyTs int64, lang string, data []byte, deviceID string,
) error {
	return d.pushers.onInsertPusher(ctx, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey, pushKeyTs, lang, data, deviceID)
}

func (d *DataBase) DeleteUserPushers(
	ctx context.Context, userID, appID, pushKey string,
) error {
	return d.pushers.deletePushers(ctx, userID, appID, pushKey)
}

func (d *DataBase) OnDeleteUserPushers(
	ctx context.Context, userID, appID, pushKey string,
) error {
	return d.pushers.onDeletePushers(ctx, userID, appID, pushKey)
}

func (d *DataBase) DeletePushersByKey(
	ctx context.Context, appID, pushKey string,
) error {
	return d.pushers.deletePushersByKey(ctx, appID, pushKey)
}

func (d *DataBase) OnDeletePushersByKey(
	ctx context.Context, appID, pushKey string,
) error {
	return d.pushers.onDeletePushersByKey(ctx, appID, pushKey)
}

func (d *DataBase) DeletePushersByKeyOnly(
	ctx context.Context, pushKey string,
) error {
	return d.pushers.deletePushersByKeyOnly(ctx, pushKey)
}

func (d *DataBase) OnDeletePushersByKeyOnly(
	ctx context.Context, pushKey string,
) error {
	return d.pushers.onDeletePushersByKeyOnly(ctx, pushKey)
}

func (d *DataBase) GetPushRulesTotal(
	ctx context.Context,
) (int, error) {
	return d.pushRules.selectPushRulesTotal(ctx)
}

func (d *DataBase) GetPushRulesEnableTotal(
	ctx context.Context,
) (int, error) {
	return d.pushRulesEnable.selectPushRulesEnableTotal(ctx)
}

func (d *DataBase) LoadPusher(
	ctx context.Context,
) ([]pushapitypes.Pusher, error) {
	return d.pushers.loadPusher(ctx)
}

func (d *DataBase) LoadPushRule(
	ctx context.Context,
) ([]pushapitypes.PushRuleData, error) {
	return d.pushRules.loadPushRule(ctx)
}

func (d *DataBase) LoadPushRuleEnable(
	ctx context.Context,
) ([]pushapitypes.PushRuleEnable, error) {
	return d.pushRulesEnable.loadPushRuleEnable(ctx)
}
