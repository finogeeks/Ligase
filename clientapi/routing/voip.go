// Copyright 2017 Michael Telatysnki <7t3chguy@gmail.com>
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
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/plugins/message/external"
)

// RequestTurnServer implements:
//     GET /voip/turnServer
func RequestTurnServer(ctx context.Context, userID string, cfg config.Dendrite) (int, core.Coder) {
	turnConfig := cfg.TURN

	// TODO Guest Support
	if len(turnConfig.URIs) == 0 || turnConfig.UserLifetime == "" {
		return http.StatusOK, nil
	}

	// Duration checked at startup, err not possible
	duration, _ := time.ParseDuration(turnConfig.UserLifetime)

	resp := &external.PostVoipTurnServerResponse{
		URIs: turnConfig.URIs,
		TTL:  int(duration.Seconds()),
	}

	if turnConfig.SharedSecret != "" {
		expiry := time.Now().Add(duration).Unix()

		resp.Username = fmt.Sprintf("%d:%s", expiry, userID)

		mac := hmac.New(sha1.New, []byte(turnConfig.SharedSecret))
		_, err := mac.Write([]byte(resp.Username))

		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}

		resp.Password = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	} else if turnConfig.Username != "" && turnConfig.Password != "" {
		resp.Username = turnConfig.Username
		resp.Password = turnConfig.Password
	} else {
		return http.StatusOK, nil
	}

	return http.StatusOK, resp
}
