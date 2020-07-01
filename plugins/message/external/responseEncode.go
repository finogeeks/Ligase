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

package external

import (
	capn "zombiezen.com/go/capnproto2"
)

func (res *GetLoginResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetLoginResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	flows, err := resCapn.NewFlows(int32(len(res.Flows)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Flows {
		flow := flows.At(i)
		if err != nil {
			return nil, err
		}
		flow.SetType(v.Type)
		stages, err := flow.NewStages(int32(len(v.Stages)))
		if err != nil {
			return nil, err
		}

		for ii, vv := range v.Stages {
			stages.Set(ii, vv)
		}
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostLoginResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostLoginResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetUserID(res.UserID)
	resCapn.SetAccessToken(res.AccessToken)
	resCapn.SetHomeServer(res.HomeServer)
	resCapn.SetDeviceID(res.DeviceID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *RegisterResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootRegisterResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetUserID(res.UserID)
	resCapn.SetAccessToken(res.AccessToken)
	resCapn.SetHomeServer(res.HomeServer)
	resCapn.SetDeviceID(res.DeviceID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *UserInteractiveResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *PostRegisterEmailResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostRegisterEmailResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetSID(res.SID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostRegisterMsisdResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostRegisterMsisdResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetSID(res.SID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostAccountPasswordEmailResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostAccountPasswordEmailResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetSID(res.SID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostAccountPasswordMsisdResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostAccountPasswordMsisdResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetSID(res.SID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetRegisterAvailResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetRegisterAvailResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetAvailable(res.Available)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetAccount3PIDResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetAccount3PIDResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	threePID, err := resCapn.NewThreePIDs()
	if err != nil {
		return nil, err
	}

	threePID.SetMedium(res.ThreePIDs.Medium)
	threePID.SetAddress(res.ThreePIDs.Address)
	threePID.SetValidatedAt(int64(res.ThreePIDs.ValidatedAt))
	threePID.SetAddedAt(int64(res.ThreePIDs.AddedAt))

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostAccount3PIDEmailResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostAccount3PIDEmailResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetSid(res.Sid)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostAccount3PIDMsisdnResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostAccount3PIDMsisdnResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetSid(res.Sid)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostVoipTurnServerResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *Device) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *DeviceList) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *DelDeviceAuthResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetPresenceListResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *PresenceJSON) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetRoomsTagsByIDResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *PostUploadKeysResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *PostQueryKeysResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *PostClaimKeysResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetAccountWhoAmI) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetJoinedMemberResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetInitialSyncResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetEventsResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetAccountWhoAmIResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetAccountWhoAmIResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetUserID(res.UserID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostUserFilterResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostUserFilterResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetFilterID(res.FilterID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetUserFilterResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *PutRoomStateResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPutRoomStateResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetEventID(res.EventID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PutRoomStateByTypeResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPutRoomStateByTypeResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetEventID(res.EventID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PutRoomStateByTypeWithTxnIDResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPutRoomStateByTypeWithTxnIDResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetEventID(res.EventID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PutRedactEventResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPutRedactEventResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetEventID(res.EventID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostCreateRoomResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostCreateRoomResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetRoomID(res.RoomID)
	resCapn.SetRoomAlias(res.RoomAlias)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetDirectoryRoomAliasResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetDirectoryRoomAliasResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetRoomID(res.RoomID)
	servers, err := resCapn.NewServers(int32(len(res.Servers)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Servers {
		servers.Set(i, v)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetJoinedRoomsResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetJoinedRoomsResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	joinedrooms, err := resCapn.NewJoinedRooms(int32(len(res.JoinedRooms)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.JoinedRooms {
		joinedrooms.Set(i, v)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetDirectoryRoomResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetDirectoryRoomResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetVisibility(res.Visibility)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetPublicRoomsResponse) Encode() ([]byte, error) {
	// msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	// if err != nil {
	// 	return nil, err
	// }
	// resCapn, err := NewRootGetPublicRoomsResponseCapn(seg)
	// if err != nil {
	// 	return nil, err
	// }

	// chunk, err := resCapn.NewChunk(int32(len(res.Chunk)))
	// if err != nil {
	// 	return nil, err
	// }
	// for i, v := range res.Chunk {
	// 	publicRoomsChunk := chunk.At(i)
	// 	aliases, err := publicRoomsChunk.NewAliases(int32(len(v.Aliases)))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	for ii, vv := range v.Aliases {
	// 		aliases.Set(ii, vv)
	// 	}
	// 	publicRoomsChunk.SetCanonicalAlias(v.CanonicalAlias)
	// 	publicRoomsChunk.SetName(v.Name)
	// 	publicRoomsChunk.SetNumJoinedMembers(int64(v.NumJoinedMembers))
	// 	publicRoomsChunk.SetRoomID(v.RoomID)
	// 	publicRoomsChunk.SetTopic(v.Topic)
	// 	publicRoomsChunk.SetWorldReadable(v.WorldReadable)
	// 	publicRoomsChunk.SetGuestCanJoin(v.GuestCanJoin)
	// 	publicRoomsChunk.SetAvatarURL(v.AvatarURL)
	// }
	// resCapn.SetNextBatch(res.NextBatch)
	// resCapn.SetPrevBatch(res.PrevBatch)
	// resCapn.SetTotal(res.Total)

	// data, err := msg.Marshal()
	// if err != nil {
	// 	return nil, err
	// }

	// return data, nil
	return json.Marshal(res)
}

func (res *PostPublicRoomsResponse) Encode() ([]byte, error) {
	// msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	// if err != nil {
	// 	return nil, err
	// }
	// resCapn, err := NewRootPostPublicRoomsResponseCapn(seg)
	// if err != nil {
	// 	return nil, err
	// }

	// chunk, err := resCapn.NewChunk(int32(len(res.Chunk)))
	// if err != nil {
	// 	return nil, err
	// }
	// for i, v := range res.Chunk {
	// 	publicRoomsChunk := chunk.At(i)
	// 	aliases, err := publicRoomsChunk.NewAliases(int32(len(v.Aliases)))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	for ii, vv := range v.Aliases {
	// 		aliases.Set(ii, vv)
	// 	}
	// 	publicRoomsChunk.SetCanonicalAlias(v.CanonicalAlias)
	// 	publicRoomsChunk.SetName(v.Name)
	// 	publicRoomsChunk.SetNumJoinedMembers(int64(v.NumJoinedMembers))
	// 	publicRoomsChunk.SetRoomID(v.RoomID)
	// 	publicRoomsChunk.SetTopic(v.Topic)
	// 	publicRoomsChunk.SetWorldReadable(v.WorldReadable)
	// 	publicRoomsChunk.SetGuestCanJoin(v.GuestCanJoin)
	// 	publicRoomsChunk.SetAvatarURL(v.AvatarURL)
	// }
	// resCapn.SetNextBatch(res.NextBatch)
	// resCapn.SetPrevBatch(res.PrevBatch)
	// resCapn.SetTotal(res.Total)

	// data, err := msg.Marshal()
	// if err != nil {
	// 	return nil, err
	// }

	// return data, nil
	return json.Marshal(res)
}

func (res *GetDisplayNameResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetDisplayNameResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetDisplayName(res.DisplayName)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetThreePIDsResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetAvatarURLResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetAvatarURLResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetAvatarURL(res.AvatarURL)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetProfileResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
	// msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	// if err != nil {
	// 	return nil, err
	// }
	// resCapn, err := NewRootGetProfileResponseCapn(seg)
	// if err != nil {
	// 	return nil, err
	// }

	// resCapn.SetAvatarURL(res.AvatarURL)
	// resCapn.SetDisplayName(res.DisplayName)

	// data, err := msg.Marshal()
	// if err != nil {
	// 	return nil, err
	// }

	// return data, nil
}

func (res *GetTurnServerResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetTurnServerResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetUserName(res.UserName)
	resCapn.SetPassword(res.Password)
	uris, err := resCapn.NewUris(int32(len(res.Uris)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Uris {
		uris.Set(i, v)
	}
	resCapn.SetTTL(int64(res.TTL))

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetPresenceResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetPresenceResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetPresence(res.Presence)
	resCapn.SetLastActiveAgo(res.LastActiveAgo)
	resCapn.SetStatusMsg(res.StatusMsg)
	resCapn.SetExtStatusMsg(res.ExtStatusMsg)
	resCapn.SetCurrentlyActive(res.CurrentlyActive)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostMediaUploadResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostMediaUploadResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetContentURI(res.ContentURI)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetDevicesResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetDevicesResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	devices, err := resCapn.NewDevices(int32(len(res.Devices)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Devices {
		device := devices.At(i)
		device.SetDeviceID(v.DeviceID)
		device.SetDisplayName(v.DisplayName)
		device.SetLastSeenIP(v.LastSeenIP)
		device.SetLastSeenTs(int64(v.LastSeenTs))
		device.SetUserID(v.UserID)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetDevicesByIDResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetDevicesByIDResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetDeviceID(res.DeviceID)
	resCapn.SetDisplayName(res.DisplayName)
	resCapn.SetLastSeenIP(res.LastSeenIP)
	resCapn.SetLastSeenTS(int64(res.LastSeenTS))

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetKeysChangesResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetKeysChangesResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	changed, err := resCapn.NewChanged(int32(len(res.Changed)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Changed {
		changed.Set(i, v)
	}
	left, err := resCapn.NewLeft(int32(len(res.Left)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Left {
		left.Set(i, v)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetRoomVisibilityRangeResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetPushersResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetPushersResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	pushers, err := resCapn.NewPusher(int32(len(res.Pusher)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Pusher {
		pusher := pushers.At(i)
		pusher.SetPushkey(v.Pushkey)
		pusher.SetKind(v.Kind)
		pusher.SetAppID(v.AppID)
		pusher.SetAppDisplayName(v.AppDisplayName)
		pusher.SetDeviceDisplayName(v.DeviceDisplayName)
		pusher.SetProfileTag(v.ProfileTag)
		pusher.SetLang(v.Lang)
		data, err := pusher.NewData()
		if err != nil {
			return nil, err
		}
		data.SetURL(v.Data.URL)
		data.SetFormat(v.Data.Format)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetPushrulesResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetPushrulesGlobalResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetPushrulesByIDResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetPushrulesEnabledByIDResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetPushrulesEnabledByIDResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetEnabled(res.Enabled)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetPushrulesActionsByIDResponse) Encode() ([]byte, error) {
	// msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	// if err != nil {
	// 	return nil, err
	// }
	// resCapn, err := NewRootGetPushrulesActionsByIDResponseCapn(seg)
	// if err != nil {
	// 	return nil, err
	// }

	// actions, err := resCapn.NewActions(int32(len(res.Actions)))
	// if err != nil {
	// 	return nil, err
	// }
	// for i, v := range res.Actions {
	// 	actions.Set(i, v)
	// }

	// data, err := msg.Marshal()
	// if err != nil {
	// 	return nil, err
	// }

	// return data, nil
	return json.Marshal(res)
}

func (res *PostUsersPushKeyResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetSyncResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetMediaPreviewURLResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootGetMediaPreviewURLResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetImage(res.Image)
	resCapn.SetSize(res.Size)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostRoomsJoinByAliasResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostRoomsJoinByAliasResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetRoomID(res.RoomID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostRoomsMembershipResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostRoomsJoinResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	resCapn.SetRoomID(res.RoomID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *PostUserSearchResponse) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}
	resCapn, err := NewRootPostUserSearchResponseCapn(seg)
	if err != nil {
		return nil, err
	}

	results, err := resCapn.NewResults(int32(len(res.Results)))
	if err != nil {
		return nil, err
	}
	for i, v := range res.Results {
		result := results.At(i)
		result.SetUserID(v.UserID)
		result.SetDisplayName(v.DisplayName)
		result.SetAvatarURL(v.AvatarURL)
		//results.Set(i, v)
	}
	resCapn.SetLimited(res.Limited)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (res *GetRoomInfoResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (res *GetProfilesResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}

func (r *ReqPostNotaryNoticeResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetFriendshipsResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetFriendshipResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetUserInfoResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetUserInfoListResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetFedDirectoryResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetMissingEventsResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetEventAuthResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *PostQueryAuthResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetEventResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetStateIDsResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetFedPublicRoomsResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *PostFedPublicRoomsResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *PostQueryClientKeysResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *PostClaimClientKeysResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (res *DismissRoomResponse) Encode() ([]byte, error) {
	return json.Marshal(res)
}
