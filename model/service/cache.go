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

package service

import (
	"sync"

	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/types"
)

type Cache interface {
	Prepare(cfg []string) (err error)

	GetMigTokenByToken(token string) (string, error)

	GetDeviceByDeviceID(deviceID string, userID string) *authtypes.Device

	GetDevicesByUserID(userID string) *[]authtypes.Device

	UpdateLocalDevice(deviceID string, userID string, displayName string)

	DeleteLocalDevice(deviceID string, userID string)

	GetPushRuleEnabled(userID, ruleID string) (string, bool)

	GetUserPusherIds(userID string) ([]string, bool)

	GetUserPushRuleIds(userID string) ([]string, bool)

	GetPusherCacheData(pusherKey string) (*pushapitypes.PusherCacheData, bool)

	GetPushRuleCacheData(ruleKey string) (*pushapitypes.PushRuleCacheData, bool)

	GetPushRuleCacheDataByID(userID, ruleID string) (*pushapitypes.PushRuleCacheData, bool)

	GetAccountFilterById(userID, filterID string) (string, bool)

	GetUserRoomTagIds(userID string) ([]string, bool)

	GetRoomTagCacheData(tagKey string) (*authtypes.RoomTagCacheData, bool)

	GetUserAccountDataIds(userID string) ([]string, bool)

	GetAccountDataCacheData(accountDataKey string) (*authtypes.AccountDataCacheData, bool)

	GetUserRoomAccountDataIds(userID string) ([]string, bool)

	GetRoomAccountDataCacheData(roomAccountDataKey string) (*authtypes.RoomAccountDataCacheData, bool)

	GetProfileLessByUserID(userID string) (string, string, bool)
	GetProfileByUserID(userID string) *authtypes.Profile
	GetDisplayNameByUser(userID string) (string, bool)
	GetAvatarURLByUser(userID string) (string, bool)

	//GetAccountFilterIDByContent(userID, content string) (string, bool)

	GetRoomTagIds(userID, roomID string) ([]string, bool)

	GetDeviceKeyIDs(userID, deviceID string) ([]string, bool)

	GetDeviceKey(keyID string) (*types.KeyHolder, bool)

	GetOneTimeKeyIDs(userID, deviceID string) ([]string, bool)

	GetOneTimeKey(keyID string) (*types.KeyHolder, bool)

	GetDeviceAlgorithm(userID, deviceID string) (*types.AlHolder, bool)

	DeleteDeviceOneTimeKey(userID, deviceID string) error

	DeleteDeviceKey(userID, deviceID string) error

	DeleteOneTimeKey(deviceID, userID, keyID, algorithm string) error

	SetDeviceAlgorithm(userID, deviceID, algorithm string) error

	SetDeviceKey(userID, deviceID, keyInfo, algorithm, signature string) error

	SetOneTimeKey(userID, deviceID, keyID, keyInfo, algorithm, signature string) error

	GetRoomUnreadCount(userID, roomID string) (int64, int64, error)

	GetPresences(userID string) (*authtypes.Presences, bool)
	SetPresences(userID, status, statusMsg, extStatusMsg string) error

	SetAccountData(userID, roomID, acctType, content string) error

	SetRoomState(roomID string, state []byte, token string) error
	GetRoomState(roomID string) ([]byte, string, error)
	GetRoomOffsets(roomNID int64) (map[string]int64, error)

	SetRoomUnreadCount(userID, roomID string, notifyCount, hlCount int64) error

	SetProfile(userID, displayName, avatar string) error
	ExpireProfile(userID string) error
	SetDisplayName(userID, displayName string) error
	SetAvatar(userID, avatar string) error

	DelProfile(userID string) error
	DelAvatar(userID string) error
	DelDisplayName(userID string) error

	SetPwdChangeDevcie(deviceID, userID string) error
	CheckPwdChangeDevice(deviceID, userID string) bool
	DelPwdChangeDevice(deviceID, userID string) error
	ExpirePwdChangeDevice(userID string) error

	GetSetting(settingKey string) (int64, error)
	GetSettingRaw(settingKey string) (string, error)
	SetSetting(settingKey string, val string) error

	GetDomains() ([]string, error)
	AddDomain(domain string) error

	GetUserInfoByUserID(userID string) (result *authtypes.UserInfo)
	SetUserInfo(userID, userName, jobNumber, mobile, landline, email string) error
	DeleteUserInfo(userID string) error

	SetRoomLatestOffset(roomId string, offset int64) error
	GetRoomLatestOffset(roomId string) (int64, error)

	SetToken(userID, device string, utl int64, roomoffsets map[string]int64) error
	GetToken(userID, device string, utl int64) (map[string]int64, error)
	DelTokens(userID, device string, utls []int64) error
	AddTokenUtl(userID, device string, utl int64) error
	GetTokenUtls(userID, device string) (utls []int64, err error)
	GetLastValidToken(userID, device string) (int64, map[string]int64, error)
}

type CacheItem struct {
	Key    interface{}
	Val    interface{}
	Offset int32
	Repo   int
	Mux    sync.Mutex
	Ref    *CacheItem
}

type LocalCache interface {
	Register(repoName string) int
	GetRegister(repoName string) int
	GetRegisterReverse(repoName int) string
	Put(repoName int, key, val interface{}) *CacheItem
	Tie(repoName int, key interface{}, ref *CacheItem)
	Get(repoName int, key interface{}) (interface{}, bool)
	Start(cap, duration int)
}
