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
	Register(dbtypes.CATEGORY_PUBLICROOM_DB_EVENT, NewPublicRoomDBEVConsumer)
}

type PublicRoomDBEVConsumer struct {
	db          model.PublicRoomAPIDatabase
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

func (s *PublicRoomDBEVConsumer) startWorker(msgChan chan *dbtypes.DBEvent) error {
	var res error
	for output := range msgChan {
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.PublicRoomDBEvents
		switch key {
		case dbtypes.PublicRoomInsertKey:
			res = s.onInsertNewRoom(context.TODO(), data.PublicRoomInsert)
		case dbtypes.PublicRoomUpdateKey:
			res = s.onUpdateRoomAttribute(context.TODO(), data.PublicRoomUpdate)
		case dbtypes.PublicRoomIncrementJoinedKey:
			res = s.onIncrementJoinedMembersInRoom(context.TODO(), data.PublicRoomJoined, 1)
		case dbtypes.PublicRoomDecrementJoinedKey:
			res = s.onDecrementJoinedMembersInRoom(context.TODO(), data.PublicRoomJoined)
		default:
			res = nil
			log.Infow("public room api db event: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.AddInt32(&item.process, 1)
		} else {
			atomic.AddInt32(&item.fail, 1)
			if s.IsDump(res.Error()) {
				bytes, _ := json.Marshal(output)
				log.Warnf("write public room api db event to db warn %v key: %s event:%s", res, dbtypes.PublicRoomDBEventKeyToStr(key), string(bytes))
			} else {
				log.Errorf("write public room api db event to db error %v key: %s", res, dbtypes.PublicRoomDBEventKeyToStr(key))
			}
		}

		if res != nil {
			if s.cfg.RetryFlushDB && !s.IsDump(res.Error()) {
				s.processError(output)
			}
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("PublicRoomDBEVConsumer process %s takes %d", dbtypes.PublicRoomDBEventKeyToStr(key), now-start)
	}

	return res
}

func (s *PublicRoomDBEVConsumer) IsDump(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key value")
}

func NewPublicRoomDBEVConsumer() ConsumerInterface {
	s := new(PublicRoomDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.PublicRoomMaxKey)
	for i := int64(0); i < dbtypes.PublicRoomMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_PUBLICROOM_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_PUBLICROOM_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_PUBLICROOM_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan *dbtypes.DBEvent, 1)
	for i := uint64(0); i < 1; i++ {
		s.msgChan[i] = make(chan *dbtypes.DBEvent, 4096)
	}

	s.mutex = new(sync.Mutex)
	s.recvMutex = new(sync.Mutex)
	s.fileName = "publicRoomDbEvErrs.txt"
	s.recoverName = "publicRoomDbEvRecover.txt"
	s.ticker = time.NewTimer(600)
	return s
}

func (s *PublicRoomDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("publicroomapi", cfg)
	if err != nil {
		log.Panicf("failed to connect to public room api db")
	}

	s.db = db.(model.PublicRoomAPIDatabase)
	s.path = cfg.RecoverPath
	s.cfg = cfg
}

func (s *PublicRoomDBEVConsumer) Start() {
	for i := uint64(0); i < 1; i++ {
		go s.startWorker(s.msgChan[i])
	}

	go s.startRecover()
}

func (s *PublicRoomDBEVConsumer) startRecover() {
	for {
		select {
		case <-s.ticker.C:
			s.ticker.Reset(time.Second * 600) //10分钟一次
			s.recover()
		}
	}
}

func (s *PublicRoomDBEVConsumer) OnMessage(dbEv *dbtypes.DBEvent) error {
	chanID := 0
	switch dbEv.Key {
	case dbtypes.PublicRoomInsertKey, dbtypes.PublicRoomUpdateKey, dbtypes.PublicRoomIncrementJoinedKey, dbtypes.PublicRoomDecrementJoinedKey:
		chanID = 0
	default:
		log.Infow("public room api db event: ignoring unknown output type", log.KeysAndValues{"key", dbEv.Key})
		return nil
	}

	s.msgChan[chanID] <- dbEv
	return nil
}

func (s *PublicRoomDBEVConsumer) onUpdateRoomAttribute(
	ctx context.Context, msg *dbtypes.PublicRoomUpdate,
) error {
	return s.db.OnUpdateRoomAttribute(ctx, msg.AttrName, msg.AttrValue, msg.RoomID)
}

func (s *PublicRoomDBEVConsumer) onIncrementJoinedMembersInRoom(
	ctx context.Context, roomID *string, n int,
) error {
	return s.db.OnIncrementJoinedMembersInRoom(ctx, *roomID, n)
}

func (s *PublicRoomDBEVConsumer) onDecrementJoinedMembersInRoom(
	ctx context.Context, roomID *string,
) error {
	return s.db.OnDecrementJoinedMembersInRoom(ctx, *roomID)
}

func (s *PublicRoomDBEVConsumer) onInsertNewRoom(
	ctx context.Context, msg *dbtypes.PublicRoomInsert,
) error {
	return s.db.OnInsertNewRoom(ctx, msg.RoomID, msg.SeqID, msg.JoinedMembers, msg.Aliases, msg.CanonicalAlias, msg.Name, msg.Topic,
		msg.WorldReadable, msg.GuestCanJoin, msg.AvatarUrl, msg.Visibility)
}

func (s *PublicRoomDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.PublicRoomMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
		}
	}

}

