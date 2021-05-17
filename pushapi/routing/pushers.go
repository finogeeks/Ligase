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
	"net/http"
	"time"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
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
		pusher := data
		if forRequest {
			pusher.PushKeyTs = 0
		}
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
	resp := pushapitypes.PushersWitchInterfaceData{}
	for _, v := range pushers.Pushers {
		resp.Pushers = append(resp.Pushers, v.ToPusherWitchInterfaceData())
	}
	if resp.Pushers == nil {
		resp.Pushers = []pushapitypes.PusherWitchInterfaceData{}
	}
	return http.StatusOK, &resp
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
	dataStr, err := json.Marshal(pushers.Data)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	addPusher := pushapitypes.Pusher{
		UserName:          device.UserID,
		DeviceID:          common.GetDeviceMac(device.ID),
		PushKey:           pushers.Pushkey,
		PushKeyTs:         time.Now().Unix(),
		Kind:              pushers.Kind,
		AppId:             pushers.AppID,
		AppDisplayName:    pushers.AppDisplayName,
		DeviceDisplayName: pushers.DeviceDisplayName,
		ProfileTag:        pushers.ProfileTag,
		Lang:              pushers.Lang,
		Append:            pushers.Append,
		Data:              string(dataStr),
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

func rpcGetUserPushData(req *pushapitypes.ReqPushUsers, cfg config.Dendrite, pushDataRepo *repos.PushDataRepo, rpcClient *common.RpcClient) *pushapitypes.RespUsersPusher {
	if req.Slot == cfg.MultiInstance.Instance {
		return getUserPushDataFromLocal(req, pushDataRepo)
	} else {
		return getUserPushDataFromRemote(req, rpcClient)
	}
}

func getUserPushDataFromLocal(req *pushapitypes.ReqPushUsers, pushDataRepo *repos.PushDataRepo) *pushapitypes.RespUsersPusher {
	data := &pushapitypes.RespUsersPusher{
		Data: make(map[string][]pushapitypes.Pusher),
	}
	ctx := context.TODO()
	for _, user := range req.Users {
		pushersData, err := pushDataRepo.GetPusher(ctx, user)
		if err != nil {
			continue
		}
		data.Data[user] = pushersData
	}
	return data
}

func getUserPushDataFromRemote(req *pushapitypes.ReqPushUsers, rpcClient *common.RpcClient) *pushapitypes.RespUsersPusher {
	data := &pushapitypes.RespUsersPusher{
		Data: make(map[string][]pushapitypes.Pusher),
	}
	payload, err := json.Marshal(req)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Marshal payload:%+v err:%v", req, err)
		return data
	}
	request := pushapitypes.PushDataRequest{
		Payload: payload,
		ReqType: types.GET_PUSHER_BATCH,
		Slot:    req.Slot,
	}
	bt, err := json.Marshal(request)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Marshal request:%+v err:%v", req, err)
		return data
	}
	r, err := rpcClient.Request(types.PushDataTopicDef, bt, 15000)
	if err != nil {
		log.Error("getUserPushDataFromRemote rpc req:%+v err:%v", req, err)
		return data
	}
	resp := pushapitypes.RpcResponse{}
	err = json.Unmarshal(r, &resp)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Unmarshal RpcResponse err:%v", err)
		return data
	}
	err = json.Unmarshal(resp.Payload, &data)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Unmarshal Rpc payload err:%v", err)
		return data
	}
	return data
}

// GetUsersPushers implements POST /_matrix/client/r0/users/pushkey
func GetUsersPushers(
	ctx context.Context,
	users *external.PostUsersPushKeyRequest,
	pushDataRepo *repos.PushDataRepo,
	cfg config.Dendrite,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	pushersRes := pushapitypes.PushersRes{}
	pushers := []pushapitypes.PusherRes{}
	pushersRes.PushersRes = pushers
	slotMembers := make(map[uint32]*pushapitypes.ReqPushUsers)
	for _, member := range users.Users {
		slot := common.CalcStringHashCode(member) % cfg.MultiInstance.Total
		if _, ok := slotMembers[slot]; ok {
			slotMembers[slot].Users = append(slotMembers[slot].Users, member)
		} else {
			slotMembers[slot] = &pushapitypes.ReqPushUsers{
				Users: []string{member},
				Slot:  slot,
			}
		}
	}
	result := make(map[string][]pushapitypes.Pusher)
	for _, req := range slotMembers {
		if len(req.Users) <= 0 {
			continue
		}
		r := rpcGetUserPushData(req, cfg, pushDataRepo, rpcClient)
		if r != nil {
			for key, pushers := range r.Data {
				result[key] = pushers
			}
		}
	}
	for user, pushers := range result {
		for _, data := range pushers {
			pusher := pushapitypes.PusherRes{
				UserName:  user,
				AppId:     data.AppId,
				Kind:      data.Kind,
				PushKey:   data.PushKey,
				PushKeyTs: data.PushKeyTs,
			}
			pushersRes.PushersRes = append(pushersRes.PushersRes, pusher)
		}
	}
	return http.StatusOK, &pushersRes
}
