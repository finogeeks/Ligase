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
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func FormatRuleId(
	kind string,
	ruleID string,
) string {
	return fmt.Sprintf("global/%s/%s", kind, ruleID)
}

func ConvertConditions(userID, kind string, pushRule pushapitypes.PushRule, forRequest bool) pushapitypes.PushRule {
	if len(pushRule.Conditions) > 0 {
		for i := 0; i < len(pushRule.Conditions); i++ {
			if pushRule.Conditions[i].Pattern != "" {
				if pushRule.Conditions[i].Pattern == "user_id" {
					pushRule.Conditions[i].Pattern = userID
				} else if pushRule.Conditions[i].Pattern == "user_localpart" {
					localPart, _, _ := gomatrixserverlib.SplitID('@', userID)
					pushRule.Conditions[i].Pattern = localPart
				}
			}
			if kind == "content" {
				pushRule.Pattern = pushRule.Conditions[0].Pattern
				if forRequest {
					pushRule.Conditions = []pushapitypes.PushCondition{}
				}
			}
			if forRequest {
				if kind == "sender" || kind == "room" {
					pushRule.Conditions = []pushapitypes.PushCondition{}
				}
			}
		}
	}
	return pushRule
}

func GetBasePushRule(ruleID string) pushapitypes.PushRule {
	rule := pushapitypes.PushRule{}

	if ruleClass, ok := BaseRuleIds()[ruleID]; ok {
		var baseRules []pushapitypes.PushRule

		switch ruleClass {
		case "override":
			if ruleID == "global/override/.m.rule.master" {
				baseRules = BasePreOverrideRules()
			} else {
				baseRules = BaseOverrideRules()
			}
		case "content":
			baseRules = BaseContentRules()
		case "underride":
			baseRules = BaseUnderRideRules()
		}
		for _, v := range baseRules {
			if v.RuleId == ruleID {
				rule = v
			}
		}
	}
	return rule
}

func GetOriginalRuleId(ruleID string) string {
	s := strings.Split(ruleID, "/")
	return s[len(s)-1]
}

func GetKindFromRuleId(ruleID string) string {
	s := strings.Split(ruleID, "/")
	return s[len(s)-2]
}

func GetRuleEnabled(userName, ruleID string, cache service.Cache) bool {
	enabledCache, ok := cache.GetPushRuleEnabled(userName, ruleID)
	if ok {
		if enabledCache == "1" {
			return true
		}
		return false
	}

	var defaultRule bool
	defaultRule = strings.HasPrefix(GetOriginalRuleId(ruleID), ".")
	if !defaultRule {
		return true
	}

	baseRule := GetBasePushRule(ruleID)
	if baseRule.RuleId == ruleID {
		return baseRule.Enabled
	}
	return false
}

func ConvertPushRule(data pushapitypes.PushRuleCacheData, defaultRule bool) (pushapitypes.PushRule, error) {
	var pushRule = pushapitypes.PushRule{}
	pushRule.RuleId = data.RuleId
	pushRule.Default = defaultRule
	err := json.Unmarshal([]byte(data.Actions), &pushRule.Actions)
	if err != nil {
		return pushRule, err
	}
	if !defaultRule {
		err1 := json.Unmarshal([]byte(data.Conditions), &pushRule.Conditions)
		if err1 != nil {
			return pushRule, err1
		}
	}

	return pushRule, nil
}

func FormatRuleResponse(rules pushapitypes.Rules) pushapitypes.RuleSet {
	ruleSet := pushapitypes.RuleSet{}

	for _, v := range rules.Sender {
		ruleSet.Sender = append(ruleSet.Sender, v)
	}
	if len(ruleSet.Sender) == 0 {
		ruleSet.Sender = []interface{}{}
	}

	for _, v := range rules.Room {
		ruleSet.Room = append(ruleSet.Room, v)
	}
	if len(ruleSet.Room) == 0 {
		ruleSet.Room = []interface{}{}
	}

	for _, v := range rules.Content {
		ruleSet.Content = append(ruleSet.Content, v)
	}
	if len(ruleSet.Content) == 0 {
		ruleSet.Content = []interface{}{}
	}

	for _, v := range rules.Override {
		newRule := pushapitypes.PushRuleWithConditions(v)
		ruleSet.Override = append(ruleSet.Override, newRule)
	}
	if len(ruleSet.Override) == 0 {
		ruleSet.Override = []interface{}{}
	}

	for _, v := range rules.UnderRide {
		newRule := pushapitypes.PushRuleWithConditions(v)
		ruleSet.UnderRide = append(ruleSet.UnderRide, newRule)
	}
	if len(ruleSet.UnderRide) == 0 {
		ruleSet.UnderRide = []interface{}{}
	}

	return ruleSet
}

