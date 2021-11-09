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

import (
	jsonRaw "encoding/json"
)

//GET /_matrix/client/r0/sync
type GetSyncRequest struct {
	Filter      string `json:"filter"`
	Since       string `json:"since"`
	FullState   string `json:"full_state"`
	SetPresence string `json:"set_presence"`
	TimeOut     string `json:"timeout"`
	From        string `json:"from"`
}

type GetSyncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join   map[string]JoinedRoom  `json:"join"`
		Invite map[string]InvitedRoom `json:"invite"`
		Leave  map[string]LeftRoom    `json:"leave"`
	} `json:"rooms"`
	Presence               Presence       `json:"presence"`
	AccountData            AccountData    `json:"account_data"`
	ToDevice               ToDevice       `json:"to_device"`
	DeviceList             DeviceLists    `json:"device_lists"`
	DeviceOneTimeKeysCount map[string]int `json:"device_one_time_keys_count"`
}

type JoinedRoom struct {
	State       State               `json:"state"`
	Timeline    Timeline            `json:"timeline"`
	Ephemeral   Ephemeral           `json:"ephemeral"`
	AccountData AccountData         `json:"account_data"`
	Unread      UnreadNotifications `json:"unread_notifications"`
}

type Ephemeral struct {
	Events []Event `json:"events"`
}

type UnreadNotifications struct {
	HighLightCount    int64 `json:"highlight_count"`
	NotificationCount int64 `json:"notification_count"`
}

type InvitedRoom struct {
	InviteState InviteState `json:"invite_state"`
}

type InviteState struct {
	Events []StrippedState `json:"events"`
}

type StrippedState struct {
	Content  EventContent `json:"content,omitempty"`
	StateKey string       `json:"state_key,omitempty"`
	Type     string       `json:"type,omitempty"`
	Sender   string       `json:"sender,omitempty"`
}

type LeftRoom struct {
	State       State       `json:"state,omitempty"`
	Timeline    Timeline    `json:"timeline,omitempty"`
	AccountData AccountData `json:"account_data,omitempty"`
}

type State struct {
	Events []StateEvent `json:"events"`
}

type StateEvent struct {
	Type           string       `json:"type,omitempty"`
	StateKey       string       `json:"state_key,omitempty"`
	Content        interface{}  `json:"content,omitempty"`
	EventID        string       `json:"event_id,omitempty"`
	Sender         string       `json:"sender,omitempty"`
	OriginServerTS int64        `json:"origin_server_ts,omitempty"`
	Unsigned       UnsignedData `json:"unsigned,omitempty"`
	PrevContent    EventContent `json:"prev_content,omitempty"`
	RoomID         string       `json:"unsigned,omitempty"`
}

type Timeline struct {
	Events    []RoomEvent `json:"events"`
	Limited   bool        `json:"limited"`
	PrevBatch string      `json:"prev_batch"`
}

type RoomEvent struct {
	Content        []byte       `json:"content,omitempty"`
	Type           string       `json:"type,omitempty"`
	EventID        string       `json:"event_id,omitempty"`
	Sender         string       `json:"sender,omitempty"`
	OriginServerTS int64        `json:"origin_server_ts,omitempty"`
	Unsigned       UnsignedData `json:"unsigned,omitempty"`
	RoomID         string       `json:"unsigned,omitempty"`
}

type UnsignedData struct {
	Age             int    `json:"age,omitempty"`
	RedactedBecause Event  `json:"redacted_because,omitempty"`
	TransactionID   string `json:"transaction_id,omitempty"`
}

type Presence struct {
	Events []Event `json:"events"`
}

type AccountData struct {
	Events []Event `json:"events"`
}

type ToDevice struct {
	Events []Event `json:"events"`
}

type DeviceLists struct {
	Changed []string `json:"changed,omitempty"`
	Left    []string `json:"left,omitempty"`
}

type Event struct {
	Content jsonRaw.RawMessage `json:"content,omitempty"`
	Type    string             `json:"type,omitempty"`
	EventID string             `json:"event_id,omitempty"`
	Sender  string             `json:"sender,omitempty"`
}

