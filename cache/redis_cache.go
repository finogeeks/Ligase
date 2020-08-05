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

package cache

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/finogeeks/ligase/model/authtypes"
	push "github.com/finogeeks/ligase/model/pushapitypes"
	e2e "github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/gomodule/redigo/redis"
)

type RedisCache struct {
	pools    []*redis.Pool
	poolSize int
}

type DeviceInfo struct {
	id            string
	userID        string
	createdTime   string
	displayName   string
	deviceType    string
	identifier    string
	lastTouchTime int64
}

func (rc *RedisCache) Prepare(cfg []string) (err error) {
	rc.poolSize = len(cfg)
	rc.pools = make([]*redis.Pool, rc.poolSize)
	for i := 0; i < rc.poolSize; i++ {
		addr := cfg[i]
		rc.pools[i] = &redis.Pool{
			MaxIdle:     200,
			MaxActive:   200,
			Wait:        true,
			IdleTimeout: 240 * time.Second,
			Dial:        func() (redis.Conn, error) { return redis.DialURL(addr) },
		}
	}
	return err
}

func (rc *RedisCache) pool() *redis.Pool {
	slot := rand.Int() % rc.poolSize
	return rc.pools[slot]
}

func (rc *RedisCache) SafeDo(commandName string, args ...interface{}) (reply interface{}, err error) {
	conn := rc.pool().Get()
	defer conn.Close()
	reply, err = conn.Do(commandName, args...)
	return reply, err
}

func (rc *RedisCache) GetMigTokenByToken(token string) (string, error) {
	key := fmt.Sprintf("%s:%s", "tokens", token)

	migToken, err := redis.String(rc.SafeDo("get", key))
	if err != nil {
		return "", err
	}
	return migToken, nil
}

func (rc *RedisCache) GetDeviceByDeviceID(deviceID string, userID string) *authtypes.Device {
	key := fmt.Sprintf("%s:%s:%s", "device", userID, deviceID)
	var device *DeviceInfo
	reply, err := redis.Values(rc.SafeDo("hmget", key, "did", "user_id", "created_ts", "display_name", "device_type", "identifier"))
	if err != nil {
		log.Warnf("cache missed for did:%s uid:%s err:%v", deviceID, userID, err)
		device = nil
	} else {
		device = &DeviceInfo{}
		reply, err = redis.Scan(reply, &device.id, &device.userID, &device.createdTime, &device.displayName, &device.deviceType, &device.identifier)
		if err != nil {
			device = nil
		} else if device.id == "" {
			device = nil
		} else {
			device.lastTouchTime = time.Now().Unix()
		}
	}
	if device != nil && device.userID != "" {
		isHuman := true
		if device.deviceType == "bot" {
			isHuman = false
		}
		return &authtypes.Device{
			ID:           device.id,
			UserID:       device.userID,
			DisplayName:  device.displayName,
			DeviceType:   device.deviceType,
			IsHuman:      isHuman,
			Identifier:   device.identifier,
			LastActiveTs: device.lastTouchTime,
		}
	}
	return nil
}

func (rc *RedisCache) GetDevicesByUserID(userID string) *[]authtypes.Device {
	devices := []authtypes.Device{}
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s", "device_list", userID)))
	if err != nil {
		log.Warnw("cache missed for device list", log.KeysAndValues{"userID", userID, "err", err})
		return &devices
	}

	deviceMap := make(map[string]bool)
	for _, deviceID := range result {
		did := deviceID.([]byte)
		deviceMap[string(did)] = true
	}

	for did := range deviceMap {
		device := rc.GetDeviceByDeviceID(did, userID)
		if device != nil && device.UserID != "" && device.DeviceType == "actual" {
			devices = append(devices, *device)
		}
	}

	return &devices
}

func (rc *RedisCache) SetPwdChangeDevcie(deviceID, userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	keyKey := fmt.Sprintf("%s:%s", "device_pwd_change", userID)
	err := conn.Send("hmset", keyKey, deviceID, "1")
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) CheckPwdChangeDevice(deviceID, userID string) bool {
	conn := rc.pool().Get()
	defer conn.Close()
	key := fmt.Sprintf("%s:%s", "device_pwd_change", userID)
	isExist, err := rc.SafeDo("hexists", key, deviceID)
	if err != nil {
		return false
	} else {
		return isExist.(int64) == 1
	}
	return false
}

func (rc *RedisCache) DelPwdChangeDevice(deviceID, userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	key := fmt.Sprintf("%s:%s", "device_pwd_change", userID)
	err := conn.Send("hdel", key, deviceID)
	if err != nil {
		return err
	}
	conn.Flush()
	return nil
}

func (rc *RedisCache) ExpirePwdChangeDevice(userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	key := fmt.Sprintf("%s:%s", "device_pwd_change", userID)
	// err := conn.Send("expire", key, fmt.Sprintf("%d", 30*24*3600))
	err := conn.Send("expire", key, strconv.Itoa(30*24*3600)) // faster than fmt.Sprintf
	if err != nil {
		return err
	}
	conn.Flush()
	return nil
}

