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
	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/json-iterator/go"
	"net/http"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func GetPushersByName(userID string, pushDataRepo *repos.PushDataRepo, forRequest bool, static *pushapitypes.StaticObj) pushapitypes.Pushers {
	ctx := context.TODO()
	pushers := pushapitypes.Pushers{}
	pushers.Pushers = []pushapitypes.Pusher{}
	pushersData, err := pushDataRepo.GetPusher(ctx, userID)
	if err != nil {
		return pushers
	}
	for _, data := range pushersData {
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
		json.Unmarshal([]byte(data.Data.(string)), &pusherData)
		pusher.Data = pusherData
		pushers.Pushers = append(pushers.Pushers, pusher)
	}
	return pushers
}

// GetPushers implements GET /_matrix/client/r0/pushers
func GetPushers(
	userID string,
	pushDataRepo *repos.PushDataRepo,
) (int, core.Coder) {
	pushers := GetPushersByName(userID, pushDataRepo, true, nil)
	return http.StatusOK, &pushers
}

//PutPushers implements POST /_matrix/client/r0/pushers/set
func PutPusher(
	ctx context.Context,
	pushers *external.PostSetPushersRequest,
	pushDB model.PushAPIDatabase,
	pushDataRepo *repos.PushDataRepo,
	device *authtypes.Device,
) (int, core.Coder) {
	log.Infof("PutPusher user %s device %s pushers:%+v", device.UserID, device.ID, pushers)
	checkResult := CheckPusherBody(pushers)
	if checkResult != "" {
		return http.StatusBadRequest, jsonerror.MissingParam(checkResult)
	}
	// delete pushers
	if pushers.Kind == "" {
		log.Infof("PutPusher delete user %s device %s kind is empty", device.UserID, device.ID, pushers)
		if err := pushDataRepo.DeleteUserPusher(ctx, device.UserID, pushers.AppID, pushers.Pushkey); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
		return http.StatusOK, nil
	} else {
		log.Infof("PutPusher user %s device %s kind:%s has kind", device.UserID, device.ID, pushers.Kind)
	}
	addPusher := pushapitypes.Pusher{
		UserName: device.UserID,
		DeviceID: common.GetDeviceMac(device.ID),
		PushKey: pushers.Pushkey,
		PushKeyTs: time.Now().Unix(),
		Kind: pushers.Kind,
		AppId: pushers.AppID,
		AppDisplayName: pushers.AppDisplayName,
		DeviceDisplayName: pushers.DeviceDisplayName,
		ProfileTag: pushers.ProfileTag,
		Lang: pushers.Lang,
		Append: pushers.Append,
		Data: pushers.Data,
	}
	if pushers.Append {
		if err := pushDataRepo.DeleteUserPusher(ctx, device.UserID, pushers.AppID, pushers.Pushkey); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	} else {
		if err := pushDataRepo.DeletePushersByKey(ctx, pushers.AppID, pushers.Pushkey); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	}
	if err := pushDataRepo.AddPusher(ctx, addPusher, true); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	log.Infof("AddPusher succ user %s device %s pusher:%+v", device.UserID, device.ID, pushers)
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

	if pushers.Kind == "http" && pushers.Data.URL == "" {
		l.PushBack("url")
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