type EventContent struct {
	AvatarURL        string       `json:"avatar_url,omitempty"`
	DisplayName      *string      `json:"displayname,omitempty"`
	MemberShip       string       `json:"membership,omitempty"`
	IsDirect         bool         `json:"is_direct,omitempty"`
	ThirdPartyInvite Invite       `json:"third_party_invite,omitempty"`
	Unsigned         UnsignedData `json:"unsigned,omitempty"`
}

//GET /_matrix/client/r0/initialSync
type GetInitialSyncRequest struct {
	Limit       int    `json:"limit"`
	Archived    bool   `json:"archived"`
	Timeout     int    `json:"timeout"`
	FullState   string `json:"full_state"`
	SetPresence string `json:"set_presence"`
	From        string `json:"from"`
	Since       string `json:"since"`
}

type GetInitialSyncResponse struct {
	End         string     `json:"end"`
	Presence    []Event    `json:"presence"`
	AccountData []Event    `json:"account_data"`
	Rooms       []RoomInfo `json:"rooms"`
}

type RoomInfo struct {
	RoomID     string `json:"room_id"`
	Membership string `json:"membership"`

	//Invite      InviteEvent     `json:"invite,omitempty"`
	Messages    PaginationChunk `json:"messages"`
	State       []StateEvent    `json:"state"`
	Visibility  string          `json:"visibility"`
	AccountData []Event         `json:"account_data"`
}

type InviteEvent struct {
	Content        []byte       `json:"content,omitempty"`
	Type           string       `json:"type,omitempty"`
	EventID        string       `json:"event_id,omitempty"`
	Sender         string       `json:"sender,omitempty"`
	OriginServerTS int          `json:"origin_server_ts,omitempty"`
	Unsigned       UnsignedData `json:"unsigned,omitempty"`
	RoomID         string       `json:"room_id"`
	PrevContent    EventContent `json:"prev_content,omitempty"`
	StateKey       string       `json:"state_key,omitempty"`
}

type Invite struct {
	DisplayName string `json:"displayname,omitempty"`
	Signed      Signed `json:"signed,omitempty"`
}

type Signed struct {
	MxID       string `json:"mxid,omitempty"`
	Signatures string `json:"signatures,omitempty"`
	Token      string `json:"token,omitempty"`
}

type PaginationChunk struct {
	Start string      `json:"start,omitempty"`
	End   string      `json:"end,omitempty"`
	Chunk []RoomEvent `json:"chunk,omitempty"`
}

//GET /_matrix/client/r0/events
type GetEventsRequest struct {
	From        string `json:"from"`
	TimeOut     string `json:"timeout"`
	FullState   string `json:"full_state"`
	SetPresence string `json:"set_presence"`
	Since       string `json:"since"`
	Filter      string `json:"filter"`
}

type GetEventsResponse struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Chunk []Event `json:"chunk"`
}

//GET /_matrix/client/r0/events/{eventId}
type GetEventByIDRequest struct {
	EventID string `json:"eventId,omitempty"`
}

type GetEventByIDResponse struct {
	Content interface{} `json:"content"`
	Type    string      `json:"type"`
}

//GET /_matrix/client/r0/rooms/{roomId}/event/{eventId}
type GetRoomEventByIDRequest struct {
	RoomID  string `json:"roomId,omitempty"`
	EventID string `json:"eventId,omitempty"`
}

type GetRoomEventByIDResponse struct {
	Content interface{} `json:"content"`
	Type    string      `json:"type"`
}

