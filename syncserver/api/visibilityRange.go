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
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetVisibilityRange{})
}

type userVisibility struct {
	UserID     string            `json:"user_id"`
	Visibility []repos.RangeItem `json:"visibility"`
}

type RoomVisibilityRange struct {
	RoomID           string           `json:"room_id"`
	VisibilityRanges []userVisibility `json:"visibility_ranges"`
}

func (r *RoomVisibilityRange) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *RoomVisibilityRange) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetVisibilityRange struct{}

func (ReqGetVisibilityRange) GetRoute() string       { return "/rooms/{roomID}/visibility_range" }
func (ReqGetVisibilityRange) GetMetricsName() string { return "visibility_range" }
func (ReqGetVisibilityRange) GetMsgType() int32      { return internals.MSG_GET_VISIBILITY_RANGE }
func (ReqGetVisibilityRange) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetVisibilityRange) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetVisibilityRange) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetVisibilityRange) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetVisibilityRange) NewRequest() core.Coder {
	return new(external.GetRoomVisibilityRangeRequest)
}
func (ReqGetVisibilityRange) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomVisibilityRangeRequest)
	req.ParseForm()
	msg.UserID = req.URL.Query().Get("userID")
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqGetVisibilityRange) NewResponse(code int) core.Coder {
	return new(RoomVisibilityRange)
}
func (ReqGetVisibilityRange) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomVisibilityRangeRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	userID := req.UserID
	roomID := req.RoomID

	states := c.rsTimeline.GetStateStreams(ctx, roomID)
	if states == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	rs := c.rsCurState.GetRoomState(roomID)
	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find visible range of the specified room")
	}
	users := make(map[string]bool)
	if len(userID) == 0 {
		joinMap := rs.GetJoinMap()
		if joinMap != nil {
			joinMap.Range(func(key, value interface{}) bool {
				users[key.(string)] = true
				return true
			})
		}
		// if len(users) == 0 { // return not found } // no one cares an empty room
	} else {
		_, isJoin := rs.GetJoinMap().Load(userID)
		if isJoin == false {
			return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
		}
		users[userID] = true
	}

	uv := userVisibility{}
	vr := &RoomVisibilityRange{RoomID: roomID}
	for u := range users {
		uv.UserID = u
		uv.Visibility = rs.GetEventVisibility(u)
		vr.VisibilityRanges = append(vr.VisibilityRanges, uv)
	}

	return http.StatusOK, vr
}
