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
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	Register(dbtypes.CATEGORY_ACCOUNT_DB_EVENT, NewAccountDBEvCacheConsumer)
}

type AccountDBEvCacheConsumer struct {
	pool PoolProviderInterface
	//msgChan chan *dbtypes.DBEvent
	msgChan chan common.ContextMsg
}

func (s *AccountDBEvCacheConsumer) startWorker(msgChan chan common.ContextMsg) {
	var res error
	for msg := range msgChan {
		output := msg.Msg.(*dbtypes.DBEvent)
		start := time.Now().UnixNano() / 1000000

		key := output.Key
		data := output.AccountDBEvents
		switch key {
		case dbtypes.AccountDataInsertKey:
			res = s.OnInsertAccountData(data.AccountDataInsert)
		case dbtypes.FilterInsertKey:
			res = s.OnInsertFilter(data.FilterInsert)
		case dbtypes.ProfileInsertKey:
			res = s.OnUpsertProfile(data.ProfileInsert)
		case dbtypes.ProfileInitKey:
			res = s.OnInitProfile(data.ProfileInsert)
		case dbtypes.RoomTagInsertKey:
			res = s.OnInsertRoomTag(data.RoomTagInsert)
		case dbtypes.RoomTagDeleteKey:
			res = s.OnDeleteRoomTag(data.RoomTagDelete)
		case dbtypes.UserInfoInsertKey:
			res = s.OnUpsertUserInfo(data.UserInfoInsert)
		case dbtypes.UserInfoInitKey:
			res = s.OnInitUserInfo(data.UserInfoInsert)
		case dbtypes.UserInfoDeleteKey:
			res = s.OnDeleteUserInfo(data.UserInfoDelete)
		default:
			res = nil
			log.Infow("account db event: ignoring unknown output type", log.KeysAndValues{"key", output.Key})
		}

		if res != nil {
			log.Errorf("write account db event to cache error %v key: %s", res, dbtypes.AccountDBEventKeyToStr(key))
		}

		now := time.Now().UnixNano() / 1000000
		log.Infof("AccountDBEvCacheConsumer process %s takes %d", dbtypes.AccountDBEventKeyToStr(key), now-start)
	}
}

func NewAccountDBEvCacheConsumer() ConsumerInterface {
	s := new(AccountDBEvCacheConsumer)
	s.msgChan = make(chan common.ContextMsg, 4096)

	return s
}

func (s *AccountDBEvCacheConsumer) SetPool(pool PoolProviderInterface) {
	s.pool = pool
}

func (s *AccountDBEvCacheConsumer) Prepare(cfg *config.Dendrite) {
}

func (s *AccountDBEvCacheConsumer) Start() {
	go s.startWorker(s.msgChan)
}

func (s *AccountDBEvCacheConsumer) OnMessage(ctx context.Context, dbEv *dbtypes.DBEvent) error {
	s.msgChan <- common.ContextMsg{Ctx: ctx, Msg: dbEv}
	return nil
}

func (s *AccountDBEvCacheConsumer) OnInsertAccountData(
	msg *dbtypes.AccountDataInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	if msg.RoomID != "" {
		roomAccountDatKey := fmt.Sprintf("%s:%s:%s:%s", "room_account_data", msg.UserID, msg.RoomID, msg.Type)
		err := conn.Send("hmset", roomAccountDatKey, "user_id", msg.UserID, "room_id", msg.RoomID, "type", msg.Type, "content", msg.Content)
		if err != nil {
			return err
		}
		err = conn.Send("hmset", fmt.Sprintf("%s:%s", "room_account_data_list", msg.UserID), roomAccountDatKey, roomAccountDatKey)
		if err != nil {
			return err
		}
	} else {
		accountDatKey := fmt.Sprintf("%s:%s:%s", "account_data", msg.UserID, msg.Type)
		err := conn.Send("hmset", accountDatKey, "user_id", msg.UserID, "type", msg.Type, "content", msg.Content)
		if err != nil {
			return err
		}
		err = conn.Send("hmset", fmt.Sprintf("%s:%s", "account_data_list", msg.UserID), accountDatKey, accountDatKey)
		if err != nil {
			return err
		}
	}

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnInsertFilter(
	msg *dbtypes.FilterInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "filter", msg.UserID, msg.FilterID), "user_id", msg.UserID, "id", msg.FilterID, "filter", msg.Filter)
	if err != nil {
		return err
	}

	/*filterHash := fn.GetStringHash(msg.Filter)
	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "filter_content", msg.UserID, filterHash), "user_id", msg.UserID, "id", msg.FilterID, "filter", msg.Filter)
	if err != nil {
		return err
	}*/

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnUpsertProfile(
	msg *dbtypes.ProfileInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", msg.UserID), "user_id", msg.UserID, "display_name", msg.DisplayName, "avatar_url", msg.AvatarUrl)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnInitProfile(
	msg *dbtypes.ProfileInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "profile", msg.UserID), "user_id", msg.UserID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnInsertRoomTag(
	msg *dbtypes.RoomTagInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	tagKey := fmt.Sprintf("%s:%s:%s:%s", "room_tags", msg.UserID, msg.RoomID, msg.Tag)

	err := conn.Send("hmset", tagKey, "user_id", msg.UserID, "room_id", msg.RoomID, "tag", msg.Tag, "content", msg.Content)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "user_tags_list", msg.UserID), tagKey, tagKey)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "room_tags_list", msg.UserID, msg.RoomID), tagKey, tagKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnDeleteRoomTag(
	msg *dbtypes.RoomTagDelete,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	tagKey := fmt.Sprintf("%s:%s:%s:%s", "room_tags", msg.UserID, msg.RoomID, msg.Tag)

	err := conn.Send("del", tagKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "user_tags_list", msg.UserID), tagKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "room_tags_list", msg.UserID, msg.RoomID), tagKey)
	if err != nil {
		return err
	}

	return nil
}

func (s *AccountDBEvCacheConsumer) OnUpsertUserInfo(
	msg *dbtypes.UserInfoInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "user_info", msg.UserID), "user_id", msg.UserID, "user_name", msg.UserName, "job_number", msg.JobNumber, "mobile", msg.Mobile, "landline", msg.Landline, "email", msg.Email, "state", msg.State)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnInitUserInfo(
	msg *dbtypes.UserInfoInsert,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s", "user_info", msg.UserID), "user_id", msg.UserID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (s *AccountDBEvCacheConsumer) OnDeleteUserInfo(
	msg *dbtypes.UserInfoDelete,
) error {
	conn := s.pool.Pool().Get()
	defer conn.Close()
	UserInfokey := fmt.Sprintf("%s:%s", "user_info", msg.UserID)

	err := conn.Send("del", UserInfokey)
	if err != nil {
		return err
	}

	return nil
}
