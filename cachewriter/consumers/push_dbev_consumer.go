// Copyright 2017 Vector Creations Ltd
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

package consumers

import (
	"fmt"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/gomodule/redigo/redis"
)

func init() {
	Register(dbtypes.CATEGORY_PUSH_DB_EVENT, NewPushDBEvCacheConsumer)
}

// DBEventDataConsumer consumes db events for roomserver.
type PushDBEvCacheConsumer struct {
	pool    PoolProviderInterface
	msgChan chan *dbtypes.DBEvent
}

func (s *PushDBEvCacheConsumer) startWorker(msgChan chan *dbtypes.DBEvent) {

	for output := range msgChan {
		//start := time.Now().UnixNano() / 1000000

		key := output.Key
		//data := output.PushDBEvents
		log.Infof("update cache type:%d cancel", key)
		//cache data ignore, has migrate push data to mem
		/*
		switch key {
		case dbtypes.PusherDeleteKey:
			res = s.onPusherDelete(data.PusherDelete)
		case dbtypes.PusherDeleteByKeyKey:
			res = s.onPusherDeleteByKey(data.PusherDeleteByKey)
		case dbtypes.PusherDeleteByKeyOnlyKey:
			res = s.onPusherDeleteByKeyOnly(data.PusherDeleteByKeyOnly)
		case dbtypes.PusherInsertKey:
			res = s.onPusherInsert(data.PusherInsert)
		case dbtypes.PushRuleUpsertKey:
			res = s.onPushRuleInsert(data.PushRuleInert)
		case dbtypes.PushRuleDeleteKey:
			res = s.onPushRuleDelete(data.PushRuleDelete)
		case dbtypes.PushRuleEnableUpsetKey:
			res = s.onPushRuleEnableInsert(data.PushRuleEnableInsert)
		default:
			res = nil
			log.Infow("push db event: ignoring unknown output type", log.KeysAndValues{"key", output.Key})
		}

		if res != nil {
			log.Errorf("write push db event to cache error %v key: %s", res, dbtypes.PushDBEventKeyToStr(key))
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("PushDBEvCacheConsumer process %s takes %d", dbtypes.PushDBEventKeyToStr(key), now-start)*/
	}
}

func NewPushDBEvCacheConsumer() ConsumerInterface {
	s := new(PushDBEvCacheConsumer)
	s.msgChan = make(chan *dbtypes.DBEvent, 1024)

	return s
}

func (s *PushDBEvCacheConsumer) SetPool(pool PoolProviderInterface) {
	s.pool = pool
}

func (s *PushDBEvCacheConsumer) Prepare(cfg *config.Dendrite) {
}

func (s *PushDBEvCacheConsumer) Start() {
	go s.startWorker(s.msgChan)
}

func (s *PushDBEvCacheConsumer) OnMessage(dbEv *dbtypes.DBEvent) error {
	s.msgChan <- dbEv
	return nil
}

func (s *PushDBEvCacheConsumer) onPusherDelete(
	msg *dbtypes.PusherDelete,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	pusherKey := fmt.Sprintf("%s:%s:%s:%s", "pusher", msg.UserID, msg.AppID, msg.PushKey)

	err := conn.Send("del", pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_user_list", msg.UserID), pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey), pusherKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *PushDBEvCacheConsumer) onPusherDeleteByKey(
	msg *dbtypes.PusherDeleteByKey,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey)))
	if err != nil {
		return err
	} else {
		for _, pusherKey := range result {
			userID, err := redis.Values(conn.Do("hmget", pusherKey, "user_name"))
			if err != nil {
				return err
			} else {
				err := conn.Send("del", pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_user_list", userID), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey), pusherKey)
				if err != nil {
					return err
				}
			}
		}
	}

	return conn.Flush()
}

func (s *PushDBEvCacheConsumer) onPusherDeleteByKeyOnly(
	msg *dbtypes.PusherDeleteByKeyOnly,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	var userID string
	var appID string

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey)))
	if err != nil {
		return err
	} else {
		for _, pusherKey := range result {
			reply, err := redis.Values(conn.Do("hmget", pusherKey, "user_name", "app_id"))
			if err != nil {
				return err
			} else {

				reply, err = redis.Scan(reply, &userID, &appID)
				if err != nil {
					return err
				}

				err := conn.Send("del", pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_user_list", userID), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", appID, msg.PushKey), pusherKey)
				if err != nil {
					return err
				}
			}
		}
	}

	return conn.Flush()
}

func (s *PushDBEvCacheConsumer) onPusherInsert(
	msg *dbtypes.PusherInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	pusherKey := fmt.Sprintf("%s:%s:%s:%s", "pusher", msg.UserID, msg.AppID, msg.PushKey)

	err := conn.Send("hmset", pusherKey, "user_name", msg.UserID, "profile_tag", msg.ProfileTag, "kind", msg.Kind, "app_id", msg.AppID,
		"app_display_name", msg.AppDisplayName, "device_display_name", msg.DeviceDisplayName, "push_key", msg.PushKey, "push_key_ts", msg.PushKeyTs,
		"lang", msg.Lang, "push_data", msg.Data, "device_id", msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hset", fmt.Sprintf("%s:%s", "pusher_user_list", msg.UserID), pusherKey, pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hset", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey, pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hset", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey), pusherKey, pusherKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *PushDBEvCacheConsumer) onPushRuleInsert(
	msg *dbtypes.PushRuleInert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule", msg.UserID, msg.RuleID)

	err := conn.Send("hmset", ruleKey, "user_name", msg.UserID, "rule_id", msg.RuleID, "priority_class", msg.PriorityClass,
		"priority", msg.Priority, "conditions", msg.Conditions, "actions", msg.Actions)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "push_rule_user_list", msg.UserID), ruleKey, ruleKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *PushDBEvCacheConsumer) onPushRuleDelete(
	msg *dbtypes.PushRuleDelete,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule", msg.UserID, msg.RuleID)

	err := conn.Send("del", ruleKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "push_rule_user_list", msg.UserID), ruleKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *PushDBEvCacheConsumer) onPushRuleEnableInsert(
	msg *dbtypes.PushRuleEnableInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	ruleKey := fmt.Sprintf("%s:%s:%s", "push_rule_enable", msg.UserID, msg.RuleID)

	err := conn.Send("hmset", ruleKey, "user_name", msg.UserID, "rule_id", msg.RuleID, "enabled", msg.Enabled)
	if err != nil {
		return err
	}

	return conn.Flush()
}