func GetUserPushRules(userID string, cache service.Cache, forRequest bool) (global pushapitypes.Rules) {
	global = pushapitypes.Rules{}
	var bases map[string]pushapitypes.PushRuleCacheData
	bases = make(map[string]pushapitypes.PushRuleCacheData)
	var rules pushapitypes.PushRuleCacheDataArray

	ruleIDs, _ := cache.GetUserPushRuleIds(userID)

	for _, rid := range ruleIDs {
		rule, _ := cache.GetPushRuleCacheData(rid)
		if rule != nil {
			if rule.PriorityClass == -1 {
				bases[rid] = *rule
			} else {
				rules = append(rules, *rule)
			}
		}
	}
	if len(rules) > 0 {
		sort.Sort(rules)
	}

	currentClass := len(pushapitypes.PriorityMap())
	kind, _ := pushapitypes.RevPriorityMap()[currentClass]
	preRules := MakeBasePreAppendRule(kind, bases)
	for _, v := range preRules {
		global = AddRules(kind, v, global)
	}

	for _, v := range rules {
		for v.PriorityClass < currentClass {
			kind1, _ := pushapitypes.RevPriorityMap()[currentClass]
			baseRules := MakeBaseAppendRule(kind1, bases)
			for _, val := range baseRules {
				global = AddRules(kind1, val, global)
			}
			currentClass -= 1
		}
		curKind, _ := pushapitypes.RevPriorityMap()[v.PriorityClass]
		curRule, _ := ConvertPushRule(v, false)
		global = AddRules(curKind, curRule, global)
	}

	for currentClass > 0 {
		kind1, _ := pushapitypes.RevPriorityMap()[currentClass]
		baseRules := MakeBaseAppendRule(kind1, bases)
		for _, v := range baseRules {
			global = AddRules(kind1, v, global)
		}
		currentClass -= 1
	}

	return FormatRules(userID, cache, global, forRequest)
}

func FormatRules(userID string, cache service.Cache, global pushapitypes.Rules, forRequest bool) pushapitypes.Rules {
	for i := 0; i < len(global.UnderRide); i++ {
		global.UnderRide[i].Enabled = GetRuleEnabled(userID, global.UnderRide[i].RuleId, cache)
		kind := GetKindFromRuleId(global.UnderRide[i].RuleId)
		global.UnderRide[i] = ConvertConditions(userID, kind, global.UnderRide[i], forRequest)
		global.UnderRide[i].RuleId = GetOriginalRuleId(global.UnderRide[i].RuleId)
	}

	for i := 0; i < len(global.Content); i++ {
		global.Content[i].Enabled = GetRuleEnabled(userID, global.Content[i].RuleId, cache)
		kind := GetKindFromRuleId(global.Content[i].RuleId)
		global.Content[i] = ConvertConditions(userID, kind, global.Content[i], forRequest)
		global.Content[i].RuleId = GetOriginalRuleId(global.Content[i].RuleId)
	}

	for i := 0; i < len(global.Override); i++ {
		global.Override[i].Enabled = GetRuleEnabled(userID, global.Override[i].RuleId, cache)
		kind := GetKindFromRuleId(global.Override[i].RuleId)
		global.Override[i] = ConvertConditions(userID, kind, global.Override[i], forRequest)
		global.Override[i].RuleId = GetOriginalRuleId(global.Override[i].RuleId)
	}

	for i := 0; i < len(global.Room); i++ {
		global.Room[i].Enabled = GetRuleEnabled(userID, global.Room[i].RuleId, cache)
		kind := GetKindFromRuleId(global.Room[i].RuleId)
		global.Room[i] = ConvertConditions(userID, kind, global.Room[i], forRequest)
		global.Room[i].RuleId = GetOriginalRuleId(global.Room[i].RuleId)
	}

	for i := 0; i < len(global.Sender); i++ {
		global.Sender[i].Enabled = GetRuleEnabled(userID, global.Sender[i].RuleId, cache)
		kind := GetKindFromRuleId(global.Sender[i].RuleId)
		global.Sender[i] = ConvertConditions(userID, kind, global.Sender[i], forRequest)
		global.Sender[i].RuleId = GetOriginalRuleId(global.Sender[i].RuleId)
	}

	return global
}

