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

//  GET /_matrix/client/r0/lru/syncserver/rooms
//  GET /_matrix/client/r0/lru/syncwriter/rooms
type GetLRURoomsRequest struct {
	Timestamp string `json:"timestamp"`
}

type GetLRURoomsResponse struct {
	Loaded int    `json:"loaded"`
	Max    int    `json:"max"`
	Server string `json:"server"`
}

//  GET /_matrix/client/r0/lru/syncaggregate/sendtodevice
type GetLRUSendToDeviceRequest struct {
	Timestamp string `json:"timestamp"`
}

type GetLRUSendToDeviceResponse struct {
	Loaded int    `json:"loaded"`
	Max    int    `json:"max"`
	Server string `json:"server"`
}

//  PUT /_matrix/client/r0/lru/syncwriter/room
type PutLRURoomRequest struct {
	RoomID string `json:"roomId"`
}

type PutLRURoomResponse struct {
	RoomID string `json:"roomId"`
}

//  PUT /_matrix/client/r0/lru/syncaggregate/sendtodevice
type PutLRUSendToDeviceRequest struct {
	UserID string `json:"userId"`
	DeviceID string `json:"deviceId"`
}

type PutLRUSendToDeviceResponse struct {
	UserID string `json:"userId"`
	DeviceID string `json:"deviceId"`
}
