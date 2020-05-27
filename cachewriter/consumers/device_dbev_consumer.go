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
	"context"
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/gomodule/redigo/redis"

	"time"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/dbtypes"

	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(dbtypes.CATEGORY_DEVICE_DB_EVENT, NewDeviceDBEvCacheConsumer)
}

type DeviceDBEvCacheConsumer struct {
	pool PoolProviderInterface
	//msgChan chan *dbtypes.DBEvent
	msgChan chan common.ContextMsg
}

func (s *DeviceDBEvCacheConsumer) startWorker(msgChan chan common.ContextMsg) {
	var res error
	for msg := range msgChan {
		output := msg.Msg.(*dbtypes.DBEvent)
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.DeviceDBEvents
		switch key {
		case dbtypes.DeviceInsertKey:
			res = s.onDeviceInsert(*data.DeviceInsert)
		case dbtypes.DeviceDeleteKey:
			res = s.onDeviceDelete(*data.DeviceDelete)
		case dbtypes.MigDeviceInsertKey:
			res = s.onMigDeviceInsert(*data.MigDeviceInsert)
		case dbtypes.DeviceRecoverKey:
			res = s.onDeviceRecover(*data.DeviceInsert)
		case dbtypes.DeviceUpdateTsKey:
			res = s.onUpdateDeviceActiveTs(*data.DeviceUpdateTs)
		default:
			res = nil
			log.Infow("device db event: ignoring unknown output type", log.KeysAndValues{"key", output.Key})
		}

		if res != nil {
			log.Errorf("write device db event to cache error %v key: %s", res, dbtypes.DeviceDBEventKeyToStr(key))
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("DeviceDBEvCacheConsumer process %s takes %d", dbtypes.DeviceDBEventKeyToStr(key), now-start)
	}
}

// NewDeviceDBEvCacheConsumer creates a new DBUpdateData consumer. Call Start() to begin consuming from room servers.
func NewDeviceDBEvCacheConsumer() ConsumerInterface {
	s := new(DeviceDBEvCacheConsumer)
	s.msgChan = make(chan common.ContextMsg, 1024)

	return s
}

func (s *DeviceDBEvCacheConsumer) SetPool(pool PoolProviderInterface) {
	s.pool = pool
}

func (s *DeviceDBEvCacheConsumer) Prepare(cfg *config.Dendrite) {
}

func (s *DeviceDBEvCacheConsumer) Start() {
	go s.startWorker(s.msgChan)
}

func (s *DeviceDBEvCacheConsumer) OnMessage(ctx context.Context, dbEv *dbtypes.DBEvent) error {
	s.msgChan <- common.ContextMsg{Ctx: ctx, Msg: dbEv}
	return nil
}

func (s *DeviceDBEvCacheConsumer) onDeviceInsert(
	msg dbtypes.DeviceInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	mac := common.GetDeviceMac(msg.DeviceID)

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac)))
	if err != nil {
		return err
	}

	deviceMap := make(map[string]bool)
	for _, deviceID := range result {
		did := deviceID.([]byte)
		deviceMap[string(did)] = true
	}

	for did := range deviceMap {
		if did != msg.DeviceID {
			err := conn.Send("del", fmt.Sprintf("%s:%s:%s", "algorithm", msg.UserID, did))
			if err != nil {
				return err
			}

			result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, did)))
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

					err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, did), keyID)
					if err != nil {
						return err
					}
				}
			}

			result, err = redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, did)))
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

					err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, did), keyID)
					if err != nil {
						return err
					}
				}
			}

			err = conn.Send("del", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, did))
			if err != nil {
				return err
			}
			err = conn.Send("hdel", fmt.Sprintf("%s:%s", "device_list", msg.UserID), did)
			if err != nil {
				return err
			}
			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), did)
			if err != nil {
				return err
			}
		}
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, msg.DeviceID), "did", msg.DeviceID,
		"user_id", msg.UserID, "created_ts", msg.CreatedTs, "display_name", msg.DisplayName, "device_type", msg.DeviceType, "identifier", msg.Identifier)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "device_list", msg.UserID), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *DeviceDBEvCacheConsumer) onDeviceRecover(
	msg dbtypes.DeviceInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	mac := common.GetDeviceMac(msg.DeviceID)

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, msg.DeviceID), "did", msg.DeviceID,
		"user_id", msg.UserID, "created_ts", msg.CreatedTs, "display_name", msg.DisplayName, "device_type", msg.DeviceType, "identifier", msg.Identifier)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "device_list", msg.UserID), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *DeviceDBEvCacheConsumer) onMigDeviceInsert(
	msg dbtypes.MigDeviceInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("set", fmt.Sprintf("%s:%s", "tokens", msg.AccessToken), msg.MigAccessToken)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *DeviceDBEvCacheConsumer) onDeviceDelete(
	msg dbtypes.DeviceDelete,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	mac := common.GetDeviceMac(msg.DeviceID)

	err := conn.Send("del", fmt.Sprintf("%s:%s:%s", "algorithm", msg.UserID, msg.DeviceID))
	if err != nil {
		return err
	}

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, msg.DeviceID)))
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

			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, msg.DeviceID), keyID)
			if err != nil {
				return err
			}
		}
	}

	result, err = redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID)))
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

			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID), keyID)
			if err != nil {
				return err
			}
		}
	}

	err = conn.Send("del", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, msg.DeviceID))
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "device_list", msg.UserID), msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), msg.DeviceID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *DeviceDBEvCacheConsumer) onUpdateDeviceActiveTs(
	msg dbtypes.DeviceUpdateTs,
) error {
	return nil
}
