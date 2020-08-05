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

func (externalReq *PostCreateRoomRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostRoomsJoinByAliasRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostRoomsMembershipRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostLoginRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostLoginAdminRequest) Decode(input []byte) error {
	t := (*PostLoginRequest)(externalReq)
	return t.Decode(input)
	//return json.Unmarshal(input, externalReq)
}

func (externalReq *PostUserFilterRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostUploadKeysRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostQueryKeysRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostClaimKeysRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetRoomJoinedMembersRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}
	reqCapn, err := ReadRootGetRoomJoinedMembersRequestCapn(msg)
	if err != nil {
		return err
	}
	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetEventsRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PutSendToDeviceRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetWhoIsRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetCasLoginRedirectRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetCasLoginTickerRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostRoomReportRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetThirdPartyProtocalByNameRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetThirdPartyLocationByProtocolRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetThirdPartyUserByProtocolRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetThirdPartyLocationRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetThirdPartyUserRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostUserOpenIDRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostRoomsKickRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomsKickRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	externalReq.Reason, err = reqCapn.Reason()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRoomsBanRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomsBanRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	externalReq.Reason, err = reqCapn.Reason()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRoomsUnbanRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomsUnbanRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRoomsLeaveRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomsLeaveRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRoomsInviteRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomsInviteRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRoomsForgetRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomsForgetRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutRoomStateByTypeWithTxnID) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		panic(err)
	}

	reqCapn, err := ReadRootPutRoomStateByTypeWithTxnIDCapn(msg)
	if err != nil {
		panic(err)
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		panic(err)
	}
	externalReq.EventType, err = reqCapn.EventType()
	if err != nil {
		panic(err)
	}
	externalReq.StateKey, err = reqCapn.StateKey()
	if err != nil {
		panic(err)
	}
	externalReq.TxnID, err = reqCapn.TxnID()
	if err != nil {
		panic(err)
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		panic(err)
	}
	externalReq.IP, err = reqCapn.Ip()
	if err != nil {
		panic(err)
	}
	return nil
}

