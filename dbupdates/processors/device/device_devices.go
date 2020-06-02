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
	dbregistry.Register("device_devices", NewDBDeviceDevicesProcessor, NewCacheDeviceDevicesProcessor)
}

type DBDeviceDevicesProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.DeviceDatabase
}

func NewDBDeviceDevicesProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBDeviceDevicesProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBDeviceDevicesProcessor) Start() {
	db, err := common.GetDBInstance("devices", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to devices db")
	}
	p.db = db.(model.DeviceDatabase)
}

func (p *DBDeviceDevicesProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.DeviceInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.DeviceDeleteKey:
		p.processDelete(ctx, inputs)
	case dbtypes.DeviceUpdateTsKey:
		p.processUpdate(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBDeviceDevicesProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.DeviceDBEvents.DeviceInsert
		err := p.db.OnInsertDevice(ctx, msg.UserID, &msg.DeviceID, &msg.DisplayName, msg.CreatedTs, msg.LastActiveTs, msg.DeviceType, msg.Identifier)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, &msg.DeviceID, &msg.DisplayName, msg.CreatedTs, msg.LastActiveTs, msg.DeviceType, msg.Identifier)
		}
	}
	return nil
}

func (p *DBDeviceDevicesProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.DeviceDBEvents.DeviceDelete
		err := p.db.OnDeleteDevice(ctx, msg.DeviceID, msg.UserID, msg.CreateTs)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.DeviceID, msg.UserID, msg.CreateTs)
		}
	}
	return nil
}

func (p *DBDeviceDevicesProcessor) processUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.DeviceDBEvents.DeviceUpdateTs
		err := p.db.OnUpdateDeviceActiveTs(ctx, msg.DeviceID, msg.UserID, msg.LastActiveTs)
		if err != nil {
			log.Error(p.name, "update err", err, msg.DeviceID, msg.UserID, msg.LastActiveTs)
		}
	}
	return nil
}

type CacheDeviceDevicesProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheDeviceDevicesProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheDeviceDevicesProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheDeviceDevicesProcessor) Start() {
}

func (p *CacheDeviceDevicesProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.DeviceDBEvents
	switch key {
	case dbtypes.DeviceInsertKey:
		return p.onDeviceInsert(ctx, data.DeviceInsert)
	case dbtypes.DeviceDeleteKey:
		return p.onDeviceDelete(ctx, data.DeviceDelete)
	case dbtypes.DeviceUpdateTsKey:
		return p.onDeviceUpdateTs(ctx, data.DeviceUpdateTs)
	case dbtypes.DeviceRecoverKey:
		return p.onDeviceRecover(ctx, data.DeviceInsert)
	}
	return nil
}

func (p *CacheDeviceDevicesProcessor) onDeviceInsert(ctx context.Context, msg *dbtypes.DeviceInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	mac := common.GetDeviceMac(msg.DeviceID)

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac)))
	if err != nil {
		return err
	}

	deviceMap := make(map[string]bool)
	for _, deviceID := range result {
		did := deviceID.([]byte)
		deviceMap[string(did)] = true
	}

	for did := range deviceMap {
		if did != msg.DeviceID {
			err := conn.Send("del", fmt.Sprintf("%s:%s:%s", "algorithm", msg.UserID, did))
			if err != nil {
				return err
			}

			result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, did)))
			if err != nil {
				return err
			} else {
				keyMap := make(map[string]bool)

				for _, key := range result {
					keyID := key.([]byte)
					keyMap[string(keyID)] = true
				}

				for keyID := range keyMap {
					err := conn.Send("del", keyID)
					if err != nil {
						return err
					}

					err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, did), keyID)
					if err != nil {
						return err
					}
				}
			}

			result, err = redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, did)))
			if err != nil {
				return err
			} else {
				keyMap := make(map[string]bool)

				for _, key := range result {
					keyID := key.([]byte)
					keyMap[string(keyID)] = true
				}

				for keyID := range keyMap {
					err := conn.Send("del", keyID)
					if err != nil {
						return err
					}

					err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, did), keyID)
					if err != nil {
						return err
					}
				}
			}

			err = conn.Send("del", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, did))
			if err != nil {
				return err
			}
			err = conn.Send("hdel", fmt.Sprintf("%s:%s", "device_list", msg.UserID), did)
			if err != nil {
				return err
			}
			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), did)
			if err != nil {
				return err
			}
		}
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, msg.DeviceID), "did", msg.DeviceID,
		"user_id", msg.UserID, "created_ts", msg.CreatedTs, "display_name", msg.DisplayName, "device_type", msg.DeviceType, "identifier", msg.Identifier)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "device_list", msg.UserID), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheDeviceDevicesProcessor) onDeviceDelete(ctx context.Context, msg *dbtypes.DeviceDelete) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	mac := common.GetDeviceMac(msg.DeviceID)

	err := conn.Send("del", fmt.Sprintf("%s:%s:%s", "algorithm", msg.UserID, msg.DeviceID))
	if err != nil {
		return err
	}

	result, err := redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, msg.DeviceID)))
	if err != nil {
		return err
	} else {
		keyMap := make(map[string]bool)

		for _, key := range result {
			keyID := key.([]byte)
			keyMap[string(keyID)] = true
		}

		for keyID := range keyMap {
			err := conn.Send("del", keyID)
			if err != nil {
				return err
			}

			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, msg.DeviceID), keyID)
			if err != nil {
				return err
			}
		}
	}

	result, err = redis.Values(conn.Do("hgetall", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID)))
	if err != nil {
		return err
	} else {
		keyMap := make(map[string]bool)

		for _, key := range result {
			keyID := key.([]byte)
			keyMap[string(keyID)] = true
		}

		for keyID := range keyMap {
			err := conn.Send("del", keyID)
			if err != nil {
				return err
			}

			err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID), keyID)
			if err != nil {
				return err
			}
		}
	}

	err = conn.Send("del", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, msg.DeviceID))
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s", "device_list", msg.UserID), msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), msg.DeviceID)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheDeviceDevicesProcessor) onDeviceUpdateTs(ctx context.Context, msg *dbtypes.DeviceUpdateTs) error {
	return nil
}

func (p *CacheDeviceDevicesProcessor) onDeviceRecover(ctx context.Context, msg *dbtypes.DeviceInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	mac := common.GetDeviceMac(msg.DeviceID)

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device", msg.UserID, msg.DeviceID), "did", msg.DeviceID,
		"user_id", msg.UserID, "created_ts", msg.CreatedTs, "display_name", msg.DisplayName, "device_type", msg.DeviceType, "identifier", msg.Identifier)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s", "device_list", msg.UserID), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_mac_list", msg.UserID, mac), msg.DeviceID, msg.DeviceID)
	if err != nil {
		return err
	}

	return conn.Flush()
}
