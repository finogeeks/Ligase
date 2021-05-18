// Copyright 2018 Vector Creations Ltd
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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
)

// UploadPKeys this function is for user upload his device key, and one-time-key to a limit at 50 set as default
func UploadPKeys(
	ctx context.Context,
	keyBody *types.UploadEncrypt,
	encryptionDB model.EncryptorAPIDatabase,
	device *authtypes.Device,
	cache service.Cache,
	rpcClient *common.RpcClient,
	syncDB model.SyncAPIDatabase,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	userID := device.UserID

	if !common.IsActualDevice(device.DeviceType) {
		return http.StatusOK, &external.PostUploadKeysResponse{
			OneTimeKeyCounts: common.DefaultKeyCount(),
		}
	}

	// var keyBody types.UploadEncrypt
	// if reqErr := httputil.UnmarshalJSONRequest(req, &keyBody); reqErr != nil {
	// 	return *reqErr
	// }
	keySpecific := turnSpecific(keyBody)
	// persist keys into encryptionDB
	err := persistKeys(ctx, encryptionDB, &keySpecific, userID, device.ID, cache, rpcClient, syncDB, idg)
	// numMap is algorithm-num map
	numMap := (QueryOneTimeKeys(userID, device.ID, cache)).(map[string]int)

	//bytes, _ := json.Marshal(numMap)
	//log.Errorf("one time key count for key upload: user:%s device:%s result:%s", userID, deviceID, string(bytes))

	if err != nil {
		return http.StatusBadGateway, &external.PostUploadKeysResponse{
			OneTimeKeyCounts: numMap,
		}
	}
	return http.StatusOK, &external.PostUploadKeysResponse{
		OneTimeKeyCounts: numMap,
	}
}

