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

package routing

import (
	"github.com/finogeeks/ligase/model/pushapitypes"
)

var GetAction = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "dont_notify")
	return actions
}

var GetAction1 = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "notify")
	tweaks := []pushapitypes.Tweak{
		{
			SetTweak: "sound",
			Value:    "default",
		},
		{
			SetTweak: "highlight",
			Value:    false,
		},
	}

	for _, v := range tweaks {
		actions = append(actions, v)
	}
	return actions
}

var GetAction2 = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "notify")
	tweaks := []pushapitypes.Tweak{
		{
			SetTweak: "sound",
			Value:    "default",
		},
		{
			SetTweak: "highlight",
		},
	}

	for _, v := range tweaks {
		actions = append(actions, v)
	}
	return actions
}

var GetAction3 = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "notify")
	tweaks := []pushapitypes.Tweak{
		{
			SetTweak: "sound",
			Value:    "default",
		},
	}

	for _, v := range tweaks {
		actions = append(actions, v)
	}
	return actions
}

var GetAction4 = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "notify")
	tweaks := []pushapitypes.Tweak{
		{
			SetTweak: "sound",
			Value:    "ring",
		},
		{
			SetTweak: "highlight",
			Value:    false,
		},
	}

	for _, v := range tweaks {
		actions = append(actions, v)
	}
	return actions
}

var GetAction5 = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "notify")
	tweaks := []pushapitypes.Tweak{
		{
			SetTweak: "highlight",
			Value:    false,
		},
	}

	for _, v := range tweaks {
		actions = append(actions, v)
	}
	return actions
}

var GetAction6 = func() []interface{} {
	var actions []interface{}
	actions = append(actions, "notify")
	tweaks := []pushapitypes.Tweak{
		{
			SetTweak: "sound",
			Value:    "default",
		},
	}

	for _, v := range tweaks {
		actions = append(actions, v)
	}
	return actions
}

var BaseRuleIds = func() map[string]string {
	rules := map[string]string{
		"global/override/.m.rule.master":            "override",
		"global/override/.m.rule.suppress_notices":  "override",
		"global/override/.m.rule.invite_for_me":     "override",
		"global/override/.m.rule.member_event":      "override",
		"global/override/.m.rule.signals":           "override",
		"global/content/.m.rule.contains_user_name": "content",
		"global/underride/.m.rule.call":             "underride",
		"global/underride/.m.rule.room_one_to_one":  "underride",
		"global/underride/.m.rule.message":          "underride",
		"global/underride/.m.rule.video":            "underride",
		"global/underride/.m.rule.encrypted":        "underride",
	}
	return rules
}

var BasePreOverrideRules = func() []pushapitypes.PushRule {
	pushRule := pushapitypes.PushRule{
		RuleId:  "global/override/.m.rule.master",
		Default: true,
		Enabled: false,
		Actions: GetAction(),
	}
	pushRule.Conditions = []pushapitypes.PushCondition{}
	pushRules := []pushapitypes.PushRule{pushRule}
	return pushRules
}

var BaseOverrideRules = func() []pushapitypes.PushRule {
	pushRules := []pushapitypes.PushRule{
		{
			RuleId:  "global/override/.m.rule.suppress_notices",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "content.msgtype",
					Pattern: "m.notice",
				},
			},
			Actions: GetAction(),
		},
		{
			RuleId:  "global/override/.m.rule.invite_for_me",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "type",
					Pattern: "m.room.member",
				},
				{
					Kind:    "event_match",
					Key:     "content.membership",
					Pattern: "invite",
				},
				{
					Kind:    "event_match",
					Key:     "state_key",
					Pattern: "user_id",
				},
			},
			Actions: GetAction1(),
		},
		{
			RuleId:  "global/override/.m.rule.member_event",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "type",
					Pattern: "m.room.member",
				},
			},
			Actions: GetAction(),
		},
		{
			RuleId:  "global/override/.m.rule.signals",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "content.msgtype",
					Pattern: "m.alert",
				},
				{
					Kind: "signal",
				},
			},
			Actions: GetAction2(),
		},
	}
	return pushRules
}

var BaseContentRules = func() []pushapitypes.PushRule {
	pushRules := []pushapitypes.PushRule{
		{
			RuleId:  "global/content/.m.rule.contains_user_name",
			Default: true,
			Enabled: false,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "content.body",
					Pattern: "user_localpart",
				},
			},
			Actions: GetAction2(),
		},
	}
	return pushRules
}

var BaseUnderRideRules = func() []pushapitypes.PushRule {
	pushRules := []pushapitypes.PushRule{
		{
			RuleId:  "global/underride/.m.rule.call",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "type",
					Pattern: "m.call.invite",
				},
			},
			Actions: GetAction4(),
		},
		{
			RuleId:  "global/underride/.m.rule.room_one_to_one",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind: "room_member_count",
					Is:   "2",
				},
			},
			Actions: GetAction1(),
		},
		{
			RuleId:  "global/underride/.m.rule.message",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "type",
					Pattern: "m.room.message",
				},
			},
			Actions: GetAction6(),
		},
		{
			RuleId:  "global/underride/.m.rule.encrypted",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "type",
					Pattern: "m.room.encrypted",
				},
			},
			Actions: GetAction5(),
		},
		{
			RuleId:  "global/underride/.m.rule.video",
			Default: true,
			Enabled: true,
			Conditions: []pushapitypes.PushCondition{
				{
					Kind:    "event_match",
					Key:     "type",
					Pattern: "m.modular.video",
				},
				{
					Kind:    "event_match",
					Key:     "content.data.isCreate",
					Pattern: "true",
				},
			},
			Actions: GetAction5(),
		},
	}
	return pushRules
}