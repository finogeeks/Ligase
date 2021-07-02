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

package api

import (
	"context"
	"net/http"

	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/pushapi/routing"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer
	pushDB       model.PushAPIDatabase
	redisCache   service.Cache
	localcache   *cache.LocalCacheRepo
	pushDataRepo *repos.PushDataRepo
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	pushDB model.PushAPIDatabase,
	redisCache service.Cache,
	rpcClient rpc.RpcClient,
	pushDataRepo *repos.PushDataRepo,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcClient = rpcClient
	c.pushDB = pushDB
	c.redisCache = redisCache

	c.localcache = new(cache.LocalCacheRepo)
	c.localcache.Start(1, cfg.Cache.DurationDefault)
	c.pushDataRepo = pushDataRepo
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("pushapi", c, c.Cfg.Rpc.ProxyPushApiTopic, &c.Cfg.Rpc.SyncServerPushApi)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyPushApiTopic
}

func init() {
	apiconsumer.SetServices("sync_server_push_api")
	apiconsumer.SetAPIProcessor(ReqGetPushers{})
	apiconsumer.SetAPIProcessor(ReqPostSetPushers{})
	apiconsumer.SetAPIProcessor(ReqGetPushRules{})
	apiconsumer.SetAPIProcessor(ReqGetPushRulesGlobal{})
	apiconsumer.SetAPIProcessor(ReqGetPushRuleByID{})
	apiconsumer.SetAPIProcessor(ReqPutPushRuleByID{})
	apiconsumer.SetAPIProcessor(ReqDelPushRuleByID{})
	apiconsumer.SetAPIProcessor(ReqGetPushRulesEnabledByID{})
	apiconsumer.SetAPIProcessor(ReqPutPushRulesEnabledByID{})
	apiconsumer.SetAPIProcessor(ReqGetPushRuleActionsByID{})
	apiconsumer.SetAPIProcessor(ReqPutPushRulesActionsByID{})
	apiconsumer.SetAPIProcessor(ReqPostUsersPushKey{})
}

type ReqGetPushers struct{}

func (ReqGetPushers) GetRoute() string       { return "/pushers" }
func (ReqGetPushers) GetMetricsName() string { return "get_pushers" }
func (ReqGetPushers) GetMsgType() int32      { return internals.MSG_GET_PUSHERS }
func (ReqGetPushers) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPushers) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPushers) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPushers) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPushers) NewRequest() core.Coder {
	return nil
}
func (ReqGetPushers) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetPushers) NewResponse(code int) core.Coder {
	return new(pushapitypes.PushersWitchInterfaceData)
}
func (ReqGetPushers) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetPushers) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	return routing.GetPushers(
		device.UserID, c.pushDataRepo,
	)
}

type ReqPostSetPushers struct{}

func (ReqPostSetPushers) GetRoute() string       { return "/pushers/set" }
func (ReqPostSetPushers) GetMetricsName() string { return "set_pusher" }
func (ReqPostSetPushers) GetMsgType() int32      { return internals.MSG_POST_PUSHERS }
func (ReqPostSetPushers) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostSetPushers) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostSetPushers) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostSetPushers) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPostSetPushers) NewRequest() core.Coder {
	return new(external.PostSetPushersRequest)
}
func (ReqPostSetPushers) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostSetPushersRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostSetPushers) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPostSetPushers) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqPostSetPushers) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.PostSetPushersRequest)
	return routing.PutPusher(
		context.Background(), req, c.pushDB, c.pushDataRepo, device,
	)
}

type ReqGetPushRules struct{}

func (ReqGetPushRules) GetRoute() string       { return "/pushrules/" }
func (ReqGetPushRules) GetMetricsName() string { return "get_push_rules" }
func (ReqGetPushRules) GetMsgType() int32      { return internals.MSG_GET_PUSHRULES }
func (ReqGetPushRules) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPushRules) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPushRules) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPushRules) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPushRules) NewRequest() core.Coder {
	return nil
}
func (ReqGetPushRules) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetPushRules) NewResponse(code int) core.Coder {
	return new(pushapitypes.GlobalRule)
}
func (ReqGetPushRules) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetPushRules) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	return routing.GetPushRules(
		device, c.pushDataRepo,
	)
}

type ReqGetPushRulesGlobal struct{}

func (ReqGetPushRulesGlobal) GetRoute() string       { return "/pushrules/global/" }
func (ReqGetPushRulesGlobal) GetMetricsName() string { return "get_push_rules" }
func (ReqGetPushRulesGlobal) GetMsgType() int32      { return internals.MSG_GET_PUSHRULES_GLOBAL }
func (ReqGetPushRulesGlobal) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPushRulesGlobal) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPushRulesGlobal) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPushRulesGlobal) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPushRulesGlobal) NewRequest() core.Coder {
	return nil
}
func (ReqGetPushRulesGlobal) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetPushRulesGlobal) NewResponse(code int) core.Coder {
	return new(pushapitypes.Rules)
}
func (ReqGetPushRulesGlobal) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetPushRulesGlobal) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	return routing.GetPushRulesGlobal(
		device, c.pushDataRepo,
	)
}