func (rc *RedisCache) GetPushRuleEnabled(userID, ruleID string) (string, bool) {
	context, err := redis.String(rc.SafeDo("hget", fmt.Sprintf("%s:%s:%s", "push_rule_enable", userID, ruleID), "enabled"))
	if err != nil {
		log.Debugw("rule enabled not cached", log.KeysAndValues{"userID", userID, "ruleID", ruleID, "error", err})
		return "", false
	}

	return context, true
}

func (rc *RedisCache) GetUserPusherIds(userID string) ([]string, bool) {
	var pushers []string
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s", "pusher_user_list", userID)))
	if err != nil {
		return []string{}, false
	} else {
		pusherMap := make(map[string]bool)

		for _, pusherID := range result {
			pid := pusherID.([]byte)
			pusherMap[string(pid)] = true
		}

		for pid := range pusherMap {
			pushers = append(pushers, pid)
		}
	}

	return pushers, true
}

func (rc *RedisCache) GetUserPushRuleIds(userID string) ([]string, bool) {
	var rules []string
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s", "push_rule_user_list", userID)))
	if err != nil {
		return []string{}, false
	} else {
		ruleMap := make(map[string]bool)

		for _, ruleID := range result {
			rid := ruleID.([]byte)
			ruleMap[string(rid)] = true
		}

		for rid := range ruleMap {
			rules = append(rules, rid)
		}
	}

	return rules, true
}

func (rc *RedisCache) GetPusherCacheData(pusherKey string) (pusher *push.PusherCacheData, ok bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", pusherKey, "user_name", "profile_tag", "kind", "app_id",
		"app_display_name", "device_display_name", "push_key", "push_key_ts", "lang", "push_data", "device_id"))
	ok = true
	if err != nil {
		log.Errorw("cache missed for pusher", log.KeysAndValues{"rule_key", pusherKey, "error", err})
		ok = false
	} else {
		pusher = &push.PusherCacheData{}
		reply, err = redis.Scan(reply, &pusher.UserName, &pusher.ProfileTag, &pusher.Kind, &pusher.AppId,
			&pusher.AppDisplayName, &pusher.DeviceDisplayName, &pusher.PushKey, &pusher.PushKeyTs, &pusher.Lang, &pusher.Data, &pusher.DeviceID)
		if err != nil {
			log.Errorw("Scan error for pusher", log.KeysAndValues{"rule_key", pusherKey, "error", err})
			ok = false
		}
	}
	return
}

func (rc *RedisCache) GetPushRuleCacheData(ruleKey string) (pushRule *push.PushRuleCacheData, ok bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", ruleKey, "user_name", "rule_id", "priority_class", "priority", "conditions", "actions"))
	ok = true
	if err != nil {
		log.Errorw("cache missed for push rule", log.KeysAndValues{"rule_key", ruleKey, "error", err})
		ok = false
	} else {
		pushRule = &push.PushRuleCacheData{}
		reply, err = redis.Scan(reply, &pushRule.UserName, &pushRule.RuleId, &pushRule.PriorityClass, &pushRule.Priority, &pushRule.Conditions, &pushRule.Actions)
		if err != nil {
			log.Errorw("Scan error for push rule", log.KeysAndValues{"rule_key", ruleKey, "error", err})
			ok = false
		}
	}
	return
}

func (rc *RedisCache) GetPushRuleCacheDataByID(userID, ruleID string) (*push.PushRuleCacheData, bool) {
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule", userID, ruleID)
	return rc.GetPushRuleCacheData(ruleKey)
}

func (rc *RedisCache) GetAccountFilterById(userID, filterID string) (string, bool) {
	context, err := redis.String(rc.SafeDo("hget", fmt.Sprintf("%s:%s:%s", "filter", userID, filterID), "filter"))
	if err != nil {
		log.Debugw("filter not cached", log.KeysAndValues{"userID", userID, "filter_id", filterID, "error", err})
		return "", false
	}

	return context, true
}

func (rc *RedisCache) GetUserRoomTagIds(userName string) ([]string, bool) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s", "user_tags_list", userName)))
	if err != nil {
		log.Warnw("cache missed for room tag list", log.KeysAndValues{"userName", userName, "err", err})
		return []string{}, false
	} else {
		var tags []string
		tagMap := make(map[string]bool)

		for _, tag := range result {
			tagID := tag.([]byte)
			tagMap[string(tagID)] = true
		}

		for tagID := range tagMap {
			tags = append(tags, tagID)
		}

		return tags, true
	}
}

func (rc *RedisCache) GetRoomTagCacheData(tagKey string) (result *authtypes.RoomTagCacheData, ok bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", tagKey, "user_id", "room_id", "tag", "content"))
	ok = true
	if err != nil {
		log.Errorw("cache missed for room tag", log.KeysAndValues{"tag_key", tagKey, "error", err})
		ok = false
	} else {
		result = &authtypes.RoomTagCacheData{}
		reply, err = redis.Scan(reply, &result.UserID, &result.RoomID, &result.Tag, &result.Content)
		if err != nil {
			log.Errorw("Scan error for room tag", log.KeysAndValues{"tag_key", tagKey, "error", err})
			ok = false
		}
	}
	return
}

