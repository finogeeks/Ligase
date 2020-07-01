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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"
	"net/http"

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
	apiconsumer.SetAPIProcessor(ReqGetEvents{})
}

type ReqGetEvents struct{}

func (ReqGetEvents) GetRoute() string       { return "/events" }
func (ReqGetEvents) GetMetricsName() string { return "events" }
func (ReqGetEvents) GetMsgType() int32      { return internals.MSG_GET_EVENTS }
func (ReqGetEvents) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetEvents) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetEvents) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetEvents) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetEvents) NewRequest() core.Coder {
	return new(external.GetEventsRequest)
}
func (ReqGetEvents) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetEventsRequest)
	req.ParseForm()
	values := req.URL.Query()
	msg.TimeOut = values.Get("timeout")
	msg.From = values.Get("from")
	msg.FullState = values.Get("full_state")
	msg.SetPresence = values.Get("set_presence")
	msg.Since = values.Get("since")
	msg.Filter = values.Get("filter")
	return nil
}
func (ReqGetEvents) NewResponse(code int) core.Coder {
	return new(syncapitypes.PaginationChunk)
}
func (r ReqGetEvents) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetEventsRequest)

	from := req.From
	httpReq := types.HttpReq{
		TimeOut:     req.TimeOut,
		FullState:   req.FullState,
		SetPresence: req.SetPresence,
		Filter:      req.Filter,
		From:        from,
		Since:       req.Since,
	}

	code, syncData := c.sm.OnSyncRequest(&httpReq, device)

	resp := new(syncapitypes.PaginationChunk)
	resp.Start = from
	resp.End = syncData.NextBatch
	r.extractEvents(syncData, resp)
	return code, resp
}

func (ReqGetEvents) extractEvents(res *syncapitypes.Response, chunk *syncapitypes.PaginationChunk) {
	respMap := res.Rooms.Join
	for _, resp := range respMap {
		chunk.Chunk = append(chunk.Chunk, resp.State.Events...)
		chunk.Chunk = append(chunk.Chunk, resp.Timeline.Events...)
	}
}
