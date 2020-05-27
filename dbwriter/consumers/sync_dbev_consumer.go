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
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	Register(dbtypes.CATEGORY_SYNC_DB_EVENT, NewSyncDBEVConsumer)
}

type SyncDBEVConsumer struct {
	db model.SyncAPIDatabase
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

func (s *SyncDBEVConsumer) startWorker(msgChan chan common.ContextMsg) error {
	var res error
	for msg := range msgChan {
		ctx := msg.Ctx
		output := msg.Msg.(*dbtypes.DBEvent)
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.SyncDBEvents
		switch key {
		case dbtypes.SyncEventInsertKey:
			if data.SyncEventInsert != nil {
				res = s.onSyncEventInsert(ctx, data.SyncEventInsert)
			}
		case dbtypes.SyncRoomStateUpdateKey:
			if data.SyncRoomStateUpdate != nil {
				res = s.onSyncRoomStateUpdate(ctx, data.SyncRoomStateUpdate)
			}
		case dbtypes.SyncClientDataInsertKey:
			if data.SyncClientDataInsert != nil {
				res = s.onSyncClientDataInsert(ctx, data.SyncClientDataInsert)
			}
		case dbtypes.SyncKeyStreamInsertKey:
			if data.SyncKeyStreamInsert != nil {
				res = s.onSyncKeyStreamInsert(ctx, data.SyncKeyStreamInsert)
			}
		case dbtypes.SyncReceiptInsertKey:
			if data.SyncReceiptInsert != nil {
				res = s.onSyncReceiptInsert(ctx, data.SyncReceiptInsert)
			}
		case dbtypes.SyncStdEventInertKey:
			if data.SyncStdEventInsert != nil {
				res = s.onSyncStdEventInsert(ctx, data.SyncStdEventInsert)
			}
		case dbtypes.SyncStdEventDeleteKey:
			if data.SyncStdEventDelete != nil {
				res = s.onSyncStdEventDelete(ctx, data.SyncStdEventDelete)
			}
		case dbtypes.SyncMacStdEventDeleteKey:
			if data.SyncMacStdEventDelete != nil {
				res = s.onSyncMacStdEventDelete(ctx, data.SyncMacStdEventDelete)
			}
		case dbtypes.SyncDeviceStdEventDeleteKey:
			if data.SyncStdEventDelete != nil {
				res = s.onSyncDeviceStdEventDelete(ctx, data.SyncStdEventDelete)
			}
		case dbtypes.SyncPresenceInsertKey:
			if data.SyncPresenceInsert != nil {
				res = s.onSyncPresenceInsert(ctx, data.SyncPresenceInsert)
			}
		case dbtypes.SyncUserReceiptInsertKey:
			if data.SyncUserReceiptInsert != nil {
				res = s.onSyncUserReceiptInsert(ctx, data.SyncUserReceiptInsert)
			}
		case dbtypes.SyncUserTimeLineInsertKey:
			if data.SyncUserTimeLineInsert != nil {
				res = s.onSyncUserTimeLineInsert(ctx, data.SyncUserTimeLineInsert)
			}
		case dbtypes.SyncOutputMinStreamInsertKey:
			if data.SyncOutputMinStreamInsert != nil {
				res = s.onSyncOutputMinStreamInsert(ctx, data.SyncOutputMinStreamInsert)
			}
		case dbtypes.SyncEventUpdateKey:
			if data.SyncEventUpdate != nil {
				res = s.onSyncOutputEventUpdate(ctx, data.SyncEventUpdate)
			}
		default:
			res = nil
			log.Infow("sync api db event: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.AddInt32(&item.process, 1)
		} else {
			atomic.AddInt32(&item.fail, 1)
			if s.IsDump(res.Error()) {
				bytes, _ := json.Marshal(output)
				log.Warnf("write sync api db event to db warn %v key: %s event:%s", res, dbtypes.SyncDBEventKeyToStr(key), string(bytes))
			} else {
				log.Errorf("write sync api db event to db error %v key: %s", res, dbtypes.SyncDBEventKeyToStr(key))
			}
		}

		if res != nil {
			if s.cfg.RetryFlushDB && !s.IsDump(res.Error()) {
				s.processError(output)
			}
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("SyncDBEVConsumer process %s takes %d", dbtypes.SyncDBEventKeyToStr(key), now-start)
	}

	return res
}

func (s *SyncDBEVConsumer) IsDump(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key value")
}

func NewSyncDBEVConsumer() ConsumerInterface {
	s := new(SyncDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.SyncMaxKey)
	for i := int64(0); i < dbtypes.SyncMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_SYNC_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_SYNC_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_SYNC_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan common.ContextMsg, 10)
	for i := uint64(0); i < 10; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 4096)
	}

	s.mutex = new(sync.Mutex)
	s.recvMutex = new(sync.Mutex)
	s.fileName = "syncDbEvErrs.txt"
	s.recoverName = "syncDbEvRecover.txt"
	s.ticker = time.NewTimer(600)
	return s
}

