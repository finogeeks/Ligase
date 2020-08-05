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

// Package auth implements authentication checks and storage.
package common

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func VerifyToken(token string, requestURI string, cache service.Cache, cfg config.Dendrite, devFilter *filter.SimpleFilter) (*authtypes.Device, *util.JSONResponse) {
	device, guest, _ := ExtractToken(cfg.Macaroon.Key, cfg.Macaroon.Id, cfg.Macaroon.Loc, token)

	if device != nil && device.ID == "" && guest == false {
		userId := device.UserID
		isAnonymous := false
		localPart, _, _ := gomatrixserverlib.SplitID('@', device.UserID)
		_, err := strconv.Atoi(localPart)
		if err == nil {
			isAnonymous = true
		}

		migToken, err := cache.GetMigTokenByToken(token)
		if err != nil || migToken == "" {
			if isAnonymous {
				device.IsHuman = true
				return device, nil
			}
			log.Infof("invalid token mig user：%s, req:%s", userId, requestURI)
			resErr := &util.JSONResponse{
				Code: 401,
				JSON: jsonerror.UnknownToken("Unknown token"),
			}
			return nil, resErr
		}
		device, _, _ = ExtractToken(cfg.Macaroon.Key, cfg.Macaroon.Id, cfg.Macaroon.Loc, migToken)
		if device == nil || device.ID == "" {
			log.Infof("guest token invalid:%s", migToken)
		}
		if isAnonymous {
			device.IsHuman = true
		}
	}
	//device = cache.GetDeviceByToken(token)
	if device == nil {
		log.Infof("invalid token device is nil,req: %s", requestURI)
		resErr := &util.JSONResponse{
			Code: 401,
			JSON: jsonerror.UnknownToken("Unknown token"),
		}
		return nil, resErr
	}
	if guest == false && devFilter != nil && !filterTokenCheck(device.UserID) {
		key := device.UserID + ":" + device.ID
		if !devFilter.Lookup(device.UserID, device.ID) {
			if strings.Contains(requestURI, "/_matrix/client/r0/voip/turnServer") {
				log.Warnf("cannot found turnserver token key:%s, req:%s", key, requestURI)
				return device, nil
			}
			log.Warnf("invalid token filter not found key:%s, req:%s", key, requestURI)
			if cache.CheckPwdChangeDevice(device.ID, device.UserID) {
				log.Infof("invalid token filter not found key:%s for pwd change, req:%s", key, requestURI)
				resErr := &util.JSONResponse{
					Code: 401,
					JSON: jsonerror.PwdChangeKick("kick password change device"),
				}
				cache.DelPwdChangeDevice(device.ID, device.UserID)
				return nil, resErr
			} else {
				log.Infof("invalid token filter not found key:%s, req:%s", key, requestURI)
				resErr := &util.JSONResponse{
					Code: 401,
					JSON: jsonerror.UnknownToken("Unknown token"),
				}
				return nil, resErr
			}
		}
	}
	return device, nil
}

func filterTokenCheck(userId string) bool {
	return strings.Contains(userId, "-qq:") || strings.Contains(userId, "-bot:") || strings.Contains(userId, "@qq_")
}

// extractAccessToken from a request, or return an error detailing what went wrong. The
// error message MUST be human-readable and comprehensible to the client.
func extractAccessToken(req *http.Request) (string, error) {
	// cf https://github.com/matrix-org/synapse/blob/v0.19.2/synapse/api/auth.py#L631
	queryToken := req.URL.Query().Get("access_token")
	if queryToken != "" {
		return queryToken, nil
	}

	authBearer := req.Header.Get("Authorization")
	if authBearer != "" {
		parts := strings.SplitN(authBearer, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return "", fmt.Errorf("invalid Authorization header")
		}
		return parts[1], nil
	}

	return "", fmt.Errorf("missing access token")
}

func ExtractAccessToken(req *http.Request) (string, error) {
	return extractAccessToken(req)
}
