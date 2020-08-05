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
	Register(dbtypes.CATEGORY_ACCOUNT_DB_EVENT, NewAccountDBEVConsumer)
}

const (
	accDBWorkerCount = 6
)

type AccountDBEVConsumer struct {
	db model.AccountsDatabase
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

func (s *AccountDBEVConsumer) startWorker(msgChan chan common.ContextMsg) error {
	var res error
	for msg := range msgChan {
		ctx := msg.Ctx
		output := msg.Msg.(*dbtypes.DBEvent)
		// start := time.Now().UnixNano() / 1000000
		start := time.Now()

		key := output.Key
		data := output.AccountDBEvents
		switch key {
		case dbtypes.AccountDataInsertKey:
			res = s.OnInsertAccountData(ctx, data.AccountDataInsert)
		case dbtypes.AccountInsertKey:
			res = s.OnInsertAccount(ctx, data.AccountInsert)
		case dbtypes.FilterInsertKey:
			res = s.OnInsertFilter(ctx, data.FilterInsert)
		case dbtypes.ProfileInsertKey:
			res = s.OnUpsertProfile(ctx, data.ProfileInsert)
		case dbtypes.ProfileInitKey:
			res = s.OnInitProfile(ctx, data.ProfileInsert)
		case dbtypes.RoomTagInsertKey:
			res = s.OnInsertRoomTag(ctx, data.RoomTagInsert)
		case dbtypes.RoomTagDeleteKey:
			res = s.OnDeleteRoomTag(ctx, data.RoomTagDelete)
		case dbtypes.DisplayNameInsertKey:
			res = s.OnUpsertDisplayName(ctx, data.ProfileInsert)
		case dbtypes.AvatarInsertKey:
			res = s.OnUpsertAvatar(ctx, data.ProfileInsert)
		case dbtypes.UserInfoInsertKey:
			res = s.OnUpsertUserInfo(ctx, data.UserInfoInsert)
		case dbtypes.UserInfoInitKey:
			res = s.OnInitUserInfo(ctx, data.UserInfoInsert)
		case dbtypes.UserInfoDeleteKey:
			res = s.OnDeleteUserInfo(ctx, data.UserInfoDelete)
		default:
			res = nil
			log.Infow("account db event: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.StoreInt64(&item.duration, int64(time.Since(start))/int64(time.Millisecond))
			atomic.AddInt32(&item.process, 1)
		} else {
			atomic.AddInt32(&item.fail, 1)
			if s.IsDump(res.Error()) {
				bytes, _ := json.Marshal(output)
				log.Warnf("write account db event to db warn %v key: %s event:%s", res, dbtypes.AccountDBEventKeyToStr(key), string(bytes))
			} else {
				log.Errorf("write account db event to db error %v key: %s", res, dbtypes.AccountDBEventKeyToStr(key))
			}
		}

		if res != nil {
			if s.cfg.RetryFlushDB && !s.IsDump(res.Error()) {
				s.processError(output)
			}
		}

		// now := time.Now().UnixNano() / 1000000
		log.Infof("AccountDBEVConsumer process %s takes %d", dbtypes.AccountDBEventKeyToStr(key), item.duration)
	}

	return res
}

func (s *AccountDBEVConsumer) IsDump(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key value")
}

func NewAccountDBEVConsumer() ConsumerInterface {
	s := new(AccountDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.AccountMaxKey)
	for i := int64(0); i < dbtypes.AccountMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_ACCOUNT_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_ACCOUNT_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_ACCOUNT_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan common.ContextMsg, accDBWorkerCount)
	for i := uint64(0); i < accDBWorkerCount; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 4096)
	}

	s.mutex = new(sync.Mutex)
	s.recvMutex = new(sync.Mutex)
	s.fileName = "accountDbEvErrs.txt"
	s.recoverName = "accountDbEvRecover.txt"
	s.ticker = time.NewTimer(600)
	return s
}

