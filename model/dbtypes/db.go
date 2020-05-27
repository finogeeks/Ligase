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

package dbtypes

import "fmt"

const (
	CATEGORY_ROOM_DB_EVENT       int64 = 1
	CATEGORY_DEVICE_DB_EVENT     int64 = 2
	CATEGORY_ACCOUNT_DB_EVENT    int64 = 3
	CATEGORY_PUSH_DB_EVENT       int64 = 4
	CATEGORY_E2E_DB_EVENT        int64 = 5
	CATEGORY_SYNC_DB_EVENT       int64 = 6
	CATEGORY_PUBLICROOM_DB_EVENT int64 = 7
	CATEGORY_PRESENCE_DB_EVENT   int64 = 8
)

func DBEventKeyToStr(category, key int64) string {
	switch category {
	case CATEGORY_ROOM_DB_EVENT:
		return RoomDBEventKeyToStr(key)
	case CATEGORY_DEVICE_DB_EVENT:
		return DeviceDBEventKeyToStr(key)
	case CATEGORY_ACCOUNT_DB_EVENT:
		return AccountDBEventKeyToStr(key)
	case CATEGORY_PUSH_DB_EVENT:
		return PushDBEventKeyToStr(key)
	case CATEGORY_E2E_DB_EVENT:
		return E2EDBEventKeyToStr(key)
	case CATEGORY_SYNC_DB_EVENT:
		return SyncDBEventKeyToStr(key)
	case CATEGORY_PUBLICROOM_DB_EVENT:
		return PublicRoomDBEventKeyToStr(key)
	case CATEGORY_PRESENCE_DB_EVENT:
		return PresenceDBEventKeyToStr(key)
	default:
		return "unknown"
	}
}

func DBEventKeyToTableStr(category, key int64) string {
	switch category {
	case CATEGORY_ROOM_DB_EVENT:
		return RoomDBEventKeyToTableStr(key)
	case CATEGORY_DEVICE_DB_EVENT:
		return DeviceDBEventKeyToTableStr(key)
	case CATEGORY_ACCOUNT_DB_EVENT:
		return AccountDBEventKeyToTableStr(key)
	case CATEGORY_PUSH_DB_EVENT:
		return PushDBEventKeyToTableStr(key)
	case CATEGORY_E2E_DB_EVENT:
		return E2EDBEventKeyToTableStr(key)
	case CATEGORY_SYNC_DB_EVENT:
		return SyncDBEventKeyToTableStr(key)
	case CATEGORY_PUBLICROOM_DB_EVENT:
		return PublicRoomDBEventKeyToTableStr(key)
	case CATEGORY_PRESENCE_DB_EVENT:
		return PresenceDBEventKeyToTableStr(key)
	default:
		return "unknown"
	}
}

func DBCategoryToStr(category int64) string {
	switch category {
	case CATEGORY_ROOM_DB_EVENT:
		return "room_dbev_consumer"
	case CATEGORY_DEVICE_DB_EVENT:
		return "device_dbev_consumer"
	case CATEGORY_ACCOUNT_DB_EVENT:
		return "account_dbev_consumer"
	case CATEGORY_PUSH_DB_EVENT:
		return "push_dbev_consumer"
	case CATEGORY_E2E_DB_EVENT:
		return "e2e_dbev_consumer"
	case CATEGORY_SYNC_DB_EVENT:
		return "sync_dbev_consumer"
	case CATEGORY_PUBLICROOM_DB_EVENT:
		return "public_room_dbev_consumer"
	case CATEGORY_PRESENCE_DB_EVENT:
		return "presence_dbev_consumer"
	default:
		return "unknown"
	}
}

type DBEvent struct {
	Key        int64 `json:"event_type,omitempty"`
	Category   int64 `json:"category,omitempty"`
	IsRecovery bool  `json:"is_recovery,omitempty"` //recover from db
	Uid        int64 `json:"uid,omitempty"`

	RoomDBEvents       RoomDBEvent       `json:"room_db_events,omitempty"`
	DeviceDBEvents     DeviceDBEvent     `json:"device_db_events,omitempty"`
	AccountDBEvents    AccountDBEvent    `json:"account_db_events,omitempty"`
	PushDBEvents       PushDBEvent       `json:"push_db_events,omitempty"`
	E2EDBEvents        E2EDBEvent        `json:"e2e_db_events,omitempty"`
	SyncDBEvents       SyncDBEvent       `json:"sync_db_events,omitempty"`
	PublicRoomDBEvents PublicRoomDBEvent `json:"public_room_db_events,omitempty"`
	PresenceDBEvents   PresenceDBEvent   `json:"presence_db_events,omitempty"`
}

func (dbe *DBEvent) GetTblName() string {
	return DBEventKeyToTableStr(dbe.Category, dbe.Key)
}

func (dbe *DBEvent) GetEventKey() string {
	return fmt.Sprintf("%s:%d", DBEventKeyToTableStr(dbe.Category, dbe.Key), dbe.Uid)
}

func (dbe *DBEvent) GetUid() int64 {
	return dbe.Uid
}

func (dbe *DBEvent) SetUid(uid int64) {
	dbe.Uid = uid
}
