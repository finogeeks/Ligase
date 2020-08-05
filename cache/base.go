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
	"encoding/json"
	"github.com/gomodule/redigo/redis"
)

//util
func (rc *RedisCache) encode(val interface{}) (interface{}, error) {
	var value interface{}
	switch v := val.(type) {
	case string, int, uint, int8, int16, int32, int64, float32, float64, bool:
		value = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		value = string(b)
	}
	return value, nil
}

//common
func (rc *RedisCache) Del(key string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	err := conn.Send("DEL", key)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) Scan(cursor uint64, match string, count int) (result []interface{}, next uint64, err error) {
	values, err := redis.Values(rc.SafeDo("scan", cursor, "match", match, "count", count))
	if err != nil {
		return []interface{}{}, cursor, err
	}
	values, err = redis.Scan(values, &cursor, &result)
	if err != nil {
		return []interface{}{}, cursor, err
	} else {
		return result, cursor, nil
	}
}

func (rc *RedisCache) TTL(key string) (ttl int64, err error) {
	return Int64(rc.SafeDo("TTL", key))
}

//string
func (rc *RedisCache) Get(key string) (interface{}, error) {
	return rc.SafeDo("GET", key)
}

func (rc *RedisCache) GetString(key string) (string, error) {
	r, err := rc.Get(key)
	return String(r, err)
}

func (rc *RedisCache) Exists(key string) (bool, error) {
	return Bool(rc.SafeDo("EXISTS", key))
}

func (rc *RedisCache) Set(key string, val interface{}, expire int64) error {
	conn := rc.pool().Get()
	defer conn.Close()
	value, err := rc.encode(val)
	if err != nil {
		return err
	}
	if expire > 0 {
		err = conn.Send("SETEX", key, expire, value)
	} else {
		err = conn.Send("SET", key, value)
	}
	if err != nil {
		return err
	}
	return conn.Flush()
}

//hset
func (rc *RedisCache) HGet(key, field string) (interface{}, error) {
	return rc.SafeDo("HGET", key, field)
}

func (rc *RedisCache) HGetString(key, field string) (string, error) {
	return String(rc.HGet(key, field))
}

func (rc *RedisCache) HSet(key, field string, val interface{}) error {
	conn := rc.pool().Get()
	defer conn.Close()
	value, err := rc.encode(val)
	if err != nil {
		return err
	}
	err = conn.Send("HSET", key, field, value)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) HDel(key, field string) error {
	conn := rc.pool().Get()
	defer conn.Close()
	err := conn.Send("HDEL", key, field)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (rc *RedisCache) HScan(key, match string, cursor uint64, count int) (result map[string]interface{}, next uint64, err error) {
	values, err := redis.Values(rc.SafeDo("hscan", key, cursor, "match", match, "count", count))
	if err != nil {
		return nil, cursor, err
	}
	var res []interface{}
	values, err = redis.Scan(values, &cursor, &res)
	if err != nil {
		return nil, cursor, err
	} else {
		result = make(map[string]interface{})
		length := len(res) / 2
		for i := 0; i < length; i++ {
			key := res[2*i]
			val := res[2*i+1]
			result[string(key.([]byte))] = val
		}
		return result, cursor, nil
	}
}

func (rc *RedisCache) HGetAll(key string) (map[string]interface{}, error) {
	result, err := redis.Values(rc.SafeDo("HGETALL", key))
	if err != nil {
		return nil, err
	} else {
		if len(result) <= 0 {
			return nil, nil
		}
		res := make(map[string]interface{})
		length := len(result) / 2
		for i := 0; i < length; i++ {
			key := result[2*i]
			val := result[2*i+1]
			res[string(key.([]byte))] = val
		}
		return res, nil
	}
}

func (rc *RedisCache) HDelMulti(key string, fields []interface{}) error {
	conn := rc.pool().Get()
	defer conn.Close()
	for _, field := range fields {
		conn.Send("HDEL", key, field)
	}
	return conn.Flush()
}

func (rc *RedisCache) HMSet(key string, val interface{}) (err error) {
	conn := rc.pool().Get()
	defer conn.Close()
	err = conn.Send("HMSET", redis.Args{}.Add(key).AddFlat(val)...)
	if err != nil {
		return
	}
	return conn.Flush()
}

func (rc *RedisCache) HMGet(key string, fields []string) ([]interface{}, error) {
	return redis.Values(rc.SafeDo("HMGET", redis.Args{}.Add(key).AddFlat(fields)...))
}

func (c *RedisCache) Expire(key string, expire int64) error {
	_, err := Bool(c.SafeDo("EXPIRE", key, expire))
	return err
}
