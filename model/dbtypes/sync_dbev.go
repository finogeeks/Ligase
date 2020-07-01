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

import "github.com/finogeeks/ligase/model/syncapitypes"

const (
	SyncEventInsertKey           int64 = 0
	SyncRoomStateUpdateKey       int64 = 1
	SyncClientDataInsertKey      int64 = 2
	SyncKeyStreamInsertKey       int64 = 3
	SyncReceiptInsertKey         int64 = 4
	SyncStdEventInertKey         int64 = 5
	SyncStdEventDeleteKey        int64 = 6
	SyncDeviceStdEventDeleteKey  int64 = 7
	SyncPresenceInsertKey        int64 = 8
	SyncUserReceiptInsertKey     int64 = 9
	SyncMacStdEventDeleteKey     int64 = 10
	SyncUserTimeLineInsertKey    int64 = 11
	SyncOutputMinStreamInsertKey int64 = 12
	SyncEventUpdateKey           int64 = 13
	SyncMaxKey                   int64 = 14
)

func SyncDBEventKeyToStr(key int64) string {
	switch key {
	case SyncEventInsertKey:
		return "SyncEventInsertKey"
	case SyncRoomStateUpdateKey:
		return "SyncRoomStateUpdateKey"
	case SyncClientDataInsertKey:
		return "SyncClientDataInsertKey"
	case SyncKeyStreamInsertKey:
		return "SyncKeyStreamInsertKey"
	case SyncReceiptInsertKey:
		return "SyncReceiptInsertKey"
	case SyncStdEventInertKey:
		return "SyncStdEventInertKey"
	case SyncStdEventDeleteKey:
		return "SyncStdEventDeleteKey"
	case SyncDeviceStdEventDeleteKey:
		return "SyncDeviceStdEventDeleteKey"
	case SyncPresenceInsertKey:
		return "SyncPresenceInsertKey"
	case SyncUserReceiptInsertKey:
		return "SyncUserReceiptInsertKey"
	case SyncMacStdEventDeleteKey:
		return "SyncMacStdEventDeleteKey"
	case SyncUserTimeLineInsertKey:
		return "SyncUserTimeLineInsertKey"
	case SyncOutputMinStreamInsertKey:
		return "SyncOutputMinStreamInsertKey"
	case SyncEventUpdateKey:
		return "SyncEventUpdateKey"
	default:
		return "unknown"
	}
}

func SyncDBEventKeyToTableStr(key int64) string {
	switch key {
	case SyncEventInsertKey, SyncEventUpdateKey:
		return "syncapi_output_room_events"
	case SyncRoomStateUpdateKey:
		return "syncapi_current_room_state"
	case SyncClientDataInsertKey:
		return "syncapi_client_data_stream"
	case SyncKeyStreamInsertKey:
		return "syncapi_key_change"
	case SyncReceiptInsertKey:
		return "syncapi_receipt_data_stream"
	case SyncStdEventInertKey, SyncStdEventDeleteKey, SyncDeviceStdEventDeleteKey, SyncMacStdEventDeleteKey:
		return "syncapi_send_to_device"
	case SyncPresenceInsertKey:
		return "syncapi_presence_data_stream"
	case SyncUserReceiptInsertKey:
		return "syncapi_user_receipt_data"
	case SyncUserTimeLineInsertKey:
		return "syncapi_user_time_line"
	case SyncOutputMinStreamInsertKey:
		return "syncapi_output_min_stream"
	default:
		return "unknown"
	}
}

