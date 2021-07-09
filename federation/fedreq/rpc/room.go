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

package rpc

import (
	"context"
	"io"
	"net/http"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func GetAliasRoomID(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	req *external.GetDirectoryRoomAliasRequest,
	response *external.GetDirectoryRoomAliasResponse,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.GetAliasRoomIDFromRemote(ctx, req, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func GetRoomState(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	req *external.GetFedRoomStateRequest,
	response *gomatrixserverlib.RespState,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.GetRoomStateFromRemote(ctx, req, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func Download(
	cfg *config.Dendrite,
	rpcCli rpc.RpcClient,
	id, destination, mediaID, width, method, fileType string,
) (io.Reader, int, http.Header, error) {
	ctx := context.Background()
	return rpcCli.DownloadFromRemote(ctx, &external.GetFedDownloadRequest{
		ID:       id,
		FileType: fileType,
		MediaID:  mediaID,
		Width:    width,
		Method:   method,
	}, destination)
}

func MakeJoin(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	roomID, userID string, ver []string,
	response *gomatrixserverlib.RespMakeJoin,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.MakeJoinToRemote(ctx, &external.GetMakeJoinRequest{
		RoomID: roomID,
		UserID: userID,
		Ver:    ver,
	}, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func SendJoin(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	roomID, eventID string,
	event gomatrixserverlib.Event,
	resp *gomatrixserverlib.RespSendJoin,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.SendJoinToRemote(ctx, &external.PutSendJoinRequest{
		RoomID:  roomID,
		EventID: eventID,
		Event:   event,
	}, destination)
	if err != nil {
		return err
	}
	*resp = *rsp
	return nil
}

func MakeLeave(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	roomID, userID string,
	response *gomatrixserverlib.RespMakeLeave,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.MakeLeaveToRemote(ctx, &external.GetMakeLeaveRequest{
		RoomID: roomID,
		UserID: userID,
	}, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func SendLeave(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	roomID, eventID string,
	event gomatrixserverlib.Event,
	resp *gomatrixserverlib.RespSendLeave,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.SendLeaveToRemote(ctx, &external.PutSendLeaveRequest{
		RoomID:  roomID,
		EventID: eventID,
		Event:   event,
	}, destination)
	if err != nil {
		return err
	}
	*resp = *rsp
	return nil
}

func SendInvite(
	cfg *config.Dendrite,
	rpcCli rpc.RpcClient,
	destination string,
	event gomatrixserverlib.Event,
	response *gomatrixserverlib.RespInvite,
) error {
	ctx := context.Background()
	rsp, err := rpcCli.SendInviteToRemote(ctx, &event, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}