func (s *SyncDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("syncapi", cfg)
	if err != nil {
		log.Panicf("failed to connect to sync api db")
	}

	s.db = db.(model.SyncAPIDatabase)
	s.path = cfg.RecoverPath
	s.cfg = cfg
}

func (s *SyncDBEVConsumer) Start() {
	for i := uint64(0); i < 10; i++ {
		go s.startWorker(s.msgChan[i])
	}

	go s.startRecover()
}

func (s *SyncDBEVConsumer) startRecover() {
	for {
		select {
		case <-s.ticker.C:
			s.ticker.Reset(time.Second * 600) //10分钟一次
			func() {
				span, ctx := common.StartSobSomSpan(context.Background(), "SyncDBEVConsumer.startRecover")
				defer span.Finish()
				s.recover(ctx)
			}()
		}
	}
}

func (s *SyncDBEVConsumer) OnMessage(ctx context.Context, dbEv *dbtypes.DBEvent) error {
	chanID := 0
	switch dbEv.Key {
	case dbtypes.SyncEventInsertKey, dbtypes.SyncEventUpdateKey:
		chanID = 0
	case dbtypes.SyncRoomStateUpdateKey:
		chanID = 1
	case dbtypes.SyncClientDataInsertKey:
		chanID = 2
	case dbtypes.SyncKeyStreamInsertKey:
		chanID = 3
	case dbtypes.SyncReceiptInsertKey:
		chanID = 4
	case dbtypes.SyncStdEventInertKey, dbtypes.SyncStdEventDeleteKey, dbtypes.SyncDeviceStdEventDeleteKey, dbtypes.SyncMacStdEventDeleteKey:
		chanID = 5
	case dbtypes.SyncPresenceInsertKey:
		chanID = 6
	case dbtypes.SyncUserReceiptInsertKey:
		chanID = 7
	case dbtypes.SyncUserTimeLineInsertKey:
		chanID = 8
	case dbtypes.SyncOutputMinStreamInsertKey:
		chanID = 9
	default:
		log.Infow("sync api db event: ignoring unknown output type", log.KeysAndValues{"key", dbEv.Key})
		return nil
	}

	s.msgChan[chanID] <- common.ContextMsg{Ctx: ctx, Msg: dbEv}
	return nil
}

func (s *SyncDBEVConsumer) onSyncEventInsert(
	ctx context.Context, msg *dbtypes.SyncEventInsert,
) error {
	return s.db.InsertEventRaw(ctx, msg.Pos, msg.RoomId, msg.EventId, msg.EventJson, msg.Add, msg.Remove, msg.Device, msg.TxnId, msg.Type, msg.DomainOffset, msg.Depth, msg.Domain, msg.OriginTs)
}

func (s *SyncDBEVConsumer) onSyncRoomStateUpdate(
	ctx context.Context, msg *dbtypes.SyncRoomStateUpdate,
) error {
	//log.Errorf("%v %v", s.db, msg)
	return s.db.UpdateRoomStateRaw(ctx, msg.RoomId, msg.EventId, msg.EventJson, msg.Type, msg.EventStateKey, msg.Membership, msg.AddPos)
}

func (s *SyncDBEVConsumer) onSyncClientDataInsert(
	ctx context.Context, msg *dbtypes.SyncClientDataInsert,
) error {
	_, err := s.db.OnUpsertClientDataStream(ctx, msg.ID, msg.UserID, msg.RoomID, msg.DataType, msg.StreamType)
	return err
}

func (s *SyncDBEVConsumer) onSyncKeyStreamInsert(
	ctx context.Context, msg *dbtypes.SyncKeyStreamInsert,
) error {
	return s.db.OnInsertKeyChange(ctx, msg.ID, msg.ChangedUserID)
}

func (s *SyncDBEVConsumer) onSyncReceiptInsert(
	ctx context.Context, msg *dbtypes.SyncReceiptInsert,
) error {
	err := s.db.OnUpsertReceiptDataStream(ctx, msg.ID, msg.EvtOffset, msg.RoomID, msg.Content)
	return err
}

func (s *SyncDBEVConsumer) onSyncStdEventInsert(
	ctx context.Context, msg *dbtypes.SyncStdEventInsert,
) error {
	_, err := s.db.OnInsertStdMessage(ctx, msg.ID, msg.StdEvent, msg.TargetUID, msg.TargetDevice, msg.Identifier)
	return err
}

func (s *SyncDBEVConsumer) onSyncStdEventDelete(
	ctx context.Context, msg *dbtypes.SyncStdEventDelete,
) error {
	return s.db.OnDeleteStdMessage(ctx, msg.ID, msg.TargetUID, msg.TargetDevice)
}

