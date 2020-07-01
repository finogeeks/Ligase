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

	"github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common/config"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	Register(dbtypes.CATEGORY_ROOM_DB_EVENT, NewRoomDBEVConsumer)
}

type RoomDBEVConsumer struct {
	db          model.RoomServerDatabase
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

func (s *RoomDBEVConsumer) startWorker(msgChan chan *dbtypes.DBEvent) error {
	var res error
	for output := range msgChan {
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.RoomDBEvents
		switch key {
		case dbtypes.EventJsonInsertKey:
			res = s.onEventJsonInsert(context.TODO(), data.EventJsonInsert)
		case dbtypes.EventInsertKey:
			res = s.onEventInsert(context.TODO(), data.EventInsert)
		case dbtypes.EventRoomInsertKey:
			res = s.onEventRoomInsert(context.TODO(), data.EventRoomInsert)
		case dbtypes.EventRoomUpdateKey:
			res = s.onEventRoomUpdate(context.TODO(), data.EventRoomUpdate)
		case dbtypes.EventStateSnapInsertKey:
			res = s.onEventStateSnapInsert(context.TODO(), data.EventStateSnapInsert)
		case dbtypes.EventInviteInsertKey:
			res = s.onEventInviteInsert(context.TODO(), data.EventInviteInsert)
		case dbtypes.EventInviteUpdateKey:
			res = s.onEventInviteUpdate(context.TODO(), data.EventInviteUpdate)
		case dbtypes.EventMembershipInsertKey:
			res = s.onEventMembershipInsert(context.TODO(), data.EventMembershipInsert)
		case dbtypes.EventMembershipUpdateKey:
			res = s.onEventMembershipUpdate(context.TODO(), data.EventMembershipUpdate)
		case dbtypes.EventMembershipForgetUpdateKey:
			res = s.onEventMembershipForgetUpdate(context.TODO(), data.EventMembershipForgetUpdate)
		case dbtypes.AliasInsertKey:
			res = s.onAliasInsert(context.TODO(), data.AliaseInsert)
		case dbtypes.AliasDeleteKey:
			res = s.onAliasDelete(context.TODO(), data.AliaseDelete)
		case dbtypes.RoomDomainInsertKey:
			res = s.onRoomDomainInsert(context.TODO(), data.RoomDomainInsert)
		case dbtypes.RoomEventUpdateKey:
			res = s.onRoomEventUpdate(context.TODO(), data.RoomEventUpdate)
		case dbtypes.RoomDepthUpdateKey:
			res = s.onRoomDepthUpdate(context.TODO(), data.RoomDepthUpdate)
		case dbtypes.SettingUpsertKey:
			res = s.onSettingUpdate(context.TODO(), data.SettingsInsert)
		default:
			res = nil
			log.Infow("room server dbevent: ignoring unknown output type", log.KeysAndValues{"key", key})
		}

		item := s.monState[key]
		if res == nil {
			atomic.AddInt32(&item.process, 1)
		} else {
			atomic.AddInt32(&item.fail, 1)
			if s.IsDump(res.Error()) {
				bytes, _ := json.Marshal(output)
				log.Warnf("write room db event to db cmd %s warn %v event:%s", dbtypes.RoomDBEventKeyToStr(key), res, string(bytes))
			} else {
				log.Errorf("write room db event to db cmd %s error %v", dbtypes.RoomDBEventKeyToStr(key), res)
			}
		}

		if res != nil {
			if s.cfg.RetryFlushDB && !s.IsDump(res.Error()) {
				s.processError(output)
			}
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("RoomDBEVConsumer process %s takes %d", dbtypes.RoomDBEventKeyToStr(key), now-start)
	}

	return res
}

func (s *RoomDBEVConsumer) IsDump(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key value")
}

func NewRoomDBEVConsumer() ConsumerInterface {
	s := new(RoomDBEVConsumer)
	//init mon
	s.monState = make([]*DBMonItem, dbtypes.EventMaxKey)
	for i := int64(0); i < dbtypes.EventMaxKey; i++ {
		if dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_ROOM_DB_EVENT, i) != "unknown" {
			item := new(DBMonItem)
			item.tablenamse = dbtypes.DBEventKeyToTableStr(dbtypes.CATEGORY_ROOM_DB_EVENT, i)
			item.method = dbtypes.DBEventKeyToStr(dbtypes.CATEGORY_ROOM_DB_EVENT, i)
			s.monState[i] = item
		}
	}

	//init worker
	s.msgChan = make([]chan *dbtypes.DBEvent, 8)
	for i := uint64(0); i < 8; i++ {
		s.msgChan[i] = make(chan *dbtypes.DBEvent, 4096)
	}

	s.mutex = new(sync.Mutex)
	s.recvMutex = new(sync.Mutex)
	s.fileName = "roomDbEvErrs.txt"
	s.recoverName = "roomDbEvRecover.txt"
	s.ticker = time.NewTimer(600)
	return s
}

func (s *RoomDBEVConsumer) Prepare(cfg *config.Dendrite) {
	db, err := common.GetDBInstance("roomserver", cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}

	s.db = db.(model.RoomServerDatabase)
	s.path = cfg.RecoverPath
	s.cfg = cfg
}

func (s *RoomDBEVConsumer) Start() {
	for i := uint64(0); i < 8; i++ {
		go s.startWorker(s.msgChan[i])
	}

	go s.startRecover()
}

func (s *RoomDBEVConsumer) startRecover() {
	for {
		select {
		case <-s.ticker.C:
			s.ticker.Reset(time.Second * 600) //10分钟一次
			s.recover()
		}
	}
}

func (s *RoomDBEVConsumer) OnMessage(dbev *dbtypes.DBEvent) error {
	chanid := 0
	switch dbev.Key {
	case dbtypes.EventJsonInsertKey:
		chanid = 0
	case dbtypes.EventInsertKey, dbtypes.RoomEventUpdateKey:
		chanid = 1
	case dbtypes.EventRoomInsertKey, dbtypes.EventRoomUpdateKey, dbtypes.RoomDepthUpdateKey:
		chanid = 2
	case dbtypes.EventStateSnapInsertKey, dbtypes.SettingUpsertKey:
		chanid = 3
	case dbtypes.EventInviteInsertKey, dbtypes.EventInviteUpdateKey:
		chanid = 4
	case dbtypes.EventMembershipInsertKey, dbtypes.EventMembershipUpdateKey, dbtypes.EventMembershipForgetUpdateKey:
		chanid = 5
	case dbtypes.AliasInsertKey, dbtypes.AliasDeleteKey:
		chanid = 6
	case dbtypes.RoomDomainInsertKey:
		chanid = 7
	default:
		log.Infow("room server dbevent: ignoring unknown output type", log.KeysAndValues{"key", dbev.Key})
		return nil
	}

	s.msgChan[chanid] <- dbev
	return nil
}

func (s *RoomDBEVConsumer) Report(mon monitor.LabeledGauge) {
	for i := int64(0); i < dbtypes.EventMaxKey; i++ {
		item := s.monState[i]
		if item != nil {
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "process").Set(float64(atomic.LoadInt32(&item.process)))
			mon.WithLabelValues("monolith", item.tablenamse, item.method, "fail").Set(float64(atomic.LoadInt32(&item.fail)))
		}
	}

}