type ReqGetPushRuleByID struct{}

func (ReqGetPushRuleByID) GetRoute() string       { return "/pushrules/{scope}/{kind}/{ruleId}" }
func (ReqGetPushRuleByID) GetMetricsName() string { return "get_push_rule" }
func (ReqGetPushRuleByID) GetMsgType() int32      { return internals.MSG_GET_PUSHRULES_BY_ID }
func (ReqGetPushRuleByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPushRuleByID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPushRuleByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPushRuleByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPushRuleByID) NewRequest() core.Coder {
	return new(external.GetPushrulesByIDRequest)
}
func (ReqGetPushRuleByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetPushrulesByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
	}
	return nil
}
func (ReqGetPushRuleByID) NewResponse(code int) core.Coder {
	return make(internals.JSONMap)
}
func (ReqGetPushRuleByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetPushRuleByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetPushrulesByIDRequest)
	return routing.GetPushRule(
		context.Background(), device, req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqPutPushRuleByID struct{}

func (ReqPutPushRuleByID) GetRoute() string       { return "/pushrules/{scope}/{kind}/{ruleId}" }
func (ReqPutPushRuleByID) GetMetricsName() string { return "set_push_rule" }
func (ReqPutPushRuleByID) GetMsgType() int32      { return internals.MSG_PUT_PUSHRULES_BY_ID }
func (ReqPutPushRuleByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutPushRuleByID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutPushRuleByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutPushRuleByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPutPushRuleByID) NewRequest() core.Coder {
	return new(external.PutPushrulesByIDRequest)
}
func (ReqPutPushRuleByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutPushrulesByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
		msg.Before = req.URL.Query().Get("before")
		msg.After = req.URL.Query().Get("after")
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutPushRuleByID) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPutPushRuleByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqPutPushRuleByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.PutPushrulesByIDRequest)
	return routing.PutPushRule(
		context.Background(), req, device, c.Cfg, req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqDelPushRuleByID struct{}

func (ReqDelPushRuleByID) GetRoute() string       { return "/pushrules/{scope}/{kind}/{ruleId}" }
func (ReqDelPushRuleByID) GetMetricsName() string { return "delete_push_rule" }
func (ReqDelPushRuleByID) GetMsgType() int32      { return internals.MSG_DEL_PUSHRULES_BY_ID }
func (ReqDelPushRuleByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqDelPushRuleByID) GetMethod() []string {
	return []string{http.MethodDelete, http.MethodOptions}
}
func (ReqDelPushRuleByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqDelPushRuleByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqDelPushRuleByID) NewRequest() core.Coder {
	return new(external.DelPushrulesByIDRequest)
}
func (ReqDelPushRuleByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.DelPushrulesByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqDelPushRuleByID) NewResponse(code int) core.Coder {
	return nil
}
func (ReqDelPushRuleByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqDelPushRuleByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.DelPushrulesByIDRequest)
	return routing.DeletePushRule(
		context.Background(), c.pushDB, device, c.Cfg, req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqGetPushRulesEnabledByID struct{}

func (ReqGetPushRulesEnabledByID) GetRoute() string {
	return "/pushrules/{scope}/{kind}/{ruleId}/enabled"
}
func (ReqGetPushRulesEnabledByID) GetMetricsName() string { return "get_push_rule_enabled" }
func (ReqGetPushRulesEnabledByID) GetMsgType() int32      { return internals.MSG_GET_PUSHRULES_ENABLED }
func (ReqGetPushRulesEnabledByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPushRulesEnabledByID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPushRulesEnabledByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPushRulesEnabledByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPushRulesEnabledByID) NewRequest() core.Coder {
	return new(external.GetPushrulesEnabledByIDRequest)
}
func (ReqGetPushRulesEnabledByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetPushrulesEnabledByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
	}
	return nil
}
func (ReqGetPushRulesEnabledByID) NewResponse(code int) core.Coder {
	return new(pushapitypes.EnabledType)
}
func (ReqGetPushRulesEnabledByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetPushRulesEnabledByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetPushrulesEnabledByIDRequest)
	return routing.GetPushRuleEnabled(
		context.Background(), device, req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqPutPushRulesEnabledByID struct{}

func (ReqPutPushRulesEnabledByID) GetRoute() string {
	return "/pushrules/{scope}/{kind}/{ruleId}/enabled"
}
func (ReqPutPushRulesEnabledByID) GetMetricsName() string { return "put_push_rule_enabled" }
func (ReqPutPushRulesEnabledByID) GetMsgType() int32      { return internals.MSG_PUT_PUSHRULES_ENABLED }
func (ReqPutPushRulesEnabledByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutPushRulesEnabledByID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutPushRulesEnabledByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutPushRulesEnabledByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPutPushRulesEnabledByID) NewRequest() core.Coder {
	return new(external.PutPushrulesEnabledByIDRequest)
}
func (ReqPutPushRulesEnabledByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutPushrulesEnabledByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutPushRulesEnabledByID) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPutPushRulesEnabledByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqPutPushRulesEnabledByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.PutPushrulesEnabledByIDRequest)
	return routing.PutPushRuleEnabled(
		context.Background(), req, device, c.Cfg,
		req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqGetPushRuleActionsByID struct{}

func (ReqGetPushRuleActionsByID) GetRoute() string {
	return "/pushrules/{scope}/{kind}/{ruleId}/actions"
}
func (ReqGetPushRuleActionsByID) GetMetricsName() string { return "get_push_rule_actions" }
func (ReqGetPushRuleActionsByID) GetMsgType() int32      { return internals.MSG_GET_PUSHRULES_ACTIONS }
func (ReqGetPushRuleActionsByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPushRuleActionsByID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPushRuleActionsByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPushRuleActionsByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPushRuleActionsByID) NewRequest() core.Coder {
	return new(external.GetPushrulesActionsByIDRequest)
}
func (ReqGetPushRuleActionsByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetPushrulesActionsByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
	}
	return nil
}
func (ReqGetPushRuleActionsByID) NewResponse(code int) core.Coder {
	return new(pushapitypes.PushActions)
}
func (ReqGetPushRuleActionsByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetPushRuleActionsByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetPushrulesActionsByIDRequest)
	return routing.GetPushRuleActions(
		context.Background(), device, req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqPutPushRulesActionsByID struct{}

func (ReqPutPushRulesActionsByID) GetRoute() string {
	return "/pushrules/{scope}/{kind}/{ruleId}/actions"
}
func (ReqPutPushRulesActionsByID) GetMetricsName() string { return "put_push_rule_actions" }
func (ReqPutPushRulesActionsByID) GetMsgType() int32      { return internals.MSG_PUT_PUSHRULES_ACTIONS }
func (ReqPutPushRulesActionsByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutPushRulesActionsByID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutPushRulesActionsByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutPushRulesActionsByID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPutPushRulesActionsByID) NewRequest() core.Coder {
	return new(external.PutPushrulesActionsByIDRequest)
}
func (ReqPutPushRulesActionsByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutPushrulesActionsByIDRequest)
	if vars != nil {
		msg.Scope = vars["scope"]
		msg.Kind = vars["kind"]
		msg.RuleID = vars["ruleId"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutPushRulesActionsByID) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPutPushRulesActionsByID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqPutPushRulesActionsByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.PutPushrulesActionsByIDRequest)
	return routing.PutPushRuleActions(
		context.Background(), req, device, c.Cfg, req.Scope, req.Kind, req.RuleID, c.pushDataRepo,
	)
}

type ReqPostUsersPushKey struct{}

func (ReqPostUsersPushKey) GetRoute() string       { return "/users/pushkey" }
func (ReqPostUsersPushKey) GetMetricsName() string { return "get_push_key" }
func (ReqPostUsersPushKey) GetMsgType() int32      { return internals.MSG_POST_USERS_PUSH_KEY }
func (ReqPostUsersPushKey) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqPostUsersPushKey) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostUsersPushKey) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostUsersPushKey) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPostUsersPushKey) NewRequest() core.Coder {
	return new(external.PostUsersPushKeyRequest)
}
func (ReqPostUsersPushKey) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostUsersPushKeyRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostUsersPushKey) NewResponse(code int) core.Coder {
	return new(pushapitypes.PushersRes)
}
func (ReqPostUsersPushKey) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.PostUsersPushKeyRequest)
	if req.Users != nil && len(req.Users) > 0 {
		return []uint32{common.CalcStringHashCode(req.Users[0]) % cfg.MultiInstance.Total}
	} else {
		return []uint32{}
	}
}
func (ReqPostUsersPushKey) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostUsersPushKeyRequest)
	if req.Users != nil && len(req.Users) > 0 {
		if !common.IsRelatedRequest(req.Users[0], c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
			return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
		} else {
			return routing.GetUsersPushers(
				context.Background(), req, c.pushDataRepo, c.Cfg, c.RpcClient,
			)
		}
	} else {
		pushersRes := pushapitypes.PushersRes{}
		pushers := []pushapitypes.PusherRes{}
		pushersRes.PushersRes = pushers
		return http.StatusOK, &pushersRes
	}
}
