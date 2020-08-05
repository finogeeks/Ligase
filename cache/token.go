package cache

import (
	"fmt"
	"sort"
	"strconv"
	"time"
	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func (rc *RedisCache) SetToken(userID, device string, utl int64, roomoffsets map[string]int64) error {
	key := fmt.Sprintf("synctoken:%s:%s:%d", userID,device, utl)
	err := rc.HMSet(key, roomoffsets)
	if err == nil {
		e := rc.Expire(key,adapter.GetTokenExpire())
		if e != nil {
			log.Errorf("set token expire key:%s err:%v", key, e)
		}
	}
	return err
}

func (rc *RedisCache) GetToken(userID, device string, utl int64) (map[string]int64, error) {
	key := fmt.Sprintf("synctoken:%s:%s:%d", userID,device, utl)
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
		e := rc.Expire(key,adapter.GetTokenExpire())
		if e != nil {
			log.Errorf("get token expire key:%s err:%v", key, e)
		}
		return r, nil
	}
}

func (rc *RedisCache) DelTokens(userID, device string, utls[]int64) error {
	for _, utl := range utls {
		key := fmt.Sprintf("synctoken:%s:%s:%d", userID,device, utl)
		if err := rc.Del(key); err != nil {
			log.Warnf("del token key:%s utl:%d err:%v", key, utl, err)
		}else{
			tokenUtl := fmt.Sprintf("allutl:%s:%s",userID, device)
			if err := rc.HDel(tokenUtl,strconv.FormatInt(utl,10)); err != nil {
				log.Warnf("del utl key:%s field:%d err:%v", tokenUtl, utl, err)
			}
		}
	}
	return nil
}

func (rc *RedisCache) AddTokenUtl(userID, device string, utl int64) error {
	key := fmt.Sprintf("allutl:%s:%s",userID, device)
	err := rc.HSet(key, strconv.FormatInt(utl,10), time.Now().UnixNano()/1000000)
	if err == nil {
		e := rc.Expire(key,adapter.GetUtlExpire())
		if e != nil {
			log.Errorf("add token utl expire key:%s err:%v", key, e)
		}
	}
	return err
}

func (rc *RedisCache) GetTokenUtls(userID, device string)(utls []int64, err error) {
	key := fmt.Sprintf("allutl:%s:%s",userID, device)
	us, err := rc.HGetAll(key)
	if err != nil || us == nil {
		return []int64{}, err
	}
	for k, _ := range us {
		utl, err := strconv.ParseInt(k, 10, 64)
		if err == nil {
			utls = append(utls, utl)
		}
	}
	e := rc.Expire(key,adapter.GetUtlExpire())
	if e != nil {
		log.Errorf("add token utl expire key:%s err:%v", key, e)
	}
	sort.Slice(utls, func(i, j int) bool {
		return utls[i] > utls[j]
	})
	return utls, nil
}

func (rc *RedisCache) GetLastValidToken(userID, device string) (int64, map[string]int64, error) {
	utls, err := rc.GetTokenUtls(userID, device)
	if err != nil {
		return 0, nil, err
	}
	if len(utls) <= 0 {
		return 0, nil, nil
	}
	token, err := rc.GetToken(userID, device, utls[0])
	return utls[0], token, err
}