func (rc *RedisCache) GetUserAccountDataIds(userName string) ([]string, bool) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s", "account_data_list", userName)))
	if err != nil {
		log.Warnw("cache missed for user account data list", log.KeysAndValues{"userName", userName, "err", err})
		return []string{}, false
	} else {
		var ids []string
		idMap := make(map[string]bool)
		for _, id := range result {
			actDataID := id.([]byte)
			idMap[string(actDataID)] = true
		}

		for actDataID := range idMap {
			ids = append(ids, actDataID)
		}
		return ids, true
	}
}

func (rc *RedisCache) GetAccountDataCacheData(accountDataKey string) (result *authtypes.AccountDataCacheData, ok bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", accountDataKey, "user_id", "type", "content"))
	ok = true
	if err != nil {
		log.Errorw("cache missed for account data", log.KeysAndValues{"account_data_key", accountDataKey, "error", err})
		ok = false
	} else {
		result = &authtypes.AccountDataCacheData{}
		reply, err = redis.Scan(reply, &result.UserID, &result.Type, &result.Content)
		if err != nil {
			log.Errorw("Scan error for account data", log.KeysAndValues{"account_data_key", accountDataKey, "error", err})
			ok = false
		}
	}
	return
}

func (rc *RedisCache) GetUserRoomAccountDataIds(userName string) ([]string, bool) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s", "room_account_data_list", userName)))
	if err != nil {
		log.Warnw("cache missed for user account data list", log.KeysAndValues{"userName", userName, "err", err})
		return []string{}, false
	} else {
		var ids []string
		idMap := make(map[string]bool)
		for _, id := range result {
			actDataID := id.([]byte)
			idMap[string(actDataID)] = true
		}

		for actDataID := range idMap {
			ids = append(ids, actDataID)
		}
		return ids, true
	}
}

func (rc *RedisCache) GetRoomAccountDataCacheData(roomAccountDataKey string) (result *authtypes.RoomAccountDataCacheData, ok bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", roomAccountDataKey, "user_id", "room_id", "type", "content"))
	ok = true
	if err != nil {
		log.Errorw("cache missed for room account data", log.KeysAndValues{"room_account_data_key", roomAccountDataKey, "error", err})
		ok = false
	} else {
		result = &authtypes.RoomAccountDataCacheData{}
		reply, err = redis.Scan(reply, &result.UserID, &result.RoomID, &result.Type, &result.Content)
		if err != nil {
			log.Errorw("Scan error for room account data", log.KeysAndValues{"room_account_data_key", roomAccountDataKey, "error", err})
			ok = false
		}
	}
	return
}

func (rc *RedisCache) GetProfileLessByUserID(userID string) (string, string, bool) {
	profileKey := fmt.Sprintf("%s:%s", "profile", userID)

	exists, err := redis.Bool(rc.SafeDo("HEXISTS", profileKey, "display_name"))
	if !exists || err != nil {
		log.Warnw("cache missed for profile:display_name", log.KeysAndValues{"userID", userID, "error", err})
		return "", "", false
	}
	exists, err = redis.Bool(rc.SafeDo("HEXISTS", profileKey, "avatar_url"))
	if !exists || err != nil {
		log.Warnw("cache missed for profile:avatar_url", log.KeysAndValues{"userID", userID, "error", err})
		return "", "", false
	}

	profile := rc.GetProfileByUserID(userID)
	if profile == nil || profile.UserID == "" {
		return "", "", false
	}

	return profile.DisplayName, profile.AvatarURL, true
}

func (rc *RedisCache) GetAvatarURLByUser(userID string) (string, bool) {
	profileKey := fmt.Sprintf("%s:%s", "profile", userID)

	exists, err := redis.Bool(rc.SafeDo("HEXISTS", profileKey, "avatar_url"))
	if !exists || err != nil {
		log.Warnw("cache missed for profile", log.KeysAndValues{"userID", userID, "error", err})
		return "", false
	}

	profile := rc.GetProfileByUserID(userID)
	if profile == nil || profile.UserID == "" {
		return "", false
	}

	return profile.AvatarURL, true
}

func (rc *RedisCache) GetDisplayNameByUser(userID string) (string, bool) {
	profileKey := fmt.Sprintf("%s:%s", "profile", userID)

	exists, err := redis.Bool(rc.SafeDo("HEXISTS", profileKey, "display_name"))
	if !exists || err != nil {
		log.Warnw("cache missed for display_name", log.KeysAndValues{"userID", userID, "error", err})
		return "", false
	}

	profile := rc.GetProfileByUserID(userID)
	if profile == nil || profile.UserID == "" {
		return "", false
	}

	return profile.DisplayName, true
}

func (rc *RedisCache) GetProfileByUserID(userID string) (result *authtypes.Profile) {
	profileKey := fmt.Sprintf("%s:%s", "profile", userID)

	reply, err := redis.Values(rc.SafeDo("hmget", profileKey, "user_id", "display_name", "avatar_url"))
	if err != nil {
		log.Errorw("Get profile error", log.KeysAndValues{"userID", userID, "error", err})
	} else {
		result = &authtypes.Profile{}
		reply, err = redis.Scan(reply, &result.UserID, &result.DisplayName, &result.AvatarURL)
		if err != nil {
			log.Errorw("Scan error for profile", log.KeysAndValues{"userID", userID, "error", err})
		}
	}
	return
}

