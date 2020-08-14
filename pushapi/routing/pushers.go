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
	"bytes"
	"container/list"
	"context"
	"github.com/finogeeks/ligase/common"
	"net/http"
	"sync/atomic"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func GetPushersByName(userID string, cache service.Cache, forRequest bool, static *pushapitypes.StaticObj) pushapitypes.Pushers {
	pushers := pushapitypes.Pushers{}
	pushers.Pushers = []pushapitypes.Pusher{}

	pusherIDs, ok := cache.GetUserPusherIds(userID)
	if static != nil {
		atomic.AddInt64(&static.PusherCount, 1)
	}
	if ok {
		for _, v := range pusherIDs {
			data, _ := cache.GetPusherCacheData(v)
			if static != nil {
				atomic.AddInt64(&static.PusherCount, 1)
			}
			if data != nil && data.UserName != "" {
				pusher := pushapitypes.Pusher{}
				pusher.UserName = data.UserName
				pusher.Lang = data.Lang
				pusher.PushKey = data.PushKey
				if !forRequest {
					pusher.PushKeyTs = data.PushKeyTs
				}
				pusher.AppDisplayName = data.AppDisplayName
				pusher.AppId = data.AppId
				pusher.Kind = data.Kind
				pusher.ProfileTag = data.ProfileTag
				pusher.DeviceDisplayName = data.DeviceDisplayName
				pusher.DeviceID = data.DeviceID
				var pusherData pushapitypes.PusherData
				json.Unmarshal([]byte(data.Data), &pusherData)
				pusher.Data = pusherData

				pushers.Pushers = append(pushers.Pushers, pusher)
			}
		}
	}

	return pushers
}

// GetPushers implements GET /_matrix/client/r0/pushers
func GetPushers(
	userID string,
	cache service.Cache,
) (int, core.Coder) {
	// if req.Method != http.MethodGet {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	pushers := GetPushersByName(userID, cache, true, nil)

	return http.StatusOK, &pushers
}

//PutPushers implements POST /_matrix/client/r0/pushers/set
func PutPusher(
	ctx context.Context,
	pushers *external.PostSetPushersRequest,
	pushDB model.PushAPIDatabase,
	device *authtypes.Device,
) (int, core.Coder) {
	// if req.Method != http.MethodPost {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	// var pushers pushapitypes.Pusher
	// if reqErr := httputil.UnmarshalJSONRequest(req, &pushers); reqErr != nil {
	// 	return *reqErr
	// }

	bytes, _ := json.Marshal(pushers)
	log.Infof("PutPusher user %s device %s kind:%s push_channel:%s append:%t pushers %s", device.UserID, device.ID, pushers.Kind, pushers.Data.PushChannel, pushers.Append, string(bytes))

	checkResult := CheckPusherBody(pushers)
	if checkResult != "" {
		return http.StatusBadRequest, jsonerror.MissingParam(checkResult)
	}

	if pushers.Kind == "" {
		// delete pushers
		log.Infof("PutPusher delete user %s device %s pushers %s", device.UserID, device.ID, string(bytes))
		if err := pushDB.DeleteUserPushers(ctx, device.UserID, pushers.AppID, pushers.Pushkey); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
		return http.StatusOK, nil
	} else {
		log.Infof("PutPusher user %s device %s kind:%s has kind", device.UserID, device.ID, pushers.Kind)
	}

	dataStr, err := json.Marshal(pushers.Data)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if pushers.Append {
		if err := pushDB.DeleteUserPushers(ctx, device.UserID, pushers.AppID, pushers.Pushkey); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	} else {
		if err := pushDB.DeletePushersByKey(ctx, pushers.AppID, pushers.Pushkey); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	}
	log.Infof("add PutPusher user %s device %s kind:%s push_channel:%s append:%t", device.UserID, device.ID, pushers.Kind, pushers.Data.PushChannel, pushers.Append)
	if err := pushDB.AddPusher(ctx, device.UserID, pushers.ProfileTag, pushers.Kind, pushers.AppID, pushers.AppDisplayName, pushers.DeviceDisplayName, pushers.Pushkey, pushers.Lang, dataStr, common.GetDeviceMac(device.ID)); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	return http.StatusOK, nil
}

func CheckPusherBody(pushers *external.PostSetPushersRequest) string {
	l := list.New()

	if pushers.Pushkey == "" {
		l.PushBack("pushkey")
	}

	if pushers.Kind != "" && pushers.Kind != "http" {
		l.PushBack("kind")
	}

	if pushers.AppID == "" {
		l.PushBack("app_id")
	}

	if pushers.AppDisplayName == "" {
		l.PushBack("app_display_name")
	}

	if pushers.DeviceDisplayName == "" {
		l.PushBack("device_display_name")
	}

	if pushers.Lang == "" {
		l.PushBack("lang")
	}

	// if pushers.Data.Format == "" {
	// 	l.PushBack("data.format")
	// }

	if pushers.Kind == "http" {
		if pushers.Data.URL == "" {
			l.PushBack("url")
		} else {
			// if v, ok := interface{}(pushers.Data).(map[string]interface{}); !ok {
			// 	l.PushBack("url")
			// } else {
			// 	var data map[string]interface{}
			// 	data = v
			// 	if _, ok := data["url"]; !ok {
			// 		l.PushBack("url")
			// 	}
			// }
		}
	}

	if l.Len() > 0 {
		var buf bytes.Buffer
		first := true
		buf.WriteString("Missing parameters: ")
		for e := l.Front(); e != nil; e = e.Next() {
			if first {
				buf.WriteString((e.Value).(string))
				first = false
			} else {
				buf.WriteString(",")
				buf.WriteString((e.Value).(string))
			}
		}
		return buf.String()
	}
	return ""
}

// GetUsersPushers implements POST /_matrix/client/r0/users/pushkey
func GetUsersPushers(
	users *external.PostUsersPushKeyRequest,
	cache service.Cache,
) (int, core.Coder) {
	// if req.Method != http.MethodPost {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	// var users pushapitypes.PusherUsers
	// if reqErr := httputil.UnmarshalJSONRequest(req, &users); reqErr != nil {
	// 	return *reqErr
	// }

	pushersRes := pushapitypes.PushersRes{}
	pushers := []pushapitypes.PusherRes{}
	pushersRes.PushersRes = pushers

	for _, user := range users.Users {
		pusherIDs, ok := cache.GetUserPusherIds(user)
		if ok {
			for _, pusherID := range pusherIDs {
				data, _ := cache.GetPusherCacheData(pusherID)
				if data != nil && data.UserName != "" {
					var pusher pushapitypes.PusherRes
					pusher.UserName = user
					pusher.AppId = data.AppId
					pusher.Kind = data.Kind
					pusher.PushKey = data.PushKey
					pusher.PushKeyTs = data.PushKeyTs
					pushersRes.PushersRes = append(pushersRes.PushersRes, pusher)
				}
			}
		}
	}

	return http.StatusOK, &pushersRes
}
