// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package processors

import (
	"context"
	"fmt"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/gomodule/redigo/redis"
)

func init() {
	dbregistry.Register("pushers", NewDBPushPusersProcessor, NewCachePushPusherProcessor)
}

type DBPushPusersProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.PushAPIDatabase
}

func NewDBPushPusersProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBPushPusersProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBPushPusersProcessor) Start() {
	db, err := common.GetDBInstance("pushapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to pushapi db")
	}
	p.db = db.(model.PushAPIDatabase)
}

func (p *DBPushPusersProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.PusherInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.PusherDeleteKey:
		p.processDelete(ctx, inputs)
	case dbtypes.PusherDeleteByKeyKey:
		p.processDelteByKey(ctx, inputs)
	case dbtypes.PusherDeleteByKeyOnlyKey:
		p.processDeleteByKeyOnly(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBPushPusersProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PusherInsert
		err := p.db.OnAddPusher(ctx, msg.UserID, msg.ProfileTag, msg.Kind, msg.AppID, msg.AppDisplayName, msg.DeviceDisplayName, msg.PushKey, msg.Lang, msg.Data, msg.DeviceID)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.ProfileTag, msg.Kind, msg.AppID, msg.AppDisplayName, msg.DeviceDisplayName, msg.PushKey, msg.Lang, msg.Data, msg.DeviceID)
		}
	}
	return nil
}

func (p *DBPushPusersProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PusherDelete
		err := p.db.OnDeleteUserPushers(ctx, msg.UserID, msg.AppID, msg.PushKey)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.UserID, msg.AppID, msg.PushKey)
		}
	}
	return nil
}

func (p *DBPushPusersProcessor) processDelteByKey(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PusherDeleteByKey
		err := p.db.OnDeletePushersByKey(ctx, msg.AppID, msg.PushKey)
		if err != nil {
			log.Error(p.name, "delete by key err", err, msg.AppID, msg.PushKey)
		}
	}
	return nil
}

func (p *DBPushPusersProcessor) processDeleteByKeyOnly(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PushDBEvents.PusherDeleteByKeyOnly
		err := p.db.OnDeletePushersByKeyOnly(ctx, msg.PushKey)
		if err != nil {
			log.Error(p.name, "delete by key only err", err, msg.PushKey)
		}
	}
	return nil
}

type CachePushPusherProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCachePushPusherProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CachePushPusherProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CachePushPusherProcessor) Start() {
}

func (p *CachePushPusherProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.PushDBEvents
	switch key {
	case dbtypes.PusherInsertKey:
		return p.onPusherInsert(ctx, data.PusherInsert)
	case dbtypes.PusherDeleteKey:
		return p.onPusherDelete(ctx, data.PusherDelete)
	case dbtypes.PusherDeleteByKeyKey:
		return p.onPusherDeleteByKey(ctx, data.PusherDeleteByKey)
	case dbtypes.PusherDeleteByKeyOnlyKey:
		return p.onPusherDeleteByKeyOnly(ctx, data.PusherDeleteByKeyOnly)
	}
	return nil
}

func (p *CachePushPusherProcessor) onPusherInsert(ctx context.Context, msg *dbtypes.PusherInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	pusherKey := fmt.Sprintf("%s:%s:%s:%s", "pusher", msg.UserID, msg.AppID, msg.PushKey)

	err := conn.Send("hmset", pusherKey, "user_name", msg.UserID, "profile_tag", msg.ProfileTag, "kind", msg.Kind, "app_id", msg.AppID,
		"app_display_name", msg.AppDisplayName, "device_display_name", msg.DeviceDisplayName, "push_key", msg.PushKey, "push_key_ts", msg.PushKeyTs,
		"lang", msg.Lang, "push_data", msg.Data, "device_id", msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hset", fmt.Sprintf("%s:%s", "pusher_user_list", msg.UserID), pusherKey, pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hset", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey, pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hset", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey), pusherKey, pusherKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CachePushPusherProcessor) onPusherDelete(ctx context.Context, msg *dbtypes.PusherDelete) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	pusherKey := fmt.Sprintf("%s:%s:%s:%s", "pusher", msg.UserID, msg.AppID, msg.PushKey)

	err := conn.Send("del", pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_user_list", msg.UserID), pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey), pusherKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CachePushPusherProcessor) onPusherDeleteByKey(ctx context.Context, msg *dbtypes.PusherDeleteByKey) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey)))
	if err != nil {
		return err
	} else {
		for _, pusherKey := range result {
			userID, err := redis.Values(conn.Do("hmget", pusherKey, "user_name"))
			if err != nil {
				return err
			} else {
				err := conn.Send("del", pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_user_list", userID), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", msg.AppID, msg.PushKey), pusherKey)
				if err != nil {
					return err
				}
			}
		}
	}

	return conn.Flush()
}

func (p *CachePushPusherProcessor) onPusherDeleteByKeyOnly(ctx context.Context, msg *dbtypes.PusherDeleteByKeyOnly) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	var userID string
	var appID string

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey)))
	if err != nil {
		return err
	} else {
		for _, pusherKey := range result {
			reply, err := redis.Values(conn.Do("hmget", pusherKey, "user_name", "app_id"))
			if err != nil {
				return err
			} else {

				reply, err = redis.Scan(reply, &userID, &appID)
				if err != nil {
					return err
				}

				err := conn.Send("del", pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_user_list", userID), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s", "pusher_key_list", msg.PushKey), pusherKey)
				if err != nil {
					return err
				}

				err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "pusher_appid_list", appID, msg.PushKey), pusherKey)
				if err != nil {
					return err
				}
			}
		}
	}

	return conn.Flush()
}
