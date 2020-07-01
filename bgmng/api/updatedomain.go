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
	"github.com/finogeeks/ligase/bgmng/routing"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"net/http"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqPostUpdateDomain{})
	apiconsumer.SetAPIProcessor(ReqGetDomain{})
}

//desktop now is use APITypeExternal, change to APITypeInternalAuth is better

type ReqPostUpdateDomain struct{}

func (ReqPostUpdateDomain) GetRoute() string       { return "/conf/servername" }
func (ReqPostUpdateDomain) GetMetricsName() string { return "update servername" }
func (ReqPostUpdateDomain) GetMsgType() int32      { return internals.MSG_POST_SERVERNAME }
func (ReqPostUpdateDomain) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqPostUpdateDomain) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostUpdateDomain) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostUpdateDomain) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostUpdateDomain) NewRequest() core.Coder {
	return new(external.ServerNameCfgRequest)
}
func (ReqPostUpdateDomain) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.ServerNameCfgRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostUpdateDomain) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPostUpdateDomain) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.ServerNameCfgRequest)
	return routing.OnUpdateServerNameCfg(req, c.db, c.cache, c.idg, c.Cfg)
}

type ReqGetDomain struct{}

func (ReqGetDomain) GetRoute() string       { return "/conf/servername" }
func (ReqGetDomain) GetMetricsName() string { return "update servername" }
func (ReqGetDomain) GetMsgType() int32      { return internals.MSG_GET_SERVERNAME }
func (ReqGetDomain) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetDomain) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetDomain) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetDomain) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqGetDomain) NewRequest() core.Coder {
	return nil
}
func (ReqGetDomain) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetDomain) NewResponse(code int) core.Coder {
	return new(external.GetServerNamesResponse)
}
func (ReqGetDomain) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	return routing.OnGetServerNameCfg(c.db, c.cache, c.Cfg)
}
