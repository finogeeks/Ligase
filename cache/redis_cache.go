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
	"sync"
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

	schTimer *time.Ticker
	device   sync.Map
	token    sync.Map
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

//定时清理缓存
func (rc *RedisCache) scheduleJob() {
	for t := range rc.schTimer.C {
		rc.device.Range(func(key, value interface{}) bool {
			d := value.(*DeviceInfo)
			//10分钟失效
			if t.Unix()-d.lastTouchTime > 600 {
				rc.device.Delete(key)
				rc.token.Delete(key)
			}
			return true
		})
	}

}

func (rc *RedisCache) Prepare(cfg []string) (err error) {
	rc.poolSize = len(cfg)
	rc.pools = make([]*redis.Pool, rc.poolSize)
	for i := 0; i < rc.poolSize; i++ {
		addr := cfg[i]
		rc.pools[i] = &redis.Pool{
			MaxIdle:     10,
			MaxActive:   200,
			Wait:        true,
			IdleTimeout: 240 * time.Second,
			Dial:        func() (redis.Conn, error) { return redis.DialURL(addr) },
		}
	}
	rc.schTimer = time.NewTicker(5 * time.Minute)
	rand.Seed(time.Now().Unix())
	go rc.scheduleJob()

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
	if dev, ok := rc.device.Load(key); ok {
		device = dev.(*DeviceInfo)
		device.lastTouchTime = time.Now().Unix()
		rc.device.Store(key, device)
	} else {
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
				rc.device.Store(key, device)
			}
		}
	}

	if device != nil && device.userID != "" {
		isHuman := true
		if device.deviceType == "bot" {
			isHuman = false
		}
		return &authtypes.Device{
			ID:          device.id,
			UserID:      device.userID,
			DisplayName: device.displayName,
			DeviceType:  device.deviceType,
			IsHuman:     isHuman,
			Identifier:  device.identifier,
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

func (rc *RedisCache) UpdateLocalDevice(deviceID string, userID string, displayName string) {
	key := fmt.Sprintf("%s:%s:%s", "device", userID, deviceID)
	var device *DeviceInfo
	if dev, ok := rc.device.Load(key); ok {
		device = dev.(*DeviceInfo)
		device.lastTouchTime = time.Now().Unix()
		device.displayName = displayName
		rc.device.Store(key, device)
	}
}

func (rc *RedisCache) DeleteLocalDevice(deviceID string, userID string) {
	key := fmt.Sprintf("%s:%s:%s", "device", userID, deviceID)
	if _, ok := rc.device.Load(key); ok {
		rc.device.Delete(key)
		rc.token.Delete(key)
	}
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

	reply, err := redis.Values(rc.SafeDo("hmget", key, "user_id", "status", "status_msg", "ext_status_msg"))
	if err != nil {
		log.Errorw("cache missed for presences", log.KeysAndValues{"userID", userID, "error", err})
		return nil, false
	} else {
		var presences authtypes.Presences

		reply, err = redis.Scan(reply, &presences.UserID, &presences.Status, &presences.StatusMsg, &presences.ExtStatusMsg)
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

func (rc *RedisCache) SetRoomState(roomID string, state []byte, token string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s", "room_state", roomID)
	err := conn.Send("HMSET", key, "state", state, "token", token)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (rc *RedisCache) GetRoomState(roomID string) ([]byte, string, error) {
	conn := rc.pool().Get()
	defer conn.Close()

	key := fmt.Sprintf("%s:%s", "room_state", roomID)
	reply, err := redis.Values(rc.SafeDo("hmget", key, "state", "token"))

	if err != nil {
		return nil, "", err
	}

	var bytes []byte
	var token string
	reply, err = redis.Scan(reply, &bytes, &token)
	return bytes, token, err
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
	reply, err := redis.Values(rc.SafeDo("hmget", fmt.Sprintf("%s:%s", "user_info", userID), "user_id", "user_name", "job_number", "mobile", "landline", "email"))
	if err != nil {
		log.Errorw("cache missed for user_info", log.KeysAndValues{"userID", userID, "error", err})
	} else {
		result = &authtypes.UserInfo{}
		reply, err = redis.Scan(reply, &result.UserID, &result.UserName, &result.JobNumber, &result.Mobile, &result.Landline, &result.Email)
		if err != nil {
			log.Errorw("Scan error for user_info", log.KeysAndValues{"userID", userID, "error", err})
		}
	}
	return
}

func (rc *RedisCache) SetUserInfo(userID, userName, jobNumber, mobile, landline, email string) error {
	conn := rc.pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "user_info", userID), "user_id", userID, "user_name", userName, "job_number", jobNumber, "mobile", mobile, "landline", landline, "email", email)
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