func MakeBaseAppendRule(kind string, modified map[string]pushapitypes.PushRuleCacheData) []pushapitypes.PushRule {
	var rules []pushapitypes.PushRule
	switch kind {
	case "underride":
		rules = BaseUnderRideRules()
	case "content":
		rules = BaseContentRules()
	case "override":
		rules = BaseOverrideRules()
	}

	for i := 0; i < len(rules); i++ {
		if data, ok := modified[rules[i].RuleId]; ok {
			err := json.Unmarshal([]byte(data.Actions), &rules[i].Actions)
			if err != nil {
				log.Errorw("actions convert error", log.KeysAndValues{"rule_id", rules[i].RuleId})
			}
		}
	}
	return rules
}

func AddRules(kind string, rule pushapitypes.PushRule, rules pushapitypes.Rules) pushapitypes.Rules {
	switch kind {
	case "underride":
		rules.UnderRide = append(rules.UnderRide, rule)
	case "sender":
		rules.Sender = append(rules.Sender, rule)
	case "room":
		rules.Room = append(rules.Room, rule)
	case "content":
		rules.Content = append(rules.Content, rule)
	case "override":
		rules.Override = append(rules.Override, rule)
	}
	return rules
}

func MakeBasePreAppendRule(kind string, modified map[string]pushapitypes.PushRuleCacheData) []pushapitypes.PushRule {
	var rules []pushapitypes.PushRule
	if kind == "override" {
		rules = BasePreOverrideRules()
	}
	for i := 0; i < len(rules); i++ {
		if data, ok := modified[rules[i].RuleId]; ok {
			err := json.Unmarshal([]byte(data.Actions), &rules[i].Actions)
			if err != nil {
				log.Errorw("actions convert error", log.KeysAndValues{"rule_id", rules[i].RuleId})
			}
		}
	}
	return rules
}

//PutPushruleActions implements PUT /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/actions
func PutPushRuleActions(
	ctx context.Context,
	actions *external.PutPushrulesActionsByIDRequest,
	pushDB model.PushAPIDatabase,
	device *authtypes.Device,
	cfg config.Dendrite,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" || kind == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	if _, ok := pushapitypes.PriorityMap()[kind]; !ok {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	// var actions pushapitypes.PushActions
	// if reqErr := httputil.UnmarshalJSONRequest(req, &actions); reqErr != nil {
	// 	return *reqErr
	// }

	data := new(types.ActDataStreamUpdate)
	data.UserID = device.UserID
	data.RoomID = ""
	data.DataType = ""
	data.StreamType = "pushRule"

	span, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(device.UserID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	var defaultRule bool
	defaultRule = strings.HasPrefix(ruleID, ".")
	insertRuleID := FormatRuleId(kind, ruleID)

	if defaultRule {
		if _, ok := BaseRuleIds()[insertRuleID]; !ok {
			return http.StatusBadRequest, jsonerror.Unknown("No rule found with rule id")
		}
	}

	actionsArray, err := json.Marshal(actions.Actions)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Actions is malformed")
	}
	actionsJSON, err := gomatrixserverlib.CanonicalJSON(actionsArray)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Actions is malformed JSON")
	}

	cacheData, _ := cache.GetPushRuleCacheDataByID(device.UserID, insertRuleID)

	if cacheData.UserName == "" {
		if !defaultRule {
			return http.StatusBadRequest, jsonerror.Unknown("No rule found with rule id")
		}

		conditions := pushapitypes.PushCondition{}
		conditionsArray, err := json.Marshal(conditions)
		if err != nil {
			return http.StatusBadRequest, jsonerror.BadJSON("Actions is malformed")
		}
		conditionsJSON, err := gomatrixserverlib.CanonicalJSON(conditionsArray)
		if err != nil {
			return http.StatusBadRequest, jsonerror.BadJSON("Actions is malformed JSON")
		}

		if err := pushDB.AddPushRule(ctx, device.UserID, insertRuleID, -1, 1, conditionsJSON, actionsJSON); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	} else {
		if err := pushDB.AddPushRule(ctx, cacheData.UserName, cacheData.RuleId, cacheData.PriorityClass, cacheData.Priority, cacheData.Conditions, actionsJSON); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	}

	return http.StatusOK, nil
}

