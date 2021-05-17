// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package routing

import (
	"context"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

// Logout handles POST /logout
func Logout(
	deviceDB model.DeviceDatabase,
	userID, deviceID string,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	tokenFilter *filter.Filter,
	rpcClient *common.RpcClient,
	rpcCli rpc.RpcClient,
) (int, core.Coder) {
	// if req.Method != http.MethodPost {
	// 	return http.StatusMethodNotAllowed, jsonerror.NotFound("Bad method")
	// }

	LogoutDevice(userID, deviceID, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient, rpcCli, "logout")

	return http.StatusOK, nil
}

func LogoutDevice(
	userID,
	deviceID string,
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	tokenFilter *filter.Filter,
	rpcClient *common.RpcClient,
	rpcCli rpc.RpcClient,
	source string,
) {
	ctx := context.TODO()
	createdTimeMS := time.Now().UnixNano() / 1000000
	log.Infof("logout user %s device %s source:%s", userID, deviceID, source)

	cache.DeleteLocalDevice(deviceID, userID)
	if err := deviceDB.RemoveDevice(ctx, deviceID, userID, createdTimeMS); err != nil {
		log.Errorf("Log out remove device error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	err := encryptDB.DeleteDeviceKeys(ctx, deviceID, userID)
	if err != nil {
		log.Errorf("Log out remove device keys error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	cache.DeleteDeviceOneTimeKey(userID, deviceID)

	cache.DeleteDeviceKey(userID, deviceID)

	err = syncDB.DeleteDeviceStdMessage(ctx, userID, deviceID)
	if err != nil {
		log.Errorf("Log out remove device std message, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	content := types.KeyUpdateContent{
		Type:                     types.ONETIMEKEYUPDATE,
		OneTimeKeyChangeUserId:   userID,
		OneTimeKeyChangeDeviceId: deviceID,
	}

	err = rpcCli.UpdateOneTimeKey(ctx, &content)
	if err != nil {
		log.Errorf("Log out pub key update, device: %s, user: %s , error: %v", deviceID, userID, err)
	}

	pubLogoutToken(ctx, userID, deviceID, rpcCli)
}

func pubLogoutToken(ctx context.Context, userID string, deviceID string, rpcCli rpc.RpcClient) {
	content := types.FilterTokenContent{
		UserID:     userID,
		DeviceID:   deviceID,
		FilterType: types.FILTERTOKENDEL,
	}
	err := rpcCli.DelFilterToken(ctx, &content)
	if err != nil {
		log.Errorf("pub logout filter token info Marshal err %v", err)
	}
}

// LogoutAll handles POST /logout/all
func LogoutAll(
	deviceDB model.DeviceDatabase,
	userID, deviceID string,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	tokenFilter *filter.Filter,
	rpcClient *common.RpcClient,
	rpcCli rpc.RpcClient,
) (int, core.Coder) {
	log.Infof("logout all user %s device %s", userID, deviceID)

	devs := cache.GetDevicesByUserID(userID)
	for _, dev := range *devs {
		LogoutDevice(dev.UserID, dev.ID, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient, rpcCli, "logout_all")
	}

	return http.StatusOK, nil
}