func (s *SyncDBEVConsumer) onSyncMacStdEventDelete(
	ctx context.Context, msg *dbtypes.SyncMacStdEventDelete,
) error {
	return s.db.OnDeleteMacStdMessage(ctx, msg.Identifier, msg.TargetUID, msg.TargetDevice)
}

func (s *SyncDBEVConsumer) onSyncDeviceStdEventDelete(
	ctx context.Context, msg *dbtypes.SyncStdEventDelete,
) error {
	return s.db.OnDeleteDeviceStdMessage(ctx, msg.TargetUID, msg.TargetDevice)
}

func (s *SyncDBEVConsumer) onSyncPresenceInsert(
	ctx context.Context, msg *dbtypes.SyncPresenceInsert,
) error {
	_, err := s.db.OnUpsertPresenceDataStream(ctx, msg.ID, msg.UserID, msg.Content)
	return err
}

func (s *SyncDBEVConsumer) onSyncUserReceiptInsert(
	ctx context.Context, msg *dbtypes.SyncUserReceiptInsert,
) error {
	_, err := s.db.OnUpsertUserReceiptData(ctx, msg.RoomID, msg.UserID, msg.Content, msg.EvtOffset)
	return err
}

func (s *SyncDBEVConsumer) onSyncUserTimeLineInsert(
	ctx context.Context, msg *dbtypes.SyncUserTimeLineInsert,
) error {
	return s.db.OnInsertUserTimeLine(ctx, msg.ID, msg.RoomID, msg.EventNID, msg.UserID, msg.RoomState, msg.Ts, msg.EventOffset)
}

func (s *SyncDBEVConsumer) onSyncOutputMinStreamInsert(
	ctx context.Context, msg *dbtypes.SyncOutputMinStreamInsert,
) error {
	return s.db.OnInsertOutputMinStream(ctx, msg.ID, msg.RoomID)
}

func (s *SyncDBEVConsumer) onSyncOutputEventUpdate(
	ctx context.Context, msg *dbtypes.SyncEventUpdate,
) error {
	return s.db.OnUpdateSyncEvent(ctx, msg.DomainOffset, msg.OriginTs, msg.Domain, msg.RoomId, msg.EventId)
}

func (s *SyncDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.E2EMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
		}
	}

}

func (s *SyncDBEVConsumer) processError(dbEv *dbtypes.DBEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	if fileObj, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err == nil {
		defer fileObj.Close()
		writeObj := bufio.NewWriterSize(fileObj, 4096)

		bytes, err := json.Marshal(dbEv)
		if err != nil {
			log.Errorf("SyncDBEVConsumer.processError marshal error %v", err)
			return
		}

		log.Infof("SyncDBEVConsumer.processError process data %s", string(bytes))
		if _, err := writeObj.WriteString(string(bytes) + "\n"); err == nil {
			if err := writeObj.Flush(); err != nil {
				log.Errorf("SyncDBEVConsumer.processError Flush err %v", err)
			}
		} else {
			log.Errorf("SyncDBEVConsumer.processError WriteString err %v", err)
		}
	} else {
		log.Errorf("SyncDBEVConsumer.processError open file %s err %v", filePath, err)
	}
}

func (s *SyncDBEVConsumer) renameRecoverFile() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
	if exists, _ := common.PathExists(filePath); exists {
		err := os.Rename(filePath, newPath)
		if err == nil {
			return true
		}
		log.Errorf("SyncDBEVConsumer.renameRecoverFile err %v", err)
	}

	return false
}

func (s *SyncDBEVConsumer) recover(ctx context.Context) {
	log.Infof("SyncDBEVConsumer start recover")
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	if s.renameRecoverFile() {
		newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
		f, err := os.Open(newPath)
		if err != nil {
			log.Errorf("SyncDBEVConsumer.recover open file %s err %v", newPath, err)
			return
		}

		rd := bufio.NewReader(f)
		for {
			line, err := rd.ReadString('\n') //以'\n'为结束符读入一行
			if err != nil || io.EOF == err {
				break
			}
			log.Infof("SyncDBEVConsumer.processError recover data %s", line)

			var dbEv dbtypes.DBEvent
			err = json.Unmarshal([]byte(line), &dbEv)
			if err != nil {
				log.Errorf("SyncDBEVConsumer.recover unmarshal err %v", err)
				continue
			}

			s.OnMessage(ctx, &dbEv)
		}

		f.Close()
		err = os.Remove(newPath)
		if err != nil {
			log.Errorf("SyncDBEVConsumer.recover remove file %s err %v", newPath, err)
		}
	}
	log.Infof("SyncDBEVConsumer finished recover")
}