//GetPushruleActions implements GET /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/actions
func GetPushRuleActions(
	ctx context.Context,
	device *authtypes.Device,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if _, ok := pushapitypes.PriorityMap()[kind]; !ok {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	actions := pushapitypes.PushActions{}
	insertedRuleID := FormatRuleId(kind, ruleID)
	data, _ := cache.GetPushRuleCacheDataByID(device.UserID, insertedRuleID)
	defaultRule := strings.HasPrefix(ruleID, ".")

	if data.UserName != "" {
		err := json.Unmarshal([]byte(data.Actions), &actions.Actions)
		if err != nil {
			httputil.LogThenErrorCtx(ctx, err)
		}
	} else {
		if !defaultRule {
			return http.StatusNotFound, jsonerror.Unknown("No rule found with rule id")
		}
		baseRule := GetBasePushRule(insertedRuleID)
		actions.Actions = baseRule.Actions
	}

	return http.StatusOK, &actions
}

//PutPushruleEnabled implements PUT /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/enabled
func PutPushRuleEnabled(
	ctx context.Context,
	enabled *external.PutPushrulesEnabledByIDRequest,
	pushDB model.PushAPIDatabase,
	device *authtypes.Device,
	cfg config.Dendrite,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if _, ok := pushapitypes.PriorityMap()[kind]; !ok {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	// var enabled pushapitypes.EnabledType
	// if reqErr := httputil.UnmarshalJSONRequest(req, &enabled); reqErr != nil {
	// 	return *reqErr
	// }

	enableValue := 0
	if enabled.Enabled == true {
		enableValue = 1
	}

	insertedRuleID := FormatRuleId(kind, ruleID)

	cacheData, _ := cache.GetPushRuleCacheDataByID(device.UserID, insertedRuleID)
	defaultRule := strings.HasPrefix(ruleID, ".")
	if cacheData.UserName == "" {
		if !defaultRule {
			return http.StatusNotFound, jsonerror.Unknown("No rule found with rule id")
		}
	}

	if err := pushDB.AddPushRuleEnable(ctx, device.UserID, insertedRuleID, enableValue); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	data := new(types.ActDataStreamUpdate)
	data.UserID = device.UserID
	data.RoomID = ""
	data.DataType = ""
	data.StreamType = "pushRule"

	span, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(device.UserID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}

//GetPushruleEnabled implements GET /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/enabled
func GetPushRuleEnabled(
	ctx context.Context,
	device *authtypes.Device,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if _, ok := pushapitypes.PriorityMap()[kind]; !ok {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	var enable pushapitypes.EnabledType
	insertedRuleID := FormatRuleId(kind, ruleID)

	data, _ := cache.GetPushRuleCacheDataByID(device.UserID, insertedRuleID)
	defaultRule := strings.HasPrefix(ruleID, ".")
	if data.UserName == "" {
		if !defaultRule {
			return http.StatusNotFound, jsonerror.Unknown("No rule found with rule id")
		}
	}

	enable.Enabled = GetRuleEnabled(device.UserID, insertedRuleID, cache)

	return http.StatusOK, &enable
}

//GetPushrule implements GET /_matrix/client/r0/pushrules
func GetPushRules(
	device *authtypes.Device,
	cache service.Cache,
) (int, core.Coder) {
	global := pushapitypes.GlobalRule{}
	rules := GetUserPushRules(device.UserID, cache, true)
	global.Global = FormatRuleResponse(rules)
	global.Device = map[string]interface{}{}

	return http.StatusOK, &global
}

//GetPushrule implements GET /_matrix/client/r0/pushrules/global/
func GetPushRulesGlobal(
	device *authtypes.Device,
	cache service.Cache,
) (int, core.Coder) {
	rules := GetUserPushRules(device.UserID, cache, true)
	ruleset := FormatRuleResponse(rules)
	return http.StatusOK, &ruleset
}

//GetPushrule implements GET /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}
func GetPushRule(
	ctx context.Context,
	device *authtypes.Device,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if _, ok := pushapitypes.PriorityMap()[kind]; !ok {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	insertedRuleID := FormatRuleId(kind, ruleID)
	data, _ := cache.GetPushRuleCacheDataByID(device.UserID, insertedRuleID)
	defaultRule := strings.HasPrefix(ruleID, ".")
	pushRule := pushapitypes.PushRule{}

	if data.UserName == "" {
		if !defaultRule {
			return http.StatusNotFound, jsonerror.Unknown("No rule found with rule id")
		} else {
			pushRule = GetBasePushRule(insertedRuleID)
			if pushRule.RuleId == "" {
				return http.StatusNotFound, jsonerror.Unknown("No rule found with rule id")
			}
		}
	} else {
		if rule, err := ConvertPushRule(*data, defaultRule); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		} else {
			if defaultRule {
				pushRule = GetBasePushRule(insertedRuleID)
				pushRule.Actions = rule.Actions
			} else {
				pushRule = rule
			}
			pushRule.Enabled = GetRuleEnabled(device.UserID, insertedRuleID, cache)
		}
	}

	if pushRule.Pattern != "" {
		if pushRule.Pattern == "user_id" {
			pushRule.Pattern = device.UserID
		} else if pushRule.Pattern == "user_localpart" {
			localPart, _, _ := gomatrixserverlib.SplitID('@', device.UserID)
			pushRule.Pattern = localPart
		}
	}

	pushRule = ConvertConditions(device.UserID, kind, pushRule, true)
	pushRule.RuleId = ruleID

	if kind == "override" || kind == "underride" {
		newRule := pushapitypes.PushRuleWithConditions{
			Actions:    pushRule.Actions,
			Default:    pushRule.Default,
			Enabled:    pushRule.Enabled,
			RuleId:     pushRule.RuleId,
			Conditions: pushRule.Conditions,
			Pattern:    pushRule.Pattern,
		}
		return http.StatusOK, &newRule
	}

	return http.StatusOK, &pushRule
}

//DeletePushrule implements DELETE /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}
func DeletePushRule(
	ctx context.Context,
	pushDB model.PushAPIDatabase,
	device *authtypes.Device,
	cfg config.Dendrite,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if _, ok := pushapitypes.PriorityMap()[kind]; !ok {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	deleteRuleID := FormatRuleId(kind, ruleID)

	cacheData, _ := cache.GetPushRuleCacheDataByID(device.UserID, deleteRuleID)
	if cacheData.UserName == "" {
		return http.StatusNotFound, jsonerror.Unknown("No rule found with rule id")
	}

	if err := pushDB.DeletePushRule(ctx, device.UserID, deleteRuleID); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	data := new(types.ActDataStreamUpdate)
	data.UserID = device.UserID
	data.RoomID = ""
	data.DataType = ""
	data.StreamType = "pushRule"

	span, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(device.UserID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}

//PutPushrule implements PUT /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}
func PutPushRule(
	ctx context.Context,
	pushRule *external.PutPushrulesByIDRequest,
	pushDB model.PushAPIDatabase,
	device *authtypes.Device,
	cfg config.Dendrite,
	scope string,
	kind string,
	ruleID string,
	cache service.Cache,
) (int, core.Coder) {
	if scope != "global" || kind == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized request")
	}

	if ruleID == "" {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized rule id")
	}

	var priorityClass int
	if v, ok := pushapitypes.PriorityMap()[kind]; ok {
		priorityClass = v
	} else {
		return http.StatusBadRequest, jsonerror.Unknown("Unrecognized kind")
	}

	if strings.HasPrefix(ruleID, ".") {
		return http.StatusBadRequest, jsonerror.Unknown("cannot add new rule_ids that start with '.'")
	}

	// var pushRule pushapitypes.PushRule
	// if reqErr := httputil.UnmarshalJSONRequest(req, &pushRule); reqErr != nil {
	// 	return *reqErr
	// }

	switch kind {
	case "override":
		if len(pushRule.Conditions) == 0 {
			return http.StatusBadRequest, jsonerror.Unknown("Missing 'conditions'")
		}
		for _, v := range pushRule.Conditions {
			if v.Kind == "" {
				return http.StatusBadRequest, jsonerror.Unknown("Condition without 'kind'")
			}
		}
	case "underride":
		if len(pushRule.Conditions) == 0 {
			return http.StatusBadRequest, jsonerror.Unknown("Missing 'conditions'")
		}
		for _, v := range pushRule.Conditions {
			if v.Kind == "" {
				return http.StatusBadRequest, jsonerror.Unknown("Condition without 'kind'")
			}
		}
	case "room":
		var condition pushapitypes.PushCondition
		condition.Kind = "event_match"
		condition.Key = "room_id"
		condition.Pattern = ruleID
		pushRule.Conditions = []external.PushCondition{(external.PushCondition)(condition)}
	case "sender":
		var condition pushapitypes.PushCondition
		condition.Kind = "event_match"
		condition.Key = "user_id"
		condition.Pattern = ruleID
		pushRule.Conditions = []external.PushCondition{(external.PushCondition)(condition)}
	case "content":
		if pushRule.Pattern == "" {
			return http.StatusBadRequest, jsonerror.Unknown("Content rule missing 'pattern'")
		}
		var condition pushapitypes.PushCondition
		condition.Kind = "event_match"
		condition.Key = "content.body"
		condition.Pattern = pushRule.Pattern
		pushRule.Conditions = []external.PushCondition{(external.PushCondition)(condition)}
	}
	if len(pushRule.Actions) == 0 {
		return http.StatusBadRequest, jsonerror.Unknown("No actions found")
	}
	//TODO check content of action

	before := pushRule.Before
	var beforeID string
	if before != "" {
		beforeID = FormatRuleId(kind, before)
	}

	after := pushRule.After
	var afterID string
	if after != "" {
		afterID = FormatRuleId(kind, after)
	}

	insertRuleID := FormatRuleId(kind, ruleID)

	conditionsArray, err := json.Marshal(pushRule.Conditions)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Conditions is malformed")
	}
	conditionsJSON, err := gomatrixserverlib.CanonicalJSON(conditionsArray)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Conditions is malformed JSON")
	}

	actionsArray, err := json.Marshal(pushRule.Actions)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Actions is malformed")
	}
	actionsJSON, err := gomatrixserverlib.CanonicalJSON(actionsArray)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Actions is malformed JSON")
	}

	if before != "" || after != "" {
		return addPushRuleWithRelated(ctx, pushDB, cfg, insertRuleID, beforeID, afterID, priorityClass, conditionsJSON, actionsJSON, cache, device.UserID)
	} else {
		return addPushRuleWithoutRelated(ctx, pushDB, cfg, insertRuleID, priorityClass, conditionsJSON, actionsJSON, cache, device.UserID)
	}
}

func addPushRuleWithoutRelated(
	ctx context.Context,
	pushDB model.PushAPIDatabase,
	cfg config.Dendrite,
	ruleID string,
	priorityClass int,
	conditions,
	actions []byte,
	cache service.Cache,
	userID string,
) (int, core.Coder) {
	priority := 0
	ruleIDs, _ := cache.GetUserPushRuleIds(userID)
	for _, rid := range ruleIDs {
		rule, _ := cache.GetPushRuleCacheData(rid)
		if rule.PriorityClass == priorityClass {
			if rule.Priority > priority {
				priority = rule.Priority
			}
		}
	}
	if priority > 0 {
		priority = priority + 1
	}

	if err := pushDB.AddPushRule(ctx, userID, ruleID, priorityClass, priority, conditions, actions); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	data := new(types.ActDataStreamUpdate)
	data.UserID = userID
	data.RoomID = ""
	data.DataType = ""
	data.StreamType = "pushRule"
	span, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(userID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}

func addPushRuleWithRelated(
	ctx context.Context,
	pushDB model.PushAPIDatabase,
	cfg config.Dendrite,
	ruleID,
	beforeID,
	afterID string,
	priorityClass int,
	conditions,
	actions []byte,
	cache service.Cache,
	userID string,
) (int, core.Coder) {
	var relatedID string
	if beforeID != "" {
		relatedID = beforeID
	} else if afterID != "" {
		relatedID = afterID
	}

	cacheData, _ := cache.GetPushRuleCacheDataByID(userID, relatedID)
	if cacheData.UserName == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("before/after rule not found")
	}

	if cacheData.PriorityClass != priorityClass {
		return http.StatusBadRequest, jsonerror.BadJSON("Given priority class does not match class of relative rule")
	}

	var priority int
	relatedPriority := cacheData.Priority
	if beforeID != "" {
		priority = relatedPriority + 1
	} else {
		priority = relatedPriority
	}

	ruleIDs, _ := cache.GetUserPushRuleIds(userID)
	for _, rid := range ruleIDs {
		rule, _ := cache.GetPushRuleCacheData(rid)
		if rule.PriorityClass == priorityClass {
			if rule.Priority >= priority {
				if err := pushDB.AddPushRule(ctx, rule.UserName, rule.RuleId, rule.PriorityClass, rule.Priority+1, rule.Conditions, rule.Actions); err != nil {
					return httputil.LogThenErrorCtx(ctx, err)
				}
			}
		}
	}

	if err := pushDB.AddPushRule(ctx, userID, ruleID, priorityClass, priority, conditions, actions); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	data := new(types.ActDataStreamUpdate)
	data.UserID = userID
	data.RoomID = ""
	data.DataType = ""
	data.StreamType = "pushRule"

	span, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(userID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}
