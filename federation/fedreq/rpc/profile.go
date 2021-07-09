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

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/rpc"
)

func GetProfile(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	req *external.GetProfileRequest,
	response *external.GetProfileResponse,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.GetProfileFromRemote(ctx, req, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func GetAvatar(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	req *external.GetProfileRequest,
	response *external.GetAvatarURLResponse,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.GetAvatarFromRemote(ctx, req, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func GetDisplayName(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	req *external.GetProfileRequest,
	response *external.GetDisplayNameResponse,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.GetDisplayNameFromRemote(ctx, req, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}

func GetUserInfo(
	cfg *config.Dendrite,
	rpcClient rpc.RpcClient,
	destination string,
	req *external.GetUserInfoRequest,
	response *external.GetUserInfoResponse,
) error {
	ctx := context.Background()
	rsp, err := rpcClient.GetUserInfoFromRemote(ctx, req, destination)
	if err != nil {
		return err
	}
	*response = *rsp
	return nil
}
