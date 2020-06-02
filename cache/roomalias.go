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

func (rc *RedisCache) GetAlias(key string) (string, error) {
	key = fmt.Sprintf("roomalias:%s", key)
	return rc.GetString(key)
}

func (rc *RedisCache) SetAlias(key, val string, expire int64) error {
	key = fmt.Sprintf("roomalias:%s", key)
	return rc.Set(key, val, expire)
}

func (rc *RedisCache) AliasExists(key string) (bool, error) {
	key = fmt.Sprintf("roomalias:%s", key)
	return rc.Exists(key)
}

func (rc *RedisCache) DelAlias(key string) error {
	key = fmt.Sprintf("roomalias:%s", key)
	return rc.Del(key)
}
