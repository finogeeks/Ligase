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

package federation

import (
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/federation/fedreq/rpc"
	"github.com/finogeeks/ligase/plugins/message/external"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type Federation struct {
	cfg       *config.Dendrite
	rpcClient rpcService.RpcClient
}

func NewFederation(
	cfg *config.Dendrite,
	rpcClient rpcService.RpcClient,
) *Federation {
	fed := &Federation{
		cfg:       cfg,
		rpcClient: rpcClient,
	}
	return fed
}

func (fed *Federation) LookupRoomAlias(
	destination string,
	roomAlias string) (external.GetDirectoryRoomAliasResponse, error) {

	var resp external.GetDirectoryRoomAliasResponse
	queryReq := external.GetDirectoryRoomAliasRequest{RoomAlias: roomAlias}
	err := rpc.GetAliasRoomID(fed.cfg, fed.rpcClient, destination, &queryReq, &resp)
	log.Infof("get profile resp: %v", resp)
	return resp, err
}

func (fed *Federation) LookupProfile(
	destination string,
	userID string) (external.GetProfileResponse, error) {

	var resp external.GetProfileResponse
	queryReq := external.GetProfileRequest{UserID: userID}
	err := rpc.GetProfile(fed.cfg, fed.rpcClient, destination, &queryReq, &resp)
	log.Infof("get profile resp: %v", resp)
	return resp, err
}

func (fed *Federation) LookupAvatarURL(
	destination string,
	userID string) (external.GetAvatarURLResponse, error) {

	var resp external.GetAvatarURLResponse
	queryReq := external.GetProfileRequest{UserID: userID}
	err := rpc.GetAvatar(fed.cfg, fed.rpcClient, destination, &queryReq, &resp)
	log.Infof("get profile resp: %v", resp)
	return resp, err
}

func (fed *Federation) LookupDisplayName(
	destination string,
	userID string) (external.GetDisplayNameResponse, error) {

	var resp external.GetDisplayNameResponse
	queryReq := external.GetProfileRequest{UserID: userID}
	err := rpc.GetDisplayName(fed.cfg, fed.rpcClient, destination, &queryReq, &resp)
	log.Infof("get profile resp: %v", resp)
	return resp, err
}

func (fed *Federation) LookupState(
	destination string,
	roomID string,
) (gomatrixserverlib.RespState, error) {

	var resp gomatrixserverlib.RespState
	queryReq := external.GetFedRoomStateRequest{RoomID: roomID}
	err := rpc.GetRoomState(fed.cfg, fed.rpcClient, destination, &queryReq, &resp)
	log.Infof("LookupState resp: %v", resp)
	return resp, err
}

func (fed *Federation) LookupUserInfo(
	destination string,
	userID string) (external.GetUserInfoResponse, error) {

	var resp external.GetUserInfoResponse
	queryReq := external.GetUserInfoRequest{UserID: userID}
	err := rpc.GetUserInfo(fed.cfg, fed.rpcClient, destination, &queryReq, &resp)
	log.Infof("get user_info resp: %v", resp)
	return resp, err
}

func (fed *Federation) MakeJoin(
	destination string,
	roomID, userID string,
	ver []string,
) (gomatrixserverlib.RespMakeJoin, error) {
	var resp gomatrixserverlib.RespMakeJoin
	err := rpc.MakeJoin(fed.cfg, fed.rpcClient, destination, roomID, userID, ver, &resp)
	return resp, err
}

func (fed *Federation) SendJoin(
	destination string,
	roomID, eventID string,
	event gomatrixserverlib.Event,
) (gomatrixserverlib.RespSendJoin, error) {
	var resp gomatrixserverlib.RespSendJoin
	err := rpc.SendJoin(fed.cfg, fed.rpcClient, destination, roomID, eventID, event, &resp)
	return resp, err
}

func (fed *Federation) MakeLeave(
	destination string,
	roomID, userID string,
) (gomatrixserverlib.RespMakeLeave, error) {
	var resp gomatrixserverlib.RespMakeLeave
	err := rpc.MakeLeave(fed.cfg, fed.rpcClient, destination, roomID, userID, &resp)
	return resp, err
}

func (fed *Federation) SendLeave(
	destination string,
	roomID, eventID string,
	event gomatrixserverlib.Event,
) (gomatrixserverlib.RespSendLeave, error) {
	var resp gomatrixserverlib.RespSendLeave
	err := rpc.SendLeave(fed.cfg, fed.rpcClient, destination, roomID, eventID, event, &resp)
	return resp, err
}

func (fed *Federation) SendInvite(
	destination string,
	event gomatrixserverlib.Event,
) (gomatrixserverlib.RespInvite, error) {
	var resp gomatrixserverlib.RespInvite
	err := rpc.SendInvite(fed.cfg, fed.rpcClient, destination, event, &resp)
	log.Infof("send invite resp: %v", resp)
	return resp, err
}
