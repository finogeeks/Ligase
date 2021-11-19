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
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncserver/extra"
)

func init() {
	apiconsumer.SetServices("sync_server_api")
	apiconsumer.SetAPIProcessor(ReqGetRoomHistory{})
}

type ReqGetRoomHistory struct{}

func (ReqGetRoomHistory) GetRoute() string       { return "/rooms/{roomID}/history" }
func (ReqGetRoomHistory) GetMetricsName() string { return "room_messages" }
func (ReqGetRoomHistory) GetMsgType() int32      { return internals.MSG_GET_ROOM_HISTORY }
func (ReqGetRoomHistory) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomHistory) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomHistory) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomHistory) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetRoomHistory) NewRequest() core.Coder {
	return new(external.GetRoomHistoryRequest)
}
func (ReqGetRoomHistory) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomHistoryRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	req.ParseForm()
	values := req.URL.Query()
	msg.Filter = values.Get("filter")
	page := values.Get("page")
	size := values.Get("size")
	var err error
	msg.Page, err = strconv.Atoi(page)
	if err != nil {
		return err
	}
	msg.Size, err = strconv.Atoi(size)
	if err != nil {
		return err
	}
	if msg.Page <= 0 {
		return errors.New("invalid page " + page)
	}
	if msg.Size <= 0 {
		return errors.New("invalid size " + size)
	}

	return nil
}
func (ReqGetRoomHistory) NewResponse(code int) core.Coder {
	return new(syncapitypes.RoomHistoryResp)
}
func (ReqGetRoomHistory) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetRoomHistoryRequest)
	return []uint32{common.CalcStringHashCode(req.RoomID) % cfg.MultiInstance.SyncServerTotal}
}
func (r ReqGetRoomHistory) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomHistoryRequest)

	userID := device.UserID
	roomID := req.RoomID

	states := c.rsTimeline.GetStateStreams(roomID)
	if states == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	userId := userID
	rs := c.rsCurState.GetRoomState(roomID)

	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	_, isJoin := rs.GetJoinMap().Load(userId)
	_, isLeave := rs.GetLeaveMap().Load(userId)

	if isJoin == false && isLeave == false {
		return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room or just forget the room")
	}

	filterStr := req.Filter
	var evFilter gomatrix.FilterPart
	if filterStr != "" {
		json.Unmarshal([]byte(filterStr), &evFilter)
		if strings.Contains(filterStr, "contains_url") == false {
			evFilter.ContainsURL = true
		}
	} else {
		evFilter.ContainsURL = true
	}

	tl := c.rmHsTimeline.GetHistory(roomID)
	if tl == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room history")
	}

	log.Infof("OnRoomHistoryRequest user:%s filter: %s", userId, filterStr)

	ranges := rs.GetEventVisibility(userID)
	rangeItems := make([]types.RangeItem, len(ranges))
	for i, v := range ranges {
		rangeItems[i] = types.RangeItem{
			Start: v.Start,
			End:   v.End,
		}
	}

	ctx := context.TODO()
	outputRoomEvents, err := r.selectFromDB(ctx, c, rs, userID, roomID, rangeItems, req.Size, (req.Page-1)*req.Size)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
	}
	if outputRoomEvents == nil {
		outputRoomEvents = []gomatrixserverlib.ClientEvent{}
	}
	count, err := c.db.SelectEventCountByRanges(ctx, roomID, rangeItems)
	if err != nil {
		log.Warnf("cannot count event, roomID: %s", roomID)
		return http.StatusInternalServerError, jsonerror.Unknown("count event error")
	}

	resp := new(syncapitypes.RoomHistoryResp)
	resp.Page = req.Page
	resp.Total = int(count)

	if c.Cfg.UseMessageFilter {
		resp.Chunk = *common.FilterEventTypes(&outputRoomEvents, &evFilter.Types, &evFilter.NotTypes)
	} else {
		resp.Chunk = outputRoomEvents
	}
	for i := 0; i < len(resp.Chunk)/2; i++ {
		resp.Chunk[i], resp.Chunk[len(resp.Chunk)-1-i] = resp.Chunk[len(resp.Chunk)-1-i], resp.Chunk[i]
	}
	resp.Size = len(resp.Chunk)

	return http.StatusOK, resp
}

func (r ReqGetRoomHistory) selectFromDB(
	ctx context.Context,
	c *InternalMsgConsumer,
	rs *repos.RoomState,
	userId, roomID string,
	rangeItems []types.RangeItem,
	limit int, offset int,
) (outputRoomEvents []gomatrixserverlib.ClientEvent, err error) {
	c.rsTimeline.QueryHitCounter.WithLabelValues("db", "RoomMessages", "getEvents").Inc()

	events, err := c.db.SelectEventHistoryByRanges(ctx, roomID, rangeItems, limit, offset)
	if err != nil {
		return
	}

	// visibilityTime := c.GetCfg().Sync.Visibility
	// nowTs := time.Now().Unix()

	for idx := range events {
		// isState := common.IsStateClientEv(&events[idx])
		// if !isState && !rs.CheckEventVisibility(userId, int64(events[idx].OriginServerTS)) {
		// 	log.Infof("getHistory select from db check visibility false %s %s ts:%d", roomID, events[idx].EventID, events[idx].OriginServerTS)
		// 	continue
		// }
		// if visibilityTime > 0 {
		// 	ts := int64(events[idx].OriginServerTS) / 1000
		// 	if ts+visibilityTime < nowTs && !isState {
		// 		log.Infof("getHistory select from db skip event %s, ts: %d", events[idx].EventID, events[idx].OriginServerTS)
		// 		continue
		// 	}
		// }

		extra.ExpandMessages(&events[idx], userId, c.rsCurState, c.displayNameRepo)
		outputRoomEvents = append(outputRoomEvents, events[idx])
	}
	return
}
