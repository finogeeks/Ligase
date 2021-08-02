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

package helper

import (
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func ToSyncServerRequest(req *pb.SyncProcessReq) *syncapitypes.SyncServerRequest {
	request := &syncapitypes.SyncServerRequest{
		RequestType:      req.RequestType,
		JoinedRooms:      req.JoinedRooms,
		UserID:           req.UserID,
		DeviceID:         req.DeviceID,
		ReceiptOffset:    req.ReceiptOffset,
		MaxReceiptOffset: req.MaxReceiptOffset,
		IsHuman:          req.IsHuman,
		Limit:            int(req.Limit),
		SyncInstance:     uint32(req.SyncInstance),
		IsFullSync:       req.IsFullSync,
		TraceID:          req.TraceID,
		Slot:             uint32(req.Slot),
	}
	for _, v := range req.JoinRooms {
		request.JoinRooms = append(request.JoinRooms, syncapitypes.SyncRoom{
			RoomID:    v.RoomID,
			RoomState: v.RoomState,
			Start:     v.Start,
			End:       v.End,
		})
	}
	for _, v := range req.InviteRooms {
		request.InviteRooms = append(request.InviteRooms, syncapitypes.SyncRoom{
			RoomID:    v.RoomID,
			RoomState: v.RoomState,
			Start:     v.Start,
			End:       v.End,
		})
	}
	for _, v := range req.LeaveRooms {
		request.LeaveRooms = append(request.LeaveRooms, syncapitypes.SyncRoom{
			RoomID:    v.RoomID,
			RoomState: v.RoomState,
			Start:     v.Start,
			End:       v.End,
		})
	}
	return request
}

func ToSyncProcessReq(request *syncapitypes.SyncServerRequest) *pb.SyncProcessReq {
	req := &pb.SyncProcessReq{
		RequestType:      request.RequestType,
		JoinedRooms:      request.JoinedRooms,
		UserID:           request.UserID,
		DeviceID:         request.DeviceID,
		ReceiptOffset:    request.ReceiptOffset,
		MaxReceiptOffset: request.MaxReceiptOffset,
		IsHuman:          request.IsHuman,
		Limit:            int32(request.Limit),
		SyncInstance:     int32(request.SyncInstance),
		IsFullSync:       request.IsFullSync,
		TraceID:          request.TraceID,
		Slot:             int32(request.Slot),
	}
	for _, v := range request.JoinRooms {
		req.JoinRooms = append(req.JoinRooms, &pb.SyncRoom{
			RoomID:    v.RoomID,
			RoomState: v.RoomState,
			Start:     v.Start,
			End:       v.End,
		})
	}
	for _, v := range request.InviteRooms {
		req.InviteRooms = append(req.InviteRooms, &pb.SyncRoom{
			RoomID:    v.RoomID,
			RoomState: v.RoomState,
			Start:     v.Start,
			End:       v.End,
		})
	}
	for _, v := range request.LeaveRooms {
		req.LeaveRooms = append(req.LeaveRooms, &pb.SyncRoom{
			RoomID:    v.RoomID,
			RoomState: v.RoomState,
			Start:     v.Start,
			End:       v.End,
		})
	}
	return req
}

func ToClientEvent(event *gomatrixserverlib.ClientEvent) *pb.ClientEvent {
	stateKey := ""
	if event.StateKey != nil {
		stateKey = *event.StateKey
	}
	return &pb.ClientEvent{
		Content:        event.Content,
		EventID:        event.EventID,
		EventNID:       event.EventNID,
		DomainOffset:   event.DomainOffset,
		Depth:          event.Depth,
		OriginServerTS: int64(event.OriginServerTS),
		RoomID:         event.RoomID,
		Sender:         event.Sender,
		StateKey:       stateKey,
		Type:           event.Type,
		Redacts:        event.Redacts,
		Hint:           event.Hint,
		Visible:        event.Visible,
		Unsigned:       event.Unsigned,
		EventOffset:    event.EventOffset,
	}
}

var emptyString = ""

func ToClientEvent2(event *pb.ClientEvent) *gomatrixserverlib.ClientEvent {
	var stateKey *string
	if event.StateKey != "" {
		stateKey = &event.StateKey
	} else {
		if event.Type != "m.room.message" && event.Type != "m.room.encrypted" {
			stateKey = &emptyString
		}
	}
	return &gomatrixserverlib.ClientEvent{
		Content:        event.Content,
		EventID:        event.EventID,
		EventNID:       event.EventNID,
		DomainOffset:   event.DomainOffset,
		Depth:          event.Depth,
		OriginServerTS: gomatrixserverlib.Timestamp(event.OriginServerTS),
		RoomID:         event.RoomID,
		Sender:         event.Sender,
		StateKey:       stateKey,
		Type:           event.Type,
		Redacts:        event.Redacts,
		Hint:           event.Hint,
		Visible:        event.Visible,
		Unsigned:       event.Unsigned,
		EventOffset:    event.EventOffset,
	}
}

func ToSyncServerRsp(result *syncapitypes.SyncServerResponse) *pb.SyncProcessRsp {
	rsp := &pb.SyncProcessRsp{
		Rooms:            &pb.SyncProcessRsp_Rooms{},
		MaxReceiptOffset: result.MaxReceiptOffset,
		AllLoaded:        result.AllLoaded,
		NewUsers:         result.NewUsers,
		Ready:            result.Ready,
		MaxRoomOffset:    result.MaxRoomOffset,
	}
	if result.Rooms.Join != nil {
		rsp.Rooms.Join = make(map[string]*pb.JoinResponse)
		for k, v := range result.Rooms.Join {
			item := &pb.JoinResponse{
				State:       &pb.JoinResponse_State{},
				Timeline:    &pb.JoinResponse_Timeline{},
				Ephemeral:   &pb.JoinResponse_Ephemeral{},
				AccountData: &pb.JoinResponse_AccountData{},
				Unread:      &pb.UnreadNotifications{},
			}
			if v.State.Events != nil {
				item.State.Events = make([]*pb.ClientEvent, 0, len(v.State.Events))
				for _, vv := range v.State.Events {
					item.State.Events = append(item.State.Events, ToClientEvent(&vv))
				}
			} else {
				item.State.Events = []*pb.ClientEvent{}
			}
			item.Timeline.Limited = v.Timeline.Limited
			item.Timeline.PrevBatch = v.Timeline.PrevBatch
			if v.Timeline.Events != nil {
				item.Timeline.Events = make([]*pb.ClientEvent, 0, len(v.Timeline.Events))
				for _, vv := range v.Timeline.Events {
					item.Timeline.Events = append(item.Timeline.Events, ToClientEvent(&vv))
				}
			}
			if v.Ephemeral.Events != nil {
				item.Ephemeral.Events = make([]*pb.ClientEvent, 0, len(v.Ephemeral.Events))
				for _, vv := range v.Ephemeral.Events {
					item.Ephemeral.Events = append(item.Ephemeral.Events, ToClientEvent(&vv))
				}
			}
			if v.AccountData.Events != nil {
				item.AccountData.Events = make([]*pb.ClientEvent, 0, len(v.AccountData.Events))
				for _, vv := range v.AccountData.Events {
					item.AccountData.Events = append(item.AccountData.Events, ToClientEvent(&vv))
				}
			}
			if v.Unread != nil {
				item.Unread = &pb.UnreadNotifications{
					HighLightCount:    v.Unread.HighLightCount,
					NotificationCount: v.Unread.NotificationCount,
				}
			}

			rsp.Rooms.Join[k] = item
		}
	}
	if result.Rooms.Invite != nil {
		rsp.Rooms.Invite = make(map[string]*pb.InviteResponse)
		for k, v := range result.Rooms.Invite {
			item := &pb.InviteResponse{
				InviteState: &pb.InviteResponse_InviteState{},
			}
			if v.InviteState.Events != nil {
				item.InviteState.Events = make([]*pb.ClientEvent, 0, len(v.InviteState.Events))
				for _, vv := range v.InviteState.Events {
					item.InviteState.Events = append(item.InviteState.Events, ToClientEvent(&vv))
				}
			}

			rsp.Rooms.Invite[k] = item
		}
	}
	if result.Rooms.Leave != nil {
		rsp.Rooms.Leave = make(map[string]*pb.LeaveResponse)
		for k, v := range result.Rooms.Leave {
			item := &pb.LeaveResponse{
				State:    &pb.LeaveResponse_State{},
				Timeline: &pb.LeaveResponse_Timeline{},
			}
			if v.State.Events != nil {
				item.State.Events = make([]*pb.ClientEvent, 0, len(v.State.Events))
				for _, vv := range v.State.Events {
					item.State.Events = append(item.State.Events, ToClientEvent(&vv))
				}
			}
			item.Timeline.Limited = v.Timeline.Limited
			item.Timeline.PrevBatch = v.Timeline.PrevBatch
			if v.Timeline.Events != nil {
				item.Timeline.Events = make([]*pb.ClientEvent, 0, len(v.Timeline.Events))
				for _, vv := range v.Timeline.Events {
					item.Timeline.Events = append(item.Timeline.Events, ToClientEvent(&vv))
				}
			}

			rsp.Rooms.Leave[k] = item
		}
	}
	return rsp
}

func ToSyncServerResponse(rsp *pb.SyncProcessRsp) *syncapitypes.SyncServerResponse {
	result := &syncapitypes.SyncServerResponse{
		Rooms:            syncapitypes.SyncServerResponseRooms{},
		MaxReceiptOffset: rsp.MaxReceiptOffset,
		AllLoaded:        rsp.AllLoaded,
		NewUsers:         rsp.NewUsers,
		Ready:            rsp.Ready,
		MaxRoomOffset:    rsp.MaxRoomOffset,
	}
	if rsp.Rooms != nil && rsp.Rooms.Join != nil {
		result.Rooms.Join = make(map[string]syncapitypes.JoinResponse)
		for k, v := range rsp.Rooms.Join {
			item := syncapitypes.JoinResponse{}
			item.State.Events = []gomatrixserverlib.ClientEvent{}
			if v.State != nil && v.State.Events != nil {
				for _, vv := range v.State.Events {
					item.State.Events = append(item.State.Events, *ToClientEvent2(vv))
				}
			}
			if v.Timeline != nil {
				item.Timeline.Limited = v.Timeline.Limited
				item.Timeline.PrevBatch = v.Timeline.PrevBatch
				item.Timeline.Events = []gomatrixserverlib.ClientEvent{}
				if v.Timeline.Events != nil {
					for _, vv := range v.Timeline.Events {
						item.Timeline.Events = append(item.Timeline.Events, *ToClientEvent2(vv))
					}
				}
			}
			item.Ephemeral.Events = []gomatrixserverlib.ClientEvent{}
			if v.Ephemeral != nil && v.Ephemeral.Events != nil {
				for _, vv := range v.Ephemeral.Events {
					item.Ephemeral.Events = append(item.Ephemeral.Events, *ToClientEvent2(vv))
				}
			}
			item.AccountData.Events = []gomatrixserverlib.ClientEvent{}
			if v.AccountData != nil && v.AccountData.Events != nil {
				for _, vv := range v.AccountData.Events {
					item.AccountData.Events = append(item.AccountData.Events, *ToClientEvent2(vv))
				}
			}
			if v.Unread != nil {
				item.Unread = &syncapitypes.UnreadNotifications{
					HighLightCount:    v.Unread.HighLightCount,
					NotificationCount: v.Unread.NotificationCount,
				}
			}

			result.Rooms.Join[k] = item
		}
	}
	if rsp.Rooms != nil && rsp.Rooms.Invite != nil {
		result.Rooms.Invite = make(map[string]syncapitypes.InviteResponse)
		for k, v := range rsp.Rooms.Invite {
			item := syncapitypes.InviteResponse{}
			item.InviteState.Events = []gomatrixserverlib.ClientEvent{}
			if v.InviteState != nil && v.InviteState.Events != nil {
				for _, vv := range v.InviteState.Events {
					item.InviteState.Events = append(item.InviteState.Events, *ToClientEvent2(vv))
				}
			}

			result.Rooms.Invite[k] = item
		}
	}
	if rsp.Rooms != nil && rsp.Rooms.Leave != nil {
		result.Rooms.Leave = make(map[string]syncapitypes.LeaveResponse)
		for k, v := range rsp.Rooms.Leave {
			item := syncapitypes.LeaveResponse{}
			item.State.Events = []gomatrixserverlib.ClientEvent{}
			if v.State != nil && v.State.Events != nil {
				for _, vv := range v.State.Events {
					item.State.Events = append(item.State.Events, *ToClientEvent2(vv))
				}
			}
			if v.Timeline != nil {
				item.Timeline.Limited = v.Timeline.Limited
				item.Timeline.PrevBatch = v.Timeline.PrevBatch
				item.Timeline.Events = []gomatrixserverlib.ClientEvent{}
				if v.Timeline.Events != nil {
					for _, vv := range v.Timeline.Events {
						item.Timeline.Events = append(item.Timeline.Events, *ToClientEvent2(vv))
					}
				}
			}

			result.Rooms.Leave[k] = item
		}
	}
	return result
}

func ToOnReceiptReq(req *types.ReceiptContent) *pb.OnReceiptReq {
	return &pb.OnReceiptReq{
		UserID:      req.UserID,
		DeviceID:    req.DeviceID,
		RoomID:      req.RoomID,
		ReceiptType: req.ReceiptType,
		EventID:     req.EventID,
	}
}

func ToReceiptContent(req *pb.OnReceiptReq) *types.ReceiptContent {
	return &types.ReceiptContent{
		UserID:      req.UserID,
		DeviceID:    req.DeviceID,
		RoomID:      req.RoomID,
		ReceiptType: req.ReceiptType,
		EventID:     req.EventID,
	}
}

func ToOnTypingReq(req *types.TypingContent) *pb.OnTypingReq {
	return &pb.OnTypingReq{
		Type:   req.Type,
		RoomID: req.RoomID,
		UserID: req.UserID,
	}
}

func ToTypingContent(req *pb.OnTypingReq) *types.TypingContent {
	return &types.TypingContent{
		Type:   req.Type,
		RoomID: req.RoomID,
		UserID: req.UserID,
	}
}

func ToOnUnreadReq(req *syncapitypes.SyncUnreadRequest) *pb.OnUnreadReq {
	return &pb.OnUnreadReq{
		JoinRooms:    req.JoinRooms,
		UserID:       req.UserID,
		SyncInstance: req.SyncInstance,
	}
}

func ToSyncUnreadRequest(req *pb.OnUnreadReq) *syncapitypes.SyncUnreadRequest {
	return &syncapitypes.SyncUnreadRequest{
		JoinRooms:    req.JoinRooms,
		UserID:       req.UserID,
		SyncInstance: req.SyncInstance,
	}
}

func ToSyncUnreadResponse(rsp *pb.OnUnreadRsp) *syncapitypes.SyncUnreadResponse {
	return &syncapitypes.SyncUnreadResponse{
		Count: rsp.Count,
	}
}

func ToOnUnreadRsp(rsp *syncapitypes.SyncUnreadResponse) *pb.OnUnreadRsp {
	return &pb.OnUnreadRsp{
		Count: rsp.Count,
	}
}

func ToUpdateOneTimeKeyReq(req *types.KeyUpdateContent) *pb.UpdateOneTimeKeyReq {
	return &pb.UpdateOneTimeKeyReq{
		UserID:   req.OneTimeKeyChangeUserId,
		DeviceID: req.OneTimeKeyChangeDeviceId,
	}
}

func ToKeyUpdateContentOneTimeKey(req *pb.UpdateOneTimeKeyReq) *types.KeyUpdateContent {
	return &types.KeyUpdateContent{
		Type:                     types.ONETIMEKEYUPDATE,
		OneTimeKeyChangeUserId:   req.UserID,
		OneTimeKeyChangeDeviceId: req.DeviceID,
	}
}

func ToUpdateDeviceKeyReq(req *types.KeyUpdateContent) *pb.UpdateDeviceKeyReq {
	ret := &pb.UpdateDeviceKeyReq{
		EventNID: req.EventNID,
	}
	for _, v := range req.DeviceKeyChanges {
		ret.DeviceKeyChanges = append(ret.DeviceKeyChanges, &pb.DeviceKeyChanges{
			Offset:        v.Offset,
			ChangedUserID: v.ChangedUserID,
		})
	}
	return ret
}

func ToKeyUpdateContentDeviceKey(req *pb.UpdateDeviceKeyReq) *types.KeyUpdateContent {
	ret := &types.KeyUpdateContent{
		Type:     types.DEVICEKEYUPDATE,
		EventNID: req.EventNID,
	}
	for _, v := range req.DeviceKeyChanges {
		ret.DeviceKeyChanges = append(ret.DeviceKeyChanges, types.DeviceKeyChanges{
			Offset:        v.Offset,
			ChangedUserID: v.ChangedUserID,
		})
	}
	return ret
}

func ToOnlinePresence(rsp *pb.GetOnlinePresenceRsp) *types.OnlinePresence {
	return &types.OnlinePresence{
		Found:        rsp.Found,
		Presence:     rsp.Presence,
		StatusMsg:    rsp.StatusMsg,
		ExtStatusMsg: rsp.ExtStatusMsg,
	}
}

func ToGetOnlinePresenceRsp(rsp *types.OnlinePresence) *pb.GetOnlinePresenceRsp {
	return &pb.GetOnlinePresenceRsp{
		Found:        rsp.Found,
		Presence:     rsp.Presence,
		StatusMsg:    rsp.StatusMsg,
		ExtStatusMsg: rsp.ExtStatusMsg,
	}
}

func ToSetReceiptLatestReq(req *syncapitypes.ReceiptUpdate) *pb.SetReceiptLatestReq {
	return &pb.SetReceiptLatestReq{
		Users:  req.Users,
		Offset: req.Offset,
		RoomID: req.RoomID,
	}
}

func ToReceiptUpdate(req *pb.SetReceiptLatestReq) *syncapitypes.ReceiptUpdate {
	return &syncapitypes.ReceiptUpdate{
		Users:  req.Users,
		Offset: req.Offset,
		RoomID: req.RoomID,
	}
}

func ToUpdateTypingReq(req *syncapitypes.TypingUpdate) *pb.UpdateTypingReq {
	return &pb.UpdateTypingReq{
		RoomID:    req.RoomID,
		UserID:    req.UserID,
		DeviceID:  req.DeviceID,
		RoomUsers: req.RoomUsers,
	}
}

func ToTypingUpdate(req *pb.UpdateTypingReq, typ string) *syncapitypes.TypingUpdate {
	return &syncapitypes.TypingUpdate{
		Type:      typ,
		RoomID:    req.RoomID,
		UserID:    req.UserID,
		DeviceID:  req.DeviceID,
		RoomUsers: req.RoomUsers,
	}
}
