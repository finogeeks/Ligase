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

package sync

import (
	"context"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"sort"
)

func (sm *SyncMng) loadMissOffsetPairFromDb(req *request, loadOffsets []int64) {
	log.Warnf("traceid:%s getProcessUtl load offset from db", req.traceId)
	if loadOffsets == nil || len(loadOffsets) <= 0 {
		return
	}
	evs, err := sm.db.SelectUserTimeLineOffset(context.TODO(), req.device.UserID, loadOffsets)
	if err != nil {
		log.Errorf("traceid:%s SelectUserTimeLineOffset from db err:%v", req.traceId, err)
		return
	}
	if evs == nil || len(evs) <= 0 {
		return
	}
	for _, ev := range evs {
		sm.updateOffsetPair(req, ev.RoomID, ev.Offset, ev.RoomOffset)
	}
}

func (sm *SyncMng) getProcessUtl(req *request, data *syncapitypes.SyncServerResponse) (utl int64) {
	var userRoomOffset *UserRoomOffset
	if val, ok := sm.syncOffset.Load(req.traceId); ok {
		userRoomOffset = val.(*UserRoomOffset)
	}
	loadOffsets := []int64{}
	for roomId, offset := range data.MaxRoomOffset {
		if userRoomOffset != nil {
			if pair, ok := userRoomOffset.Offsets[roomId]; !ok {
				loadOffsets = append(loadOffsets, offset)
			} else {
				if _, ok := pair[offset]; !ok {
					loadOffsets = append(loadOffsets, offset)
				}
			}
		} else {
			loadOffsets = append(loadOffsets, offset)
		}
	}
	if len(loadOffsets) > 0 {
		sm.loadMissOffsetPairFromDb(req, loadOffsets)
	}
	if val, ok := sm.syncOffset.Load(req.traceId); ok {
		userRoomOffset = val.(*UserRoomOffset)
	}
	if userRoomOffset == nil {
		return req.marks.utlProcess
	}
	utlProcess := int64(0)
	utlRoomProcess := int64(0)
	hasMissOffset := false
	for roomId, offset := range data.MaxRoomOffset {
		if pair, ok := userRoomOffset.Offsets[roomId]; ok {
			if utl, ok := pair[offset]; ok {
				if utlProcess < utl {
					utlProcess = utl
				}
			} else {
				hasMissOffset = true
				log.Warnf("traceid:%d user:%s roomId:%s roomoffset:%d userRoomOffset miss offset pair", req.traceId, req.device.UserID, roomId, offset)
			}
		} else {
			hasMissOffset = true
			log.Warnf("traceid:%d user:%s roomId:%s roomoffset:%d userRoomOffset miss whole offsets", req.traceId, req.device.UserID, roomId, offset)
		}
		if utlRoomProcess < offset {
			utlRoomProcess = offset
		}
	}
	sm.syncOffset.Delete(req.traceId)
	if hasMissOffset {
		return utlRoomProcess
	} else {
		return utlProcess
	}
}

func (sm *SyncMng) addSyncData(req *request, res *syncapitypes.Response, data *syncapitypes.SyncServerResponse) bool {
	res.Lock.Lock()
	defer res.Lock.Unlock()

	if data.Rooms.Join != nil {
		for roomID, resp := range data.Rooms.Join {
			res.Rooms.Join[roomID] = resp
		}
	}

	if data.Rooms.Invite != nil {
		for roomID, resp := range data.Rooms.Invite {
			res.Rooms.Invite[roomID] = resp
		}
	}

	if req.marks.utlRecv == 0 {
		res.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
	} else {
		if data.Rooms.Leave != nil {
			for roomID, resp := range data.Rooms.Leave {
				res.Rooms.Leave[roomID] = resp
			}
		}
	}
	aggProcess := req.marks.utlProcess
	req.marks.utlProcess = sm.getProcessUtl(req, data)
	log.Infof("update utlProcess addSyncData traceid:%s userid:%s process:%d aggprocess:%d", req.traceId, req.device.UserID, req.marks.utlProcess, aggProcess)

	if req.device.IsHuman {
		if req.marks.recpProcess < data.MaxReceiptOffset {
			req.marks.recpProcess = data.MaxReceiptOffset
		}
		if data.NewUsers != nil { //初次加入房间，把房间成员的presence信息带回去以更新头像昵称 TODO 去重
			for _, user := range data.NewUsers {
				feed := sm.presenceStreamRepo.GetHistoryByUserID(user)
				if feed == nil {
					continue
				}
				var presenceEvent gomatrixserverlib.ClientEvent
				err := json.Unmarshal(feed.DataStream.Content, &presenceEvent)
				if err != nil {
					log.Errorw("addReceipt: Unmarshal json error for presence", log.KeysAndValues{
						"userID", user, "device", req.device.ID, "eventJson", string(feed.DataStream.Content), "error", err,
					})
					return false
				}
				res.Presence.Events = append(res.Presence.Events, presenceEvent)
				log.Infof("add presence new user %s %d %s", req.device.UserID, feed.GetOffset(), feed.DataStream.Content)
			}
		}
	}
	return true
}

