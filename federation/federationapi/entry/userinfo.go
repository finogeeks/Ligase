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
	Register(model.CMD_FED_USER_INFO, GetUserInfo)
}

func getUserInfoFromCache(msg *model.GobMessage, cache service.Cache) (*authtypes.UserInfo, error) {
	if msg == nil {
		return nil, errors.New("msg from connector is nil")
	}

	userID := string(msg.Body)
	log.Infof("get user info, userID: %s", userID)

	userInfo := cache.GetUserInfoByUserID(userID)
	if userInfo == nil || userInfo.UserID == "" {
		return nil, errors.New("get empty user info")
	}

	return userInfo, nil
}

func GetUserInfo(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	userInfo, err := getUserInfoFromCache(msg, cache)
	if err != nil {
		return &model.GobMessage{}, err
	}

	body, _ := (&external.GetUserInfoResponse{
		UserName:  userInfo.UserName,
		JobNumber: userInfo.JobNumber,
		Mobile:    userInfo.Mobile,
		Landline:  userInfo.Landline,
		Email:     userInfo.Email,
		State:     userInfo.State,
	}).Encode()
	retMsg := &model.GobMessage{
		Body: body,
	}
	log.Infof("GetUserInfo, ret.body: %v", retMsg.Body)
	return retMsg, nil
}
