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
	"github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/storage/model"
	"sync/atomic"
	"time"

	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/dbtypes"
)

func init() {
	Register(dbtypes.CATEGORY_E2E_DB_EVENT, NewE2EDBEVConsumer)
}

type E2EDBEVConsumer struct {
	db       model.EncryptorAPIDatabase
	msgChan  []chan *dbtypes.DBEvent
	monState []*DBMonItem
}

func (s *E2EDBEVConsumer) startWorker(msgChan chan *dbtypes.DBEvent) error {
	var res error
	for output := range msgChan {
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.E2EDBEvents
		switch key {
		case dbtypes.DeviceKeyInsertKey:
			res = s.onDeviceKeyInsert(context.TODO(), data.KeyInsert)
		case dbtypes.OneTimeKeyInsertKey:
			res = s.onOneTimeKeyInsert(context.TODO(), data.KeyInsert)
		case dbtypes.OneTimeKeyDeleteKey:
			res = s.onOneTimeKeyDelete(context.TODO(), data.KeyDelete)
		case dbtypes.MacOneTimeKeyDeleteKey:
			res = s.onMacOneTimeKeyDelete(context.TODO(), data.MacKeyDelete)
		case dbtypes.AlInsertKey:
			res = s.onAlInsert(context.TODO(), data.AlInsert)
		case dbtypes.DeviceAlDeleteKey:
			res = s.onAlDelete(context.TODO(), data.DeviceKeyDelete)
		case dbtypes.MacDeviceAlDeleteKey:
			res = s.onMacAlDelete(context.TODO(), data.MacKeyDelete)
		case dbtypes.DeviceKeyDeleteKey:
			res = s.onDeviceKeyDelete(context.TODO(), data.DeviceKeyDelete)
		case dbtypes.MacDeviceKeyDeleteKey:
			res = s.onMacDeviceKeyDelete(context.TODO(), data.MacKeyDelete)
		case dbtypes.DeviceOneTimeKeyDeleteKey:
			res = s.onDeviceOneTimeKeyDelete(context.TODO(), data.DeviceKeyDelete)
		default:
			res = nil
			log.Infow("encrypt api db event: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.AddInt32(&item.process, 1)
		} else {
			log.Error("write encrypt api db event to db error %v key: %s", res, dbtypes.E2EDBEventKeyToStr(key))
			atomic.AddInt32(&item.fail, 1)
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("E2EDBEVConsumer process %s takes %d", dbtypes.E2EDBEventKeyToStr(key), now-start)
	}

	return res
}

func NewE2EDBEVConsumer() ConsumerInterface {
	s := new(E2EDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.E2EMaxKey)
	for i := int64(0); i < dbtypes.E2EMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_E2E_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_E2E_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_E2E_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan *dbtypes.DBEvent, 3)
	for i := uint64(0); i < 3; i++ {
		s.msgChan[i] = make(chan *dbtypes.DBEvent, 4096)
	}
	return s
}

func (s *E2EDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("encryptoapi", cfg)
	if err != nil {
		log.Panicf("failed to connect to encrypt api db")
	}

	s.db = db.(model.EncryptorAPIDatabase)
}

func (s *E2EDBEVConsumer) Start() {
	for i := uint64(0); i < 3; i++ {
		go s.startWorker(s.msgChan[i])
	}
}

func (s *E2EDBEVConsumer) OnMessage(dbEv *dbtypes.DBEvent) error {
	chanID := 0
	switch dbEv.Key {
	case dbtypes.DeviceKeyInsertKey, dbtypes.DeviceKeyDeleteKey, dbtypes.MacDeviceKeyDeleteKey:
		chanID = 0
	case dbtypes.OneTimeKeyInsertKey, dbtypes.OneTimeKeyDeleteKey, dbtypes.DeviceOneTimeKeyDeleteKey, dbtypes.MacOneTimeKeyDeleteKey:
		chanID = 1
	case dbtypes.AlInsertKey, dbtypes.DeviceAlDeleteKey, dbtypes.MacDeviceAlDeleteKey:
		chanID = 2
	default:
		log.Infow("encrypt api db event: ignoring unknown output type", log.KeysAndValues{"key", dbEv.Key})
		return nil
	}

	s.msgChan[chanID] <- dbEv
	return nil
}

func (s *E2EDBEVConsumer) onDeviceKeyInsert(
	ctx context.Context, msg *dbtypes.KeyInsert,
) error {
	return s.db.OnInsertDeviceKey(ctx, msg.DeviceID, msg.UserID, msg.KeyInfo, msg.Algorithm, msg.Signature, msg.Identifier)
}

func (s *E2EDBEVConsumer) onOneTimeKeyInsert(
	ctx context.Context, msg *dbtypes.KeyInsert,
) error {
	return s.db.OnInsertOneTimeKey(ctx, msg.DeviceID, msg.UserID, msg.KeyID, msg.KeyInfo, msg.Algorithm, msg.Signature, msg.Identifier)
}

func (s *E2EDBEVConsumer) onOneTimeKeyDelete(
	ctx context.Context, msg *dbtypes.KeyDelete,
) error {
	return s.db.OnDeleteOneTimeKey(ctx, msg.DeviceID, msg.UserID, msg.KeyID, msg.Algorithm)
}

func (s *E2EDBEVConsumer) onMacOneTimeKeyDelete(
	ctx context.Context, msg *dbtypes.MacKeyDelete,
) error {
	return s.db.OnDeleteMacOneTimeKey(ctx, msg.DeviceID, msg.UserID, msg.Identifier)
}

func (s *E2EDBEVConsumer) onAlInsert(
	ctx context.Context, msg *dbtypes.AlInsert,
) error {
	return s.db.OnInsertAl(ctx, msg.UserID, msg.DeviceID, msg.Algorithm, msg.Identifier)
}

func (s *E2EDBEVConsumer) onAlDelete(
	ctx context.Context, msg *dbtypes.DeviceKeyDelete,
) error {
	return s.db.OnDeleteAl(ctx, msg.UserID, msg.DeviceID)
}

func (s *E2EDBEVConsumer) onMacAlDelete(
	ctx context.Context, msg *dbtypes.MacKeyDelete,
) error {
	return s.db.OnDeleteMacAl(ctx, msg.UserID, msg.DeviceID, msg.Identifier)
}

func (s *E2EDBEVConsumer) onDeviceKeyDelete(
	ctx context.Context, msg *dbtypes.DeviceKeyDelete,
) error {
	return s.db.OnDeleteDeviceKey(ctx, msg.DeviceID, msg.UserID)
}

func (s *E2EDBEVConsumer) onMacDeviceKeyDelete(
	ctx context.Context, msg *dbtypes.MacKeyDelete,
) error {
	return s.db.OnDeleteMacDeviceKey(ctx, msg.DeviceID, msg.UserID, msg.Identifier)
}

func (s *E2EDBEVConsumer) onDeviceOneTimeKeyDelete(
	ctx context.Context, msg *dbtypes.DeviceKeyDelete,
) error {
	return s.db.OnDeleteDeviceOneTimeKey(ctx, msg.DeviceID, msg.UserID)
}

func (s *E2EDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.E2EMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
		}
	}

}
