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
	"github.com/gomodule/redigo/redis"
)

func Int64(reply interface{}, err error) (int64, error) {
	return redis.Int64(reply, err)
}

func String(reply interface{}, err error) (string, error) {
	return redis.String(reply, err)
}

func Bool(reply interface{}, err error) (bool, error) {
	return redis.Bool(reply, err)
}