func (s *AccountDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("accounts", cfg)
	if err != nil {
		log.Panicf("failed to connect to account db")
	}

	s.db = db.(model.AccountsDatabase)
	s.path = cfg.RecoverPath
	s.cfg = cfg
}

func (s *AccountDBEVConsumer) Start() {
	for i := uint64(0); i < accDBWorkerCount; i++ {
		go s.startWorker(s.msgChan[i])
	}

	go s.startRecover()
}

func (s *AccountDBEVConsumer) startRecover() {
	for {
		select {
		case <-s.ticker.C:
			s.ticker.Reset(time.Second * 600) //10分钟一次
			func() {
				span, ctx := common.StartSobSomSpan(context.Background(), "AccountDBEVConsumer.startRecover")
				defer span.Finish()
				s.recover(ctx)
			}()
		}
	}
}

func (s *AccountDBEVConsumer) OnMessage(ctx context.Context, dbEv *dbtypes.DBEvent) error {
	chanID := 0
	switch dbEv.Key {
	case dbtypes.AccountDataInsertKey:
		chanID = 0
	case dbtypes.AccountInsertKey:
		chanID = 1
	case dbtypes.FilterInsertKey:
		chanID = 2
	case dbtypes.ProfileInsertKey, dbtypes.ProfileInitKey, dbtypes.AvatarInsertKey, dbtypes.DisplayNameInsertKey:
		chanID = 3
	case dbtypes.RoomTagInsertKey, dbtypes.RoomTagDeleteKey:
		chanID = 4
	case dbtypes.UserInfoInsertKey, dbtypes.UserInfoInitKey, dbtypes.UserInfoDeleteKey:
		chanID = 5
	default:
		log.Infow("device db event: ignoring unknown output type", log.KeysAndValues{"key", dbEv.Key})
		return nil
	}

	s.msgChan[chanID] <- common.ContextMsg{Ctx: ctx, Msg: dbEv}
	return nil
}

func (s *AccountDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.AccountMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "duration").Set(float64(atomic.LoadInt64(&item.duration)))
		}
	}

}

func (s *AccountDBEVConsumer) OnInsertAccountData(
	ctx context.Context, msg *dbtypes.AccountDataInsert,
) error {
	return s.db.OnInsertAccountData(ctx, msg.UserID, msg.RoomID, msg.Type, msg.Content)
}

func (s *AccountDBEVConsumer) OnInsertAccount(
	ctx context.Context, msg *dbtypes.AccountInsert,
) error {
	return s.db.OnInsertAccount(ctx, msg.UserID, msg.PassWordHash, msg.AppServiceID, msg.CreatedTs)
}

func (s *AccountDBEVConsumer) OnInsertFilter(
	ctx context.Context, msg *dbtypes.FilterInsert,
) error {
	log.Infof("OnInsertFilter %s %s %s", msg.Filter, msg.FilterID, msg.UserID)
	return s.db.OnInsertFilter(ctx, msg.Filter, msg.FilterID, msg.UserID)
}

func (s *AccountDBEVConsumer) OnUpsertProfile(
	ctx context.Context, msg *dbtypes.ProfileInsert,
) error {
	return s.db.OnUpsertProfile(ctx, msg.UserID, msg.DisplayName, msg.AvatarUrl)
}

func (s *AccountDBEVConsumer) OnUpsertDisplayName(
	ctx context.Context, msg *dbtypes.ProfileInsert,
) error {
	return s.db.OnUpsertDisplayName(ctx, msg.UserID, msg.DisplayName)
}

func (s *AccountDBEVConsumer) OnUpsertAvatar(
	ctx context.Context, msg *dbtypes.ProfileInsert,
) error {
	return s.db.OnUpsertAvatar(ctx, msg.UserID, msg.AvatarUrl)
}

func (s *AccountDBEVConsumer) OnInitProfile(
	ctx context.Context, msg *dbtypes.ProfileInsert,
) error {
	return s.db.OnInitProfile(ctx, msg.UserID, msg.DisplayName, msg.AvatarUrl)
}

