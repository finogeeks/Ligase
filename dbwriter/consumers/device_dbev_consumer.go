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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(dbtypes.CATEGORY_DEVICE_DB_EVENT, NewDeviceDBEVConsumer)
}

type DeviceDBEVConsumer struct {
	db model.DeviceDatabase
	//msgChan     []chan *dbtypes.DBEvent
	msgChan     []chan common.ContextMsg
	monState    []*DBMonItem
	path        string
	fileName    string
	recoverName string
	mutex       *sync.Mutex
	recvMutex   *sync.Mutex
	ticker      *time.Timer
	cfg         *config.Dendrite
}

func (s *DeviceDBEVConsumer) startWorker(msgChan chan common.ContextMsg) error {
	var res error
	for msg := range msgChan {
		ctx := msg.Ctx
		output := msg.Msg.(*dbtypes.DBEvent)
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.DeviceDBEvents
		switch key {
		case dbtypes.DeviceInsertKey:
			res = s.onDeviceInsert(ctx, data.DeviceInsert)
		case dbtypes.DeviceDeleteKey:
			res = s.onDeviceDelete(ctx, data.DeviceDelete)
		case dbtypes.MigDeviceInsertKey:
			res = s.onMigDeviceInsert(ctx, data.MigDeviceInsert)
		case dbtypes.DeviceUpdateTsKey:
			res = s.onUpdateDeviceActiveTs(ctx, data.DeviceUpdateTs)
		default:
			res = nil
			log.Infow("device db event: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.AddInt32(&item.process, 1)
		} else {
			atomic.AddInt32(&item.fail, 1)
			if s.IsDump(res.Error()) {
				bytes, _ := json.Marshal(output)
				log.Warnf("write device db event to db warn %v key: %s event:%s", res, dbtypes.DeviceDBEventKeyToStr(key), string(bytes))
			} else {
				log.Errorf("write device db event to db error %v key: %s", res, dbtypes.DeviceDBEventKeyToStr(key))
			}
		}

		if res != nil {
			if s.cfg.RetryFlushDB && !s.IsDump(res.Error()) {
				s.processError(output)
			}
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("DeviceDBEVConsumer process %s takes %d", dbtypes.DeviceDBEventKeyToStr(key), now-start)
	}

	return res
}

func (s *DeviceDBEVConsumer) IsDump(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key value")
}

func NewDeviceDBEVConsumer() ConsumerInterface {
	s := new(DeviceDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.DeviceMaxKey)
	for i := int64(0); i < dbtypes.DeviceMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_DEVICE_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_DEVICE_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_DEVICE_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan common.ContextMsg, 1)
	for i := uint64(0); i < 1; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 4096)
	}

	s.mutex = new(sync.Mutex)
	s.recvMutex = new(sync.Mutex)
	s.fileName = "deviceDbEvErrs.txt"
	s.recoverName = "deviceDbEvRecover.txt"
	s.ticker = time.NewTimer(600)
	return s
}

func (s *DeviceDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("devices", cfg)
	if err != nil {
		log.Panicf("failed to connect to device db")
	}

	s.db = db.(model.DeviceDatabase)
	s.path = cfg.RecoverPath
	s.cfg = cfg
}

func (s *DeviceDBEVConsumer) Start() {
	for i := uint64(0); i < 1; i++ {
		go s.startWorker(s.msgChan[i])
	}

	go s.startRecover()
}

func (s *DeviceDBEVConsumer) startRecover() {
	for {
		select {
		case <-s.ticker.C:
			s.ticker.Reset(time.Second * 600) //10分钟一次
			func() {
				span, ctx := common.StartSobSomSpan(context.Background(), "DeviceDBEVConsumer.startRecover")
				defer span.Finish()
				s.recover(ctx)
			}()
		}
	}
}

func (s *DeviceDBEVConsumer) OnMessage(ctx context.Context, dbEv *dbtypes.DBEvent) error {
	chanID := 0
	switch dbEv.Key {
	case dbtypes.DeviceInsertKey, dbtypes.DeviceDeleteKey, dbtypes.MigDeviceInsertKey, dbtypes.DeviceUpdateTsKey:
		chanID = 0
	default:
		log.Infow("device db event: ignoring unknown output type", log.KeysAndValues{"key", dbEv.Key})
		return nil
	}

	s.msgChan[chanID] <- common.ContextMsg{Ctx: ctx, Msg: dbEv}
	return nil
}

