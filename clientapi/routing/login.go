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
	"strings"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
)

func passwordLogin() *external.GetLoginResponse {
	f := &external.GetLoginResponse{}
	s := external.Flow{"m.login.password", []string{"m.login.password"}}
	f.Flows = append(f.Flows, s)
	return f
}

func providerLogin(
	userID string,
	ctx context.Context,
	r external.PostLoginRequest,
	cfg config.Dendrite,
	deviceDB model.DeviceDatabase,
	accountDB model.AccountsDatabase,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	admin bool,
	idg *uid.UidGenerator,
	tokenFilter *filter.Filter,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	if admin == true {
		if r.Password != cfg.Authorization.AuthorizeCode {
			return http.StatusUnauthorized, jsonerror.Unknown("password incorrect")
		}
	}

	_, err := accountDB.CreateAccount(ctx, userID, "", "", "")
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create account: " + err.Error())
	}

	devID := &r.DeviceID
	domain, _ := common.DomainFromID(userID)
	if r.DeviceID == "" {
		devID = nil
	}

	human := true
	if r.IsHuman != nil {
		human = *r.IsHuman
	}

	deviceID, deviceType, err := common.BuildDevice(idg, devID, human, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create device: " + err.Error())
	}

	mac := common.GetDeviceMac(deviceID)

	//异步删db
	err = encryptDB.DeleteMacKeys(context.TODO(), deviceID, userID, mac)
	if err != nil {
		log.Errorf("Login remove device keys error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}
	err = syncDB.DeleteMacStdMessage(context.TODO(), mac, userID, deviceID)
	if err != nil {
		log.Errorf("Login remove std message error, device: %s ,  user: %s , error: %v", deviceID, userID, err)
	}

	token, err := common.BuildToken(cfg.Macaroon.Key, r.User, domain, r.User, r.DeviceID, false, deviceID, deviceType, human)
	if err != nil {
		httputil.LogThenErrorCtx(ctx, err)
	}

	dev, err := deviceDB.CreateDevice(
		ctx, userID, deviceID, deviceType, r.InitialDisplayName, human, devID, -1,
	)

	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create device: " + err.Error())
	}

	log.Infof("login success user %s device %s token %s", dev.UserID, dev.ID, token)

	if cfg.PubLoginInfo {
		content := types.LoginInfoContent{
			UserID:      userID,
			DeviceID:    dev.ID,
			Token:       token,
			DisplayName: dev.DisplayName,
			Identifier:  dev.Identifier,
		}

		bytes, err := json.Marshal(content)
		if err == nil {
			log.Infof("pub login info %s", string(bytes))
			rpcClient.Pub(types.LoginTopicDef, bytes)
		} else {
			log.Errorf("pub login info  Marshal err %v", err)
		}
	}
	pubLoginToken(userID, deviceID, rpcClient)
	return http.StatusOK, &external.PostLoginResponse{
		UserID:      dev.UserID,
		AccessToken: token,
		HomeServer:  domain,
		DeviceID:    dev.ID,
	}
}

func pubLoginToken(userID string, deviceID string, rpcClient *common.RpcClient) {
	content := types.FilterTokenContent{
		UserID:     userID,
		DeviceID:   deviceID,
		FilterType: types.FILTERTOKENADD,
	}
	bytes, err := json.Marshal(content)
	if err == nil {
		log.Infof("pub login filter token info %s", string(bytes))
		rpcClient.Pub(types.FilterTokenTopicDef, bytes)
	} else {
		log.Errorf("pub login filter token info Marshal err %v", err)
	}
}

// Login implements GET and POST /login
func LoginPost(
	ctx context.Context,
	req *external.PostLoginRequest,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	cfg config.Dendrite,
	admin bool,
	idg *uid.UidGenerator,
	tokenFilter *filter.Filter,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	// var r external.PostLoginRequest
	// resErr := httputil.UnmarshalJSONRequest(req, &r)
	// if resErr != nil {
	// 	return *resErr
	// }
	//util.GetLogger(req.Context()).WithField("user", r.User).Info("Processing login request")

	// r.User can either be a user ID or just the userID... or other things maybe.
	localPart, domain, err := gomatrixserverlib.SplitID('@', req.User)
	if err != nil {
		return http.StatusBadRequest, jsonerror.InvalidUsername("User ID must be @localpart:domain")
	}

	if req.User == "" || localPart == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("'user' must be supplied.")
	}

	if common.CheckValidDomain(string(domain), cfg.Matrix.ServerName) == false {
		return http.StatusBadRequest, jsonerror.InvalidUsername("User ID not ours")
	}

	if strings.EqualFold(cfg.Authorization.AuthorizeMode, "provider") {
		return providerLogin(req.User, ctx, *req, cfg, deviceDB, accountDB, encryptDB, syncDB, admin, idg, tokenFilter, rpcClient)
	}

	return http.StatusServiceUnavailable, jsonerror.Unknown("Internal Server Error")
}

func LoginGet(
	ctx context.Context,
	req *external.GetLoginRequest,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	cfg config.Dendrite,
	admin bool,
	idg *uid.UidGenerator,
	tokenFilter *filter.Filter,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	// TODO: support other forms of login other than password, depending on config options
	return http.StatusOK, passwordLogin()
}
