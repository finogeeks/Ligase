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

package external

//GET /_matrix/client/r0/devices
//response
type DeviceList struct {
	Devices []Device `json:"devices"`
}

//GET /_matrix/client/r0/devices/{deviceId}
type GetDeviceRequest struct {
	DeviceID string `json:"device_id"`
}

//response
type Device struct {
	DeviceID    string `json:"device_id"`
	DisplayName string `json:"display_name"`
	LastSeenIP  string `json:"last_seen_ip"`
	LastSeenTs  int    `json:"last_seen_ts"`

	UserID string `json:"user_id,omitempty"` //deprecated
}

//PUT /_matrix/client/r0/devices/{deviceId}
type PutDeviceRequest struct {
	DeviceID    string `json:"device_id"`
	DisplayName string `json:"display_name,omitempty"`
}

//DELETE /_matrix/client/r0/devices/{deviceId}
type DelDeviceRequest struct {
	DeviceID string         `json:"device_id"`
	Auth     DeviceAuthDict `json:"auth"`
}

type PostDelDevicesRequest struct {
	Devices []string       `json:"devices"`
	Auth    DeviceAuthDict `json:"auth"`
}

type DeviceAuthDict struct {
	Type    string `json:"type"`
	Session string `json:"session"`

	Password string `json:"password"` //deprecated
	User     string `json:"user"`     //deprecated
}

type DelDeviceAuthResponse struct {
	Flows   []Flow                 `json:"flows"`
	Params  map[string]interface{} `json:"params"`
	Session string                 `json:"session"`
	//Completed []string               `json:"completed"`
}