/*func (rc *RedisCache) GetAccountFilterIDByContent(userID, content string) (string, bool) {
	context, err := redis.String(rc.SafeDo("hget", fmt.Sprintf("%s:%s:%s", "filter_content", userID, content), "id"))
	if err != nil {
		log.Debugw("filter not cached", log.KeysAndValues{"userID", userID, "content", content, "error", err})
		return "", false
	}

	return context, true
}*/

func (rc *RedisCache) GetRoomTagIds(userName, roomID string) ([]string, bool) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s:%s", "room_tags_list", userName, roomID)))
	if err != nil {
		log.Warnw("cache missed for room tag list", log.KeysAndValues{"userName", userName, "err", err})
		return []string{}, false
	} else {
		var tags []string
		tagMap := make(map[string]bool)

		for _, tag := range result {
			tagID := tag.([]byte)
			tagMap[string(tagID)] = true
		}

		for tagID := range tagMap {
			tags = append(tags, tagID)
		}

		return tags, true
	}
}

func (rc *RedisCache) GetDeviceKeyIDs(userID, deviceID string) ([]string, bool) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s:%s", "device_key_list", userID, deviceID)))
	if err != nil {
		log.Warnw("cache missed for device key list", log.KeysAndValues{"userID", userID, "deviceID", deviceID, "err", err})
		return []string{}, false
	} else {
		var keys []string
		keyMap := make(map[string]bool)

		for _, key := range result {
			keyID := key.([]byte)
			keyMap[string(keyID)] = true
		}

		for keyID := range keyMap {
			keys = append(keys, keyID)
		}

		return keys, true
	}
}

func (rc *RedisCache) GetDeviceKey(keyID string) (*e2e.KeyHolder, bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", keyID, "device_id", "user_id", "key_info", "algorithm", "signature"))
	var result e2e.KeyHolder
	if err != nil {
		log.Errorw("cache missed for device key", log.KeysAndValues{"keyID", keyID, "error", err})
		return nil, false
	} else {
		result = e2e.KeyHolder{}
		reply, err = redis.Scan(reply, &result.DeviceID, &result.UserID, &result.Key, &result.KeyAlgorithm, &result.Signature)
		if err != nil {
			log.Errorw("Scan error for device key", log.KeysAndValues{"keyID", keyID, "error", err})
		}
		return &result, true
	}
}

func (rc *RedisCache) GetOneTimeKeyIDs(userID, deviceID string) ([]string, bool) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s:%s:%s", "one_time_key_list", userID, deviceID)))
	if err != nil {
		log.Warnw("cache missed for one time key list", log.KeysAndValues{"userID", userID, "deviceID", deviceID, "err", err})
		return []string{}, false
	} else {
		var keys []string
		keyMap := make(map[string]bool)

		for _, key := range result {
			keyID := key.([]byte)
			keyMap[string(keyID)] = true
		}

		for keyID := range keyMap {
			keys = append(keys, keyID)
		}

		return keys, true
	}
}

func (rc *RedisCache) GetOneTimeKey(keyID string) (*e2e.KeyHolder, bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", keyID, "device_id", "user_id", "key_id", "key_info", "algorithm", "signature"))
	var result e2e.KeyHolder
	if err != nil {
		log.Errorw("cache missed for one time key", log.KeysAndValues{"keyID", keyID, "error", err})
		return nil, false
	} else {
		result = e2e.KeyHolder{}
		reply, err = redis.Scan(reply, &result.DeviceID, &result.UserID, &result.KeyID, &result.Key, &result.KeyAlgorithm, &result.Signature)
		if err != nil {
			log.Errorw("Scan error for one time key", log.KeysAndValues{"keyID", keyID, "error", err})
		}
		return &result, true
	}
}

func (rc *RedisCache) GetDeviceAlgorithm(userID, deviceID string) (*e2e.AlHolder, bool) {
	reply, err := redis.Values(rc.SafeDo("hmget", fmt.Sprintf("%s:%s:%s", "algorithm", userID, deviceID), "device_id", "user_id", "algorithms"))
	var result e2e.AlHolder
	if err != nil {
		log.Errorw("cache missed for device algorithm", log.KeysAndValues{"userID", userID, "deviceID", deviceID, "error", err})
		return nil, false
	} else {
		result = e2e.AlHolder{}
		reply, err = redis.Scan(reply, &result.DeviceID, &result.UserID, &result.SupportedAlgorithm)
		if err != nil {
			log.Errorw("Scan error for device algorithm", log.KeysAndValues{"userID", userID, "deviceID", deviceID, "error", err})
		}
		return &result, true
	}
}

