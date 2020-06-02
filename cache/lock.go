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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/gomodule/redigo/redis"
	"time"
)

func (rc *RedisCache) Lock(lockKey string, expire, wait int) (lockToken string, err error) {
	b := make([]byte, 16)
	rand.Read(b)
	token := base64.StdEncoding.EncodeToString(b)
	endTime := time.Now().Add(time.Duration(wait) * time.Second).UnixNano()
	retry := 1
	if wait < 0 {
		locked := rc.doLock(lockKey, token, expire, retry)
		if locked {
			return token, nil
		}
	} else if wait == 0 {
		for {
			locked := rc.doLock(lockKey, token, expire, retry)
			if locked {
				return token, nil
			}
			retry++
		}
	} else {
		for time.Now().UnixNano() <= endTime {
			locked := rc.doLock(lockKey, token, expire, retry)
			if locked {
				return token, nil
			}
			retry++
		}
	}
	ttl, err := rc.TTL(lockKey)
	if err != nil {
		log.Warnf("get lock key:%s ttl failed with err:%v", lockKey, err)
	}
	return "", errors.New(fmt.Sprintf("lock key:%s failed with max retry timeout, lock will expire in %d s", lockKey, ttl))
}

func (rc *RedisCache) doLock(lockKey, token string, expire, retry int) bool {
	if retry > 1 { // sleep if not first time
		time.Sleep(100 * time.Millisecond)
	}
	v, err := rc.SafeDo("SET", lockKey, token, "EX", expire, "NX")
	if err == nil {
		if v == nil {
			log.Warnf("lock key:%s failed, get times:%d", lockKey, retry)
		} else {
			log.Infof("lock key:%s success, get times:%d", lockKey, retry)
			return true
		}
	} else {
		log.Warnf("lock key:%s failed with redis err:%v", lockKey, err)
	}
	return false
}

func (rc *RedisCache) UnLock(lockKey, token string, force bool) (err error) {
	if force {
		return rc.Del(lockKey)
	}
	val, err := rc.Get(lockKey)
	if err != nil {
		log.Errorf("unlock get lock key:%s token faild with redis err:%v", lockKey, err)
		return err
	}
	lockToken, e := String(val, err)
	if e != nil {
		if e == redis.ErrNil {
			log.Warnf("unlock parse lock key:%s token has expired", lockKey)
		} else {
			log.Errorf("unlock parse lock key:%s token faild with err:%v", lockKey, e)
		}
		return e
	}
	if lockToken != token {
		return errors.New(fmt.Sprintf("unlock key:%s token:%s is not equal lock token:%s unlock failed", lockKey, token, lockToken))
	}
	return rc.Del(lockKey)
}
