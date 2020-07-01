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

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/storage/model"
)

// handle /user/{userId}/[rooms/{roomId}/]account_data/{type}
// handle /user/{userID}/account_data/{type} with roomID empty
func SaveAccountData(
	ctx context.Context,
	accountDB model.AccountsDatabase,
	deviceID string,
	cfg config.Dendrite,
	userID,
	roomID,
	dataType string,
	content []byte,
	cache service.Cache,
) (int, core.Coder) {
	// if req.Method != http.MethodPut {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	// if userID != device.UserID {
	// 	return util.JSONResponse{
	// 		Code: http.StatusForbidden,
	// 		JSON: jsonerror.Forbidden("userID does not match the current user"),
	// 	}
	// }

	// defer req.Body.Close() // nolint: errcheck

	// body, err := ioutil.ReadAll(req.Body)
	// if err != nil {
	// 	return httputil.LogThenError(req, err)
	// }
	body := content

	log.Infof("SaveAccountData user %s device_id %s room_id %s data_type %s data %", userID, deviceID, roomID, dataType, string(body))

	data := new(types.ActDataStreamUpdate)
	data.UserID = userID
	data.RoomID = roomID
	data.DataType = dataType
	data.StreamType = "accountData"
	if roomID != "" {
		data.StreamType = "roomAccountData"
	}

	if err := cache.SetAccountData(userID, roomID, dataType, string(body)); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if err := accountDB.SaveAccountData(
		ctx, userID, roomID, dataType, string(body),
	); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if err := common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys: []byte(userID),
			Obj:  data,
		}); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	return http.StatusOK, nil
}