func (rc *RedisCache) DeleteDeviceOneTimeKey(userID, deviceID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "one_time_key_list", userID, deviceID)))
	if err != nil {
		return err
	} else {
		keyMap := make(map[string]bool)

		for _, key := range result {
			keyID := key.([]byte)
			keyMap[string(keyID)] = true
		}

		for keyID := range keyMap {
			err := conn.Send("del", keyID)
			if err != nil {
				return err
			}

			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", userID, deviceID), keyID)
			if err != nil {
				return err
			}
		}
	}

	return conn.Flush()
}

func (rc *RedisCache) DeleteDeviceKey(userID, deviceID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("del", fmt.Sprintf("%s:%s:%s", "algorithm", userID, deviceID))
	if err != nil {
		return err
	}

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_key_list", userID, deviceID)))
	if err != nil {
		return err
	} else {
		keyMap := make(map[string]bool)

		for _, key := range result {
			keyID := key.([]byte)
			keyMap[string(keyID)] = true
		}

		for keyID := range keyMap {
			err := conn.Send("del", keyID)
			if err != nil {
				return err
			}

			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_key_list", userID, deviceID), keyID)
			if err != nil {
				return err
			}
		}
	}

	return conn.Flush()
}

func (rc *RedisCache) DeleteOneTimeKey(deviceID, userID, keyID, algorithm string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	keyKey := fmt.Sprintf("%s:%s:%s:%s:%s", "one_time_key", userID, deviceID, keyID, algorithm)

	err := conn.Send("del", keyKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", userID, deviceID), keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) SetDeviceAlgorithm(userID, deviceID, algorithm string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "algorithm", userID, deviceID),
		"device_id", deviceID, "user_id", userID, "algorithms", algorithm)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) SetDeviceKey(userID, deviceID, keyInfo, algorithm, signature string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	keyKey := fmt.Sprintf("%s:%s:%s:%s", "device_key", userID, deviceID, algorithm)

	err := conn.Send("hmset", keyKey, "device_id", deviceID, "user_id", userID, "key_info", keyInfo,
		"algorithm", algorithm, "signature", signature)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_key_list", userID, deviceID), keyKey, keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) SetOneTimeKey(userID, deviceID, keyID, keyInfo, algorithm, signature string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	keyKey := fmt.Sprintf("%s:%s:%s:%s:%s", "one_time_key", userID, deviceID, keyID, algorithm)

	err := conn.Send("hmset", keyKey, "device_id", deviceID, "user_id", userID, "key_id", keyID,
		"key_info", keyInfo, "algorithm", algorithm, "signature", signature)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "one_time_key_list", userID, deviceID), keyKey, keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) GetRoomUnreadCount(userID, roomID string) (int64, int64, error) {
	key := fmt.Sprintf("%s:%s:%s", "unread_count", userID, roomID)

	reply, err := redis.Values(rc.SafeDo("hmget", key, "highlight_count", "notification_count"))
	if err != nil {
		log.Errorw("cache missed for unread count", log.KeysAndValues{"roomID", roomID, "userID", userID, "error", err})
		return int64(0), int64(0), err
	} else {
		var oriHlCount int64
		var oriNtfCount int64

		reply, err = redis.Scan(reply, &oriHlCount, &oriNtfCount)
		if err != nil {
			log.Errorw("Scan error for  unread count", log.KeysAndValues{"roomID", roomID, "userID", userID, "error", err})
			return int64(0), int64(0), err
		}

		return oriHlCount, oriNtfCount, nil
	}
}

func (rc *RedisCache) GetPresences(userID string) (*authtypes.Presences, bool) {
	key := fmt.Sprintf("%s:%s", "presences", userID)

	reply, err := redis.Values(rc.SafeDo("hmget", key, "user_id", "status", "status_msg", "ext_status_msg","server_status"))
	if err != nil {
		log.Errorw("cache missed for presences", log.KeysAndValues{"userID", userID, "error", err})
		return nil, false
	} else {
		var presences authtypes.Presences

		reply, err = redis.Scan(reply, &presences.UserID, &presences.Status, &presences.StatusMsg, &presences.ExtStatusMsg, &presences.ServerStatus)
		if err != nil {
			log.Errorw("Scan error for presences", log.KeysAndValues{"userID", userID, "error", err})
			return nil, false
		}

		return &presences, true
	}
}

func (rc *RedisCache) SetPresences(userID, status, statusMsg, extStatusMsg string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s", "presences", userID)
	err := conn.Send("hmset", key, "status", status, "status_msg", statusMsg, "ext_status_msg", extStatusMsg)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) SetPresencesServerStatus(userID, serverStatus string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	key := fmt.Sprintf("%s:%s", "presences", userID)
	err := conn.Send("hmset", key, "server_status", serverStatus)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) SetAccountData(userID, roomID, acctType, content string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	if roomID != "" {
		roomAccountDatKey := fmt.Sprintf("%s:%s:%s:%s", "room_account_data", userID, roomID, acctType)
		err := conn.Send("hmset", roomAccountDatKey, "user_id", userID, "room_id", roomID, "type", acctType, "content", content)
		if err != nil {
			return err
		}
		err = conn.Send("hmset", fmt.Sprintf("%s:%s", "room_account_data_list", userID), roomAccountDatKey, roomAccountDatKey)
		if err != nil {
			return err
		}
	} else {
		accountDatKey := fmt.Sprintf("%s:%s:%s", "account_data", userID, acctType)
		err := conn.Send("hmset", accountDatKey, "user_id", userID, "type", acctType, "content", content)
		if err != nil {
			return err
		}
		err = conn.Send("hmset", fmt.Sprintf("%s:%s", "account_data_list", userID), accountDatKey, accountDatKey)
		if err != nil {
			return err
		}
	}

	return conn.Flush()
}