func (s *DeviceDBEVConsumer) onDeviceInsert(
	ctx context.Context, msg *dbtypes.DeviceInsert,
) error {
	return s.db.OnInsertDevice(ctx, msg.UserID, &msg.DeviceID, &msg.DisplayName, msg.CreatedTs, msg.LastActiveTs, msg.DeviceType, msg.Identifier)
}

func (s *DeviceDBEVConsumer) onMigDeviceInsert(
	ctx context.Context, msg *dbtypes.MigDeviceInsert,
) error {
	return s.db.OnInsertMigDevice(ctx, msg.AccessToken, msg.MigAccessToken, msg.DeviceID, msg.UserID)
}

func (s *DeviceDBEVConsumer) onDeviceDelete(
	ctx context.Context, msg *dbtypes.DeviceDelete,
) error {
	err := s.db.OnDeleteDevice(ctx, msg.DeviceID, msg.UserID, msg.CreateTs)
	if err != nil {
		return err
	}

	return s.db.RemoveMigDevice(ctx, msg.DeviceID, msg.UserID)
}

func (s *DeviceDBEVConsumer) onUpdateDeviceActiveTs(
	ctx context.Context, msg *dbtypes.DeviceUpdateTs,
) error {
	err := s.db.OnUpdateDeviceActiveTs(ctx, msg.DeviceID, msg.UserID, msg.LastActiveTs)
	return err
}

func (s *DeviceDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.DeviceMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
		}
	}

}

func (s *DeviceDBEVConsumer) processError(dbEv *dbtypes.DBEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	if fileObj, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err == nil {
		defer fileObj.Close()
		writeObj := bufio.NewWriterSize(fileObj, 4096)

		bytes, err := json.Marshal(dbEv)
		if err != nil {
			log.Errorf("DeviceDBEVConsumer.processError marshal error %v", err)
			return
		}

		log.Infof("DeviceDBEVConsumer.processError process data %s", string(bytes))
		if _, err := writeObj.WriteString(string(bytes) + "\n"); err == nil {
			if err := writeObj.Flush(); err != nil {
				log.Errorf("DeviceDBEVConsumer.processError Flush err %v", err)
			}
		} else {
			log.Errorf("DeviceDBEVConsumer.processError WriteString err %v", err)
		}
	} else {
		log.Errorf("DeviceDBEVConsumer.processError open file %s err %v", filePath, err)
	}
}

func (s *DeviceDBEVConsumer) renameRecoverFile() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
	if exists, _ := common.PathExists(filePath); exists {
		err := os.Rename(filePath, newPath)
		if err == nil {
			return true
		}
		log.Errorf("DeviceDBEVConsumer.renameRecoverFile err %v", err)
	}

	return false
}

func (s *DeviceDBEVConsumer) recover(ctx context.Context) {
	log.Infof("DeviceDBEVConsumer start recover")
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	if s.renameRecoverFile() {
		newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
		f, err := os.Open(newPath)
		if err != nil {
			log.Errorf("DeviceDBEVConsumer.recover open file %s err %v", newPath, err)
			return
		}

		rd := bufio.NewReader(f)
		for {
			line, err := rd.ReadString('\n') //以'\n'为结束符读入一行
			if err != nil || io.EOF == err {
				break
			}
			log.Infof("DeviceDBEVConsumer.processError recover data %s", line)

			var dbEv dbtypes.DBEvent
			err = json.Unmarshal([]byte(line), &dbEv)
			if err != nil {
				log.Errorf("DeviceDBEVConsumer.recover unmarshal err %v", err)
				continue
			}

			s.OnMessage(ctx, &dbEv)
		}

		f.Close()
		err = os.Remove(newPath)
		if err != nil {
			log.Errorf("DeviceDBEVConsumer.recover remove file %s err %v", newPath, err)
		}
	}
	log.Infof("DeviceDBEVConsumer finished recover")
}
