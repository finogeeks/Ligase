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
	jsoniter "github.com/json-iterator/go"
	capn "zombiezen.com/go/capnproto2"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func (externalReq *PostCreateRoomRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostRoomsJoinByAliasRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostRoomsMembershipRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostLoginRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostLoginAdminRequest) Encode() ([]byte, error) {
	t := (*PostLoginRequest)(externalReq)
	return t.Encode()
}

func (externalReq *PostUserFilterRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostUploadKeysRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostQueryKeysRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostClaimKeysRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetRoomJoinedMembersRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomJoinedMembersRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetEventsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PutSendToDeviceRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetWhoIsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetCasLoginRedirectRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetCasLoginTickerRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostRoomReportRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetThirdPartyProtocalByNameRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetThirdPartyLocationByProtocolRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetThirdPartyUserByProtocolRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetThirdPartyLocationRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetThirdPartyUserRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostUserOpenIDRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostRoomsKickRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomsKickRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetUserID(externalReq.UserID)
	reqCapn.SetReason(externalReq.Reason)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRoomsBanRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomsBanRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetUserID(externalReq.UserID)
	reqCapn.SetReason(externalReq.Reason)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRoomsUnbanRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
	}

	reqCapn, err := NewRootPostRoomsUnbanRequestCapn(seg)
	if err != nil {
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetUserID(externalReq.UserID)
	data, err := msg.Marshal()
	if err != nil {
	}

	return data, nil
}

func (externalReq *PostRoomsLeaveRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomsLeaveRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRoomsInviteRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomsInviteRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetUserID(externalReq.UserID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRoomsForgetRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomsForgetRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRoomStateByTypeWithTxnID) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRoomStateByTypeWithTxnIDCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventType(externalReq.EventType)
	reqCapn.SetStateKey(externalReq.StateKey)
	reqCapn.SetTxnID(externalReq.TxnID)
	reqCapn.SetContent(externalReq.Content)
	reqCapn.SetIp(externalReq.IP)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRoomStateByType) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRoomStateByTypeCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventType(externalReq.EventType)
	reqCapn.SetContent(externalReq.Content)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRoomStateByTypeAndStateKey) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRoomStateByTypeAndStateKeyCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventType(externalReq.EventType)
	reqCapn.SetStateKey(externalReq.StateKey)
	reqCapn.SetContent(externalReq.Content)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRedactEventRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRedactEventRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventID(externalReq.EventID)
	reqCapn.SetReason(externalReq.Reason)
	reqCapn.SetContent(externalReq.Content)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRedactWithTxnIDEventRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRedactWithTxnIDEventRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventID(externalReq.EventID)
	reqCapn.SetTxnID(externalReq.TxnID)
	reqCapn.SetReason(externalReq.Reason)
	reqCapn.SetContent(externalReq.Content)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRegisterRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRegisterRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetBindEmail(externalReq.BindEmail)
	reqCapn.SetUsername(externalReq.Username)
	reqCapn.SetPassword(externalReq.Password)
	reqCapn.SetDeviceID(externalReq.DeviceID)
	reqCapn.SetInitialDisplayName(externalReq.InitialDisplayName)
	reqCapn.SetInhibitLogin(externalReq.InhibitLogin)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetDomain(externalReq.Domain)
	reqCapn.SetAccessToken(externalReq.AccessToken)
	reqCapn.SetRemoteAddr(externalReq.RemoteAddr)
	reqCapn.SetAdmin(externalReq.Admin)

	auth, err := reqCapn.NewAuth()
	if err != nil {
		return nil, err
	}

	auth.SetType(externalReq.Auth.Type)
	auth.SetSession(externalReq.Auth.Session)
	auth.SetMac(externalReq.Auth.Mac)
	auth.SetResponse(externalReq.Auth.Response)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *LegacyRegisterRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetDirectoryRoomAliasRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetDirectoryRoomAliasRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomAlias(externalReq.RoomAlias)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutDirectoryRoomAliasRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutDirectoryRoomAliasRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetRoomAlias(externalReq.RoomAlias)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *DelDirectoryRoomAliasRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootDelDirectoryRoomAliasRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomAlias(externalReq.RoomAlias)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetDirectoryRoomRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetDirectoryRoomRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutDirectoryRoomRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutDirectoryRoomRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetVisibility(externalReq.Visibility)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetLoginRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetLoginRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetIsAdmin(externalReq.IsAdmin)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetLoginAdminRequest) Encode() ([]byte, error) {
	t := (*GetLoginRequest)(externalReq)
	return t.Encode()
}

