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
	"fmt"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

// ClientEvent is an event which is fit for consumption by clients, in accordance with the specification.
type ClientEvent gomatrixserverlib.ClientEvent

func (r *ClientEvent) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ClientEvent) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

func init() {
	apiconsumer.SetServices("sync_aggregate_api")
	apiconsumer.SetAPIProcessor(ReqGetEventWithID{})
}

type ReqGetEventWithID struct{}

func (ReqGetEventWithID) GetRoute() string       { return "/events/{eventId}" }
func (ReqGetEventWithID) GetMetricsName() string { return "events" }
func (ReqGetEventWithID) GetMsgType() int32      { return internals.MSG_GET_EVENTS_WITH_ID }
func (ReqGetEventWithID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetEventWithID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetEventWithID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetEventWithID) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetEventWithID) NewRequest() core.Coder {
	return new(external.GetEventByIDRequest)
}
func (ReqGetEventWithID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetEventByIDRequest)
	if vars != nil {
		msg.EventID = vars["eventId"]
	}
	return nil
}
func (ReqGetEventWithID) NewResponse(code int) core.Coder {
	return new(ClientEvent)
}
func (ReqGetEventWithID) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetEventWithID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetEventByIDRequest)

	eventID := req.EventID
	userID := device.UserID

	//TODO 可见性校验
	event, _, err := c.db.StreamEvents(context.TODO(), []string{eventID})
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
	}

	if len(event) > 0 {
		roomID := event[0].RoomID
		isJoin, err := c.userTimeLine.CheckIsJoinRoom(userID, roomID)
		if err != nil {
			return http.StatusInternalServerError, jsonerror.NotFound(fmt.Sprintf("Could not find user joined rooms %s", userID))
		}

		if isJoin == false {
			return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room or just forget the room")
		}

		return http.StatusOK, (*ClientEvent)(&event[0])
	}

	return http.StatusNotFound, jsonerror.NotFound(fmt.Sprintf("Could not find event %s", eventID))
}
