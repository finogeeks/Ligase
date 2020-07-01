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
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
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
) (int, core.Coder) {
	// if req.Method != http.MethodPost {
	// 	return http.StatusMethodNotAllowed, jsonerror.NotFound("Bad method")
	// }

	LogoutDevice(userID, deviceID, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient)

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
) {
	createdTimeMS := time.Now().UnixNano() / 1000000
	log.Infof("logout user %s device %s", userID, deviceID)

	cache.DeleteLocalDevice(deviceID, userID)
	if err := deviceDB.RemoveDevice(context.TODO(), deviceID, userID, createdTimeMS); err != nil {
		log.Errorf("Log out remove device error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	err := encryptDB.DeleteDeviceKeys(context.TODO(), deviceID, userID)
	if err != nil {
		log.Errorf("Log out remove device keys error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	cache.DeleteDeviceOneTimeKey(userID, deviceID)

	cache.DeleteDeviceKey(userID, deviceID)

	err = syncDB.DeleteDeviceStdMessage(context.TODO(), userID, deviceID)
	if err != nil {
		log.Errorf("Log out remove device std message, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	content := types.KeyUpdateContent{
		Type:                     types.ONETIMEKEYUPDATE,
		OneTimeKeyChangeUserId:   userID,
		OneTimeKeyChangeDeviceId: deviceID,
	}

	bytes, err := json.Marshal(content)
	if err == nil {
		rpcClient.Pub(types.KeyUpdateTopicDef, bytes)
	} else {
		log.Errorf("Log out pub key update, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	pubLogoutToken(userID, deviceID, rpcClient)
}

func pubLogoutToken(userID string, deviceID string, rpcClient *common.RpcClient) {
	content := types.FilterTokenContent{
		UserID:     userID,
		DeviceID:   deviceID,
		FilterType: types.FILTERTOKENDEL,
	}
	bytes, err := json.Marshal(content)
	if err == nil {
		log.Infof("pub logout filter token info %s", string(bytes))
		rpcClient.Pub(types.FilterTokenTopicDef, bytes)
	} else {
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
) (int, core.Coder) {
	log.Infof("logout all user %s device %s", userID, deviceID)

	devs := cache.GetDevicesByUserID(userID)
	for _, dev := range *devs {
		LogoutDevice(dev.UserID, dev.ID, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient)
	}

	return http.StatusOK, nil
}
