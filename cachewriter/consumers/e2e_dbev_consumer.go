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
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"time"
)

func init() {
	Register(dbtypes.CATEGORY_E2E_DB_EVENT, NewE2EDBEvCacheConsumer)
}

type E2EDBEvCacheConsumer struct {
	pool PoolProviderInterface
	//msgChan chan *dbtypes.DBEvent
	msgChan chan common.ContextMsg
}

func (s *E2EDBEvCacheConsumer) startWorker(msgChan chan common.ContextMsg) {
	var res error
	for msg := range msgChan {
		output := msg.Msg.(*dbtypes.DBEvent)
		if output.IsRecovery {
			start := time.Now().UnixNano() / 1000000

			key := output.Key
			data := output.E2EDBEvents
			switch key {
			case dbtypes.DeviceKeyInsertKey:
				res = s.onDeviceKeyInsert(*data.KeyInsert)
			case dbtypes.OneTimeKeyInsertKey:
				res = s.onOneTimeKeyInsert(*data.KeyInsert)
			case dbtypes.OneTimeKeyDeleteKey:
				res = s.onOneTimeKeyDelete(*data.KeyDelete)
			case dbtypes.AlInsertKey:
				res = s.onAlInsert(*data.AlInsert)
			default:
				res = nil
				log.Infow("encrypt api db event: ignoring unknown output type", log.KeysAndValues{"key", output.Key})
			}

			if res != nil {
				log.Errorf("write encrypt api db event to cache error %v key: %s", res, dbtypes.E2EDBEventKeyToStr(key))
			}

			now := time.Now().UnixNano() / 1000000
			log.Infof("E2EDBEvCacheConsumer process %s takes %d", dbtypes.E2EDBEventKeyToStr(key), now-start)
		}
	}
}

// NewE2EDBEvCacheConsumer creates a new DBUpdateData consumer. Call Start() to begin consuming from room servers.
func NewE2EDBEvCacheConsumer() ConsumerInterface {
	s := new(E2EDBEvCacheConsumer)
	s.msgChan = make(chan common.ContextMsg, 1024)

	return s
}

func (s *E2EDBEvCacheConsumer) SetPool(pool PoolProviderInterface) {
	s.pool = pool
}

func (s *E2EDBEvCacheConsumer) Prepare(cfg *config.Dendrite) {
}

func (s *E2EDBEvCacheConsumer) Start() {
	go s.startWorker(s.msgChan)
}

func (s *E2EDBEvCacheConsumer) OnMessage(ctx context.Context, dbEv *dbtypes.DBEvent) error {
	s.msgChan <- common.ContextMsg{Ctx: ctx, Msg: dbEv}
	return nil
}

func (s *E2EDBEvCacheConsumer) onDeviceKeyInsert(
	msg dbtypes.KeyInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	keyKey := fmt.Sprintf("%s:%s:%s:%s", "device_key", msg.UserID, msg.DeviceID, msg.Algorithm)

	err := conn.Send("hmset", keyKey, "device_id", msg.DeviceID, "user_id", msg.UserID, "key_info", msg.KeyInfo,
		"algorithm", msg.Algorithm, "signature", msg.Signature)
	if err != nil {
		return err
	}
	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, msg.DeviceID), keyKey, keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *E2EDBEvCacheConsumer) onOneTimeKeyInsert(
	msg dbtypes.KeyInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	keyKey := fmt.Sprintf("%s:%s:%s:%s:%s", "one_time_key", msg.UserID, msg.DeviceID, msg.KeyID, msg.Algorithm)

	err := conn.Send("hmset", keyKey, "device_id", msg.DeviceID, "user_id", msg.UserID, "key_id", msg.KeyID,
		"key_info", msg.KeyInfo, "algorithm", msg.Algorithm, "signature", msg.Signature)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID), keyKey, keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *E2EDBEvCacheConsumer) onOneTimeKeyDelete(
	msg dbtypes.KeyDelete,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	keyKey := fmt.Sprintf("%s:%s:%s:%s:%s", "one_time_key", msg.UserID, msg.DeviceID, msg.KeyID, msg.Algorithm)

	err := conn.Send("del", keyKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID), keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *E2EDBEvCacheConsumer) onAlInsert(
	msg dbtypes.AlInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "algorithm", msg.UserID, msg.DeviceID),
		"device_id", msg.DeviceID, "user_id", msg.UserID, "algorithms", msg.Algorithm)
	if err != nil {
		return err
	}

	return conn.Flush()
}