func (rc *RedisCache) GetRoomOffsets(roomNID int64) (map[string]int64, error) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("offsets:%d", roomNID)))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("GetRoomOffsets cache missed %d", roomNID))
	} else {
		res := make(map[string]int64)
		length := len(result) / 2
		for i := 0; i < length; i++ {
			key := string(result[2*i].([]byte))
			val := string(result[2*i+1].([]byte))
			offset, _ := strconv.ParseInt(val, 10, 64)
			res[key] = offset
		}

		return res, nil
	}
}

func (rc *RedisCache) SetRoomUnreadCount(userID, roomID string, notifyCount, hlCount int64) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s:%s", "unread_count", userID, roomID)

	err := conn.Send("hmset", key, "highlight_count", hlCount, "notification_count", notifyCount)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) DelProfile(userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("DEL", fmt.Sprintf("%s:%s", "profile", userID))
	if err != nil {
		return err
	}

	return conn.Flush()
}
func (rc *RedisCache) DelAvatar(userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("HDEL", fmt.Sprintf("%s:%s", "profile", userID), "avatar_url")
	if err != nil {
		return err
	}

	return conn.Flush()
}
func (rc *RedisCache) DelDisplayName(userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("HDEL", fmt.Sprintf("%s:%s", "profile", userID), "display_name")
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) SetProfile(userID, displayName, avatar string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", userID), "user_id", userID, "display_name", displayName, "avatar_url", avatar)
	if err != nil {
		return err
	}

	return conn.Flush()
}
func (rc *RedisCache) ExpireProfile(userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	// 10~30 days
	err := conn.Send("expire", fmt.Sprintf("%s:%s", "profile", userID), strconv.Itoa(10*24*3600+rand.Intn(20*24*3600)))
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) SetDisplayName(userID, displayName string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", userID), "user_id", userID, "display_name", displayName)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) SetAvatar(userID, avatar string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", userID), "user_id", userID, "avatar_url", avatar)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) GetSetting(settingKey string) (int64, error) {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s", "setting", settingKey)
	ret, err := redis.String(rc.SafeDo("get", key))
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseInt(ret, 0, 64)
	return val, err
}

func (rc *RedisCache) GetSettingRaw(settingKey string) (string, error) {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s", "setting", settingKey)
	ret, err := redis.String(rc.SafeDo("get", key))
	if err != nil {
		return "", err
	}
	return ret, err
}

func (rc *RedisCache) SetSetting(settingKey string, val string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s", "setting", settingKey)
	err := conn.Send("set", key, val)
	return err
}

func (rc *RedisCache) AddDomain(domain string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	key := fmt.Sprintf("%s", "setting:domains")
	err := conn.Send("hmset", key, domain, 1)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) GetDomains() ([]string, error) {
	result, err := redis.Values(rc.SafeDo("hgetall", fmt.Sprintf("%s", "setting:domains")))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("domain cache missed %s", "setting:domains"))
	} else {
		domains := []string{}
		length := len(result) / 2
		for i := 0; i < length; i++ {
			key := string(result[2*i].([]byte))
			domains = append(domains, key)
		}
		return domains, nil
	}
}

func (rc *RedisCache) GetUserInfoByUserID(userID string) (result *authtypes.UserInfo) {
	reply, err := redis.Values(rc.SafeDo("hmget", fmt.Sprintf("%s:%s", "user_info", userID), "user_id", "user_name", "job_number", "mobile", "landline", "email","state"))
	if err != nil {
		log.Errorw("cache missed for user_info", log.KeysAndValues{"userID", userID, "error", err})
	} else {
		result = &authtypes.UserInfo{}
		reply, err = redis.Scan(reply, &result.UserID, &result.UserName, &result.JobNumber, &result.Mobile, &result.Landline, &result.Email,&result.State)
		if err != nil {
			log.Errorw("Scan error for user_info", log.KeysAndValues{"userID", userID, "error", err})
		}
	}
	return
}

func (rc *RedisCache) SetUserInfo(userID, userName, jobNumber, mobile, landline, email string, state int) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "user_info", userID), "user_id", userID, "user_name", userName, "job_number", jobNumber, "mobile", mobile, "landline", landline, "email", email, "state", state)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) DeleteUserInfo(userID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	UserInfokey := fmt.Sprintf("%s:%s", "user_info", userID)
	err := conn.Send("del", UserInfokey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) GetFakePartition(key string) (int32, error) {
	p, err := redis.Int(rc.SafeDo("DECR", "fakepartition:"+key))
	if err != nil {
		return 0, err
	}
	return int32(p), nil
}

func (rc *RedisCache) AssignFedSendRecPartition(roomID, domain string, partition int32) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := "fedpart:" + roomID + ":" + domain
	reply, err := redis.String(conn.Do("SET", key, partition, "NX", "PX", 30000))
	if err != nil {
		return err
	}
	if strings.ToLower(reply) != "ok" {
		return errors.New("set fedpart not ok")
	}

	return nil
}

