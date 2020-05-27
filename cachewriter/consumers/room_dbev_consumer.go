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
	"time"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(dbtypes.CATEGORY_ROOM_DB_EVENT, NewRoomDBEvCacheConsumer)
}

// DBEventDataConsumer consumes db events for roomserver.
type RoomDBEvCacheConsumer struct {
	pool PoolProviderInterface
	//msgChan []chan *dbtypes.DBEvent
	msgChan []chan common.ContextMsg
}

func (s *RoomDBEvCacheConsumer) startWorker(msgChan chan common.ContextMsg) {
	var res error
	for msg := range msgChan {
		ctx := msg.Ctx
		output := msg.Msg.(*dbtypes.DBEvent)
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.RoomDBEvents
		switch key {
		case dbtypes.EventInsertKey:
			res = s.onEventInsert(ctx, *data.EventInsert)
			/*
				case dbtypes.EventJsonInsertKey:
					res = s.onEventJsonInsert(context.TODO(), *data.EventJsonInsert)
				case dbtypes.EventInsertKey:
					res = s.onEventInsert(context.TODO(), *data.EventInsert)
				case dbtypes.EventRoomInsertKey:
					res = s.onEventRoomInsert(context.TODO(), *data.EventRoomInsert)
				case dbtypes.EventRoomUpdateKey:
					res = s.onEventRoomUpdate(context.TODO(), *data.EventRoomUpdate)
				case dbtypes.EventStateSnapInsertKey:
					res = s.onEventStateSnapInsert(context.TODO(), *data.EventStateSnapInsert)
				case dbtypes.EventInviteInsertKey:
					res = s.onEventInviteInsert(context.TODO(), *data.EventInviteInsert)
				case dbtypes.EventInviteUpdateKey:
					res = s.onEventInviteUpdate(context.TODO(), *data.EventInviteUpdate)
				case dbtypes.EventMembershipInsertKey:
					res = s.onEventMembershipInsert(context.TODO(), *data.EventMembershipInsert)
				case dbtypes.EventMembershipUpdateKey:
					res = s.onEventMembershipUpdate(context.TODO(), *data.EventMembershipUpdate)
				case dbtypes.EventMembershipForgetUpdateKey:
					res = s.onEventMembershipForgetUpdate(context.TODO(), *data.EventMembershipForgetUpdate)
			*/
		case dbtypes.SettingUpsertKey:
			res = s.onEventSettingUpsert(ctx, *data.SettingsInsert)
		default:
			res = nil
			//log.Infow("dbevent: ignoring unknown output type", log.KeysAndValues{"key", output.Key})
		}

		if res != nil {
			log.Errorf("write room cache event to cache error: %v key: %s", res, dbtypes.RoomDBEventKeyToStr(key))
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("RoomDBEvCacheConsumer process %s takes %d", dbtypes.RoomDBEventKeyToStr(key), now-start)
	}
}

// NewDBUpdateDataConsumer creates a new DBUpdateData consumer. Call Start() to begin consuming from room servers.
func NewRoomDBEvCacheConsumer() ConsumerInterface {
	s := new(RoomDBEvCacheConsumer)

	s.msgChan = make([]chan common.ContextMsg, 10)
	for i := uint64(0); i < 10; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 16384)
		go s.startWorker(s.msgChan[i])
	}

	return s
}

func (s *RoomDBEvCacheConsumer) SetPool(pool PoolProviderInterface) {
	s.pool = pool
}

//todo set db
func (s *RoomDBEvCacheConsumer) Prepare(cfg *config.Dendrite) {
}

// Start consuming from room servers
func (s *RoomDBEvCacheConsumer) Start() {
	for i := uint64(0); i < 10; i++ {
		go s.startWorker(s.msgChan[i])
	}
}

func (s *RoomDBEvCacheConsumer) OnMessage(ctx context.Context, dbev *dbtypes.DBEvent) error {
	chanid := 0
	switch dbev.Key {
	case dbtypes.EventJsonInsertKey:
		chanid = 0
	case dbtypes.EventInsertKey, dbtypes.SettingUpsertKey:
		chanid = 3
	case dbtypes.EventRoomInsertKey, dbtypes.EventRoomUpdateKey:
		chanid = 4
	case dbtypes.EventStateSnapInsertKey:
		chanid = 6
	case dbtypes.EventInviteInsertKey, dbtypes.EventInviteUpdateKey:
		chanid = 7
	case dbtypes.EventMembershipInsertKey, dbtypes.EventMembershipUpdateKey, dbtypes.EventMembershipForgetUpdateKey:
		chanid = 8
	default:
		log.Infow("room server cache dbevent: ignoring unknown output type", log.KeysAndValues{"key", dbev.Key})
		return nil
	}

	s.msgChan[chanid] <- common.ContextMsg{Ctx: ctx, Msg: dbev}
	return nil
}

