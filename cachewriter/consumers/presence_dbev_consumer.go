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
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/dbtypes"
	"time"
)

func init() {
	Register(dbtypes.CATEGORY_PRESENCE_DB_EVENT, NewPresenceDBEvCacheConsumer)
}

type PresenceDBEvCacheConsumer struct {
	pool    PoolProviderInterface
	msgChan chan *dbtypes.DBEvent
}

func (s *PresenceDBEvCacheConsumer) startWorker(msgChan chan *dbtypes.DBEvent) {
	var res error
	for output := range msgChan {
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.PresenceDBEvents
		switch key {
		case dbtypes.PresencesInsertKey:
			res = s.OnInsertPresences(data.PresencesInsert)
		default:
			res = nil
			log.Infow("presence db event: ignoring unknown output type", log.KeysAndValues{"key", output.Key})
		}

		if res != nil {
			log.Errorf("write presence db event to cache error %v key: %s", res, dbtypes.PresenceDBEventKeyToStr(key))
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("PresenceDBEvCacheConsumer process %s takes %d", dbtypes.PresenceDBEventKeyToStr(key), now-start)
	}
}

func NewPresenceDBEvCacheConsumer() ConsumerInterface {
	s := new(PresenceDBEvCacheConsumer)
	s.msgChan = make(chan *dbtypes.DBEvent, 4096)

	return s
}

func (s *PresenceDBEvCacheConsumer) SetPool(pool PoolProviderInterface) {
	s.pool = pool
}

func (s *PresenceDBEvCacheConsumer) Prepare(cfg *config.Dendrite) {
}

func (s *PresenceDBEvCacheConsumer) Start() {
	go s.startWorker(s.msgChan)
}

func (s *PresenceDBEvCacheConsumer) OnMessage(dbEv *dbtypes.DBEvent) error {
	s.msgChan <- dbEv
	return nil
}

func (s *PresenceDBEvCacheConsumer) OnInsertPresences(
	msg *dbtypes.PresencesInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	presencesKey := fmt.Sprintf("%s:%s", "presences", msg.UserID)
	err := conn.Send("hmset", presencesKey, "user_id", msg.UserID, "status", msg.Status, "status_msg", msg.StatusMsg, "ext_status_msg", msg.ExtStatusMsg)
	if err != nil {
		return err
	}

	return conn.Flush()
}