type SyncDBEvent struct {
	SyncEventInsert           *SyncEventInsert           `json:"sync_event_insert,omitempty"`
	SyncRoomStateUpdate       *SyncRoomStateUpdate       `json:"room_state_update,omitempty"`
	SyncClientDataInsert      *SyncClientDataInsert      `json:"sync_client_data_insert,omitempty"`
	SyncKeyStreamInsert       *SyncKeyStreamInsert       `json:"sync_key_stream_insert,omitempty"`
	SyncReceiptInsert         *SyncReceiptInsert         `json:"sync_receipt_insert,omitempty"`
	SyncStdEventInsert        *SyncStdEventInsert        `json:"sync_std_event_insert,omitempty"`
	SyncStdEventDelete        *SyncStdEventDelete        `json:"sync_std_event_delete,omitempty"`
	SyncPresenceInsert        *SyncPresenceInsert        `json:"sync_presence_insert,omitempty"`
	SyncUserReceiptInsert     *SyncUserReceiptInsert     `json:"sync_user_receipt_insert,omitempty"`
	SyncMacStdEventDelete     *SyncMacStdEventDelete     `json:"sync_mac_std_delete,omitempty"`
	SyncUserTimeLineInsert    *SyncUserTimeLineInsert    `json:"sync_user_time_line_insert,omitempty"`
	SyncOutputMinStreamInsert *SyncOutputMinStreamInsert `json:"sync_output_min_stream_insert,omitempty"`
	SyncEventUpdate           *SyncEventUpdate           `json:"sync_output_event_update,omitempty"`
}

type SyncEventUpdate struct {
	DomainOffset int64  `json:"domain_offset"`
	Domain       string `json:"domain"`
	OriginTs     int64  `json:"origin_ts"`
	RoomId       string `json:"room_id"`
	EventId      string `json:"event_id"`
}

type SyncOutputMinStreamInsert struct {
	ID     int64  `json:"id"`
	RoomID string `json:"room_id"`
}

type SyncUserTimeLineInsert struct {
	ID          int64  `json:"id"`
	RoomID      string `json:"room_id"`
	EventNID    int64  `json:"event_id"`
	UserID      string `json:"user_id"`
	RoomState   string `json:"room_state"`
	Ts          int64  `json:"ts"`
	EventOffset int64  `json:"event_offset"`
}

type SyncUserReceiptInsert struct {
	UserID    string `json:"user_id"`
	RoomID    string `json:"room_id"`
	Content   string `json:"content"`
	EvtOffset int64  `json:"event_offset"`
}

type SyncClientDataInsert struct {
	ID         int64  `json:"id"`
	UserID     string `json:"user_id"`
	RoomID     string `json:"room_id"`
	DataType   string `json:"data_type"`
	StreamType string `json:"stream_type"`
}

type SyncKeyStreamInsert struct {
	ID            int64  `json:"id"`
	ChangedUserID string `json:"changed_user_id"`
}

type SyncReceiptInsert struct {
	ID        int64  `json:"id"`
	EvtOffset int64  `json:"evt_offset"`
	RoomID    string `json:"room_id"`
	Content   string `json:"content"`
}

type SyncPresenceInsert struct {
	ID      int64  `json:"id"`
	UserID  string `json:"user_id"`
	Content string `json:"content"`
}

type SyncStdEventInsert struct {
	ID           int64                  `json:"id"`
	StdEvent     syncapitypes.StdHolder `json:"std_event"`
	TargetUID    string                 `json:"target_uid"`
	TargetDevice string                 `json:"target_device"`
	Identifier   string                 `json:"identifier"`
}

type SyncStdEventDelete struct {
	ID           int64  `json:"id"`
	TargetUID    string `json:"target_uid"`
	TargetDevice string `json:"target_device"`
}

type SyncMacStdEventDelete struct {
	Identifier   string `json:"identifier"`
	TargetUID    string `json:"target_uid"`
	TargetDevice string `json:"target_device"`
}

type SyncEventInsert struct {
	Pos          int64    `json:"position"`
	RoomId       string   `json:"room_id"`
	EventId      string   `json:"event_id"`
	EventJson    []byte   `json:"event_json"`
	Add          []string `json:"add"`
	Remove       []string `json:"remove"`
	Device       string   `json:"device"`
	TxnId        string   `json:"txnId"`
	Type         string   `json:"event_type"`
	DomainOffset int64    `json:"domain_offset"`
	Depth        int64    `json:"depth"`
	Domain       string   `json:"domain"`
	OriginTs     int64    `json:"origin_ts"`
}

type SyncRoomStateUpdate struct {
	RoomId        string `json:"room_id"`
	EventId       string `json:"event_id"`
	Type          string `json:"event_type"`
	EventJson     []byte `json:"event_json"`
	EventStateKey string `json:"event_state_key"`
	Membership    string `json:"membership"`
	AddPos        int64  `json:"add_position"`
}

type SyncRoomStateDelete struct {
	EventId string `json:"event_id"`
}