//GET /_matrix/client/r0/rooms/{roomId}/state/{eventType}/{stateKey}
type GetRoomStateByTypeAndStateKeyRequest struct {
	RoomID    string `json:"roomId,omitempty"`
	EventType string `json:"eventType,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
}

//GET /_matrix/client/r0/rooms/{roomId}/state/{eventType}
type GetRoomStateByTypeRequest struct {
	RoomID    string `json:"roomId,omitempty"`
	EventType string `json:"eventType,omitempty"`
}

//GET /_matrix/client/r0/rooms/{roomId}/state
type GetRoomStateRequest struct {
	RoomID string `json:"roomId,omitempty"`
}

type GetRoomStateResponse struct {
	StateEvents []StateEvent
}

// GET /_matrix/client/r0/rooms/{roomId}/members
type GetRoomMembersRequest struct {
	RoomID string `json:"roomId,omitempty"`
}

type GetRoomMembersResponse struct {
	Chunk []MemberEvent `json:"chunk,omitempty"`
}

type MemberEvent struct {
	Content        []byte       `json:"content,omitempty"`
	Type           string       `json:"type,omitempty"`
	EventID        string       `json:"event_id,omitempty"`
	Sender         string       `json:"sender,omitempty"`
	OriginServerTS int          `json:"origin_server_ts,omitempty"`
	Unsigned       UnsignedData `json:"unsigned,omitempty"`
	RoomID         string       `json:"room_id"`
	PrevContent    EventContent `json:"prev_content,omitempty"`
	StateKey       string       `json:"state_key,omitempty"`
}

//GET /_matrix/client/r0/rooms/{roomId}/joined_members
type GetRoomJoinedMembersRequest struct {
	RoomID string `json:"roomId,omitempty"`
}

type GetRoomJoinedMembersResponse struct {
	Joined map[string]RoomMember `json:"joined,omitempty"`
}

type RoomMember struct {
	AvatarURL   string  `json:"avatar_url,omitempty"`
	DisplayName *string `json:"displayname,omitempty"`
}

//GET /_matrix/client/r0/rooms/{roomId}/messages
type GetRoomMessagesRequest struct {
	RoomID string `json:"roomId,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Dir    string `json:"dir,omitempty"`
	Limit  string `json:"limit,omitempty"`
	Filter string `json:"filter,omitempty"`
}

type GetRoomMessagesResponse struct {
	Start string      `json:"start,omitempty"`
	End   string      `json:"end,omitempty"`
	Chunk []RoomEvent `json:"chunk,omitempty"`
}

type GetRoomHistoryRequest struct {
	RoomID string `json:"roomId,omitempty"`
	Filter string `json:"filter,omitempty"`
	Page   int    `json:"page,omitempty"`
	Size   int    `json:"size,omitempty"`
}

//GET /_matrix/client/r0/rooms/{roomId}/initialSync
type GetRoomInitialSyncRequest struct {
	RoomID string `json:"roomId,omitempty"`
}

// PUT /_matrix/client/r0/rooms/{roomId}/state/{eventType}/{stateKey}
type PutRoomStateByTypeAndStateKey struct {
	RoomID    string `json:"roomId,omitempty"`
	EventType string `json:"eventType,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
	Content   []byte `json:"content,omitempty"`
}

type PutRoomStateResponse struct {
	EventID string `json:"event_id,omitempty"`
}

// PUT /_matrix/client/r0/rooms/{roomId}/state/{eventType}
type PutRoomStateByType struct {
	RoomID    string `json:"roomId,omitempty"`
	EventType string `json:"eventType,omitempty"`
	Content   []byte `json:"content,omitempty"`
}

// respoonse
type PutRoomStateByTypeResponse struct {
	EventID string `json:"event_id"`
}

// PUT /_matrix/client/r0/rooms/{roomId}/send/{eventType}/{txnId}
type PutRoomStateByTypeWithTxnID struct {
	RoomID    string `json:"roomId,omitempty"`
	EventType string `json:"eventType,omitempty"`
	StateKey  string `json:"stateKey,omitempty"`
	TxnID     string `json:"txnId,omitempty"`
	Content   []byte `json:"content,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// response
type PutRoomStateByTypeWithTxnIDResponse struct {
	EventID string `json:"event_id"`
}

// PUT /_matrix/client/r0/rooms/{roomId}/redact/{eventId}/
type PutRedactEventRequest struct {
	RoomID  string `json:"roomId,omitempty"`
	EventID string `json:"eventId,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Content []byte `json:"content,omitempty"`
}

// PUT /_matrix/client/r0/rooms/{roomId}/redact/{eventId}/{txnId}
type PutRedactWithTxnIDEventRequest struct {
	RoomID  string `json:"roomId,omitempty"`
	EventID string `json:"eventId,omitempty"`
	TxnID   string `json:"txnId,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Content []byte `json:"content,omitempty"`
}

// response
type PutRedactEventResponse struct {
	EventID string `json:"event_id"`
}

type PostReportMissingEventsRequest struct {
	RoomID string  `json:"roomID`
	Depth  []int64 `json:"depth"`
}