func (s *AccountDBEVConsumer) OnInsertRoomTag(
	ctx context.Context, msg *dbtypes.RoomTagInsert,
) error {
	return s.db.OnInsertRoomTag(ctx, msg.UserID, msg.RoomID, msg.Tag, msg.Content)
}

func (s *AccountDBEVConsumer) OnDeleteRoomTag(
	ctx context.Context, msg *dbtypes.RoomTagDelete,
) error {
	return s.db.OnDeleteRoomTag(ctx, msg.UserID, msg.RoomID, msg.Tag)
}

func (s *AccountDBEVConsumer) OnUpsertUserInfo(
	ctx context.Context, msg *dbtypes.UserInfoInsert,
) error {
	return s.db.OnUpsertUserInfo(ctx, msg.UserID, msg.UserName, msg.JobNumber, msg.Mobile, msg.Landline, msg.Email, msg.State)
}

func (s *AccountDBEVConsumer) OnInitUserInfo(
	ctx context.Context, msg *dbtypes.UserInfoInsert,
) error {
	return s.db.OnInitUserInfo(ctx, msg.UserID, msg.UserName, msg.JobNumber, msg.Mobile, msg.Landline, msg.Email, msg.State)
}

func (s *AccountDBEVConsumer) OnDeleteUserInfo(
	ctx context.Context, msg *dbtypes.UserInfoDelete,
) error {
	return s.db.OnDeleteUserInfo(ctx, msg.UserID)
}

func (s *AccountDBEVConsumer) processError(dbEv *dbtypes.DBEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	if fileObj, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err == nil {
		defer fileObj.Close()
		writeObj := bufio.NewWriterSize(fileObj, 4096)

		bytes, err := json.Marshal(dbEv)
		if err != nil {
			log.Errorf("AccountDBEVConsumer.processError marshal error %v", err)
			return
		}

		log.Infof("AccountDBEVConsumer.processError process data %s", string(bytes))
		if _, err := writeObj.WriteString(string(bytes) + "\n"); err == nil {
			if err := writeObj.Flush(); err != nil {
				log.Errorf("AccountDBEVConsumer.processError Flush err %v", err)
			}
		} else {
			log.Errorf("AccountDBEVConsumer.processError WriteString err %v", err)
		}
	} else {
		log.Errorf("AccountDBEVConsumer.processError open file %s err %v", filePath, err)
	}
}

func (s *AccountDBEVConsumer) renameRecoverFile() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
	if exists, _ := common.PathExists(filePath); exists {
		err := os.Rename(filePath, newPath)
		if err == nil {
			return true
		}
		log.Errorf("AccountDBEVConsumer.renameRecoverFile err %v", err)
	}

	return false
}

func (s *AccountDBEVConsumer) recover(ctx context.Context) {
	log.Infof("AccountDBEVConsumer start recover")
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	if s.renameRecoverFile() {
		newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
		f, err := os.Open(newPath)
		if err != nil {
			log.Errorf("AccountDBEVConsumer.recover open file %s err %v", newPath, err)
			return
		}

		rd := bufio.NewReader(f)
		for {
			line, err := rd.ReadString('\n') //以'\n'为结束符读入一行
			if err != nil || io.EOF == err {
				break
			}
			log.Infof("AccountDBEVConsumer.processError recover data %s", line)

			var dbEv dbtypes.DBEvent
			err = json.Unmarshal([]byte(line), &dbEv)
			if err != nil {
				log.Errorf("AccountDBEVConsumer.recover unmarshal err %v", err)
				continue
			}

			s.OnMessage(ctx, &dbEv)
		}

		f.Close()
		err = os.Remove(newPath)
		if err != nil {
			log.Errorf("AccountDBEVConsumer.recover remove file %s err %v", newPath, err)
		}
	}
	log.Infof("AccountDBEVConsumer finished recover")
}