func (rc *RedisCache) UnassignFedSendRecPartition(roomID, domain string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := "fedpart:" + roomID + ":" + domain
	err := conn.Send("DEL", key)

	return err
}

func (rc *RedisCache) ExpireFedSendRecPartition(roomID, domain string) error {
	key := "fedpart:" + roomID + ":" + domain
	reply, err := redis.Values(rc.SafeDo("PEXPIRE", key, 30000))
	if err != nil {
		return err
	}
	result := 0
	reply, err = redis.Scan(reply, &result)
	if err != nil {
		return err
	}
	if result == 0 {
		return errors.New(key + " not exists or expire failed")
	}

	return nil
}

func (rc *RedisCache) AddFedPendingRooms(keys []string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	args := make([]interface{}, len(keys)+1)
	args[0] = "fedsender:pendding"
	for i := 0; i < len(keys); i++ {
		args[i+1] = keys[i]
	}
	err := conn.Send("SADD", args...)
	return err
}

func (rc *RedisCache) DelFedPendingRooms(keys []string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	args := make([]interface{}, len(keys)+1)
	args[0] = "fedsender:pendding"
	for i := 0; i < len(keys); i++ {
		args[i+1] = keys[i]
	}
	err := conn.Send("SREM", args...)
	return err
}

func (rc *RedisCache) QryFedPendingRooms() ([]string, error) {
	reply, err := redis.Values(rc.SafeDo("SMEMBERS", "fedsender:pendding"))
	if err != nil {
		return nil, err
	}
	var roomIDs []string
	var roomID string
	for {
		reply, err = redis.Scan(reply, &roomID)
		if err != nil {
			break
		}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs, nil
}

func (rc *RedisCache) StoreFedSendRec(roomID, domain, eventID string, sendTimes, pendingSize int32, domainOffset int64) error {
	conn := rc.pool().Get()
	defer conn.Close()
	err := conn.Send("HMSET", "fedsend:"+roomID+":"+domain,
		"eventID", eventID,
		"sendTimes", sendTimes,
		"pendingSize", pendingSize,
		"domainOffset", domainOffset,
	)
	return err
}

func (rc *RedisCache) GetFedSendRec(roomID, domain string) (eventID string, sendTimes, pendingSize int32, domainOffset int64, err error) {
	reply, err := redis.Values(rc.SafeDo("HMGET", "fedsend:"+roomID+":"+domain,
		"eventID", "sendTimes", "pendingSize", "domainOffset"))
	if err != nil {
		return
	}
	if reply == nil {
		err = errors.New("redis value nil")
		return
	}
	allNil := true
	for _, v := range reply {
		if v != nil {
			allNil = false
			break
		}
	}
	if allNil {
		err = errors.New("redis value nil")
		return
	}

	reply, err = redis.Scan(reply, &eventID, &sendTimes, &pendingSize, &domainOffset)
	return
}

func (rc *RedisCache) IncrFedRoomPending(roomID, domain string, amt int) error {
	conn := rc.pool().Get()
	defer conn.Close()

	const SCRIPT_INCR = `
redis.call('SADD', 'fedsender:pendding', ARGV[2])
if redis.call('EXISTS', KEYS[1]) == 1 then
	redis.call('HINCRBY', KEYS[1], 'pendingSize', ARGV[1])
	return 1
else
	return 0
end
`
	lua := redis.NewScript(1, SCRIPT_INCR)
	_, err := lua.Do(conn, "fedsend:"+roomID+":"+domain, amt, roomID+"|"+domain)
	return err
}

func (rc *RedisCache) IncrFedRoomDomainOffset(roomID, domain, eventID string, domainOffset int64, penddingDecr int32) error {
	conn := rc.pool().Get()
	defer conn.Close()

	const SCRIPT_INCR = `
if redis.call('EXISTS', KEYS[1]) == 1 then
	redis.call('HMSET', KEYS[1], 'domainOffset', ARGV[1], "eventID", ARGV[2])
	local pendingSize = redis.call('HINCRBY',KEYS[1], 'pendingSize', ARGV[3])
	if pendingSize <= 0 then
		redis.call('SREM', 'fedsender:pendding', ARGV[4])
	end
	redis.call('HINCRBY', KEYS[1], 'sendTimes', 1)
	return 1
else
	return 0
end
`
	lua := redis.NewScript(1, SCRIPT_INCR)
	err := lua.Send(conn, "fedsend:"+roomID+":"+domain, domainOffset, eventID, -penddingDecr, roomID+"|"+domain)
	return err
}

func (rc *RedisCache) FreeFedSendRec(roomID, domain string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	err := conn.Send("DEL", "fedsend:"+roomID+":"+domain)
	return err
}

func (rc *RedisCache) AssignFedBackfillRecPartition(roomID string, partition int32) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := "fedbackfillpart:" + roomID
	reply, err := redis.String(conn.Do("SET", key, partition, "NX", "PX", 30000))
	if err != nil {
		return err
	}
	if strings.ToLower(reply) == "ok" {
		return errors.New("set fedbackfillpart not ok")
	}

	return err
}

