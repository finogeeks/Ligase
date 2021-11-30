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
	"net/http"
	"strconv"

	"github.com/finogeeks/ligase/common"
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
	apiconsumer.SetServices("sync_server_api")
	apiconsumer.SetAPIProcessor(ReqGetRoomInitialSync{})
}

type ReqGetRoomInitialSync struct{}

func (ReqGetRoomInitialSync) GetRoute() string       { return "/rooms/{roomID}/initialSync" }
func (ReqGetRoomInitialSync) GetMetricsName() string { return "rooms_initial_sync" }
func (ReqGetRoomInitialSync) GetMsgType() int32      { return internals.MSG_GET_ROOM_INITIAL_SYNC }
func (ReqGetRoomInitialSync) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomInitialSync) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomInitialSync) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomInitialSync) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetRoomInitialSync) NewRequest() core.Coder {
	return new(external.GetRoomInitialSyncRequest)
}
func (ReqGetRoomInitialSync) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomInitialSyncRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqGetRoomInitialSync) NewResponse(code int) core.Coder {
	return new(syncapitypes.RoomInfo)
}
func (ReqGetRoomInitialSync) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetRoomInitialSyncRequest)
	return []uint32{common.CalcStringHashCode(req.RoomID) % cfg.MultiInstance.SyncServerTotal}
}
func (ReqGetRoomInitialSync) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomInitialSyncRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.SyncServerTotal, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	roomID := req.RoomID
	userID := device.UserID

	states := c.rsTimeline.GetStateStreams(roomID)
	if states == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}
	userId := userID
	rs := c.rsCurState.GetRoomState(roomID)
	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	val, isLeave := rs.GetLeaveMap().Load(userId)

	roominfo := new(syncapitypes.RoomInfo)
	roominfo.RoomID = roomID
	limitOffset := int64(-1)
	if isLeave == true {
		limitOffset = val.(int64)
	}

	tl := c.rsTimeline.GetStates(roomID)
	if tl == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	var feeds []feedstypes.Feed
	tl.ForRange(func(offset int, feed feedstypes.Feed) bool {
		if feed == nil {
			log.Errorf("RoomInitSyncRpcConsumer.processOnRoomInitSync get feed nil offset %d", offset)
			tl.Console()
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

		if limitOffset != -1 && stream.GetOffset() > limitOffset {
			break
		}

		ev := stream.GetEv()
		if ev.Type == "m.room.member" && *ev.StateKey == userId {
			member := external.MemberContent{}
			json.Unmarshal(ev.Content, &member)
			roominfo.Membership = member.Membership
		}

		if rs.CheckEventVisibility(userId, stream.GetOffset()) {
			roominfo.State = append(roominfo.State, *ev)
		}

		if ev.Type == "m.room.visibility" {
			vi := common.VisibilityContent{}
			json.Unmarshal(ev.Content, &vi)
			roominfo.Visibility = vi.Visibility
		}
	}

	tl = c.rmHsTimeline.GetHistory(roomID)
	if tl == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room history")
	}

	feeds = []feedstypes.Feed{}
	tl.ForRange(func(offset int, feed feedstypes.Feed) bool {
		if feed == nil {
			log.Errorf("RoomInitSyncRpcConsumer.processOnRoomInitSync get feed nil offset %d", offset)
			tl.Console()
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

		if limitOffset != -1 && stream.GetOffset() > limitOffset {
			break
		}

		ev := stream.GetEv()
		if common.IsStateEventType(ev.Type) == false && rs.CheckEventVisibility(userId, stream.GetOffset()) {
			if roominfo.Messages.Start == "" {
				roominfo.Messages.Start = strconv.FormatInt(stream.GetOffset(), 10)
			}
			roominfo.Messages.Chunk = append(roominfo.Messages.Chunk, *ev)
			roominfo.Messages.End = strconv.FormatInt(stream.GetOffset(), 10)
		}
	}

	//bytes, _ := json.Marshal(roominfo)
	//log.Errorf("OnRoomInitialSync res:%v", string(bytes))

	return http.StatusOK, roominfo
}
