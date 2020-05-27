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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetRoomMembers{})
}

type ReqGetRoomMembers struct{}

func (ReqGetRoomMembers) GetRoute() string       { return "/rooms/{roomID}/members" }
func (ReqGetRoomMembers) GetMetricsName() string { return "rooms_members" }
func (ReqGetRoomMembers) GetMsgType() int32      { return internals.MSG_GET_ROOM_MEMBERS }
func (ReqGetRoomMembers) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomMembers) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomMembers) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomMembers) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetRoomMembers) NewRequest() core.Coder {
	return new(external.GetRoomMembersRequest)
}
func (ReqGetRoomMembers) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomMembersRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqGetRoomMembers) NewResponse(code int) core.Coder {
	return new(syncapitypes.MemberResponse)
}
func (ReqGetRoomMembers) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomMembersRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	roomID := req.RoomID
	userID := device.UserID

	state := c.rsTimeline.GetStateStreams(ctx, roomID)
	if state == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	} else {
		userId := userID
		rs := c.rsCurState.GetRoomState(roomID)
		if rs == nil {
			return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
		}

		_, isJoin := rs.GetJoinMap().Load(userId)
		val, isLeave := rs.GetLeaveMap().Load(userId)

		if isJoin == false && isLeave == false {
			return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
		}

		states := c.rsTimeline.GetStates(ctx, roomID)
		if states == nil {
			return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
		}

		resp := new(syncapitypes.MemberResponse)
		limit := int64(-1)
		if isLeave == true {
			limit = val.(int64)
		}

		cont := make(map[string]gomatrixserverlib.ClientEvent)
		var feeds []feedstypes.Feed
		states.ForRange(func(offset int, feed feedstypes.Feed) bool {
			if feed == nil {
				log.Errorf("RoomMemberRpcConsumer.processOnRoomMember get feed nil offset %d", offset)
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

			event := *stream.GetEv()
			if limit != -1 && stream.GetOffset() > limit {
				break
			}

			if event.Type == "m.room.member" {
				member := external.MemberContent{}
				if err := json.Unmarshal(event.Content, &member); err != nil {
					continue
				}
				cont[*event.StateKey] = event
			}
		}

		for _, ev := range cont {
			resp.Chunk = append(resp.Chunk, ev)
		}

		return http.StatusOK, resp
	}
}