func (s *PublicRoomDBEVConsumer) processError(dbEv *dbtypes.DBEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	if fileObj, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err == nil {
		defer fileObj.Close()
		writeObj := bufio.NewWriterSize(fileObj, 4096)

		bytes, err := json.Marshal(dbEv)
		if err != nil {
			log.Errorf("PublicRoomDBEVConsumer.processError marshal error %v", err)
			return
		}

		log.Infof("PublicRoomDBEVConsumer.processError process data %s", string(bytes))
		if _, err := writeObj.WriteString(string(bytes) + "\n"); err == nil {
			if err := writeObj.Flush(); err != nil {
				log.Errorf("PublicRoomDBEVConsumer.processError Flush err %v", err)
			}
		} else {
			log.Errorf("PublicRoomDBEVConsumer.processError WriteString err %v", err)
		}
	} else {
		log.Errorf("PublicRoomDBEVConsumer.processError open file %s err %v", filePath, err)
	}
}

func (s *PublicRoomDBEVConsumer) renameRecoverFile() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
	if exists, _ := common.PathExists(filePath); exists {
		err := os.Rename(filePath, newPath)
		if err == nil {
			return true
		}
		log.Errorf("PublicRoomDBEVConsumer.renameRecoverFile err %v", err)
	}

	return false
}

func (s *PublicRoomDBEVConsumer) recover() {
	log.Infof("PublicRoomDBEVConsumer start recover")
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	if s.renameRecoverFile() {
		newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
		f, err := os.Open(newPath)
		if err != nil {
			log.Errorf("PublicRoomDBEVConsumer.recover open file %s err %v", newPath, err)
			return
		}

		rd := bufio.NewReader(f)
		for {
			line, err := rd.ReadString('\n') //以'\n'为结束符读入一行
			if err != nil || io.EOF == err {
				break
			}
			log.Infof("PublicRoomDBEVConsumer.processError recover data %s", line)

			var dbEv dbtypes.DBEvent
			err = json.Unmarshal([]byte(line), &dbEv)
			if err != nil {
				log.Errorf("PublicRoomDBEVConsumer.recover unmarshal err %v", err)
				continue
			}

			s.OnMessage(&dbEv)
		}

		f.Close()
		err = os.Remove(newPath)
		if err != nil {
			log.Errorf("PublicRoomDBEVConsumer.recover remove file %s err %v", newPath, err)
		}
	}
	log.Infof("PublicRoomDBEVConsumer finished recover")
}
