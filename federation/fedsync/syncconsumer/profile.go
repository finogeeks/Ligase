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

package syncconsumer

import (
	"context"

	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func GetProfile(
	fedClient *client.FedClientWrap,
	profileReq *external.GetProfileRequest,
	destination string,
) external.GetProfileResponse {
	response, err := fedClient.LookupProfile(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation LookupProfile error: %v", err)
		return external.GetProfileResponse{}
	}

	log.Infof("LookupProfile return: %v", response)
	resp := external.GetProfileResponse{
		AvatarURL:   response.AvatarURL,
		DisplayName: response.DisplayName,
	}
	return resp
}

func GetAvatar(
	fedClient *client.FedClientWrap,
	profileReq *external.GetProfileRequest,
	destination string,
) external.GetAvatarURLResponse {
	response, err := fedClient.LookupAvatarURL(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation avatar error: %v", err)
		return external.GetAvatarURLResponse{}
	}

	log.Infof("LookupProfile return: %v", response)
	resp := external.GetAvatarURLResponse{
		AvatarURL: response.AvatarURL,
	}
	return resp
}

func GetDisplayName(
	fedClient *client.FedClientWrap,
	profileReq *external.GetProfileRequest,
	destination string,
) external.GetDisplayNameResponse {
	response, err := fedClient.LookupDisplayName(context.Background(), destination, profileReq.UserID)
	if err != nil {
		log.Errorf("federation display name error: %v", err)
		return external.GetDisplayNameResponse{}
	}

	log.Infof("LookupProfile return: %v", response)
	resp := external.GetDisplayNameResponse{
		DisplayName: response.DisplayName,
	}
	return resp
}

func GetUserInfo(
	fedClient *client.FedClientWrap,
	userInfoReq *external.GetUserInfoRequest,
	destination string,
) external.GetUserInfoResponse {
	response, err := fedClient.LookupUserInfo(context.Background(), destination, userInfoReq.UserID)
	if err != nil {
		log.Errorf("federation LookupUserInfo error: %v", err)
		return external.GetUserInfoResponse{}
	}

	log.Infof("LookupUserInfo return: %v", response)
	resp := external.GetUserInfoResponse{
		UserName:  response.UserName,
		JobNumber: response.JobNumber,
		Mobile:    response.Mobile,
		Landline:  response.Landline,
		Email:     response.Email,
		State:     response.State,
	}
	return resp
}