func (sm *SyncMng) filterSyncData(req *request, res *syncapitypes.Response) *syncapitypes.Response {
	if req.filter != nil {
		for _, roomID := range req.filter.Room.NotRooms {
			delete(res.Rooms.Invite, roomID)
			delete(res.Rooms.Join, roomID)
			delete(res.Rooms.Leave, roomID)
			log.Infof("del NotRooms roomId:%s traceid:%s by filter", roomID, req.traceId)
		}
		if (req.marks.utlRecv == 0) && (req.filter.Room.IncludeLeave == false) {
			for roomID := range res.Rooms.Leave {
				//log.Debugf("-----------------del room %s", roomID)
				delete(res.Rooms.Leave, roomID)
				log.Infof("del not IncludeLeave roomId:%s traceid:%s by filter", roomID, req.traceId)
			}
		}
		for roomID, joinResponse := range res.Rooms.Join {
			joinResponse.State.Events = *common.FilterEventTypes(&joinResponse.State.Events, &req.filter.Room.State.Types, &req.filter.Room.State.NotTypes)
			joinResponse.Timeline.Events = *common.FilterEventTypes(&joinResponse.Timeline.Events, &req.filter.Room.Timeline.Types, &req.filter.Room.Timeline.NotTypes)
			res.Rooms.Join[roomID] = joinResponse
		}
		for roomID, inviteResponse := range res.Rooms.Invite {
			inviteResponse.InviteState.Events = *common.FilterEventTypes(&inviteResponse.InviteState.Events, &req.filter.Room.State.Types, &req.filter.Room.State.NotTypes)
			res.Rooms.Invite[roomID] = inviteResponse
		}
	}
	return res
}

func (sm *SyncMng) FillSortEventOffset(res *syncapitypes.Response, req *request) {
	maxOffset := int64(-1)
	for _, join := range res.Rooms.Join {
		sort.Sort(syncapitypes.ClientEvents(join.State.Events))
		if len(join.State.Events) > 0 {
			if maxOffset < join.State.Events[len(join.State.Events)-1].EventOffset {
				maxOffset = join.State.Events[len(join.State.Events)-1].EventOffset
			}
		}
		sort.Sort(syncapitypes.ClientEvents(join.Timeline.Events))
		if len(join.Timeline.Events) > 0 {
			if maxOffset < join.Timeline.Events[len(join.Timeline.Events)-1].EventOffset {
				maxOffset = join.Timeline.Events[len(join.Timeline.Events)-1].EventOffset
			}
		}
	}
	for _, invite := range res.Rooms.Invite {
		sort.Sort(syncapitypes.ClientEvents(invite.InviteState.Events))
		if len(invite.InviteState.Events) > 0 {
			if maxOffset < invite.InviteState.Events[len(invite.InviteState.Events)-1].EventOffset {
				maxOffset = invite.InviteState.Events[len(invite.InviteState.Events)-1].EventOffset
			}
		}
	}
	for _, leave := range res.Rooms.Leave {
		sort.Sort(syncapitypes.ClientEvents(leave.State.Events))
		if len(leave.State.Events) > 0 {
			if maxOffset < leave.State.Events[len(leave.State.Events)-1].EventOffset {
				maxOffset = leave.State.Events[len(leave.State.Events)-1].EventOffset
			}
		}
		sort.Sort(syncapitypes.ClientEvents(leave.Timeline.Events))
		if len(leave.Timeline.Events) > 0 {
			if maxOffset < leave.Timeline.Events[len(leave.Timeline.Events)-1].EventOffset {
				maxOffset = leave.Timeline.Events[len(leave.Timeline.Events)-1].EventOffset
			}
		}
	}
	log.Infof("SyncMng FillSortEventOffset traceid:%s, maxoffset:%d, next:%s", req.traceId, maxOffset, res.NextBatch)
}