// QueryPKeys this function is for user query other's device key
func QueryPKeys(
	ctx context.Context,
	queryRq *external.PostQueryKeysRequest,
	deviceID string,
	cache service.Cache,
	federation *gomatrixserverlib.FederationClient,
	serverName []string,
) (int, core.Coder) {
	var err error
	// var queryRq types.QueryRequest
	queryRp := &external.PostQueryKeysResponse{}
	queryRp.Failures = make(map[string]interface{})
	queryRp.DeviceKeys = make(map[string]map[string]external.DeviceKeys)
	// if reqErr := httputil.UnmarshalJSONRequest(req, &queryRq); reqErr != nil {
	// 	return *reqErr
	// }

	/*
		federation consideration: when user id is in federation, a
		query is needed to ask fed for keys.
		domain --------+ fed (keys)
		domain +--tout-- timer
	*/

	// todo: Add federation processing at specific userID.
	if false /*federation judgement*/ {
		tout := queryRq.TimeOut
		if tout == 0 {
			tout = int64(10 * time.Second)
		}
		stimuCh := make(chan int)
		go func() {
			time.Sleep(time.Duration(tout) * 1000 * 1000)
			close(stimuCh)
		}()
		select {
		case <-stimuCh:
			queryRp.Failures = make(map[string]interface{})
			// todo: key in this map is restricted to username at the end, yet a mocked one.
			queryRp.Failures["@alice:localhost"] = "ran out of offered time"
		case <-make(chan interface{}):
			// todo : here goes federation chan , still a mocked one
		}
	}

	log.Infow("Query Other Users DeviceKey", log.KeysAndValues{"my device", deviceID, "target userIDs", queryRq.DeviceKeys})

	// query one's device key from user corresponding to uid
	for uid, arr := range queryRq.DeviceKeys {
		queryRp.DeviceKeys[uid] = make(map[string]external.DeviceKeys)
		deviceKeysQueryMap := queryRp.DeviceKeys[uid]
		// backward compatible to old interface
		midArr := []string{}
		// figure out device list from devices described as device which is actually deviceID
		if devices, ok := interface{}(arr).([]string); ok {
			for _, device := range devices {
				midArr = append(midArr, device)
			}
		}

		server, _ := common.DomainFromID(uid)

		if len(midArr) == 0 {
			devices := cache.GetDevicesByUserID(uid)
			for _, device := range *devices {
				midArr = append(midArr, device.ID)
			}
		}

		/* federation consideration */
		if common.CheckValidDomain(server, serverName) == false {
			umap := make(map[string][]string)
			umap[uid] = midArr
			rq := &gomatrixserverlib.QueryRequest{
				umap,
			}
			federation.LookupDeviceKeys(ctx, gomatrixserverlib.ServerName(server), rq)
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
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	return http.StatusOK, queryRp
}

// ClaimOneTimeKeys claim for one time key that may be used in session exchange in olm encryption
func ClaimOneTimeKeys(
	ctx context.Context,
	claimRq *external.PostClaimKeysRequest,
	cache service.Cache,
	encryptionDB model.EncryptorAPIDatabase,
	rpcClient *common.RpcClient,
) (int, core.Coder) {
	// var claimRq types.ClaimRequest
	claimRp := &external.PostClaimKeysResponse{}
	claimRp.Failures = make(map[string]interface{})
	claimRp.DeviceKeys = make(map[string]map[string]map[string]interface{})
	// if reqErr := httputil.UnmarshalJSONRequest(req, &claimRq); reqErr != nil {
	// 	return *reqErr
	// }

	/*
		federation consideration: when user id is in federation, a query is needed to ask fed for keys
		domain --------+ fed (keys)
		domain +--tout-- timer
	*/
	// todo: Add federation processing at specific userID.
	if false /*federation judgement*/ {
		tout := claimRq.TimeOut
		stimuCh := make(chan int)
		go func() {
			time.Sleep(time.Duration(tout) * 1000 * 1000)
			close(stimuCh)
		}()
		select {
		case <-stimuCh:
			claimRp.Failures = make(map[string]interface{})
			// todo: key in this map is restricted to username at the end, yet a mocked one.
			claimRp.Failures["@alice:localhost"] = "ran out of offered time"
		case <-make(chan interface{}):
			// todo : here goes federation chan , still a mocked one
		}
	}

	content := claimRq.OneTimeKeys
	for uid, detail := range content {
		claimRp.DeviceKeys[uid] = make(map[string]map[string]interface{})
		for deviceID, al := range detail {
			var alTyp int

			if strings.Contains(al, "signed") {
				alTyp = types.ONETIMEKEYOBJECT
			} else {
				alTyp = types.ONETIMEKEYSTRING
			}

			key := pickOne(cache, uid, deviceID, encryptionDB)
			if key == nil || key.UserID == "" {
				continue
			}

			keyPreMap := claimRp.DeviceKeys[uid]
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
			claimRp.DeviceKeys[uid][deviceID] = keymap

			content := types.KeyUpdateContent{
				Type:                     types.ONETIMEKEYUPDATE,
				OneTimeKeyChangeUserId:   uid,
				OneTimeKeyChangeDeviceId: deviceID,
			}
			bytes, err := json.Marshal(content)
			if err == nil {
				rpcClient.Pub(types.KeyUpdateTopicDef, bytes)
			} else {
				log.Errorf("ClaimOneTimeKeys pub key update err %v", err)
				return httputil.LogThenErrorCtx(ctx, err)
			}
		}
	}

	return http.StatusOK, claimRp
}

// todo: check through interface for duplicate and what type of request should it be
// whether device or one time or both of them
func checkUpload(req *types.UploadEncryptSpecific, typ int) bool {
	if typ == types.BODYDEVICEKEY {
		devicekey := req.DeviceKeys
		if devicekey.UserID == "" {
			return false
		}
	}
	if typ == types.BODYONETIMEKEY {
		if req.OneTimeKey.KeyString == nil || req.OneTimeKey.KeyObject == nil {
			return false
		}
	}
	return true
}

// QueryOneTimeKeys todo: complete this field through claim type
func QueryOneTimeKeys(
	userID, deviceID string,
	cache service.Cache,
) interface{} {
	result := common.InitKeyCount()
	keyIDs, ok := cache.GetOneTimeKeyIDs(userID, deviceID)
	if ok {
		for _, keyID := range keyIDs {
			key, exists := cache.GetOneTimeKey(keyID)
			if exists && key.UserID != "" {
				if _, ok := result[key.KeyAlgorithm]; ok {
					count := result[key.KeyAlgorithm] + 1
					result[key.KeyAlgorithm] = count
				} else {
					result[key.KeyAlgorithm] = 1
				}
			}
		}
	}
	return result

}

// persist both device keys and one time keys
func persistKeys(
	ctx context.Context,
	database model.EncryptorAPIDatabase,
	body *types.UploadEncryptSpecific,
	userID,
	deviceID string,
	cache service.Cache,
	rpcClient *common.RpcClient,
	syncDB model.SyncAPIDatabase,
	idg *uid.UidGenerator,
) (err error) {
	// in order to persist keys , a check filtering duplicate should be processed
	// true stands for counterparts are in request
	// situation 1: only device keys
	// situation 2: both device keys and one time keys
	// situation 3: only one time keys
	mac := common.GetDeviceMac(deviceID)
	if checkUpload(body, types.BODYDEVICEKEY) {
		err = database.OnDeleteDeviceOneTimeKey(ctx, deviceID, userID)
		if err != nil {
			return
		}

		err = cache.DeleteDeviceOneTimeKey(userID, deviceID)
		if err != nil {
			return
		}

		deviceKeys := body.DeviceKeys
		al := deviceKeys.Algorithm

		err = cache.SetDeviceAlgorithm(userID, deviceID, strings.Join(al, ","))
		if err != nil {
			return
		}

		err = persistAl(ctx, database, userID, deviceID, mac, al)
		if err != nil {
			return
		}

		if checkUpload(body, types.BODYONETIMEKEY) {
			if err = bothKeyProcess(ctx, body, userID, deviceID, database, deviceKeys, cache); err != nil {
				return
			}
		} else {
			if err = deviceKeyProcess(ctx, userID, deviceID, database, deviceKeys, cache); err != nil {
				return
			}
		}

		offset, _ := idg.Next()
		err := syncDB.InsertKeyChange(ctx, userID, offset)
		if err != nil {
			return err
		}

		content := types.KeyUpdateContent{
			Type: types.DEVICEKEYUPDATE,
			DeviceKeyChanges: []types.DeviceKeyChanges{
				{
					ChangedUserID: userID,
					Offset:        offset,
				},
			},
		}
		bytes, err := json.Marshal(content)
		if err == nil {
			rpcClient.Pub(types.KeyUpdateTopicDef, bytes)
		} else {
			return err
		}

		content = types.KeyUpdateContent{
			Type:                     types.ONETIMEKEYUPDATE,
			OneTimeKeyChangeUserId:   userID,
			OneTimeKeyChangeDeviceId: deviceID,
		}
		bytes, err = json.Marshal(content)
		if err == nil {
			rpcClient.Pub(types.KeyUpdateTopicDef, bytes)
		} else {
			return err
		}
	} else {
		if checkUpload(body, types.BODYONETIMEKEY) {
			if err = otmKeyProcess(ctx, body, userID, deviceID, database, cache); err != nil {
				return
			}
			content := types.KeyUpdateContent{
				Type:                     types.ONETIMEKEYUPDATE,
				OneTimeKeyChangeUserId:   userID,
				OneTimeKeyChangeDeviceId: deviceID,
			}
			bytes, err := json.Marshal(content)
			if err == nil {
				rpcClient.Pub(types.KeyUpdateTopicDef, bytes)
			} else {
				return err
			}
		} else {
			return errors.New("failed to touch keys")
		}
	}
	return err
}

// make keys instantiated to specific struct from keybody interface{}
func turnSpecific(
	cont *types.UploadEncrypt,
) (spec types.UploadEncryptSpecific) {
	// both device keys are coordinate
	spec.DeviceKeys = cont.DeviceKeys
	spec.OneTimeKey.KeyString = make(map[string]string)
	spec.OneTimeKey.KeyObject = make(map[string]types.KeyObject)
	mapStringInterface := cont.OneTimeKey
	for key, val := range mapStringInterface {
		value, ok := val.(string)
		if ok {
			spec.OneTimeKey.KeyString[key] = value
		} else {
			valueObject := types.KeyObject{}
			target, _ := json.Marshal(val)
			err := json.Unmarshal(target, &valueObject)
			if err != nil {
				continue
			}
			spec.OneTimeKey.KeyObject[key] = valueObject
		}
	}
	return
}

func persistAl(
	ctx context.Context,
	encryptDB model.EncryptorAPIDatabase,
	uid, device, identifier string,
	al []string,
) (err error) {
	err = encryptDB.InsertAl(ctx, uid, device, strings.Join(al, ","), identifier)
	return
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

func pickOne(
	cache service.Cache,
	uid, device string,
	encryptionDB model.EncryptorAPIDatabase,
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

func bothKeyProcess(
	ctx context.Context,
	body *types.UploadEncryptSpecific,
	userID, deviceID string,
	database model.EncryptorAPIDatabase,
	deviceKeys types.DeviceKeys,
	cache service.Cache,
) (err error) {
	// insert one time keys firstly
	onetimeKeys := body.OneTimeKey
	mac := common.GetDeviceMac(deviceID)
	for alKeyID, val := range onetimeKeys.KeyString {
		al := (strings.Split(alKeyID, ":"))[0]
		keyID := (strings.Split(alKeyID, ":"))[1]
		keyInfo := val
		sig := ""

		err = cache.SetOneTimeKey(userID, deviceID, keyID, keyInfo, al, sig)
		if err != nil {
			return
		}

		err = database.InsertOneTimeKey(ctx, deviceID, userID, keyID, keyInfo, al, sig, mac)
		if err != nil {
			return
		}
	}
	for alKeyID, val := range onetimeKeys.KeyObject {
		al := (strings.Split(alKeyID, ":"))[0]
		keyID := (strings.Split(alKeyID, ":"))[1]
		keyInfo := val.Key
		sig := val.Signature[userID][fmt.Sprintf("%s:%s", "ed25519", deviceID)]

		err = cache.SetOneTimeKey(userID, deviceID, keyID, keyInfo, al, sig)
		if err != nil {
			return
		}

		err = database.InsertOneTimeKey(ctx, deviceID, userID, keyID, keyInfo, al, sig, mac)
		if err != nil {
			return
		}
	}
	// insert device keys
	keys := deviceKeys.Keys
	sigs := deviceKeys.Signature
	for alDevice, key := range keys {
		al := (strings.Split(alDevice, ":"))[0]
		keyInfo := key
		sig := sigs[userID][fmt.Sprintf("%s:%s", "ed25519", deviceID)]

		err = cache.SetDeviceKey(userID, deviceID, keyInfo, al, sig)
		if err != nil {
			return
		}

		err = database.InsertDeviceKey(ctx, deviceID, userID, keyInfo, al, sig, mac)
		if err != nil {
			return
		}
	}
	return
}

func deviceKeyProcess(
	ctx context.Context,
	userID, deviceID string,
	database model.EncryptorAPIDatabase,
	deviceKeys types.DeviceKeys,
	cache service.Cache,
) (err error) {
	keys := deviceKeys.Keys
	sigs := deviceKeys.Signature
	mac := common.GetDeviceMac(deviceID)
	for alDevice, key := range keys {
		al := (strings.Split(alDevice, ":"))[0]
		keyInfo := key
		sig := sigs[userID][fmt.Sprintf("%s:%s", "ed25519", deviceID)]

		err = cache.SetDeviceKey(userID, deviceID, keyInfo, al, sig)
		if err != nil {
			return
		}

		err = database.InsertDeviceKey(ctx, deviceID, userID, keyInfo, al, sig, mac)
		if err != nil {
			return
		}
	}
	return
}

func otmKeyProcess(
	ctx context.Context,
	body *types.UploadEncryptSpecific,
	userID, deviceID string,
	database model.EncryptorAPIDatabase,
	cache service.Cache,
) (err error) {
	onetimeKeys := body.OneTimeKey
	mac := common.GetDeviceMac(deviceID)
	for alKeyID, val := range onetimeKeys.KeyString {
		al := (strings.Split(alKeyID, ":"))[0]
		keyID := (strings.Split(alKeyID, ":"))[1]
		keyInfo := val
		sig := ""

		err = cache.SetOneTimeKey(userID, deviceID, keyID, keyInfo, al, sig)
		if err != nil {
			return
		}

		err = database.InsertOneTimeKey(ctx, deviceID, userID, keyID, keyInfo, al, sig, mac)
		if err != nil {
			return
		}
	}
	for alKeyID, val := range onetimeKeys.KeyObject {
		al := (strings.Split(alKeyID, ":"))[0]
		keyID := (strings.Split(alKeyID, ":"))[1]
		keyInfo := val.Key
		sig := val.Signature[userID][fmt.Sprintf("%s:%s", "ed25519", deviceID)]

		err = cache.SetOneTimeKey(userID, deviceID, keyID, keyInfo, al, sig)
		if err != nil {
			return
		}

		err = database.InsertOneTimeKey(ctx, deviceID, userID, keyID, keyInfo, al, sig, mac)
		if err != nil {
			return
		}
	}
	return
}
