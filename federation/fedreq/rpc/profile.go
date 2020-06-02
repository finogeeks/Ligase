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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/plugins/message/external"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func GetProfile(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	req *external.GetProfileRequest,
	response *external.GetProfileResponse,
) error {
	data, err := RpcRequest(rpcClient, string(destination), cfg.Rpc.FedProfileTopic, req)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}

func GetAvatar(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	req *external.GetProfileRequest,
	response *external.GetAvatarURLResponse,
) error {
	data, err := RpcRequest(rpcClient, string(destination), cfg.Rpc.FedAvatarTopic, req)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}

func GetDisplayName(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	req *external.GetProfileRequest,
	response *external.GetDisplayNameResponse,
) error {
	data, err := RpcRequest(rpcClient, string(destination), cfg.Rpc.FedDisplayNameTopic, req)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}

func GetUserInfo(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	req *external.GetUserInfoRequest,
	response *external.GetUserInfoResponse,
) error {
	data, err := RpcRequest(rpcClient, string(destination), cfg.Rpc.FedUserInfoTopic, req)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}
