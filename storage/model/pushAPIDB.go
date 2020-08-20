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

package model

import (
	"context"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
)

type PushAPIDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)

	WriteDBEvent(update *dbtypes.DBEvent) error

	RecoverCache()

	AddPushRuleEnable(
		ctx context.Context, userID, ruleID string, enable int,
	) error

	OnAddPushRuleEnable(
		ctx context.Context, userID, ruleID string, enable int,
	) error

	DeletePushRule(
		ctx context.Context, userID, ruleID string,
	) error

	OnDeletePushRule(
		ctx context.Context, userID, ruleID string,
	) error

	AddPushRule(
		ctx context.Context, userID, ruleID string, priorityClass, priority int, conditions, actions []byte,
	) error

	OnAddPushRule(
		ctx context.Context, userID, ruleID string, priorityClass, priority int, conditions, actions []byte,
	) error

	AddPusher(
		ctx context.Context, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey string, pushKeyTs int64,  lang string, data []byte, deviceID string,
	) error

	OnAddPusher(
		ctx context.Context, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey string, pushKeyTs int64,  lang string, data []byte, deviceID string,
	) error

	DeleteUserPushers(
		ctx context.Context, userID, appID, pushKey string,
	) error

	OnDeleteUserPushers(
		ctx context.Context, userID, appID, pushKey string,
	) error

	DeletePushersByKey(
		ctx context.Context, appID, pushKey string,
	) error

	OnDeletePushersByKey(
		ctx context.Context, appID, pushKey string,
	) error

	DeletePushersByKeyOnly(
		ctx context.Context, pushKey string,
	) error

	OnDeletePushersByKeyOnly(
		ctx context.Context, pushKey string,
	) error

	GetPushRulesTotal(
		ctx context.Context,
	) (int, error)

	GetPushRulesEnableTotal(
		ctx context.Context,
	) (int, error)

	LoadPusher(
		ctx context.Context,
	)([]pushapitypes.Pusher, error)

	LoadPushRule(
		ctx context.Context,
	)([]pushapitypes.PushRuleData, error)

	LoadPushRuleEnable(
		ctx context.Context,
	)([]pushapitypes.PushRuleEnable, error)
}
