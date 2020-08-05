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

package types

import (
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var TypingTopicDef = "sync-typing-topic"
var StdTopicDef = "sync-std-topic"
var KcTopicDef = "sync-kc-topic"
var KeyUpdateTopicDef = "sync-key-update-topic"
var EventTopicDef = "sync-event-topic"
var SyncTopicDef = "sync-sync-topic"
var JoinedRoomTopicDef = "sync-joined-room-topic"
var ReceiptTopicDef = "sync-receipt-topic"
var EventUpdateTopicDef = "sync-event-update-topic"
var TypingUpdateTopicDef = "sync-typing-update-topic"
var ReceiptUpdateTopicDef = "sync-receipt-update-topic"
var SyncServerTopicDef = "sync-server-topic"
var LoginTopicDef = "login-info-topic"
var UnreadReqTopicDef = "sync-unread-req-topic"
var SyncUnreadTopicDef = "sync-server-unread-topic"
var EduTopicDef = "fed-edu-topic"
var ProfileUpdateTopicDef = "fed-profile-update-topic"
var FilterTokenTopicDef = "filter-token-topic"
var DeviceStateUpdateDef = "sync-device-state-update-topic"
var VerifyTokenTopicDef = "proxy-verify-token-topic"
var PresenceTopicDef = "sync-presence-topic"
var RCSEventTopicDef = "rcs-event-topic"

const (
	//proxy -> front
	PUSH_API_GROUP       = "pushapi"
	ENCRY_API_GROUP      = "encryptoapi"
	CLIENT_API_GROUP     = "clientapi"
	PUBLICROOM_API_GROUP = "publicroomapi"
	RCSSERVER_API_GROUP  = "rcsserverapi"

	//other server -> front
	PROFILE_RPC_GROUP    = "profilerpc"
	PUBLICROOM_RPC_GROUP = "publicroomrpc"
	RCSSERVER_RPC_GROUP  = "rcsserverrpc"
	ROOMINPUT_RPC_GROUP  = "roominputrpc"
	ROOOMALIAS_RPC_GROUP = "roomaliasrpc"
	ROOMQRY_PRC_GROUP    = "roomqryrpc"
)

//dist_lock prefix
const (
	LOCK_INSTANCE_PREFIX      = "dist_lock_instance:"
	LOCK_ROOMSTATE_PREFIX     = "dist_lock_roomstate:"
	LOCK_ROOMSTATE_EXT_PREFIX = "dist_lock_roomstate_ext:"
)

const (
	DEVICEKEYUPDATE  = "DeviceKeyUpdate"
	ONETIMEKEYUPDATE = "OneTimeKeyUpdate"
)

const (
	FILTERTOKENADD = "FilterTokenAdd"
	FILTERTOKENDEL = "FilterTokenDel"
)

const (
	DB_EXCEED_TIME         = 1000
	CHECK_LOAD_EXCEED_TIME = 5000
)

type DeviceState struct {
	UserID   string `json:"user_id,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	State    int    `json:"state,omitempty"`
}

type PushKeyContent struct {
	AppID   string `json:"app_id"`
	PushKey string `json:"pushkey"`
}

type NotifyDeviceState struct {
	UserID    string           `json:"user_id"`
	DeviceID  string           `json:"device_id"`
	Pushkeys  []PushKeyContent `json:"pushkeys"`
	LastState int              `json:"last_state"`
	CurState  int              `json:"cur_state"`
}

type NotifyUserState struct {
	UserID    string `json:"user_id"`
	LastState int    `json:"last_state"`
	CurState  int    `json:"cur_state"`
}

type ProfileContent struct {
	UserID       string `json:"user_id,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	AvatarUrl    string `json:"avatar_url,omitempty"`
	Presence     string `json:"presence,omitempty"`
	StatusMsg    string `json:"status_msg,omitempty"`
	ExtStatusMsg string `json:"ext_status_msg,omitempty"`

	// ext user_info
	UserName  string `json:"user_name,omitempty"`
	JobNumber string `json:"job_number,omitempty"`
	Mobile    string `json:"mobile,omitempty"`
	Landline  string `json:"landline,omitempty"`
	Email     string `json:"email,omitempty"`
	State     int    `json:"state,omitempty"`
}

type ReceiptContent struct {
	UserID      string `json:"user_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	RoomID      string `json:"room_id,omitempty"`
	ReceiptType string `json:"receipt_type,omitempty"`
	EventID     string `json:"event_id,omitempty"`
	Source 		string `json:"source,omitemty"`
}

type TypingContent struct {
	Type   string `json:"type,omitempty"`
	RoomID string `json:"room_id,omitempty"`
	UserID string `json:"user_id,omitempty"`
}

type SysManageContent struct {
	Type  string `json:"type,omitempty"`
	Reply string
}

type StdContent struct {
	StdRequest StdRequest `json:"std_request,omitempty"`
	Sender     string     `json:"sender,omitempty"`
	EventType  string     `json:"event_type,omitempty"`
	Reply      string
}

type StdRequest struct {
	Sender map[string]map[string]interface{} `json:"messages"`
}

type KeyChangeContent struct {
	FromPos int64  `json:"from_pos,omitempty"`
	ToPos   int64  `json:"to_pos,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Reply   string
}

type KeyUpdateContent struct {
	Type                     string             `json:"type"`
	OneTimeKeyChangeUserId   string             `json:"one_time_key_change_user"`
	OneTimeKeyChangeDeviceId string             `json:"one_time_key_change_device"`
	DeviceKeyChanges         []DeviceKeyChanges `json:"device_key_changes"`
	EventNID                 int64              `json:"event_id"`
	Reply                    string
}

type DeviceKeyChanges struct {
	Offset        int64  `json:"off_set"`
	ChangedUserID string `json:"device_key_change_user"`
}

type EventContent struct {
	EventID string `json:"event_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Reply   string
}

type UnreadReqContent struct {
	UserID string `json:"user_id,omitempty"`
	Reply  string
}

type SyncContent struct {
	Request HttpReq          `json:"request,omitempty"`
	Device  authtypes.Device `json:"device,omitempty"`
	Reply   string           `json:"omitempty"`
}

type RoomMsgContent struct {
	Request HttpReq `json:"request,omitempty"`
	UserID  string  `json:"user_id,omitempty"`
	RoomID  string  `json:"room_id,omitempty"`
	Reply   string
}

type HttpReq struct {
	TimeOut     string `json:"time_out,omitempty"`
	FullState   string `json:"full_state,omitempty"`
	SetPresence string `json:"set_presence,omitempty"`
	Filter      string `json:"filter,omitempty"`
	From        string `json:"from,omitempty"`
	Since       string `json:"since,omitempty"`
	Limit       string `json:"limit,omitempty"`
	Dir         string `json:"dir,omitempty"`
	TraceId     string `json:"trace_id,omitempty"`
}

type JoinedRoomContent struct {
	UserID string `json:"user_id,omitempty"`
	Reply  string
}

type RoomMemberContent struct {
	UserID string `json:"user_id,omitempty"`
	RoomID string `json:"room_id,omitempty"`
	Reply  string
}

type RoomStateByTypeContent struct {
	RoomID   string `json:"room_id,omitempty"`
	EvType   string `json:"ev_type,omitempty"`
	StateKey string `json:"state_key,omitempty"`
	Reply    string
}

type RoomStateContent struct {
	UserID string `json:"user_id,omitempty"`
	RoomID string `json:"room_id,omitempty"`
	Reply  string
}

type RoomInitSyncContent struct {
	UserID string `json:"user_id,omitempty"`
	RoomID string `json:"room_id,omitempty"`
	Reply  string
}

func (p *RoomInitSyncContent) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *RoomInitSyncContent) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type VisibilityRangeContent struct {
	RoomID string `json:"room_id,omitempty"`
	UserID string `json:"user_id,omitempty"`
	Reply  string
}

type UserReceipt struct {
	UserID    string
	RoomID    string
	EvtOffset int64
	Content   []byte
	Written   bool
}

type ReceiptStream struct {
	RoomID        string `json:"room_id"`
	Content       []byte `json:"content"`
	ReceiptOffset int64  `json:"offset"`
}

type PresenceStream struct {
	UserID  string
	Content []byte
}

type KeyChangeStream struct {
	ChangedUserID string
}

/* presence types */
type PresenceJSON struct {
	AvatarURL       string `json:"avatar_url"`
	DisplayName     string `json:"displayname"`
	LastActiveAgo   int64  `json:"last_active_ago"`
	Presence        string `json:"presence"` //required
	CurrentlyActive bool   `json:"currently_active"`
	UserID          string `json:"user_id"` //required
	StatusMsg       string `json:"status_msg"`
	ExtStatusMsg    string `json:"ext_status_msg"`

	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state"`

	LastPresence	    string `json:"last_presence"`
	LastStatusMsg       string `json:"last_status_msg"`
	LastExtStatusMsg    string `json:"last_ext_status_msg"`
}

type PresenceShowJSON struct {
	AvatarURL       string `json:"avatar_url"`
	DisplayName     string `json:"displayname"`
	LastActiveAgo   int64  `json:"last_active_ago"`
	Presence        string `json:"presence"` //required
	CurrentlyActive bool   `json:"currently_active"`
	UserID          string `json:"user_id"` //required
	StatusMsg       string `json:"status_msg"`
	ExtStatusMsg    string `json:"ext_status_msg"`
}

type ActDataStreamUpdate struct {
	UserID     string `json:"user_id"`
	RoomID     string `json:"room_id"`
	DataType   string `json:"data_type"`
	StreamType string `json:"stream_type"`
}

type ProfileStreamUpdate struct {
	UserID         string       `json:"user_id"`
	DeviceID       string       `json:"device_id"`
	Presence       PresenceJSON `json:"presence"`
	IsUpdateBase   bool         `json:"is_update_base"` //matrix /presence/{userID}/status only update presence, status_msg, ext_status_msg
}

type StdEvent struct {
	Sender  string      `json:"sender"`
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

type CompressContent struct {
	Content    []byte `json:"content"`
	Compressed bool   `json:"compressed"`
}

type RedactUnsigned struct {
	Age             int64                          `json:"age,omitempty"`
	PrevContent     []byte                         `json:"prev_content,omitempty"`
	TransactionID   string                         `json:"transaction_id,omitempty"`
	RedactedBecause *gomatrixserverlib.ClientEvent `json:"redacted_because,omitempty"`
	UpdatedBecause  *gomatrixserverlib.ClientEvent `json:"updated_because,omitempty"`
}

type Unsigned struct {
	TransactionID string          `json:"transaction_id,omitempty"`
	Relations     *EventRelations `json:"m.relations,omitempty"`
}

type EventRelations struct {
	RelayTo *OriginInRelayTo `json:"m.in_reply_to,omitempty"`
	Anno    *Annotations     `json:"m.annotation,omitempty"`
}

type MInRelayTo struct {
	MRelayTo InRelayTo `json:"m.in_reply_to"`
}

type InRelayTo struct {
	Sender         string      `json:"sender"`
	EventID        string      `json:"event_id"`
	Content        interface{} `json:"content"`
	OriginServerTs int64       `json:"origin_server_ts"`
}

type OriginInRelayTo struct {
	Chunk []string `json:"chunk"`
}

type Annotations struct {
	Chunk []*Annotation `json:"chunk"`
}

type Annotation struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Count int    `json:"count"`
}

//emoji message relay
type ReactionContent struct {
	EventID string `json:"event_id"`
	Key     string `json:"key"`
	RelType string `json:"rel_type"`
}

type LoginInfoContent struct {
	UserID      string `json:"user_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	Token       string `json:"token,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Identifier  string `json:"identifier,omitempty"`
}

type FilterTokenContent struct {
	UserID     string `json:"user_id,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	FilterType string `json:"filter_type,omitempty"`
}

type VerifyTokenRequest struct {
	Token      string `json:"device"`
	RequestURI string `json:"request_uri"`
}

type VerifyTokenResponse struct {
	Device authtypes.Device `json:"device"`
	Error  string           `json:"error"`
}

type OnlinePresence struct {
	Reply        string `json:"reply,omitempty"`
	UserID       string `json:"user_id"`
	Found        bool   `json:"found"`
	Presence     string `json:"presence,omitempty"`
	StatusMsg    string `json:"status_msg,omitempty"`
	ExtStatusMsg string `json:"ext_status_msg,omitempty"`
}

type RoomStateExt struct {
	PreStateId  string `json:"pre_state_id"`
	LastStateId string `json:"last_state_id"`
	PreMsgId    string `json:"pre_msg_id"`
	LastMsgId   string `json:"last_msg_id"`
	Depth       int64  `json:"depth"`
	HasUpdate   bool   `json:"has_update"`
	//must consistent with depth, db not has
	OutEventOffset int64 `json:"out_event_offset"`
	//domain:offset
	Domains map[string]int64 `json:"domains"`
}