func (rc *RedisCache) UnassignFedBackfillRecPartition(roomID string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := "fedbackfillpart:" + roomID
	err := conn.Send("DEL", key)

	return err
}

func (rc *RedisCache) ExpireFedBackfillRecPartition(roomID string) error {
	key := "fedbackfillpart:" + roomID
	reply, err := redis.Values(rc.SafeDo("PEXPIRE", key, 30000))
	if err != nil {
		return err
	}
	result := 0
	reply, err = redis.Scan(reply, &result)
	if err != nil {
		return err
	}
	if result == 0 {
		return errors.New(key + " not exists or expire failed")
	}

	return nil
}

func (rc *RedisCache) AddFedBackfillRooms(keys []string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	args := make([]interface{}, len(keys)+1)
	args[0] = "fedbackfill:pendding"
	for i := 0; i < len(keys); i++ {
		args[i+1] = keys[i]
	}
	err := conn.Send("SADD", args...)
	return err
}

func (rc *RedisCache) DelFedBackfillRooms(roomIDs []string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	args := make([]interface{}, len(roomIDs)+1)
	args[0] = "fedbackfill:pendding"
	for i := 0; i < len(roomIDs); i++ {
		args[i+1] = roomIDs[i]
	}
	err := conn.Send("SREM", args...)
	return err
}

func (rc *RedisCache) QryFedBackfillRooms() ([]string, error) {
	reply, err := redis.Values(rc.SafeDo("SMEMBERS", "fedbackfill:pendding"))
	if err != nil {
		return nil, err
	}
	var roomIDs []string
	var roomID string
	for {
		reply, err = redis.Scan(reply, &roomID)
		if err != nil {
			break
		}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs, nil
}

func (rc *RedisCache) StoreFedBackfillRec(roomID string, depth int64, finished bool, finishedDomains string, states string) (loaded bool, err error) {
	conn := rc.pool().Get()
	defer conn.Close()

	const SCRIPT_INCR = `
if redis.call('EXISTS', KEYS[1]) == 0 then
	redis.call('HMSET', KEYS[1], 'depth', ARGV[1], 'finished', ARGV[2], 'finishedDomains', ARGV[3], 'states': ARGV4)
	return 1
else
	return 0
end
`
	lua := redis.NewScript(1, SCRIPT_INCR)
	reply, err := redis.Int(lua.Do(conn, "fedbackfill:"+roomID, depth, finished, finishedDomains, states))
	if err != nil {
		return false, err
	}
	return reply == 0, nil
}

func (rc *RedisCache) UpdateFedBackfillRec(roomID string, depth int64, finished bool, finishedDomains string, states string) (updated bool, err error) {
	conn := rc.pool().Get()
	defer conn.Close()

	const SCRIPT_INCR = `
if redis.call('EXISTS', KEYS[1]) == 1 then
	redis.call('HMSET', KEYS[1], 'depth', ARGV[1], 'finished', ARGV[2], 'finishedDomains', ARGV[3], 'states': ARGV4)
	return 1
else
	return 0
end
`
	lua := redis.NewScript(1, SCRIPT_INCR)
	reply, err := redis.Int(lua.Do(conn, "fedbackfill:"+roomID, depth, finished, finishedDomains, states))
	if err != nil {
		return false, err
	}
	return reply == 0, nil
}

func (rc *RedisCache) GetFedBackfillRec(roomID string) (depth int64, finished bool, finishedDomains string, states string, err error) {
	reply, err := redis.Values(rc.SafeDo("HMGET", "fedbackfill:"+roomID,
		"depth", "finished", "finishedDomains", "states"))
	if err != nil {
		return
	}
	if reply == nil {
		err = errors.New("redis value nil")
		return
	}
	allNil := true
	for _, v := range reply {
		if v != nil {
			allNil = false
			break
		}
	}
	if allNil {
		err = errors.New("redis value nil")
		return
	}

	reply, err = redis.Scan(reply, &depth, &finished, &finishedDomains, &states)
	return
}

func (rc *RedisCache) FreeFedBackfill(roomID string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	err := conn.Send("DEL", "fedbackfill:"+roomID)
	return err
}

func (rc *RedisCache) SetRoomLatestOffset(roomId string, offset int64) error {
	conn := rc.pool().Get()
	defer conn.Close()
	key := fmt.Sprintf("%s:%s", "roomlatestoffset", roomId)
	err := conn.Send("set", key, offset)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) GetRoomLatestOffset(roomId string) (int64, error) {
	key := fmt.Sprintf("%s:%s", "roomlatestoffset", roomId)
	offset, err := redis.Int64(rc.SafeDo("get", key))
	if err != nil {
		return -1, err
	}
	return offset, err
}