//event struct key: 'event:nid', fields: room_nid, event_type_nid, state_key_nid, snapshot_nid, depth, event_id, ref, json
func (s *RoomDBEvCacheConsumer) onEventJsonInsert(
	ctx context.Context, msg dbtypes.EventJsonInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	_, err := conn.Do("hmset", fmt.Sprintf("event:%d", msg.EventNid), "json", msg.EventJson)
	return err
}

func (s *RoomDBEvCacheConsumer) onEventInsert(
	ctx context.Context, msg dbtypes.EventInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	_, err := conn.Do("hmset", fmt.Sprintf("offsets:%d", msg.RoomNid), "depth", msg.Depth,
		msg.Domain, msg.Offset)
	//log.Infof("onEventInsert room:%d set depth %d domain:%s offset:%d", msg.RoomNid, msg.Depth, msg.Domain, msg.Offset)
	return err
}

//room struct: key: 'room:nid', fields: room_id, latest_ev_nids, last_send, snap_id
func (s *RoomDBEvCacheConsumer) onEventRoomInsert(
	ctx context.Context, msg dbtypes.EventRoomInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	err := conn.Send("hmset", fmt.Sprintf("room:%d", msg.RoomNid), "room_id", msg.RoomId)
	err = conn.Send("set", fmt.Sprintf("room:%s", msg.RoomId), msg.RoomNid)
	err = conn.Flush()
	return err
}

func (s *RoomDBEvCacheConsumer) onEventRoomUpdate(
	ctx context.Context, msg dbtypes.EventRoomUpdate,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	_, err := conn.Do("hmset", fmt.Sprintf("room:%d", msg.RoomNid), "latest_ev_nids", msg.LatestEventNids,
		"last_send", msg.LastEventSentNid, "snap_id", msg.StateSnapNid)
	return err
}

//snap struct: key: 'roomstatesnap:nid', fields: snap_id, block_nids
func (s *RoomDBEvCacheConsumer) onEventStateSnapInsert(
	ctx context.Context, msg dbtypes.EventStateSnapInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	_, err := conn.Do("hmset", fmt.Sprintf("roomstatesnap:%d", msg.RoomNid), "snap_id", msg.StateSnapNid,
		"block_nids", msg.StateBlockNids)
	return err
}

func (s *RoomDBEvCacheConsumer) onEventInviteInsert(
	ctx context.Context, msg dbtypes.EventInviteInsert,
) error {
	//err := s.db.InsertInvite(ctx, msg.EventId, msg.RoomNid, msg.Target, msg.Sender, msg.Content)
	//return err
	return nil
}

func (s *RoomDBEvCacheConsumer) onEventInviteUpdate(
	ctx context.Context, msg dbtypes.EventInviteUpdate,
) error {
	//err := s.db.InviteUpdate(ctx, msg.RoomNid, msg.Target)
	//return err
	return nil
}

func (s *RoomDBEvCacheConsumer) onEventMembershipInsert(
	ctx context.Context, msg dbtypes.EventMembershipInsert,
) error {
	//err := s.db.MembershipInsert(ctx, msg.RoomID, msg.Target)
	//return err
	return nil
}

func (s *RoomDBEvCacheConsumer) onEventMembershipUpdate(
	ctx context.Context, msg dbtypes.EventMembershipUpdate,
) error {
	//err := s.db.MembershipUpdate(ctx, msg.RoomID, msg.Target, msg.Sender, msg.Membership, msg.EventID)
	//return err
	return nil
}

func (s *RoomDBEvCacheConsumer) onEventMembershipForgetUpdate(
	ctx context.Context, msg dbtypes.EventMembershipForgetUpdate,
) error {
	//err := s.db.MembershipForgetUpdate(ctx, msg.RoomID, msg.Target, msg.ForgetID)
	//return err
	return nil
}

func (s *RoomDBEvCacheConsumer) onEventSettingUpsert(
	ctx context.Context, msg dbtypes.SettingsInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	pusherKey := fmt.Sprintf("%s:%s", "setting", msg.SettingKey)

	err := conn.Send("set", pusherKey, msg.Val)
	if err != nil {
		return err
	}

	return conn.Flush()
}
