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

package entry

import (
	"context"
	"errors"

	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(model.CMD_FED_PROFILE, GetProfile)
	Register(model.CMD_FED_AVATARURL, GetAvatarURL)
	Register(model.CMD_FED_DISPLAYNAME, GetDisplayName)
}

func getProfileFromCache(ctx context.Context,
	msg *model.GobMessage, cache service.Cache) (*authtypes.Profile, *authtypes.Presences, error) {
	if msg == nil {
		return nil, nil, errors.New("msg from connector is nil")
	}

	userID := string(msg.Body)
	displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)
	log.Infof("fed get profile, userID: %s, displayName: %s, avatarURL: %s", userID, displayName, avatarURL)

	profile := &authtypes.Profile{}
	profile.DisplayName = displayName
	profile.AvatarURL = avatarURL
	profile.UserID = userID

	presence, ok := cache.GetPresences(userID)
	if !ok || presence.UserID != userID {
		presence = nil
	}

	return profile, presence, nil
}

func GetProfile(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	profile, presence, err := getProfileFromCache(ctx, msg, cache)
	if err != nil {
		return &model.GobMessage{}, err
	}

	resp := &external.GetProfileResponse{
		AvatarURL:   profile.AvatarURL,
		DisplayName: profile.DisplayName,
	}
	if presence != nil {
		resp.Status = presence.Status
		resp.StatusMsg = presence.StatusMsg
		resp.ExtStatusMsg = presence.ExtStatusMsg
	}
	body, _ := resp.Encode()
	retMsg := &model.GobMessage{
		Body: body,
	}
	log.Infof("GetProfile, ret.body: %v", retMsg.Body)
	return retMsg, nil
}

func GetAvatarURL(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	profile, _, err := getProfileFromCache(ctx, msg, cache)
	if err != nil {
		return &model.GobMessage{}, err
	}

	body, _ := (&external.GetAvatarURLResponse{
		AvatarURL: profile.AvatarURL,
	}).Encode()
	retMsg := &model.GobMessage{
		Body: body,
	}
	log.Infof("GetAvatar, ret.body: %v", retMsg.Body)
	return retMsg, nil
}

func GetDisplayName(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	profile, _, err := getProfileFromCache(ctx, msg, cache)
	if err != nil {
		return &model.GobMessage{}, err
	}

	body, _ := (&external.GetDisplayNameResponse{
		DisplayName: profile.DisplayName,
	}).Encode()
	retMsg := &model.GobMessage{
		Body: body,
	}
	log.Infof("GetDisplayName, ret.body: %v", retMsg.Body)
	return retMsg, nil
}
