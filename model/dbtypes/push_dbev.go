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

package dbtypes

const (
	PusherDeleteKey          int64 = 0
	PusherDeleteByKeyKey     int64 = 1
	PusherDeleteByKeyOnlyKey int64 = 2
	PusherInsertKey          int64 = 3
	PushRuleUpsertKey        int64 = 4
	PushRuleDeleteKey        int64 = 5
	PushRuleEnableUpsetKey   int64 = 6
	PushMaxKey               int64 = 7
)

func PushDBEventKeyToStr(key int64) string {
	switch key {
	case PusherDeleteKey:
		return "PusherDelete"
	case PusherDeleteByKeyKey:
		return "PusherDeleteByKey"
	case PusherDeleteByKeyOnlyKey:
		return "PusherDeleteByKeyOnly"
	case PusherInsertKey:
		return "PusherInsert"
	case PushRuleUpsertKey:
		return "PushRuleUpsert"
	case PushRuleDeleteKey:
		return "PushRuleDelete"
	case PushRuleEnableUpsetKey:
		return "PushRuleEnableUpset"
	default:
		return "unknown"
	}
}

func PushDBEventKeyToTableStr(key int64) string {
	switch key {
	case PusherDeleteKey, PusherDeleteByKeyKey, PusherDeleteByKeyOnlyKey, PusherInsertKey:
		return "pushers"
	case PushRuleUpsertKey, PushRuleDeleteKey:
		return "push_rules"
	case PushRuleEnableUpsetKey:
		return "push_rules_enable"
	default:
		return "unknown"
	}
}

type PushDBEvent struct {
	PusherDelete          *PusherDelete          `json:"pusher_delete,omitempty"`
	PusherDeleteByKey     *PusherDeleteByKey     `json:"pusher_delete_by_key,omitempty"`
	PusherDeleteByKeyOnly *PusherDeleteByKeyOnly `json:"pusher_delete_by_key_only,omitempty"`
	PusherInsert          *PusherInsert          `json:"pusher_insert,omitempty"`
	PushRuleInert         *PushRuleInert         `json:"push_rule_insert,omitempty"`
	PushRuleDelete        *PushRuleDelete        `json:"push_rule_delete,omitempty"`
	PushRuleEnableInsert  *PushRuleEnableInsert  `json:"push_rule_enabled_insert,omitempty"`
}

type PushRuleEnableInsert struct {
	UserID  string `json:"user_id"`
	RuleID  string `json:"rule_id"`
	Enabled int    `json:"enabled"`
}

type PushRuleDelete struct {
	UserID string `json:"user_id"`
	RuleID string `json:"rule_id"`
}

type PushRuleInert struct {
	UserID        string `json:"user_id"`
	RuleID        string `json:"rule_id"`
	PriorityClass int    `json:"priority_class"`
	Priority      int    `json:"priority"`
	Conditions    []byte `json:"conditions"`
	Actions       []byte `json:"actions"`
}

type PusherInsert struct {
	UserID            string `json:"user_id"`
	ProfileTag        string `json:"profile_tag"`
	Kind              string `json:"kind"`
	AppID             string `json:"app_id"`
	AppDisplayName    string `json:"app_display_name"`
	DeviceDisplayName string `json:"device_display_name"`
	PushKey           string `json:"push_key"`
	PushKeyTs         int64  `json:"push_key_ts"`
	Lang              string `json:"lang"`
	Data              []byte `json:"data"`
	DeviceID          string `json:"device_id"`
}

type PusherDeleteByKeyOnly struct {
	PushKey string `json:"push_key"`
}

type PusherDeleteByKey struct {
	AppID   string `json:"app_id"`
	PushKey string `json:"push_key"`
}

type PusherDelete struct {
	UserID  string `json:"user_id"`
	AppID   string `json:"app_id"`
	PushKey string `json:"push_key"`
}
