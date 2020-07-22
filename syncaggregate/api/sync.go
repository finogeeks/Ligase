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
	"fmt"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"

	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetSync{})
}

type ReqGetSync struct{}

func (ReqGetSync) GetRoute() string                     { return "/sync" }
func (ReqGetSync) GetMetricsName() string               { return "sync" }
func (ReqGetSync) GetMsgType() int32                    { return internals.MSG_GET_SYNC }
func (ReqGetSync) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetSync) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetSync) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetSync) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetSync) NewRequest() core.Coder               { return new(external.GetSyncRequest) }
func (ReqGetSync) NewResponse(code int) core.Coder {
	return new(syncapitypes.Response)
}
func (ReqGetSync) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetSyncRequest)
	req.ParseForm()
	query := req.URL.Query()
	msg.TimeOut = query.Get("timeout")
	msg.FullState = query.Get("full_state")
	msg.SetPresence = query.Get("set_presence")
	msg.Filter = query.Get("filter")
	msg.From = query.Get("from")
	msg.Since = query.Get("since")
	return nil
}
func (ReqGetSync) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetSyncRequest)
	traceId, _ := c.idg.Next()
	httpReq := types.HttpReq{
		TimeOut:     req.TimeOut,
		FullState:   req.FullState,
		SetPresence: req.SetPresence,
		Filter:      req.Filter,
		From:        req.From,
		Since:       req.Since,
		TraceId:     fmt.Sprintf("%d", traceId),
	}

	return c.sm.OnSyncRequest(&httpReq, device)
}
