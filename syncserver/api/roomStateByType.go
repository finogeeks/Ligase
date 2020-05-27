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
	"github.com/finogeeks/ligase/common"
	"net/http"

	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetRoomStateByType{})
}

type ContentRawJSON []byte

func (r *ContentRawJSON) Encode() ([]byte, error) {
	return []byte(*r), nil
}

func (r *ContentRawJSON) Decode(input []byte) error {
	*r = *(*ContentRawJSON)(&input)
	return nil
}

type ReqGetRoomStateByType struct{}

func (ReqGetRoomStateByType) GetRoute() string       { return "/rooms/{roomID}/state/{type}" }
func (ReqGetRoomStateByType) GetMetricsName() string { return "room_state" }
func (ReqGetRoomStateByType) GetMsgType() int32      { return internals.MSG_GET_ROOM_EVENT_WITH_TYPE }
func (ReqGetRoomStateByType) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomStateByType) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomStateByType) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomStateByType) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetRoomStateByType) NewRequest() core.Coder {
	return new(external.GetRoomStateByTypeRequest)
}
func (ReqGetRoomStateByType) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomStateByTypeRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventType = vars["type"]
	}
	return nil
}
func (ReqGetRoomStateByType) NewResponse(code int) core.Coder {
	return make(internals.JSONMap)
}
func (ReqGetRoomStateByType) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomStateByTypeRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	roomID := req.RoomID
	evType := req.EventType
	stateKey := ""

	states := c.rsTimeline.GetStateStreams(ctx, roomID)
	if states != nil {
		rs := c.rsCurState.GetRoomState(roomID)
		if rs == nil {
			return http.StatusNotFound, jsonerror.NotFound("cannot find state")
		}

		stream := rs.GetState(evType, stateKey)
		if stream != nil {
			event := stream.GetEv()
			return http.StatusOK, (*ContentRawJSON)(&event.Content)
		}
	}

	return http.StatusNotFound, jsonerror.NotFound("cannot find state")
}