func (externalReq *GetUserFilterRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetUserFilterRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	reqCapn.SetFilterID(externalReq.FilterID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetProfileRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetProfileRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetAvatarURLRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetAvatarURLRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutAvatarURLRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutAvatarURLRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	reqCapn.SetAvatarURL(externalReq.AvatarURL)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetDisplayNameRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetDisplayNameRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutDisplayNameRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutDisplayNameRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	reqCapn.SetDisplayName(externalReq.DisplayName)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostAccount3PIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostAccount3PIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetBind(externalReq.Bind)
	creds, err := reqCapn.NewThreePIDCreds()
	if err != nil {
		return nil, err
	}

	creds.SetClientSecret(externalReq.ThreePIDCreds.ClientSecret)
	creds.SetIdServer(externalReq.ThreePIDCreds.IdServer)
	creds.SetSid(externalReq.ThreePIDCreds.Sid)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostAccount3PIDDelRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostAccount3PIDDelRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetMedium(externalReq.Medium)
	reqCapn.SetAddress(externalReq.Address)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostAccount3PIDEmailRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostAccount3PIDEmailRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetPath(externalReq.Path)
	reqCapn.SetClientSecret(externalReq.ClientSecret)
	reqCapn.SetEmail(externalReq.Email)
	reqCapn.SetSendAttempt(externalReq.SendAttempt)
	reqCapn.SetNextLink(externalReq.NextLink)
	reqCapn.SetIdServer(externalReq.IdServer)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostAccount3PIDMsisdnRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostAccount3PIDMsisdnRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetPath(externalReq.Path)
	reqCapn.SetClientSecret(externalReq.ClientSecret)
	reqCapn.SetCountry(externalReq.Country)
	reqCapn.SetPhoneNumber(externalReq.PhoneNumber)
	reqCapn.SetSendAttempt(externalReq.SendAttempt)
	reqCapn.SetNextLink(externalReq.NextLink)
	reqCapn.SetIdServer(externalReq.IdServer)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetDeviceRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetDeviceRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetDeviceID(externalReq.DeviceID)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutDeviceRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutDeviceRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetDeviceID(externalReq.DeviceID)
	reqCapn.SetDisplayName(externalReq.DisplayName)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *DelDeviceRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootDelDeviceRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetDeviceID(externalReq.DeviceID)
	auth, err := reqCapn.NewAuth()
	if err != nil {
		return nil, err
	}

	auth.SetType(externalReq.Auth.Type)
	auth.SetSession(externalReq.Auth.Session)
	auth.SetPassword(externalReq.Auth.Password)
	auth.SetUser(externalReq.Auth.User)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutPresenceRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetPresenceRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetPresenceRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetPresenceListRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetPresenceListRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetJoinedMemberRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetJoinedMemberRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomsTagsByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomsTagsByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserId(externalReq.UserId)
	reqCapn.SetRoomId(externalReq.RoomId)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRoomsTagsByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRoomsTagsByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserId(externalReq.UserId)
	reqCapn.SetRoomId(externalReq.RoomId)
	reqCapn.SetTag(externalReq.Tag)
	// reqCapn.SetOrder(externalReq.Order)
	reqCapn.SetContent(externalReq.Content)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *DelRoomsTagsByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootDelRoomsTagsByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserId(externalReq.UserId)
	reqCapn.SetRoomId(externalReq.RoomId)
	reqCapn.SetTag(externalReq.Tag)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutUserAccountDataRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutUserAccountDataRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserId(externalReq.UserId)
	reqCapn.SetType(externalReq.Type)
	reqCapn.SetContent(externalReq.Content)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutRoomAccountDataRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRoomAccountDataRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserId(externalReq.UserId)
	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetType(externalReq.Type)
	reqCapn.SetContent(externalReq.Content)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostPresenceListRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostPresenceListRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	invite, err := reqCapn.NewInvite(int32(len(externalReq.Invite)))
	if err != nil {
		return nil, err
	}

	for i, v := range externalReq.Invite {
		invite.Set(i, v)
	}

	drop, err := reqCapn.NewDrop(int32(len(externalReq.Drop)))
	if err != nil {
		return nil, err
	}

	for i, v := range externalReq.Drop {
		drop.Set(i, v)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostMediaUploadRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostMediaUploadRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetFileName(externalReq.FileName)
	reqCapn.SetContentType(externalReq.ContentType)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetMediaDownloadRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetMediaDownloadRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetServerName(externalReq.ServerName)
	reqCapn.SetMediaID(externalReq.MediaID)
	reqCapn.SetAllowRemote(externalReq.AllowRemote)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetMediaThumbnailRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetMediaThumbnailRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetServerName(externalReq.ServerName)
	reqCapn.SetMediaID(externalReq.MediaID)
	reqCapn.SetWidth(int64(externalReq.Width))
	reqCapn.SetHeight(int64(externalReq.Height))
	reqCapn.SetMethod(int64(externalReq.Method))
	reqCapn.SetAllowRemote(externalReq.AllowRemote)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetPublicRoomsRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetPublicRoomsRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetLimit(int64(externalReq.Limit))
	reqCapn.SetSince(externalReq.Since)
	filter, err := reqCapn.NewFilter()
	if err != nil {
		return nil, err
	}
	filter.SetSearchTerm(externalReq.Filter.SearchTerms)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
	return json.Marshal(externalReq)
}

func (externalReq *PostPublicRoomsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostSetPushersRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostSetPushersRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetPushkey(externalReq.Pushkey)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetAppID(externalReq.AppID)
	reqCapn.SetAppDisplayName(externalReq.AppDisplayName)
	reqCapn.SetDeviceDisplayName(externalReq.DeviceDisplayName)
	reqCapn.SetProfileTag(externalReq.ProfileTag)
	reqCapn.SetLang(externalReq.Lang)
	reqCapn.SetAppend(externalReq.Append)

	dataCapn, err := reqCapn.NewData()
	if err != nil {
		return nil, err
	}

	dataCapn.SetURL(externalReq.Data.URL)
	dataCapn.SetFormat(externalReq.Data.Format)
	dataCapn.SetPushType(externalReq.Data.PushType)
	dataCapn.SetPushChannel(externalReq.Data.PushChannel)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetPushrulesByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetPushrulesByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetScope(externalReq.Scope)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetRuleID(externalReq.RuleID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutPushrulesByIDRequest) Encode() ([]byte, error) {
	// msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	// if err != nil {
	// 	return nil, err
	// }

	// reqCapn, err := NewRootPutPushrulesByIDRequestCapn(seg)
	// if err != nil {
	// 	return nil, err
	// }

	// reqCapn.SetPattern(externalReq.Pattern)
	// reqCapn.SetScope(externalReq.Scope)
	// reqCapn.SetKind(externalReq.Kind)
	// reqCapn.SetRuleID(externalReq.RuleID)
	// reqCapn.SetBefore(externalReq.Before)
	// reqCapn.SetAfter(externalReq.After)

	// action, err := reqCapn.NewActions(int32(len(externalReq.Actions)))
	// if err != nil {
	// 	return nil, err
	// }

	// for i, v := range externalReq.Actions {
	// 	action.Set(i, v)
	// }

	// condList, err := reqCapn.NewConditions(int32(len(externalReq.Conditions)))
	// if err != nil {
	// 	return nil, err
	// }

	// for i, v := range externalReq.Conditions {
	// 	cond := condList.At(i)
	// 	cond.SetKind(v.Kind)
	// 	cond.SetKey(v.Key)
	// 	cond.SetPattern(v.Pattern)
	// 	cond.SetIs(v.Is)
	// }

	// data, err := msg.Marshal()
	// if err != nil {
	// 	return nil, err
	// }

	// return data, nil
	return json.Marshal(externalReq)
}

func (externalReq *DelPushrulesByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootDelPushrulesByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetScope(externalReq.Scope)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetRuleID(externalReq.RuleID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetPushrulesEnabledByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetPushrulesEnabledByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetScope(externalReq.Scope)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetRuleID(externalReq.RuleID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutPushrulesEnabledByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutPushrulesEnabledByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetScope(externalReq.Scope)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetRuleID(externalReq.RuleID)
	reqCapn.SetEnabled(externalReq.Enabled)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetPushrulesActionsByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetPushrulesActionsByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetScope(externalReq.Scope)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetRuleID(externalReq.RuleID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PutPushrulesActionsByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutPushrulesActionsByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetScope(externalReq.Scope)
	reqCapn.SetKind(externalReq.Kind)
	reqCapn.SetRuleID(externalReq.RuleID)

	actions, err := reqCapn.NewActions(int32(len(externalReq.Actions)))
	for i, v := range externalReq.Actions {
		actions.Set(i, v)
	}

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostUsersPushKeyRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetSyncRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetSyncRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetFilter(externalReq.Filter)
	reqCapn.SetSince(externalReq.Since)
	reqCapn.SetFullState(externalReq.FullState)
	reqCapn.SetSetPresence(externalReq.SetPresence)
	reqCapn.SetTimeOut(externalReq.TimeOut)
	reqCapn.SetFrom(externalReq.From)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetInitialSyncRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetInitialSyncRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetLimit(int64(externalReq.Limit))
	reqCapn.SetArchived(externalReq.Archived)
	reqCapn.SetTimeout(int64(externalReq.Timeout))
	reqCapn.SetFullState(externalReq.FullState)
	reqCapn.SetSetPresence(externalReq.SetPresence)
	reqCapn.SetFrom(externalReq.From)
	reqCapn.SetSince(externalReq.Since)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomInitialSyncRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomInitialSyncRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomStateRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomStateRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomStateByTypeRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomStateByTypeRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventType(externalReq.EventType)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomStateByTypeAndStateKeyRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomStateByTypeAndStateKeyRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetEventType(externalReq.EventType)
	reqCapn.SetStateKey(externalReq.StateKey)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomMembersRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetRoomMembersRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomMessagesRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
	// msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	// if err != nil {
	// 	return nil, err
	// }

	// reqCapn, err := NewRootGetRoomMessagesRequestCapn(seg)
	// if err != nil {
	// 	return nil, err
	// }

	// reqCapn.SetRoomID(externalReq.RoomID)
	// reqCapn.SetFrom(externalReq.From)
	// reqCapn.SetDir(externalReq.Dir)
	// reqCapn.SetLimit(externalReq.Limit)
	// reqCapn.SetFilter(externalReq.Filter)

	// data, err := msg.Marshal()
	// if err != nil {
	// 	return nil, err
	// }

	// return data, nil
}

func (externalReq *GetEventByIDRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetEventByIDRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetEventID(externalReq.EventID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetKeysChangesRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootGetKeysChangesRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetFrom(externalReq.From)
	reqCapn.SetTo(externalReq.To)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *GetRoomVisibilityRangeRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PutRoomUserTypingRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPutRoomUserTypingRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetUserID(externalReq.UserID)
	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetTyping(externalReq.Typing)
	reqCapn.SetTimeOut(int64(externalReq.TimeOut))

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRoomReceiptRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomReceiptRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetReceiptType(externalReq.ReceiptType)
	reqCapn.SetEventID(externalReq.EventID)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostRoomReadMarkersRequest) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	reqCapn, err := NewRootPostRoomReadMarkersRequestCapn(seg)
	if err != nil {
		return nil, err
	}

	reqCapn.SetRoomID(externalReq.RoomID)
	reqCapn.SetReceiptType(externalReq.ReceiptType)
	reqCapn.SetFullyRead(externalReq.FullyRead)
	reqCapn.SetRead(externalReq.Read)

	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (externalReq *PostSystemManagerRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetUserUnread) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetFedBackFillRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostReportRoomRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostDelDevicesRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetRoomEventContextRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostReportDeviceState) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *ServerNameCfgRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetServerNamesResponse) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *ReqGetSettingRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *ReqPutSettingRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetRoomInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetProfilesRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostReportMissingEventsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *ReqPostNotaryNoticeRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetFriendshipsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PutFedInviteRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetFriendshipRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetUserInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostUserInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PutUserInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetUserInfoListRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *DeleteUserInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetFedDirectoryRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetMakeJoinRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PutSendJoinRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetMakeLeaveRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PutSendLeaveRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetMissingEventsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetEventAuthRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostQueryAuthRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetEventRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetStateIDsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *GetFedPublicRoomsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostFedPublicRoomsRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostQueryClientKeysRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *PostClaimClientKeysRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}

func (externalReq *DismissRoomRequest) Encode() ([]byte, error) {
	return json.Marshal(externalReq)
}
