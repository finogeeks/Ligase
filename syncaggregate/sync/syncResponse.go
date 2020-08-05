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
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"sort"
)

func (sm *SyncMng) updateHasNewEvent(req *request, data *syncapitypes.SyncServerResponse){
	if req.hasNewEvent {
		return
	}
	for _, events := range data.Rooms.Join {
		if len(events.State.Events) > 0 {
			req.hasNewEvent = true
			return
		}
		if len(events.Timeline.Events) > 0 {
			req.hasNewEvent = true
			return
		}
	}
	for _, events := range data.Rooms.Invite {
		if len(events.InviteState.Events) > 0 {
			req.hasNewEvent = true
			return
		}
	}

	for _, events := range data.Rooms.Leave {
		if len(events.State.Events) > 0 {
			req.hasNewEvent = true
			return
		}
		if len(events.Timeline.Events) > 0 {
			req.hasNewEvent = true
			return
		}
	}
}

func (sm *SyncMng) freshToken(req *request,  res *syncapitypes.Response) {
	if !req.hasNewEvent {
		if req.isFullSync {
			sm.updateFullSyncNotData(req)
			return
		}
		req.marks.utlProcess = req.marks.utlRecv
		log.Infof("traceid:%s after sync has no new event userId:%s device:%s utl:%d token not change isfullsync:%t", req.traceId, req.device.UserID, req.device.Identifier, req.marks.utlRecv, req.isFullSync)
		return
	}
	//maxroomoffset to large
	//log.Infof("traceid:%s freshToken maxroomoffset:%+v", req.traceId, req.MaxRoomOffset)
	offsets := make(map[string]int64)
	for roomId, offset := range req.MaxRoomOffset {
		offsets[roomId] = offset
	}
	for k ,v := range req.offsets {
		if _, ok := offsets[k]; !ok {
			offsets[k] = v
		}
	}
	utl, _ := sm.userTimeLine.Idg.Next()
	err := sm.userTimeLine.UpdateToken(req.device.UserID, req.device.ID, utl, offsets)
	if err != nil {
		sm.clearSyncData(res)
		req.marks.utlProcess = req.marks.utlRecv
		log.Infof("traceid:%s after sync update token err:%v reset token userId:%s device:%s utl:%d", req.traceId, err, req.device.UserID, req.device.Identifier, req.marks.utlProcess)
		return
	}
	req.marks.utlProcess = utl
	//offsets too large
	//log.Infof("traceid:%s after sync update token info userId:%s device:%s utl:%d offsets:%+v", req.traceId, req.device.UserID, req.device.Identifier, utl, offsets)
	return
}

func (sm *SyncMng) clearSyncData(res *syncapitypes.Response) {
	res.Rooms.Join = nil
	res.Rooms.Join = nil
	res.Rooms.Leave = nil
}

func (sm *SyncMng) updateFullSyncNotData(req *request){
	utl, _ := sm.userTimeLine.Idg.Next()
	roomOffset := make(map[string]int64)
	roomOffset[fmt.Sprintf("!default:%s", common.GetDomainByUserID(req.device.UserID))] = -1
	err := sm.userTimeLine.UpdateToken(req.device.UserID, req.device.ID,utl, roomOffset)
	if err != nil {
		req.marks.utlProcess = req.marks.utlRecv
		log.Infof("traceid:%s after full sync not data update token err:%v reset token userId:%s device:%s utl:%d ishuman:%t", req.traceId, err, req.device.UserID, req.device.ID, req.marks.utlProcess, req.device.IsHuman)
		return
	}
	req.marks.utlProcess = utl
	log.Infof("traceid:%s after full sync not data update token userId:%s device:%s utl:%d ishuman:%t", req.traceId, req.device.UserID, req.device.ID, req.marks.utlProcess, req.device.IsHuman)
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
	for roomId, offset := range data.MaxRoomOffset {
		req.MaxRoomOffset[roomId] = offset
	}
	sm.updateHasNewEvent(req,data)
	if req.device.IsHuman {
		log.Infof("traceid:%s addSyncData user:%s recpProcess:%d maxReceiptOffset:%d", req.traceId, req.device.UserID, req.marks.recpProcess, data.MaxReceiptOffset)
		if req.marks.recpProcess < data.MaxReceiptOffset {
			req.marks.recpProcess = data.MaxReceiptOffset
		}
		if data.NewUsers != nil { //初次加入房间，把房间成员的presence信息带回去以更新头像昵称 TODO 去重
			for _, user := range data.NewUsers {
				if user == req.device.UserID {
					continue
				}
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
	for _, join := range res.Rooms.Join {
		sort.Sort(syncapitypes.ClientEvents(join.State.Events))
		sort.Sort(syncapitypes.ClientEvents(join.Timeline.Events))
	}
	for _, invite := range res.Rooms.Invite {
		sort.Sort(syncapitypes.ClientEvents(invite.InviteState.Events))
	}
	for _, leave := range res.Rooms.Leave {
		sort.Sort(syncapitypes.ClientEvents(leave.State.Events))
		sort.Sort(syncapitypes.ClientEvents(leave.Timeline.Events))
	}
	log.Infof("SyncMng FillSortEventOffset traceid:%s, next:%s", req.traceId, res.NextBatch)
}
