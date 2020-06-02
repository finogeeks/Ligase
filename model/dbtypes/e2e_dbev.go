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
	DeviceKeyInsertKey        int64 = 0
	OneTimeKeyInsertKey       int64 = 1
	OneTimeKeyDeleteKey       int64 = 2
	AlInsertKey               int64 = 3
	DeviceAlDeleteKey         int64 = 4
	DeviceKeyDeleteKey        int64 = 5
	DeviceOneTimeKeyDeleteKey int64 = 6
	MacOneTimeKeyDeleteKey    int64 = 7
	MacDeviceKeyDeleteKey     int64 = 8
	MacDeviceAlDeleteKey      int64 = 9
	E2EMaxKey                 int64 = 10
)

func E2EDBEventKeyToStr(key int64) string {
	switch key {
	case DeviceKeyInsertKey:
		return "DeviceKeyInsert"
	case OneTimeKeyInsertKey:
		return "OneTimeKeyInsert"
	case OneTimeKeyDeleteKey:
		return "OneTimeKeyDelete"
	case AlInsertKey:
		return "AlInsert"
	case DeviceAlDeleteKey:
		return "DeviceAlDelete"
	case DeviceKeyDeleteKey:
		return "DeviceKeyDelete"
	case DeviceOneTimeKeyDeleteKey:
		return "DeviceOneTimeKeyDelete"
	case MacOneTimeKeyDeleteKey:
		return "MacOneTimeKeyDeleteKey"
	case MacDeviceKeyDeleteKey:
		return "MacDeviceKeyDeleteKey"
	case MacDeviceAlDeleteKey:
		return "MacDeviceAlDeleteKey"
	default:
		return "unknown"
	}
}

func E2EDBEventKeyToTableStr(key int64) string {
	switch key {
	case DeviceKeyInsertKey, DeviceKeyDeleteKey, MacDeviceKeyDeleteKey:
		return "encrypt_device_key"
	case OneTimeKeyInsertKey, OneTimeKeyDeleteKey, DeviceOneTimeKeyDeleteKey, MacOneTimeKeyDeleteKey:
		return "encrypt_onetime_key"
	case AlInsertKey, DeviceAlDeleteKey, MacDeviceAlDeleteKey:
		return "encrypt_algorithm"
	default:
		return "unknown"
	}
}

type E2EDBEvent struct {
	KeyInsert       *KeyInsert       `json:"key_insert,omitempty"`
	KeyDelete       *KeyDelete       `json:"key_delete,omitempty"`
	AlInsert        *AlInsert        `json:"al_insert,omitempty"`
	DeviceKeyDelete *DeviceKeyDelete `json:"device_key_delete,omitempty"`
	MacKeyDelete    *MacKeyDelete    `json:"mac_key_delete,omitempty"`
}

type DeviceKeyDelete struct {
	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id"`
}

type AlInsert struct {
	DeviceID   string `json:"device_id"`
	UserID     string `json:"user_id"`
	Algorithm  string `json:"algorithm"`
	Identifier string `json:"identifier"`
}

type KeyInsert struct {
	DeviceID   string `json:"device_id"`
	UserID     string `json:"user_id"`
	KeyID      string `json:"key_id"`
	KeyInfo    string `json:"key_info"`
	Algorithm  string `json:"algorithm"`
	Signature  string `json:"signature"`
	Identifier string `json:"identifier"`
}

type KeyDelete struct {
	DeviceID  string `json:"device_id"`
	UserID    string `json:"user_id"`
	KeyID     string `json:"key_id"`
	Algorithm string `json:"algorithm"`
}

type MacKeyDelete struct {
	DeviceID   string `json:"device_id"`
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
}
