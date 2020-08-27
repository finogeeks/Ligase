// Copyright 2017 Paul Tötterman <paul.totterman@iki.fi>
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
	"github.com/finogeeks/ligase/model/authtypes"
	"net/http"
	"strings"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// GetDeviceByID handles /device/{deviceID}
func GetDeviceByID(
	cache service.Cache,
	userID, deviceID string,
) (int, core.Coder) {
	dev := cache.GetDeviceByDeviceID(deviceID, userID)

	if dev != nil && dev.ID == deviceID {
		return http.StatusOK, &external.Device{
			DeviceID:    dev.ID,
			UserID:      dev.UserID,
			DisplayName: dev.DisplayName,
		}
	}

	return http.StatusNotFound, jsonerror.NotFound("Unknown device")
}

// GetDevicesByUserID handles /devices
func GetDevicesByUserID(
	cache service.Cache, userID string,
) (int, core.Coder) {
	deviceList := cache.GetDevicesByUserID(userID)

	res := &external.DeviceList{}

	for _, dev := range *deviceList {
		res.Devices = append(res.Devices, external.Device{
			DeviceID:    dev.ID,
			UserID:      dev.UserID,
			DisplayName: dev.DisplayName,
		})
	}

	return http.StatusOK, res
}

// UpdateDeviceByID handles PUT on /devices/{deviceID}
func UpdateDeviceByID(
	ctx context.Context, r *external.PutDeviceRequest,
	deviceDB model.DeviceDatabase, userID, deviceID string,
	toDeviceID string, cache service.Cache,
) (int, core.Coder) {
	// if req.Method != http.MethodPut {
	// 	return http.StatusMethodNotAllowed, jsonerror.NotFound("Bad Method")
	// }

	log.Infof("update device, user %s device %s updated %s", userID, deviceID, toDeviceID)

	// ctx := req.Context()
	dev := cache.GetDeviceByDeviceID(toDeviceID, userID)
	if dev == nil {
		return http.StatusNotFound, jsonerror.NotFound("Unknown device")
	}

	if dev.UserID != userID {
		return http.StatusForbidden, jsonerror.Forbidden("device not owned by current user")
	}

	// defer req.Body.Close() // nolint: errcheck
	// var r external.PutDeviceRequest
	// if resErr := httputil.UnmarshalJSONRequest(req, &r); resErr != nil {
	// 	return *resErr
	// }
	cache.UpdateLocalDevice(toDeviceID, userID, r.DisplayName)
	if err := deviceDB.InsertDevice(ctx, userID, &toDeviceID, &r.DisplayName, dev.DeviceType, dev.Identifier); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	return http.StatusOK, nil
}

func buildDelRequestFlow() *external.DelDeviceAuthResponse {
	f := &external.DelDeviceAuthResponse{}
	s := external.Flow{"m.login.password", []string{"m.login.password"}}
	f.Flows = append(f.Flows, s)
	f.Session = util.RandomString(sessionIDLength)

	return f
}

// DeleteDeviceByID handles Delete on /devices/{deviceID}
// Deletes the given device, and invalidates any access token associated with it.
//accountDB model.AccountsDatabase, deviceDB model.DeviceDatabase,
func DeleteDeviceByID(
	delReq *external.DelDeviceRequest,
	deviceID string,
	cfg config.Dendrite,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	tokenFilter *filter.Filter,
	syncDB model.SyncAPIDatabase,
	deviceDB model.DeviceDatabase,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	// var delReq external.DelDeviceRequest
	// if reqErr := httputil.UnmarshalJSONRequest(req, &delReq); reqErr != nil {
	// 	return util.JSONResponse{
	// 		Code: 401,
	// 		JSON: jsonerror.BadJSON("The request body could not be decoded into valid JSON. "),
	// 	}
	// }

	//check para
	//if delReq.Auth.Type == "" {
	//	return http.StatusUnauthorized, buildDelRequestFlow()
	//}

	log.Infof("delete device, user %s device %s", delReq.Auth.User, deviceID)
	//check auth

	if strings.HasPrefix(delReq.Auth.User, "@") {
		domain, err := common.DomainFromID(delReq.Auth.User)
		if err != nil {
			return http.StatusBadRequest, jsonerror.InvalidUsername("Invalid username")
		}

		if common.CheckValidDomain(domain, cfg.Matrix.ServerName) == false {
			return http.StatusBadRequest, jsonerror.InvalidUsername("Invalid username")
		}
	}

	LogoutDevice(delReq.Auth.User, deviceID, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient, "del_device")

	return http.StatusOK, nil
}

func DeleteDevices(
	req *external.PostDelDevicesRequest,
	device *authtypes.Device,
	cache service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	tokenFilter *filter.Filter,
	syncDB model.SyncAPIDatabase,
	deviceDB model.DeviceDatabase,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	if req.Devices == nil || len(req.Devices) == 0 {
		deviceList := cache.GetDevicesByUserID(device.UserID)
		hasPwdDevice := false
		for _, dev := range *deviceList {
			if dev.ID == device.ID && req.Auth.Type == "m.change_password" {
				continue
			} else {
				log.Infof("delete devices in batch type %s user %s device %s ", req.Auth.Type, dev.UserID, dev.ID)
				if req.Auth.Type == "m.change_password" {
					log.Infof("cache pwd change user %s device %s ", dev.UserID, dev.ID)
					cache.SetPwdChangeDevcie(dev.ID, device.UserID)
					hasPwdDevice = true
				}
				LogoutDevice(dev.UserID, dev.ID, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient, "del_devices")
			}
		}
		if hasPwdDevice {
			cache.ExpirePwdChangeDevice(device.UserID)
		}
	} else {
		for _, deviceId := range req.Devices {
			LogoutDevice(device.UserID, deviceId, deviceDB, cache, encryptDB, syncDB, tokenFilter, rpcClient, "del_devices")
		}
	}

	return http.StatusOK, nil
}
