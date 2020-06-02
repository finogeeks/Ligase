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

const (
	DeviceInsertKey    int64 = 0
	DeviceDeleteKey    int64 = 1
	MigDeviceInsertKey int64 = 2
	DeviceRecoverKey   int64 = 3
	DeviceUpdateTsKey  int64 = 4
	DeviceMaxKey       int64 = 5
)

func DeviceDBEventKeyToStr(key int64) string {
	switch key {
	case DeviceInsertKey:
		return "DeviceInsert"
	case DeviceDeleteKey:
		return "DeviceDelete"
	case MigDeviceInsertKey:
		return "MigDeviceInsertKey"
	case DeviceRecoverKey:
		return "DeviceRecoverKey"
	case DeviceUpdateTsKey:
		return "DeviceUpdateTsKey"
	default:
		return "unknown"
	}
}

func DeviceDBEventKeyToTableStr(key int64) string {
	switch key {
	case DeviceInsertKey, DeviceDeleteKey, MigDeviceInsertKey, DeviceRecoverKey, DeviceUpdateTsKey:
		return "device_devices"
	default:
		return "unknown"
	}
}

type DeviceDBEvent struct {
	DeviceInsert    *DeviceInsert    `json:"device_insert,omitempty"`
	DeviceDelete    *DeviceDelete    `json:"device_delete,omitempty"`
	MigDeviceInsert *MigDeviceInsert `json:"mig_device_insert,omitempty"`
	DeviceUpdateTs  *DeviceUpdateTs  `json:"device_update_ts,omitempty"`
}

type DeviceInsert struct {
	DeviceID     string `json:"device_id"`
	DisplayName  string `json:"display_name"`
	UserID       string `json:"user_id"`
	CreatedTs    int64  `json:"created_ts"`
	DeviceType   string `json:"device_type"`
	Identifier   string `json:"identifier"`
	LastActiveTs int64  `json:"last_active_ts"`
}

type DeviceDelete struct {
	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id"`
	CreateTs int64  `json:"create_ts"`
}

type MigDeviceInsert struct {
	DeviceID       string `json:"device_id"`
	UserID         string `json:"user_id"`
	AccessToken    string `json:"access_token"`
	MigAccessToken string `json:"mig_access_token"`
}

type DeviceUpdateTs struct {
	DeviceID     string `json:"device_id"`
	UserID       string `json:"user_id"`
	LastActiveTs int64  `json:"last_active_ts"`
}
