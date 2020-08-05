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

func (res *GetLoginResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetLoginResponseCapn(msg)
	if err != nil {
		return err
	}
	flows, err := resCapn.Flows()
	if err != nil {
		return err
	}
	flowsLen := flows.Len()
	res.Flows = make([]Flow, flowsLen)
	for i := 0; i < flowsLen; i++ {
		flow := flows.At(i)
		res.Flows[i].Type, _ = flow.Type()
		stages, err := flow.Stages()
		if err != nil {
			return err
		}
		stagesLen := stages.Len()
		res.Flows[i].Stages = make([]string, stagesLen)
		for j := 0; j < stagesLen; j++ {
			res.Flows[i].Stages[j], err = stages.At(i)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (res *PostLoginResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostLoginResponseCapn(msg)
	if err != nil {
		return err
	}

	res.UserID, err = resCapn.UserID()
	if err != nil {
		return err
	}
	res.AccessToken, err = resCapn.AccessToken()
	if err != nil {
		return err
	}
	res.HomeServer, err = resCapn.HomeServer()
	if err != nil {
		return err
	}
	res.DeviceID, err = resCapn.DeviceID()
	if err != nil {
		return err
	}
	return nil
}

func (res *RegisterResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootRegisterResponseCapn(msg)
	if err != nil {
		return err
	}

	res.UserID, err = resCapn.UserID()
	if err != nil {
		return err
	}
	res.AccessToken, err = resCapn.AccessToken()
	if err != nil {
		return err
	}
	res.HomeServer, err = resCapn.HomeServer()
	if err != nil {
		return err
	}
	res.DeviceID, err = resCapn.DeviceID()
	if err != nil {
		return err
	}
	return nil
}

func (res *UserInteractiveResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostRegisterEmailResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostRegisterEmailResponseCapn(msg)
	if err != nil {
		return err
	}

	res.SID, err = resCapn.SID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostRegisterMsisdResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostRegisterMsisdResponseCapn(msg)
	if err != nil {
		return err
	}

	res.SID, err = resCapn.SID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostAccountPasswordEmailResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostAccountPasswordEmailResponseCapn(msg)
	if err != nil {
		return err
	}

	res.SID, err = resCapn.SID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostAccountPasswordMsisdResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostAccountPasswordMsisdResponseCapn(msg)
	if err != nil {
		return err
	}

	res.SID, err = resCapn.SID()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetRegisterAvailResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetRegisterAvailResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Available = resCapn.Available()
	return nil
}

func (res *GetAccount3PIDResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetAccount3PIDResponseCapn(msg)
	if err != nil {
		return err
	}
	threePIDsCapn, err := resCapn.ThreePIDs()
	if err != nil {
		return err
	}
	res.ThreePIDs.Medium, _ = threePIDsCapn.Medium()
	res.ThreePIDs.Address, _ = threePIDsCapn.Address()
	res.ThreePIDs.ValidatedAt = int(threePIDsCapn.ValidatedAt())

	return nil
}

func (res *PostAccount3PIDEmailResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostAccount3PIDEmailResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Sid, err = resCapn.Sid()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostAccount3PIDMsisdnResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostAccount3PIDMsisdnResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Sid, err = resCapn.Sid()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostVoipTurnServerResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *Device) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *DeviceList) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *DelDeviceAuthResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetPresenceListResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PresenceJSON) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetRoomsTagsByIDResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostUploadKeysResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostQueryKeysResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostClaimKeysResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetAccountWhoAmI) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetJoinedMemberResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetInitialSyncResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetEventsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetAccountWhoAmIResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetAccountWhoAmIResponseCapn(msg)
	if err != nil {
		return err
	}

	res.UserID, err = resCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostUserFilterResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostUserFilterResponseCapn(msg)
	if err != nil {
		return err
	}

	res.FilterID, err = resCapn.FilterID()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetUserFilterResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PutRoomStateResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPutRoomStateResponseCapn(msg)
	if err != nil {
		return err
	}

	res.EventID, err = resCapn.EventID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PutRoomStateByTypeResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPutRoomStateByTypeResponseCapn(msg)
	if err != nil {
		return err
	}

	res.EventID, err = resCapn.EventID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PutRoomStateByTypeWithTxnIDResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPutRoomStateByTypeWithTxnIDResponseCapn(msg)
	if err != nil {
		return err
	}

	res.EventID, err = resCapn.EventID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PutRedactEventResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPutRedactEventResponseCapn(msg)
	if err != nil {
		return err
	}

	res.EventID, err = resCapn.EventID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostCreateRoomResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostCreateRoomResponseCapn(msg)
	if err != nil {
		return err
	}

	res.RoomID, err = resCapn.RoomID()
	if err != nil {
		return err
	}
	res.RoomAlias, err = resCapn.RoomAlias()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetDirectoryRoomAliasResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetDirectoryRoomAliasResponseCapn(msg)
	if err != nil {
		return err
	}

	res.RoomID, err = resCapn.RoomID()
	serversCapn, _ := resCapn.Servers()
	serversLen := serversCapn.Len()
	res.Servers = make([]string, serversLen)
	for i := 0; i < serversLen; i++ {
		res.Servers[i], err = serversCapn.At(i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (res *GetJoinedRoomsResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetJoinedRoomsResponseCapn(msg)
	if err != nil {
		return err
	}
	joinedRoomsCapn, err := resCapn.JoinedRooms()
	len := joinedRoomsCapn.Len()
	res.JoinedRooms = make([]string, len)
	for i := 0; i < len; i++ {
		res.JoinedRooms[i], err = joinedRoomsCapn.At(i)
		if err != nil {
			return err
		}
	}

	return nil
}

func (res *GetDirectoryRoomResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetDirectoryRoomResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Visibility, err = resCapn.Visibility()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetPublicRoomsResponse) Decode(input []byte) error {
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// resCapn, err := ReadRootGetPublicRoomsResponseCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// chunks, err := resCapn.Chunk()
	// if err != nil {
	// 	return err
	// }

	// len := chunks.Len()
	// res.Chunk = make([]PublicRoomsChunk, len)
	// for i := 0; i < len; i++ {
	// 	chunk := chunks.At(i)
	// 	aliases, _ := chunk.Aliases()
	// 	res.Chunk[i].Aliases = make([]string, aliases.Len())
	// 	for j := 0; j < aliases.Len(); j++ {
	// 		res.Chunk[i].Aliases[j], _ = aliases.At(j)
	// 	}
	// 	res.Chunk[i].CanonicalAlias, _ = chunk.CanonicalAlias()
	// 	res.Chunk[i].Name, _ = chunk.Name()
	// 	res.Chunk[i].NumJoinedMembers = chunk.NumJoinedMembers()
	// 	res.Chunk[i].RoomID, _ = chunk.RoomID()
	// 	res.Chunk[i].Topic, _ = chunk.Topic()
	// 	res.Chunk[i].WorldReadable = chunk.WorldReadable()
	// 	res.Chunk[i].GuestCanJoin = chunk.GuestCanJoin()
	// 	res.Chunk[i].AvatarURL, _ = chunk.AvatarURL()
	// }

	// res.NextBatch, err = resCapn.NextBatch()
	// if err != nil {
	// 	return err
	// }
	// res.PrevBatch, err = resCapn.PrevBatch()
	// if err != nil {
	// 	return err
	// }
	// res.Total = resCapn.Total()
	// if err != nil {
	// 	return err
	// }
	// return nil
	return json.Unmarshal(input, res)
}

func (res *PostPublicRoomsResponse) Decode(input []byte) error {
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// resCapn, err := ReadRootPostPublicRoomsResponseCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// chunks, err := resCapn.Chunk()
	// if err != nil {
	// 	return err
	// }

	// len := chunks.Len()
	// res.Chunk = make([]PublicRoomsChunk, len)
	// for i := 0; i < len; i++ {
	// 	chunk := chunks.At(i)
	// 	aliases, _ := chunk.Aliases()
	// 	res.Chunk[i].Aliases = make([]string, aliases.Len())
	// 	for j := 0; j < aliases.Len(); j++ {
	// 		res.Chunk[i].Aliases[j], _ = aliases.At(j)
	// 	}
	// 	res.Chunk[i].CanonicalAlias, _ = chunk.CanonicalAlias()
	// 	res.Chunk[i].Name, _ = chunk.Name()
	// 	res.Chunk[i].NumJoinedMembers = chunk.NumJoinedMembers()
	// 	res.Chunk[i].RoomID, _ = chunk.RoomID()
	// 	res.Chunk[i].Topic, _ = chunk.Topic()
	// 	res.Chunk[i].WorldReadable = chunk.WorldReadable()
	// 	res.Chunk[i].GuestCanJoin = chunk.GuestCanJoin()
	// 	res.Chunk[i].AvatarURL, _ = chunk.AvatarURL()
	// }

	// res.NextBatch, err = resCapn.NextBatch()
	// if err != nil {
	// 	return err
	// }
	// res.PrevBatch, err = resCapn.PrevBatch()
	// if err != nil {
	// 	return err
	// }
	// res.Total = resCapn.Total()
	// if err != nil {
	// 	return err
	// }
	// return nil
	return json.Unmarshal(input, res)
}

func (res *GetDisplayNameResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetDisplayNameResponseCapn(msg)
	if err != nil {
		return err
	}

	res.DisplayName, err = resCapn.DisplayName()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetThreePIDsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetAvatarURLResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetAvatarURLResponseCapn(msg)
	if err != nil {
		return err
	}

	res.AvatarURL, err = resCapn.AvatarURL()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetProfileResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// resCapn, err := ReadRootGetProfileResponseCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// res.AvatarURL, err = resCapn.AvatarURL()
	// if err != nil {
	// 	return err
	// }
	// res.DisplayName, err = resCapn.DisplayName()
	// if err != nil {
	// 	return err
	// }
	// return nil
}

func (res *GetTurnServerResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetTurnServerResponseCapn(msg)
	if err != nil {
		return err
	}

	uris, err := resCapn.Uris()
	if err != nil {
		return err
	}

	len := uris.Len()
	res.Uris = make([]string, len)
	for i := 0; i < len; i++ {
		res.Uris[i], err = uris.At(i)
		if err != nil {
			return err
		}
	}

	res.UserName, err = resCapn.UserName()
	if err != nil {
		return err
	}
	res.Password, err = resCapn.Password()
	if err != nil {
		return err
	}
	res.TTL = int(resCapn.TTL())
	return nil
}

func (res *GetPresenceResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetPresenceResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Presence, err = resCapn.Presence()
	if err != nil {
		return err
	}
	res.LastActiveAgo = resCapn.LastActiveAgo()
	if err != nil {
		return err
	}
	res.StatusMsg, err = resCapn.StatusMsg()
	if err != nil {
		return err
	}
	res.ExtStatusMsg, err = resCapn.ExtStatusMsg()
	if err != nil {
		return err
	}
	res.CurrentlyActive = resCapn.CurrentlyActive()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostMediaUploadResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostMediaUploadResponseCapn(msg)
	if err != nil {
		return err
	}

	res.ContentURI, err = resCapn.ContentURI()
	if err != nil {
		return err
	}
	return nil
}

func (res *GetDevicesResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetDevicesResponseCapn(msg)
	if err != nil {
		return err
	}

	devices, err := resCapn.Devices()
	if err != nil {
		return err
	}

	len := devices.Len()
	res.Devices = make([]Device, len)
	for i := 0; i < len; i++ {
		device := devices.At(i)
		res.Devices[i].DeviceID, _ = device.DeviceID()
		res.Devices[i].DisplayName, _ = device.DisplayName()
		res.Devices[i].LastSeenIP, _ = device.LastSeenIP()
		res.Devices[i].LastSeenTs = int(device.LastSeenTs())
		res.Devices[i].UserID, _ = device.UserID()
	}
	return nil
}

func (res *GetDevicesByIDResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetDevicesByIDResponseCapn(msg)
	if err != nil {
		return err
	}

	res.DeviceID, err = resCapn.DeviceID()
	if err != nil {
		return err
	}
	res.DisplayName, err = resCapn.DisplayName()
	if err != nil {
		return err
	}
	res.LastSeenIP, err = resCapn.LastSeenIP()
	if err != nil {
		return err
	}
	res.LastSeenTS = int(resCapn.LastSeenTS())
	return nil
}

func (res *GetKeysChangesResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetKeysChangesResponseCapn(msg)
	if err != nil {
		return err
	}

	changed, err := resCapn.Changed()
	if err != nil {
		return err
	}

	len := changed.Len()
	res.Changed = make([]string, len)
	for i := 0; i < len; i++ {
		res.Changed[i], err = changed.At(i)
		if err != nil {
			return err
		}
	}

	left, err := resCapn.Left()
	if err != nil {
		return err
	}

	len = left.Len()
	res.Left = make([]string, len)
	for i := 0; i < len; i++ {
		res.Left[i], err = left.At(i)
		if err != nil {
			return err
		}
	}

	return nil
}

func (res *GetRoomVisibilityRangeResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetPushersResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetPushersResponseCapn(msg)
	if err != nil {
		return err
	}

	pushers, err := resCapn.Pusher()
	if err != nil {
		return err
	}

	len := pushers.Len()
	res.Pusher = make([]Pusher, len)
	for i := 0; i < len; i++ {
		pusher := pushers.At(i)
		res.Pusher[i].Pushkey, _ = pusher.Pushkey()
		res.Pusher[i].Kind, _ = pusher.Kind()
		res.Pusher[i].AppID, _ = pusher.AppID()
		res.Pusher[i].AppDisplayName, _ = pusher.AppDisplayName()
		res.Pusher[i].DeviceDisplayName, _ = pusher.DeviceDisplayName()
		res.Pusher[i].ProfileTag, _ = pusher.ProfileTag()
		res.Pusher[i].Lang, _ = pusher.Lang()
		data, err := pusher.Data()
		if err != nil {
			return err
		}
		res.Pusher[i].Data.URL, _ = data.URL()
		res.Pusher[i].Data.Format, _ = data.Format()
	}

	return nil
}

func (res *GetPushrulesResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetPushrulesGlobalResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetPushrulesByIDResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetPushrulesEnabledByIDResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetPushrulesEnabledByIDResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Enabled = resCapn.Enabled()
	return nil
}

func (res *GetPushrulesActionsByIDResponse) Decode(input []byte) error {
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// resCapn, err := ReadRootGetPushrulesActionsByIDResponseCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// actions, err := resCapn.Actions()
	// if err != nil {
	// 	return err
	// }

	// len := actions.Len()
	// res.Actions[i]=make([],len)
	// for i := 0; i < len; i++ {
	// 	res.Actions[i], err = actions.At(i)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// return nil
	return json.Unmarshal(input, res)
}

func (res *PostUsersPushKeyResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetSyncResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetMediaPreviewURLResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootGetMediaPreviewURLResponseCapn(msg)
	if err != nil {
		return err
	}

	res.Image, err = resCapn.Image()
	if err != nil {
		return err
	}
	res.Size = resCapn.Size()
	return nil
}

func (res *PostRoomsJoinByAliasResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostRoomsJoinByAliasResponseCapn(msg)
	if err != nil {
		return err
	}

	res.RoomID, err = resCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostRoomsMembershipResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostRoomsJoinResponseCapn(msg)
	if err != nil {
		return err
	}

	res.RoomID, err = resCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (res *PostUserSearchResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := ReadRootPostUserSearchResponseCapn(msg)
	if err != nil {
		return err
	}

	results, err := resCapn.Results()
	if err != nil {
		return err
	}

	len := results.Len()
	res.Results = make([]User, len)
	for i := 0; i < len; i++ {
		result := results.At(i)
		res.Results[i].UserID, _ = result.UserID()
		res.Results[i].DisplayName, _ = result.DisplayName()
		res.Results[i].AvatarURL, _ = result.AvatarURL()
	}

	res.Limited = resCapn.Limited()
	return nil
}

func (res *GetRoomInfoResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetProfilesResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

/*func (res *GetRoomEventContextResponse) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	resCapn, err := readgetroomeventconte(msg)
	if err != nil {
		return err
	}

	res.End, err = resCapn.
	if err != nil {
		return err
	}
	res.Size = resCapn.Size()
	return nil
}
*/

func (res *ReqPostNotaryNoticeResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetFriendshipsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetFriendshipResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetUserInfoResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetUserInfoListResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetFedDirectoryResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetMissingEventsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetEventAuthResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostQueryAuthResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetEventResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetStateIDsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *GetFedPublicRoomsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostFedPublicRoomsResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostQueryClientKeysResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *PostClaimClientKeysResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}

func (res *DismissRoomResponse) Decode(input []byte) error {
	return json.Unmarshal(input, res)
}