func (externalReq *PutRoomStateByType) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRoomStateByTypeCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.EventType, err = reqCapn.EventType()
	if err != nil {
		return err
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutRoomStateByTypeAndStateKey) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRoomStateByTypeAndStateKeyCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.EventType, err = reqCapn.EventType()
	if err != nil {
		return err
	}
	externalReq.StateKey, err = reqCapn.StateKey()
	if err != nil {
		return err
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutRedactEventRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRedactEventRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.EventID, err = reqCapn.EventID()
	if err != nil {
		return err
	}
	externalReq.Reason, err = reqCapn.Reason()
	if err != nil {
		return err
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutRedactWithTxnIDEventRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRedactWithTxnIDEventRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.EventID, err = reqCapn.EventID()
	if err != nil {
		return err
	}
	externalReq.TxnID, err = reqCapn.TxnID()
	if err != nil {
		return err
	}
	externalReq.Reason, err = reqCapn.Reason()
	if err != nil {
		return err
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRegisterRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRegisterRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.BindEmail = reqCapn.BindEmail()
	externalReq.Username, err = reqCapn.Username()
	if err != nil {
		return err
	}
	externalReq.Password, err = reqCapn.Password()
	if err != nil {
		return err
	}
	externalReq.DeviceID, err = reqCapn.DeviceID()
	if err != nil {
		return err
	}
	externalReq.InitialDisplayName, err = reqCapn.InitialDisplayName()
	if err != nil {
		return err
	}
	externalReq.InhibitLogin = reqCapn.InhibitLogin()
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.Domain, err = reqCapn.Domain()
	if err != nil {
		return err
	}
	externalReq.AccessToken, err = reqCapn.AccessToken()
	if err != nil {
		return err
	}
	externalReq.RemoteAddr, err = reqCapn.RemoteAddr()
	if err != nil {
		return err
	}
	externalReq.Admin = reqCapn.Admin()
	authCapn, err := reqCapn.Auth()
	externalReq.Auth.Type, _ = authCapn.Type()
	externalReq.Auth.Session, _ = authCapn.Session()
	externalReq.Auth.Mac, _ = authCapn.Mac()
	externalReq.Auth.Response, _ = authCapn.Response()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *LegacyRegisterRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetDirectoryRoomAliasRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetDirectoryRoomAliasRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomAlias, err = reqCapn.RoomAlias()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutDirectoryRoomAliasRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutDirectoryRoomAliasRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.RoomAlias, err = reqCapn.RoomAlias()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *DelDirectoryRoomAliasRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootDelDirectoryRoomAliasRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomAlias, err = reqCapn.RoomAlias()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetDirectoryRoomRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetDirectoryRoomRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutDirectoryRoomRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutDirectoryRoomRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.Visibility, err = reqCapn.Visibility()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetLoginRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetLoginRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.IsAdmin = reqCapn.IsAdmin()
	return nil
}

func (externalReq *GetLoginAdminRequest) Decode(input []byte) error {
	t := (*GetLoginRequest)(externalReq)
	return t.Decode(input)
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// reqCapn, err := ReadRootGetLoginRequestCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// externalReq.IsAdmin = reqCapn.IsAdmin()
	// return nil
}

func (externalReq *GetUserFilterRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetUserFilterRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	externalReq.FilterID, err = reqCapn.FilterID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetProfileRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetProfileRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetAvatarURLRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetAvatarURLRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutAvatarURLRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutAvatarURLRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	externalReq.AvatarURL, err = reqCapn.AvatarURL()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetDisplayNameRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetDisplayNameRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutDisplayNameRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutDisplayNameRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	externalReq.DisplayName, err = reqCapn.DisplayName()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostAccount3PIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostAccount3PIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Bind = reqCapn.Bind()
	threePIDCredsCapn, err := reqCapn.ThreePIDCreds()
	if err != nil {
		return err
	}
	externalReq.ThreePIDCreds.ClientSecret, _ = threePIDCredsCapn.ClientSecret()
	externalReq.ThreePIDCreds.IdServer, _ = threePIDCredsCapn.IdServer()
	externalReq.ThreePIDCreds.Sid, _ = threePIDCredsCapn.Sid()
	return nil
}

func (externalReq *PostAccount3PIDDelRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostAccount3PIDDelRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Medium, err = reqCapn.Medium()
	if err != nil {
		return err
	}
	externalReq.Address, err = reqCapn.Address()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostAccount3PIDEmailRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostAccount3PIDEmailRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Path, err = reqCapn.Path()
	if err != nil {
		return err
	}
	externalReq.ClientSecret, err = reqCapn.ClientSecret()
	if err != nil {
		return err
	}
	externalReq.Email, err = reqCapn.Email()
	if err != nil {
		return err
	}
	externalReq.SendAttempt, err = reqCapn.SendAttempt()
	if err != nil {
		return err
	}
	externalReq.NextLink, err = reqCapn.NextLink()
	if err != nil {
		return err
	}
	externalReq.IdServer, err = reqCapn.IdServer()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostAccount3PIDMsisdnRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostAccount3PIDMsisdnRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Path, err = reqCapn.Path()
	if err != nil {
		return err
	}
	externalReq.ClientSecret, err = reqCapn.ClientSecret()
	if err != nil {
		return err
	}
	externalReq.Country, err = reqCapn.Country()
	if err != nil {
		return err
	}
	externalReq.PhoneNumber, err = reqCapn.PhoneNumber()
	if err != nil {
		return err
	}
	externalReq.SendAttempt, err = reqCapn.SendAttempt()
	if err != nil {
		return err
	}
	externalReq.NextLink, err = reqCapn.NextLink()
	if err != nil {
		return err
	}
	externalReq.IdServer, err = reqCapn.IdServer()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetDeviceRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetDeviceRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.DeviceID, err = reqCapn.DeviceID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutDeviceRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutDeviceRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.DeviceID, err = reqCapn.DeviceID()
	if err != nil {
		return err
	}
	externalReq.DisplayName, err = reqCapn.DisplayName()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *DelDeviceRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootDelDeviceRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.DeviceID, err = reqCapn.DeviceID()
	if err != nil {
		return err
	}

	authCapn, err := reqCapn.Auth()
	if err != nil {
		return err
	}
	externalReq.Auth.Type, _ = authCapn.Type()
	externalReq.Auth.Session, _ = authCapn.Session()
	externalReq.Auth.Password, _ = authCapn.Password()
	externalReq.Auth.User, _ = authCapn.User()
	return nil
}

func (externalReq *PutPresenceRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetPresenceRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetPresenceRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetPresenceListRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetPresenceListRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetJoinedMemberRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetJoinedMemberRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomsTagsByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetRoomsTagsByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserId, err = reqCapn.UserId()
	if err != nil {
		return err
	}
	externalReq.RoomId, err = reqCapn.RoomId()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutRoomsTagsByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRoomsTagsByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserId, err = reqCapn.UserId()
	if err != nil {
		return err
	}
	externalReq.RoomId, err = reqCapn.RoomId()
	if err != nil {
		return err
	}
	externalReq.Tag, err = reqCapn.Tag()
	if err != nil {
		return err
	}
	// externalReq.Order = reqCapn.Order()
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *DelRoomsTagsByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootDelRoomsTagsByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserId, err = reqCapn.UserId()
	if err != nil {
		return err
	}
	externalReq.RoomId, err = reqCapn.RoomId()
	if err != nil {
		return err
	}
	externalReq.Tag, err = reqCapn.Tag()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutUserAccountDataRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutUserAccountDataRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserId, err = reqCapn.UserId()
	if err != nil {
		return err
	}
	externalReq.Type, err = reqCapn.Type()
	if err != nil {
		return err
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutRoomAccountDataRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRoomAccountDataRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserId, err = reqCapn.UserId()
	if err != nil {
		return err
	}
	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.Type, err = reqCapn.Type()
	if err != nil {
		return err
	}
	externalReq.Content, err = reqCapn.Content()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostPresenceListRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostPresenceListRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostMediaUploadRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostMediaUploadRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.FileName, err = reqCapn.FileName()
	if err != nil {
		return err
	}
	externalReq.ContentType, err = reqCapn.ContentType()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetMediaDownloadRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetMediaDownloadRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.ServerName, err = reqCapn.ServerName()
	if err != nil {
		return err
	}
	externalReq.MediaID, err = reqCapn.MediaID()
	if err != nil {
		return err
	}
	externalReq.AllowRemote, err = reqCapn.AllowRemote()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetMediaThumbnailRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetMediaThumbnailRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.ServerName, err = reqCapn.ServerName()
	if err != nil {
		return err
	}
	externalReq.MediaID, err = reqCapn.MediaID()
	if err != nil {
		return err
	}
	externalReq.Width = int(reqCapn.Width())
	externalReq.Height = int(reqCapn.Height())
	externalReq.Method = int(reqCapn.Method())
	externalReq.AllowRemote, err = reqCapn.AllowRemote()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetPublicRoomsRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetPublicRoomsRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Limit = reqCapn.Limit()
	externalReq.Since, err = reqCapn.Since()
	if err != nil {
		return err
	}
	filter, err := reqCapn.Filter()
	if err != nil {
		return err
	}
	externalReq.Filter.SearchTerms, err = filter.SearchTerm()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostPublicRoomsRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostSetPushersRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostSetPushersRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Pushkey, err = reqCapn.Pushkey()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.AppID, err = reqCapn.AppID()
	if err != nil {
		return err
	}
	externalReq.AppDisplayName, err = reqCapn.AppDisplayName()
	if err != nil {
		return err
	}
	externalReq.DeviceDisplayName, err = reqCapn.DeviceDisplayName()
	if err != nil {
		return err
	}
	externalReq.ProfileTag, err = reqCapn.ProfileTag()
	if err != nil {
		return err
	}
	externalReq.Lang, err = reqCapn.Lang()
	if err != nil {
		return err
	}
	externalReq.Append = reqCapn.Append()

	dataCapn, err := reqCapn.Data()
	if err != nil {
		return err
	}
	externalReq.Data.URL, _ = dataCapn.URL()
	externalReq.Data.Format, _ = dataCapn.Format()
	externalReq.Data.PushType, _ = dataCapn.PushType()
	externalReq.Data.PushChannel, _ = dataCapn.PushChannel()
	return nil
}

func (externalReq *GetPushrulesByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetPushrulesByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Scope, err = reqCapn.Scope()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.RuleID, err = reqCapn.RuleID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutPushrulesByIDRequest) Decode(input []byte) error {
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// reqCapn, err := ReadRootPutPushrulesByIDRequestCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// externalReq.Pattern, err = reqCapn.Pattern()
	// if err != nil {
	// 	return err
	// }
	// externalReq.Scope, err = reqCapn.Scope()
	// if err != nil {
	// 	return err
	// }
	// externalReq.Kind, err = reqCapn.Kind()
	// if err != nil {
	// 	return err
	// }
	// externalReq.RuleID, err = reqCapn.RuleID()
	// if err != nil {
	// 	return err
	// }
	// externalReq.Before, err = reqCapn.Before()
	// if err != nil {
	// 	return err
	// }
	// externalReq.After, err = reqCapn.After()
	// if err != nil {
	// 	return err
	// }

	// actionsCapn, err := reqCapn.Actions()
	// if err != nil {
	// 	return err
	// }
	// len := actionsCapn.Len()
	// externalReq.Actions = make([],len)
	// for i := 0; i < len; i++ {
	// 	externalReq.Actions[i], _ = actionsCapn.At(i)
	// }

	// condsCapn, err := reqCapn.Conditions()
	// if err != nil {
	// 	return err
	// }

	// len = condsCapn.Len()
	// externalReq.Conditions =make([],len)
	// for i := 0; i < len; i++ {
	// 	condCapn := condsCapn.At(i)
	// 	externalReq.Conditions[i].Kind, _ = condCapn.Kind()
	// 	externalReq.Conditions[i].Key, _ = condCapn.Key()
	// 	externalReq.Conditions[i].Pattern, _ = condCapn.Pattern()
	// 	externalReq.Conditions[i].Is, _ = condCapn.Is()
	// }
	// return nil
	return json.Unmarshal(input, externalReq)
}

func (externalReq *DelPushrulesByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootDelPushrulesByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Scope, err = reqCapn.Scope()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.RuleID, err = reqCapn.RuleID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetPushrulesEnabledByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetPushrulesEnabledByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Scope, err = reqCapn.Scope()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.RuleID, err = reqCapn.RuleID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutPushrulesEnabledByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutPushrulesEnabledByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Scope, err = reqCapn.Scope()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.RuleID, err = reqCapn.RuleID()
	if err != nil {
		return err
	}
	externalReq.Enabled = reqCapn.Enabled()
	return nil
}

func (externalReq *GetPushrulesActionsByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetPushrulesActionsByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Scope, err = reqCapn.Scope()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.RuleID, err = reqCapn.RuleID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PutPushrulesActionsByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutPushrulesActionsByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Scope, err = reqCapn.Scope()
	if err != nil {
		return err
	}
	externalReq.Kind, err = reqCapn.Kind()
	if err != nil {
		return err
	}
	externalReq.RuleID, err = reqCapn.RuleID()
	if err != nil {
		return err
	}

	actionsCapn, err := reqCapn.Actions()
	if err != nil {
		return err
	}
	len := actionsCapn.Len()
	externalReq.Actions = make([]string, len)
	for i := 0; i < len; i++ {
		externalReq.Actions[i], _ = actionsCapn.At(i)
	}
	return nil
}

func (externalReq *PostUsersPushKeyRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetSyncRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetSyncRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Filter, err = reqCapn.Filter()
	if err != nil {
		return err
	}
	externalReq.Since, err = reqCapn.Since()
	if err != nil {
		return err
	}
	externalReq.FullState, err = reqCapn.FullState()
	if err != nil {
		return err
	}
	externalReq.SetPresence, err = reqCapn.SetPresence()
	if err != nil {
		return err
	}
	externalReq.TimeOut, err = reqCapn.TimeOut()
	if err != nil {
		return err
	}
	externalReq.From, err = reqCapn.From()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetInitialSyncRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetInitialSyncRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.Limit = int(reqCapn.Limit())
	externalReq.Archived = reqCapn.Archived()
	externalReq.Timeout = int(reqCapn.Timeout())
	externalReq.FullState, err = reqCapn.FullState()
	if err != nil {
		return err
	}
	externalReq.SetPresence, err = reqCapn.SetPresence()
	if err != nil {
		return err
	}
	externalReq.From, err = reqCapn.From()
	if err != nil {
		return err
	}
	externalReq.Since, err = reqCapn.Since()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomInitialSyncRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetRoomInitialSyncRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomStateRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetRoomStateRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomStateByTypeRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetRoomStateByTypeRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.EventType, err = reqCapn.EventType()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomStateByTypeAndStateKeyRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetRoomStateByTypeAndStateKeyRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.EventType, err = reqCapn.EventType()
	if err != nil {
		return err
	}
	externalReq.StateKey, err = reqCapn.StateKey()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomMembersRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetRoomMembersRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomMessagesRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
	// msg, err := capn.Unmarshal(input)
	// if err != nil {
	// 	return err
	// }

	// reqCapn, err := ReadRootGetRoomMessagesRequestCapn(msg)
	// if err != nil {
	// 	return err
	// }

	// externalReq.RoomID, err = reqCapn.RoomID()
	// if err != nil {
	// 	return err
	// }
	// externalReq.From, err = reqCapn.From()
	// if err != nil {
	// 	return err
	// }
	// externalReq.Dir, err = reqCapn.Dir()
	// if err != nil {
	// 	return err
	// }
	// externalReq.Limit, err = reqCapn.Limit()
	// if err != nil {
	// 	return err
	// }
	// externalReq.Filter, err = reqCapn.Filter()
	// if err != nil {
	// 	return err
	// }
	// return nil
}

func (externalReq *GetEventByIDRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetEventByIDRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.EventID, err = reqCapn.EventID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetKeysChangesRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootGetKeysChangesRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.From, err = reqCapn.From()
	if err != nil {
		return err
	}
	externalReq.To, err = reqCapn.To()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *GetRoomVisibilityRangeRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PutRoomUserTypingRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPutRoomUserTypingRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.UserID, err = reqCapn.UserID()
	if err != nil {
		return err
	}
	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.Typing = reqCapn.Typing()
	externalReq.TimeOut = int(reqCapn.TimeOut())
	return nil
}

func (externalReq *PostRoomReceiptRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomReceiptRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.ReceiptType, err = reqCapn.ReceiptType()
	if err != nil {
		return err
	}
	externalReq.EventID, err = reqCapn.EventID()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostRoomReadMarkersRequest) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	reqCapn, err := ReadRootPostRoomReadMarkersRequestCapn(msg)
	if err != nil {
		return err
	}

	externalReq.RoomID, err = reqCapn.RoomID()
	if err != nil {
		return err
	}
	externalReq.ReceiptType, err = reqCapn.ReceiptType()
	if err != nil {
		return err
	}
	externalReq.FullyRead, err = reqCapn.FullyRead()
	if err != nil {
		return err
	}
	externalReq.Read, err = reqCapn.Read()
	if err != nil {
		return err
	}
	return nil
}

func (externalReq *PostSystemManagerRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetUserUnread) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetFedBackFillRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostReportRoomRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostDelDevicesRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetRoomEventContextRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *PostReportDeviceState) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *ServerNameCfgRequest) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}

func (externalReq *GetServerNamesResponse) Decode(input []byte) error {
	return json.Unmarshal(input, externalReq)
}
func (externalReq *ReqGetSettingRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *ReqPutSettingRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetRoomInfoRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetProfilesRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PostReportMissingEventsRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *ReqPostNotaryNoticeRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetFriendshipsRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PutFedInviteRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetFriendshipRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetUserInfoRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PostUserInfoRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PutUserInfoRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetUserInfoListRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *DeleteUserInfoRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetFedDirectoryRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetMakeJoinRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PutSendJoinRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetMakeLeaveRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PutSendLeaveRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetMissingEventsRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetEventAuthRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PostQueryAuthRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetEventRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetStateIDsRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *GetFedPublicRoomsRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PostFedPublicRoomsRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PostQueryClientKeysRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *PostClaimClientKeysRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}

func (externalReq *DismissRoomRequest) Decode(data []byte) error {
	return json.Unmarshal(data, externalReq)
}
