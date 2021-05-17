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

package entry

import (
	"context"
	"fmt"
	"strings"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(model.CMD_FED_CLIENT_KEYS, QueryClientKeys)
	Register(model.CMD_FED_CLIENT_KEYS_CLAIM, ClaimClientKeys)
}

func QueryClientKeys(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	var reqParam external.PostQueryClientKeysRequest
	reqParam.Decode(msg.Body)

	resp := external.PostQueryClientKeysResponse{DeviceKeys: map[string]map[string]external.DeviceKeys{}}

	// query one's device key from user corresponding to uid
	for uid, arr := range reqParam.DeviceKeys {
		resp.DeviceKeys[uid] = make(map[string]external.DeviceKeys)
		deviceKeysQueryMap := resp.DeviceKeys[uid]
		// backward compatible to old interface
		midArr := []string{}
		// figure out device list from devices described as device which is actually deviceID
		if devices, ok := interface{}(arr).([]string); ok {
			for _, device := range devices {
				midArr = append(midArr, device)
			}
		}

		server, _ := common.DomainFromID(uid)
		if !common.CheckValidDomain(server, cfg.GetServerName()) {
			return &model.GobMessage{}, fmt.Errorf("userID is not ours")
		}

		if len(midArr) == 0 {
			devices := cache.GetDevicesByUserID(uid)
			for _, device := range *devices {
				midArr = append(midArr, device.ID)
			}
		}

		for _, device := range midArr {
			log.Infof("QueryPKeys for %s", device)
			deviceKeyIDs, ok := cache.GetDeviceKeyIDs(uid, device)
			if ok {
				for _, deviceKeyID := range deviceKeyIDs {
					log.Infof("QueryPKeys for %s", deviceKeyID)
					key, exists := cache.GetDeviceKey(deviceKeyID)
					if exists && key.UserID != "" {
						log.Infof("QueryPKeys for %s %s %s", key.DeviceID, key.UserID, key.KeyID)
						deviceKeysQueryMap = presetDeviceKeysQueryMap(deviceKeysQueryMap, uid, *key)
						// load for accomplishment
						single := deviceKeysQueryMap[key.DeviceID]
						resKey := fmt.Sprintf("%s:%s", key.KeyAlgorithm, key.DeviceID)
						resBody := key.Key
						single.Keys[resKey] = resBody
						single.DeviceID = key.DeviceID
						single.UserID = key.UserID
						single.Signatures[uid][fmt.Sprintf("%s:%s", "ed25519", key.DeviceID)] = key.Signature
						single.Algorithms = takeAL(key.UserID, key.DeviceID, cache)
						device := cache.GetDeviceByDeviceID(key.DeviceID, uid)
						if device != nil {
							single.Unsigned.DeviceDisplayName = device.DisplayName
						}
						deviceKeysQueryMap[key.DeviceID] = single
					}
				}
			}
		}
	}

	body, _ := resp.Encode()
	return &model.GobMessage{Body: body}, nil
}

func ClaimClientKeys(msg *model.GobMessage, cache service.Cache, rpcClient roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	var reqParam external.PostClaimClientKeysRequest
	reqParam.Decode(msg.Body)

	resp := external.PostClaimClientKeysResponse{OneTimeKeys: map[string]map[string]map[string]interface{}{}}

	content := reqParam.OneTimeKeys
	for uid, detail := range content {
		resp.OneTimeKeys[uid] = make(map[string]map[string]interface{})
		for deviceID, al := range detail {
			var alTyp int

			if strings.Contains(al, "signed") {
				alTyp = types.ONETIMEKEYOBJECT
			} else {
				alTyp = types.ONETIMEKEYSTRING
			}

			key := pickOne(cache, uid, deviceID)
			if key == nil || key.UserID == "" {
				continue
			}

			keyPreMap := resp.OneTimeKeys[uid]
			keymap := keyPreMap[deviceID]
			if keymap == nil {
				keymap = make(map[string]interface{})
			}

			switch alTyp {
			case types.ONETIMEKEYSTRING:
				keymap[fmt.Sprintf("%s:%s", al, key.KeyID)] = key.Key
			case types.ONETIMEKEYOBJECT:
				sig := make(map[string]map[string]string)
				sig[uid] = make(map[string]string)
				sig[uid][fmt.Sprintf("%s:%s", "ed25519", deviceID)] = key.Signature
				keymap[fmt.Sprintf("%s:%s", al, key.KeyID)] = types.KeyObject{Key: key.Key, Signature: sig}
			}
			resp.OneTimeKeys[uid][deviceID] = keymap

			content := types.KeyUpdateContent{
				Type:                     types.ONETIMEKEYUPDATE,
				OneTimeKeyChangeUserId:   uid,
				OneTimeKeyChangeDeviceId: deviceID,
			}
			ctx := context.Background()
			err := rpcCli.UpdateOneTimeKey(ctx, &content)
			if err != nil {
				log.Errorf("ClaimOneTimeKeys %s %s pub key update err %v", deviceID, uid, err)
				return &model.GobMessage{}, err
			}
		}
	}

	body, _ := resp.Encode()
	return &model.GobMessage{Body: body}, nil
}

func takeAL(
	uid,
	device string,
	cache service.Cache,
) []string {
	alStr, ok := cache.GetDeviceAlgorithm(uid, device)
	if ok && alStr.SupportedAlgorithm != "" {
		return strings.Split(alStr.SupportedAlgorithm, ",")
	}
	return []string{}
}

func presetDeviceKeysQueryMap(
	deviceKeysQueryMap map[string]external.DeviceKeys,
	uid string,
	key types.KeyHolder,
) map[string]external.DeviceKeys {
	// preset for complicated nested map struct
	if _, ok := deviceKeysQueryMap[key.DeviceID]; !ok {
		// make consistency
		deviceKeysQueryMap[key.DeviceID] = external.DeviceKeys{}
	}
	if deviceKeysQueryMap[key.DeviceID].Signatures == nil {
		mid := make(map[string]map[string]string)
		midMap := deviceKeysQueryMap[key.DeviceID]
		midMap.Signatures = mid
		deviceKeysQueryMap[key.DeviceID] = midMap
	}
	if deviceKeysQueryMap[key.DeviceID].Keys == nil {
		mid := make(map[string]string)
		midMap := deviceKeysQueryMap[key.DeviceID]
		midMap.Keys = mid
		deviceKeysQueryMap[key.DeviceID] = midMap
	}
	if _, ok := deviceKeysQueryMap[key.DeviceID].Signatures[uid]; !ok {
		// make consistency
		deviceKeysQueryMap[key.DeviceID].Signatures[uid] = make(map[string]string)
	}
	return deviceKeysQueryMap
}

func pickOne(
	cache service.Cache,
	uid, device string,
) *types.KeyHolder {
	keyIDs, ok := cache.GetOneTimeKeyIDs(uid, device)
	if ok {
		for _, keyID := range keyIDs {
			key, exists := cache.GetOneTimeKey(keyID)
			if exists {
				if key.UserID != "" {
					encryptionDB.OnDeleteOneTimeKey(context.TODO(), key.DeviceID, key.UserID, key.KeyID, key.KeyAlgorithm)
					cache.DeleteOneTimeKey(key.DeviceID, key.UserID, key.KeyID, key.KeyAlgorithm)
					return key
				}
			}
		}
	}

	return nil
}
