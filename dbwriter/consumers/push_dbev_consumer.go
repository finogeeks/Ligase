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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	Register(dbtypes.CATEGORY_PUSH_DB_EVENT, NewPushDBEVConsumer)
}

type PushDBEVConsumer struct {
	db          model.PushAPIDatabase
	msgChan     []chan *dbtypes.DBEvent
	monState    []*DBMonItem
	path        string
	fileName    string
	recoverName string
	mutex       *sync.Mutex
	recvMutex   *sync.Mutex
	ticker      *time.Timer
	cfg         *config.Dendrite
}

func (s *PushDBEVConsumer) startWorker(msgChan chan *dbtypes.DBEvent) error {
	var res error
	for output := range msgChan {
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.PushDBEvents
		switch key {
		case dbtypes.PusherDeleteKey:
			res = s.onPusherDelete(context.TODO(), data.PusherDelete)
		case dbtypes.PusherDeleteByKeyKey:
			res = s.onPusherDeleteByKey(context.TODO(), data.PusherDeleteByKey)
		case dbtypes.PusherDeleteByKeyOnlyKey:
			res = s.onPusherDeleteByKeyOnly(context.TODO(), data.PusherDeleteByKeyOnly)
		case dbtypes.PusherInsertKey:
			res = s.onPusherInsert(context.TODO(), data.PusherInsert)
		case dbtypes.PushRuleUpsertKey:
			res = s.onPushRuleInsert(context.TODO(), data.PushRuleInert)
		case dbtypes.PushRuleDeleteKey:
			res = s.onPushRuleDelete(context.TODO(), data.PushRuleDelete)
		case dbtypes.PushRuleEnableUpsetKey:
			res = s.onPushRuleEnableInsert(context.TODO(), data.PushRuleEnableInsert)
		default:
			res = nil
			log.Infow("push db event: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.AddInt32(&item.process, 1)
		} else {
			atomic.AddInt32(&item.fail, 1)
			if s.IsDump(res.Error()) {
				bytes, _ := json.Marshal(output)
				log.Warnf("write push db event to db warn %v key: %s event:%s", res, dbtypes.PushDBEventKeyToStr(key), string(bytes))
			} else {
				log.Error("write push db event to db error %v key: %s", res, dbtypes.PushDBEventKeyToStr(key))
			}
		}

		if res != nil {
			if s.cfg.RetryFlushDB && !s.IsDump(res.Error()) {
				s.processError(output)
			}
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("PushDBEVConsumer process %s takes %d", dbtypes.PushDBEventKeyToStr(key), now-start)
	}

	return res
}

func (s *PushDBEVConsumer) IsDump(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key value")
}

func NewPushDBEVConsumer() ConsumerInterface {
	s := new(PushDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.PushMaxKey)
	for i := int64(0); i < dbtypes.PushMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_PUSH_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_PUSH_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_PUSH_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan *dbtypes.DBEvent, 3)
	for i := uint64(0); i < 3; i++ {
		s.msgChan[i] = make(chan *dbtypes.DBEvent, 4096)
	}

	s.mutex = new(sync.Mutex)
	s.recvMutex = new(sync.Mutex)
	s.fileName = "pushDbEvErrs.txt"
	s.recoverName = "pushDbEvRecover.txt"
	s.ticker = time.NewTimer(600)
	return s
}

func (s *PushDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("pushapi", cfg)
	if err != nil {
		log.Panicf("failed to connect to push db %v", err)
	}

	s.db = db.(model.PushAPIDatabase)
	s.path = cfg.RecoverPath
	s.cfg = cfg
}

func (s *PushDBEVConsumer) Start() {
	for i := uint64(0); i < 3; i++ {
		go s.startWorker(s.msgChan[i])
	}

	go s.startRecover()
}

func (s *PushDBEVConsumer) startRecover() {
	for {
		select {
		case <-s.ticker.C:
			s.ticker.Reset(time.Second * 600) //10分钟一次
			s.recover()
		}
	}
}

func (s *PushDBEVConsumer) OnMessage(dbEv *dbtypes.DBEvent) error {
	chanID := 0
	switch dbEv.Key {
	case dbtypes.PusherDeleteKey, dbtypes.PusherDeleteByKeyKey, dbtypes.PusherDeleteByKeyOnlyKey, dbtypes.PusherInsertKey:
		chanID = 0
	case dbtypes.PushRuleUpsertKey, dbtypes.PushRuleDeleteKey:
		chanID = 1
	case dbtypes.PushRuleEnableUpsetKey:
		chanID = 2
	default:
		log.Infow("push db event: ignoring unknown output type", log.KeysAndValues{"key", dbEv.Key})
		return nil
	}

	s.msgChan[chanID] <- dbEv
	return nil
}

func (s *PushDBEVConsumer) onPusherDelete(
	ctx context.Context, msg *dbtypes.PusherDelete,
) error {
	return s.db.OnDeleteUserPushers(ctx, msg.UserID, msg.AppID, msg.PushKey)
}

func (s *PushDBEVConsumer) onPusherDeleteByKey(
	ctx context.Context, msg *dbtypes.PusherDeleteByKey,
) error {
	return s.db.OnDeletePushersByKey(ctx, msg.AppID, msg.PushKey)
}

func (s *PushDBEVConsumer) onPusherDeleteByKeyOnly(
	ctx context.Context, msg *dbtypes.PusherDeleteByKeyOnly,
) error {
	return s.db.OnDeletePushersByKeyOnly(ctx, msg.PushKey)
}

func (s *PushDBEVConsumer) onPusherInsert(
	ctx context.Context, msg *dbtypes.PusherInsert,
) error {
	return s.db.OnAddPusher(ctx, msg.UserID, msg.ProfileTag, msg.Kind, msg.AppID, msg.AppDisplayName, msg.DeviceDisplayName, msg.PushKey, msg.PushKeyTs, msg.Lang, msg.Data, msg.DeviceID)
}

func (s *PushDBEVConsumer) onPushRuleInsert(
	ctx context.Context, msg *dbtypes.PushRuleInert,
) error {
	return s.db.OnAddPushRule(ctx, msg.UserID, msg.RuleID, msg.PriorityClass, msg.Priority, msg.Conditions, msg.Actions)
}

func (s *PushDBEVConsumer) onPushRuleDelete(
	ctx context.Context, msg *dbtypes.PushRuleDelete,
) error {
	return s.db.OnDeletePushRule(ctx, msg.UserID, msg.RuleID)
}

func (s *PushDBEVConsumer) onPushRuleEnableInsert(
	ctx context.Context, msg *dbtypes.PushRuleEnableInsert,
) error {
	return s.db.OnAddPushRuleEnable(ctx, msg.UserID, msg.RuleID, msg.Enabled)
}

func (s *PushDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.PushMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
		}
	}

}

