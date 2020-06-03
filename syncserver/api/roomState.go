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
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetRoomState{})
}

type ReqGetRoomState struct{}

func (ReqGetRoomState) GetRoute() string       { return "/rooms/{roomID}/state" }
func (ReqGetRoomState) GetMetricsName() string { return "room_state" }
func (ReqGetRoomState) GetMsgType() int32      { return internals.MSG_GET_ROOM_STATE }
func (ReqGetRoomState) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomState) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomState) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomState) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetRoomState) NewRequest() core.Coder {
	return new(external.GetRoomStateRequest)
}
func (ReqGetRoomState) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomStateRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqGetRoomState) NewResponse(code int) core.Coder {
	return &internals.JSONArr{}
}
func (ReqGetRoomState) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomStateRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	roomID := req.RoomID
	userID := device.UserID

	state := c.rsTimeline.GetStateStreams(ctx, roomID)
	if state == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find state")
	}
	resp := syncapitypes.StateEventInStateResp{}
	rs := c.rsCurState.GetRoomState(roomID)
	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find state")
	}

	states := c.rsTimeline.GetStates(ctx, roomID)
	if states == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find state")
	}

	var feeds []feedstypes.Feed
	states.ForRange(func(offset int, feed feedstypes.Feed) bool {
		if feed == nil {
			log.Errorf("RoomStateRpcConsumer.processOnRoomState get feed nil offset %d", offset)
			states.Console()
		} else {
			feeds = append(feeds, feed)
		}
		return true
	})
	for _, feed := range feeds {
		if feed == nil {
			continue
		}

		stream := feed.(*feedstypes.StreamEvent)

		if rs.CheckEventVisibility(userID, stream.GetOffset()) {
			event := *stream.GetEv()
			stateEvent := syncapitypes.StateEventInState{
				ClientEvent: event,
			}

			var prevEventRef syncapitypes.PrevEventRef
			if len(event.Unsigned) > 0 {
				if err := json.Unmarshal(event.Unsigned, &prevEventRef); err != nil {
					return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
				}
				// Fills the previous state event ID if the state event replaces another
				// state event
				if len(prevEventRef.ReplacesState) > 0 {
					stateEvent.ReplacesState = prevEventRef.ReplacesState
				}
				// Fill the previous event if the state event references a previous event
				if prevEventRef.PrevContent != nil {
					stateEvent.PrevContent = prevEventRef.PrevContent
				}
			}

			resp = append(resp, stateEvent)
		}
	}

	return http.StatusOK, resp
}
