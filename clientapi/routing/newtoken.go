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

package routing

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func GenNewToken(
	ctx context.Context,
	deviceDB model.DeviceDatabase,
	device *authtypes.Device,
	tokenFilter *filter.Filter,
	idg *uid.UidGenerator,
	cfg config.Dendrite,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	rpcClient *common.RpcClient,
	rpcCli rpc.RpcClient,
	ip string,
) (int, core.Coder) {
	mac := common.GetDeviceMac(device.ID)
	mac = fmt.Sprintf("%s-%s", mac, "gen")

	deviceID, deviceType, err := common.BuildDevice(idg, &mac, true, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create device: " + err.Error())
	}

	domain, _ := common.DomainFromID(device.UserID)

	//异步删db
	err = encryptDB.DeleteMacKeys(context.TODO(), deviceID, device.UserID, mac)
	if err != nil {
		log.Errorf("Login remove device keys error, device: %s ,  user: %s , error: %v", deviceID, device.UserID, err)
	}
	err = syncDB.DeleteMacStdMessage(context.TODO(), mac, device.UserID, deviceID)
	if err != nil {
		log.Errorf("Login remove std message error, device: %s ,  user: %s , error: %v", deviceID, device.UserID, err)
	}

	token, err := common.BuildToken(cfg.Macaroon.Key, device.UserID, domain, device.UserID, mac, false, deviceID, deviceType, true)
	if err != nil {
		httputil.LogThenErrorCtx(ctx, err)
	}

	dev, err := deviceDB.CreateDevice(
		ctx, device.UserID, deviceID, deviceType, &device.DisplayName, true, &mac, -1,
	)

	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create device: " + err.Error())
	}

	log.Infof("login success user %s device %s ip:%s token %s", dev.UserID, dev.ID, ip, token)
	pubLoginToken(dev.UserID, dev.ID, rpcCli)
	pubLoginInfo(dev.UserID, ip, device.DisplayName, "newtoken", cfg)
	return http.StatusOK, &external.PostLoginResponse{
		UserID:      dev.UserID,
		AccessToken: token,
		HomeServer:  domain,
		DeviceID:    dev.ID,
	}
}

func GetSuperAdminToken(
	ctx context.Context,
	deviceDB model.DeviceDatabase,
	device *authtypes.Device,
	tokenFilter *filter.Filter,
	idg *uid.UidGenerator,
	cfg config.Dendrite,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	rpcClient *common.RpcClient,
	rpcCli rpc.RpcClient,
	ip string,
) (int, core.Coder) {
	userID := "super_admin"
	domain := "super_domain"
	mac := "super-gen"
	displayName := "super_displayname"

	oldDevID, oldDevType, ts, err := deviceDB.CheckDevice(ctx, mac, userID)
	log.Debugf("super token, oldDevID: %s, oldDevtype: %s", oldDevID, oldDevType)

	if oldDevID != "" {
		token, err := common.BuildSuperAdminToken(cfg.Macaroon.Key, userID, domain, userID, mac, false, oldDevID, oldDevType, true, ts)
		if err != nil {
			httputil.LogThenErrorCtx(ctx, err)
		}

		log.Infof("login success super user %s get token %s", userID, token)
		return http.StatusOK, &external.PostLoginResponse{
			UserID:      userID,
			AccessToken: token,
			HomeServer:  domain,
			DeviceID:    oldDevID,
		}
	}

	deviceID, deviceType, err := common.BuildDevice(idg, &mac, true, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to get super device: " + err.Error())
	}

	//异步删db
	err = encryptDB.DeleteMacKeys(context.TODO(), deviceID, userID, mac)
	if err != nil {
		log.Errorf("Login remove device keys error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}
	err = syncDB.DeleteMacStdMessage(context.TODO(), mac, userID, deviceID)
	if err != nil {
		log.Errorf("Login remove std message error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	createTs := time.Now().UnixNano() / 1000000
	token, err := common.BuildSuperAdminToken(cfg.Macaroon.Key, userID, domain, userID, mac, false, deviceID, deviceType, true, createTs)
	if err != nil {
		httputil.LogThenErrorCtx(ctx, err)
	}

	dev, err := deviceDB.CreateDevice(
		ctx, userID, deviceID, deviceType, &displayName, true, &mac, createTs,
	)

	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create super device: " + err.Error())
	}

	log.Infof("login success super user %s device %s ip:%s gen token %s", userID, dev.ID, ip, token)
	pubLoginToken(userID, dev.ID, rpcCli)
	pubLoginInfo(userID, displayName, ip, "super", cfg)
	return http.StatusOK, &external.PostLoginResponse{
		UserID:      userID,
		AccessToken: token,
		HomeServer:  domain,
		DeviceID:    dev.ID,
	}
}