func (s *RoomDBEVConsumer) onEventJsonInsert(
	ctx context.Context, msg *dbtypes.EventJsonInsert,
) error {
	err := s.db.InsertEventJSON(ctx, msg.EventNid, msg.EventJson, msg.EventType)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventInsert(
	ctx context.Context, msg *dbtypes.EventInsert,
) error {
	err := s.db.InsertEvent(ctx, msg.EventNid,
		msg.RoomNid, msg.EventType, msg.EventStateKey, msg.EventId,
		msg.RefSha, msg.AuthEventNids, msg.Depth,
		msg.StateSnapNid, msg.RefEventId, msg.Sha,
		msg.Offset, msg.Domain)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventRoomInsert(
	ctx context.Context, msg *dbtypes.EventRoomInsert,
) error {
	err := s.db.InsertRoomNID(ctx, msg.RoomNid, msg.RoomId)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventRoomUpdate(
	ctx context.Context, msg *dbtypes.EventRoomUpdate,
) error {
	err := s.db.UpdateLatestEventNIDs(ctx, msg.RoomNid,
		msg.LatestEventNids,
		msg.LastEventSentNid,
		msg.StateSnapNid,
		msg.Version,
		msg.Depth)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventStateSnapInsert(
	ctx context.Context, msg *dbtypes.EventStateSnapInsert,
) error {
	err := s.db.InsertStateRaw(ctx, msg.StateSnapNid, msg.RoomNid, msg.StateBlockNids)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventInviteInsert(
	ctx context.Context, msg *dbtypes.EventInviteInsert,
) error {
	err := s.db.InsertInvite(ctx, msg.EventId, msg.RoomNid, msg.Target, msg.Sender, msg.Content)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventInviteUpdate(
	ctx context.Context, msg *dbtypes.EventInviteUpdate,
) error {
	err := s.db.InviteUpdate(ctx, msg.RoomNid, msg.Target)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventMembershipInsert(
	ctx context.Context, msg *dbtypes.EventMembershipInsert,
) error {
	err := s.db.MembershipInsert(ctx, msg.RoomNID, msg.Target, msg.RoomID, msg.MembershipNID, msg.EventNID)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventMembershipUpdate(
	ctx context.Context, msg *dbtypes.EventMembershipUpdate,
) error {
	err := s.db.MembershipUpdate(ctx, msg.RoomID, msg.Target, msg.Sender, msg.Membership, msg.EventNID, msg.Version)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onEventMembershipForgetUpdate(
	ctx context.Context, msg *dbtypes.EventMembershipForgetUpdate,
) error {
	err := s.db.MembershipForgetUpdate(ctx, msg.RoomID, msg.Target, msg.ForgetID)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onAliasInsert(
	ctx context.Context, msg *dbtypes.AliaseInsert,
) error {
	err := s.db.AliaseInsertRaw(ctx, msg.Alias, msg.RoomID)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onAliasDelete(
	ctx context.Context, msg *dbtypes.AliaseDelete,
) error {
	err := s.db.AliaseDeleteRaw(ctx, msg.Alias)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onRoomDomainInsert(
	ctx context.Context, msg *dbtypes.RoomDomainInsert,
) error {
	err := s.db.RoomDomainsInsertRaw(ctx, msg.RoomNid, msg.Domain, msg.Offset)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onRoomDepthUpdate(
	ctx context.Context, msg *dbtypes.RoomDepthUpdate,
) error {
	err := s.db.OnUpdateRoomDepth(ctx, msg.Depth, msg.RoomNid)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onRoomEventUpdate(
	ctx context.Context, msg *dbtypes.RoomEventUpdate,
) error {
	err := s.db.OnUpdateRoomEvent(ctx, msg.EventNid, msg.RoomNid, msg.Depth, msg.Offset, msg.Domain)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) onSettingUpdate(
	ctx context.Context, msg *dbtypes.SettingsInsert,
) error {
	err := s.db.SettingsInsertRaw(ctx, msg.SettingKey, msg.Val)
	if err != nil {
		log.Errorf("err :%v", err)
	}
	return err
}

func (s *RoomDBEVConsumer) processError(dbEv *dbtypes.DBEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	if fileObj, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err == nil {
		defer fileObj.Close()
		writeObj := bufio.NewWriterSize(fileObj, 4096)

		bytes, err := json.Marshal(dbEv)
		if err != nil {
			log.Errorf("RoomDBEVConsumer.processError marshal error %v", err)
			return
		}

		log.Infof("RoomDBEVConsumer.processError process data %s", string(bytes))
		if _, err := writeObj.WriteString(string(bytes) + "\n"); err == nil {
			if err := writeObj.Flush(); err != nil {
				log.Errorf("RoomDBEVConsumer.processError Flush err %v", err)
			}
		} else {
			log.Errorf("RoomDBEVConsumer.processError WriteString err %v", err)
		}
	} else {
		log.Errorf("RoomDBEVConsumer.processError open file %s err %v", filePath, err)
	}
}

func (s *RoomDBEVConsumer) renameRecoverFile() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	filePath := fmt.Sprintf("%s/%s", s.path, s.fileName)
	newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
	if exists, _ := common.PathExists(filePath); exists {
		err := os.Rename(filePath, newPath)
		if err == nil {
			return true
		}
		log.Errorf("RoomDBEVConsumer.renameRecoverFile err %v", err)
	}

	return false
}

func (s *RoomDBEVConsumer) recover() {
	log.Infof("RoomDBEVConsumer start recover")
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	if s.renameRecoverFile() {
		newPath := fmt.Sprintf("%s/%s", s.path, s.recoverName)
		f, err := os.Open(newPath)
		if err != nil {
			log.Errorf("RoomDBEVConsumer.recover open file %s err %v", newPath, err)
			return
		}

		rd := bufio.NewReader(f)
		for {
			line, err := rd.ReadString('\n') //以'\n'为结束符读入一行
			if err != nil || io.EOF == err {
				break
			}
			log.Infof("RoomDBEVConsumer.processError recover data %s", line)

			var dbEv dbtypes.DBEvent
			err = json.Unmarshal([]byte(line), &dbEv)
			if err != nil {
				log.Errorf("RoomDBEVConsumer.recover unmarshal err %v", err)
				continue
			}

			s.OnMessage(&dbEv)
		}

		f.Close()
		err = os.Remove(newPath)
		if err != nil {
			log.Errorf("RoomDBEVConsumer.recover remove file %s err %v", newPath, err)
		}
	}
	log.Infof("RoomDBEVConsumer finished recover")
}
