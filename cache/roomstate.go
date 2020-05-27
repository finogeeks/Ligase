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

package cache

import (
	"errors"
	"fmt"
	"github.com/finogeeks/ligase/model/types"
	"github.com/gomodule/redigo/redis"
	"strings"
)

const (
	ROOM_STATE_PREFIX     = "room_state"
	ROOM_STATE_EXT_PREFIX = "room_state_ext"
	EXT_DOMAIN_PROFIX     = "domain:"
)

func (rc *RedisCache) SetRoomState(roomID string, state []byte, token string) error {
	key := fmt.Sprintf("%s:%s", ROOM_STATE_PREFIX, roomID)
	val := make(map[string]interface{})
	val["state"] = state
	val["token"] = token
	return rc.HMSet(key, val)
}

func (rc *RedisCache) GetRoomState(roomID string) ([]byte, string, error) {
	key := fmt.Sprintf("%s:%s", ROOM_STATE_PREFIX, roomID)
	reply, err := rc.HMGet(key, []string{"state", "token"})
	var bytes []byte
	var token string
	reply, err = redis.Scan(reply, &bytes, &token)
	return bytes, token, err
}

func (rc *RedisCache) GetRoomStateExt(roomID string) (*types.RoomStateExt, error) {
	key := fmt.Sprintf("%s:%s", ROOM_STATE_EXT_PREFIX, roomID)
	state, err := rc.HGetAll(key)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	ext := &types.RoomStateExt{
		Domains: make(map[string]int64),
	}
	for k, v := range state {
		switch k {
		case "pre_state_id":
			ext.PreStateId, _ = String(v, nil)
		case "last_state_id":
			ext.LastStateId, _ = String(v, nil)
		case "pre_msg_id":
			ext.PreMsgId, _ = String(v, nil)
		case "last_msg_id":
			ext.LastMsgId, _ = String(v, nil)
		case "depth":
			ext.Depth, _ = Int64(v, nil)
		case "has_update":
			ext.HasUpdate, _ = Bool(v, nil)
		case "out_event_offset":
			ext.OutEventOffset, _ = Int64(v, nil)
		default:
			if strings.HasPrefix(k, EXT_DOMAIN_PROFIX) {
				ext.Domains[strings.TrimPrefix(k, EXT_DOMAIN_PROFIX)], _ = Int64(v, nil)
			}
		}
	}
	return ext, nil
}

func (rc *RedisCache) UpdateRoomStateExt(roomID string, ext map[string]interface{}) error {
	key := fmt.Sprintf("%s:%s", ROOM_STATE_EXT_PREFIX, roomID)
	return rc.HMSet(key, ext)
}

func (rc *RedisCache) SetRoomStateExt(roomID string, roomstateExt *types.RoomStateExt) error {
	if roomstateExt == nil {
		return errors.New("set roomstate ext nil")
	}
	ext := make(map[string]interface{})
	ext["pre_state_id"] = roomstateExt.PreStateId
	ext["last_state_id"] = roomstateExt.LastStateId
	ext["pre_msg_id"] = roomstateExt.PreMsgId
	ext["last_msg_id"] = roomstateExt.LastMsgId
	ext["depth"] = roomstateExt.Depth
	ext["has_update"] = roomstateExt.HasUpdate
	ext["out_event_offset"] = roomstateExt.OutEventOffset
	for k, v := range roomstateExt.Domains {
		ext[EXT_DOMAIN_PROFIX+k] = v
	}
	key := fmt.Sprintf("%s:%s", ROOM_STATE_EXT_PREFIX, roomID)
	return rc.HMSet(key, ext)
}
