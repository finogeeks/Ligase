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
	"fmt"
)

func (rc *RedisCache) SetUserRoomMemberShip(roomID, userID string, mType int64) error {
	key := fmt.Sprintf("membership:%s", userID)
	return rc.HSet(key, roomID, mType)
}

func (rc *RedisCache) SetUserRoomMemberShipMulti(userID string, memberships map[string]int64) error {
	key := fmt.Sprintf("membership:%s", userID)
	return rc.HMSet(key, memberships)
}

func (rc *RedisCache) GetUserRoomMemberShip(userID string) (map[string]int64, error) {
	key := fmt.Sprintf("membership:%s", userID)
	result, err := rc.HGetAll(key)
	if err != nil {
		return nil, err
	} else {
		if result == nil {
			return nil, nil
		}
		r := make(map[string]int64)
		for k, v := range result {
			r[k], _ = Int64(v, nil)
		}
		return r, nil
	}
}

func (rc *RedisCache) CheckUserRoomMemberShipExists(userID string) (bool, error) {
	key := fmt.Sprintf("membership:%s", userID)
	return rc.Exists(key)
}