func (s *PushDBEVConsumer) processError(dbEv *dbtypes.DBEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	if fileObj, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err == nil {
		defer fileObj.Close()
		writeObj := bufio.NewWriterSize(fileObj, 4096)

		bytes, err := json.Marshal(dbEv)
		if err != nil {
			log.Errorf("PushDBEVConsumer.processError marshal error %v", err)
			return
		}

		log.Infof("PushDBEVConsumer.processError process data %s", string(bytes))
		if _, err := writeObj.WriteString(string(bytes) + "\n"); err == nil {
			if err := writeObj.Flush(); err != nil {
				log.Errorf("PushDBEVConsumer.processError Flush err %v", err)
			}
		} else {
			log.Errorf("PushDBEVConsumer.processError WriteString err %v", err)
		}
	} else {
		log.Errorf("PushDBEVConsumer.processError open file %s err %v", filePath, err)
	}
}

func (s *PushDBEVConsumer) renameRecoverFile() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
	if exists, _ := common.PathExists(filePath); exists {
		err := os.Rename(filePath, newPath)
		if err == nil {
			return true
		}
		log.Errorf("PushDBEVConsumer.renameRecoverFile err %v", err)
	}

	return false
}

func (s *PushDBEVConsumer) recover() {
	log.Infof("PushDBEVConsumer start recover")
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	if s.renameRecoverFile() {
		newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
		f, err := os.Open(newPath)
		if err != nil {
			log.Errorf("PushDBEVConsumer.recover open file %s err %v", newPath, err)
			return
		}

		rd := bufio.NewReader(f)
		for {
			line, err := rd.ReadString('\n') //以'\n'为结束符读入一行
			if err != nil || io.EOF == err {
				break
			}
			log.Infof("PushDBEVConsumer.processError recover data %s", line)

			var dbEv dbtypes.DBEvent
			err = json.Unmarshal([]byte(line), &dbEv)
			if err != nil {
				log.Errorf("PushDBEVConsumer.recover unmarshal err %v", err)
				continue
			}

			s.OnMessage(&dbEv)
		}

		f.Close()
		err = os.Remove(newPath)
		if err != nil {
			log.Errorf("PushDBEVConsumer.recover remove file %s err %v", newPath, err)
		}
	}
	log.Infof("PushDBEVConsumer finished recover")
}
