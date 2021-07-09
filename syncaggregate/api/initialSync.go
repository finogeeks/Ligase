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
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func init() {
	apiconsumer.SetServices("sync_aggregate_api")
	apiconsumer.SetAPIProcessor(ReqGetInitialSync{})
}

// Response represents a /initialSync API response
type ResponseInitialSync struct {
	End         string                          `json:"end"`
	Presence    []gomatrixserverlib.ClientEvent `json:"presence"`
	Rooms       []syncapitypes.RoomInfo         `json:"rooms"`
	AccountData []gomatrixserverlib.ClientEvent `json:"account_data"`
}

func (p *ResponseInitialSync) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *ResponseInitialSync) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type ReqGetInitialSync struct{}

func (ReqGetInitialSync) GetRoute() string       { return "/initialSync" }
func (ReqGetInitialSync) GetMetricsName() string { return "initial_sync" }
func (ReqGetInitialSync) GetMsgType() int32      { return internals.MSG_GET_INITIAL_SYNC }
func (ReqGetInitialSync) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetInitialSync) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetInitialSync) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetInitialSync) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetInitialSync) NewRequest() core.Coder {
	return new(external.GetInitialSyncRequest)
}
func (ReqGetInitialSync) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetInitialSync) NewResponse(code int) core.Coder {
	return new(ResponseInitialSync)
}
func (ReqGetInitialSync) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	return []uint32{common.CalcStringHashCode(device.UserID) % cfg.MultiInstance.Total}
}
func (ReqGetInitialSync) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetInitialSyncRequest)

	archived := req.Archived
	filter := new(gomatrix.Filter)
	if archived == true {
		filter.Room.IncludeLeave = true
	} else {
		filter.Room.IncludeLeave = false
	}
	filterBytes, _ := json.Marshal(filter)

	httpReq := types.HttpReq{
		TimeOut:     strconv.Itoa(req.Timeout),
		FullState:   req.FullState,
		SetPresence: req.SetPresence,
		Filter:      string(filterBytes),
		From:        req.From,
		Since:       req.Since,
	}

	code, syncData := c.sm.OnSyncRequest(&httpReq, device)

	resp := &ResponseInitialSync{}
	resp.End = syncData.NextBatch
	resp.Presence = syncData.Presence.Events
	resp.AccountData = syncData.AccountData.Events

	for roomID, ev := range syncData.Rooms.Join {
		roomInfo := syncapitypes.RoomInfo{}
		roomInfo.RoomID = roomID
		roomInfo.Membership = "join"
		roomInfo.Messages.Start = ev.Timeline.PrevBatch
		roomInfo.Messages.Chunk = ev.Timeline.Events
		roomInfo.State = ev.State.Events
		roomInfo.AccountData = ev.AccountData.Events
		resp.Rooms = append(resp.Rooms, roomInfo)
	}

	for roomID, ev := range syncData.Rooms.Invite {
		roomInfo := syncapitypes.RoomInfo{}
		roomInfo.RoomID = roomID
		roomInfo.Membership = "invite"
		roomInfo.State = ev.InviteState.Events
		resp.Rooms = append(resp.Rooms, roomInfo)
	}

	if archived == true {
		for roomID, ev := range syncData.Rooms.Leave {
			roomInfo := syncapitypes.RoomInfo{}
			roomInfo.RoomID = roomID
			roomInfo.Membership = "leave"
			roomInfo.State = ev.State.Events
			resp.Rooms = append(resp.Rooms, roomInfo)
		}
	}
	return code, resp
}
