// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package syncapitypes

import (
	jsonRaw "encoding/json"
	"strconv"
	"sync"

	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// StreamPosition represents the offset in the sync stream a client is at.
type StreamPosition int64

// String implements the Stringer interface.
func (sp StreamPosition) String() string {
	return strconv.FormatInt(int64(sp), 10)
}

// PrevEventRef represents a reference to a previous event in a state event upgrade
type PrevEventRef struct {
	PrevContent   jsonRaw.RawMessage `json:"prev_content"`
	ReplacesState string             `json:"replaces_state"`
	PrevSender    string             `json:"prev_sender"`
	PreOffset     int64              `json:"prev_offset"`
}

// Response represents a /sync API response. See https://matrix.org/docs/spec/client_server/r0.2.0.html#get-matrix-client-r0-sync
type Response struct {
	NextBatch   string `json:"next_batch"`
	AccountData struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"account_data"`
	Presence struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"presence"`
	Rooms struct {
		Join   map[string]JoinResponse   `json:"join"`
		Invite map[string]InviteResponse `json:"invite"`
		Leave  map[string]LeaveResponse  `json:"leave"`
	} `json:"rooms"`
	/* extensions */
	// ToDevice send to device extension
	ToDevice ToDevice `json:"to_device"`
	// DeviceList encryptoapi key management /keyChange extension
	DeviceList DeviceLists `json:"device_lists"`
	// compatibility with no definition todo: del it
	SignNum map[string]int `json:"device_one_time_keys_count"`
	Lock    *sync.Mutex    `json:"-"`
}

func (p *Response) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Response) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type SyncServerResponseRooms struct {
	Join   map[string]JoinResponse   `json:"join,omitempty"`
	Invite map[string]InviteResponse `json:"invite,omitempty"`
	Leave  map[string]LeaveResponse  `json:"leave,omitempty"`
}

type SyncServerResponse struct {
	Rooms            SyncServerResponseRooms `json:"rooms,omitempty"`
	MaxReceiptOffset int64                   `json:"max_receipt_offset,omitempty"`
	AllLoaded        bool                    `json:"all_loaded,omitempty"`
	NewUsers         []string                `json:"new_users,omitempty"`
	Ready            bool                    `json:"ready,omitempty"`
	MaxRoomOffset    map[string]int64        `json:"max_room_offset,omitempty"`
}

type ToDevice struct {
	StdEvent []types.StdEvent `json:"events"`
}

type DeviceLists struct {
	Changed []string `json:"changed"`
}

type PaginationChunk struct {
	Start string                          `json:"start"`
	Chunk []gomatrixserverlib.ClientEvent `json:"chunk"`
	End   string                          `json:"end"`
}

func (p *PaginationChunk) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PaginationChunk) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type RoomInfo struct {
	RoomID     string `json:"room_id"`
	Membership string `json:"membership"`
	//Invite      gomatrixserverlib.ClientEvent   `json:"invite"`
	Messages    PaginationChunk                 `json:"messages"`
	State       []gomatrixserverlib.ClientEvent `json:"state"`
	Visibility  string                          `json:"visibility", omitempty`
	AccountData []gomatrixserverlib.ClientEvent `json:"account_data"`
	Presence    struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"presence"`
}

func (r *RoomInfo) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *RoomInfo) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type StdHolder struct {
	StreamID int64  `json:"stream_id"`
	Sender   string `json:"sender"`
	EventTyp string `json:"event_type"`
	Event    []byte `json:"event"`
}

// NewResponse creates an empty response with initialised maps.
func NewResponse(pos StreamPosition) *Response {
	res := Response{}
	// Make sure we send the next_batch as a string. We don't want to confuse clients by sending this
	// as an integer even though (at the moment) it is.
	res.NextBatch = pos.String()
	// Pre-initialise the maps. Synapse will return {} even if there are no rooms under a specific section,
	// so let's do the same thing. Bonus: this means we can't get dreaded 'assignment to entry in nil map' errors.
	res.Rooms.Join = make(map[string]JoinResponse)
	res.Rooms.Invite = make(map[string]InviteResponse)
	res.Rooms.Leave = make(map[string]LeaveResponse)

	// Also pre-intialise empty slices or else we'll insert 'null' instead of '[]' for the value.
	// TODO: We really shouldn't have to do all this to coerce encoding/json to Do The Right Thing. We should
	//       really be using our own Marshal/Unmarshal implementations otherwise this may prove to be a CPU bottleneck.
	//       This also applies to NewJoinResponse, NewInviteResponse and NewLeaveResponse.
	res.AccountData.Events = make([]gomatrixserverlib.ClientEvent, 0)
	res.Presence.Events = make([]gomatrixserverlib.ClientEvent, 0)
	res.ToDevice.StdEvent = make([]types.StdEvent, 0)
	res.DeviceList.Changed = make([]string, 0)
	res.Lock = new(sync.Mutex)
	return &res
}

// JoinResponse represents a /sync response for a room which is under the 'join' key.
type JoinResponse struct {
	State struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"state"`
	Timeline struct {
		Events    []gomatrixserverlib.ClientEvent `json:"events"`
		Limited   bool                            `json:"limited"`
		PrevBatch string                          `json:"prev_batch"`
	} `json:"timeline"`
	Ephemeral struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"ephemeral"`
	AccountData struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"account_data"`
	Unread *UnreadNotifications `json:"unread_notifications,omitempty"`
}

type UnreadNotifications struct {
	HighLightCount    int64 `json:"highlight_count"`
	NotificationCount int64 `json:"notification_count"`
}

// NewJoinResponse creates an empty response with initialised arrays.
func NewJoinResponse() *JoinResponse {
	res := JoinResponse{}
	res.State.Events = make([]gomatrixserverlib.ClientEvent, 0)
	res.Timeline.Events = make([]gomatrixserverlib.ClientEvent, 0)
	res.Ephemeral.Events = make([]gomatrixserverlib.ClientEvent, 0)
	res.AccountData.Events = make([]gomatrixserverlib.ClientEvent, 0)
	return &res
}

// InviteResponse represents a /sync response for a room which is under the 'invite' key.
type InviteResponse struct {
	InviteState struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"invite_state"`
}

// NewInviteResponse creates an empty response with initialised arrays.
func NewInviteResponse() *InviteResponse {
	res := InviteResponse{}
	res.InviteState.Events = make([]gomatrixserverlib.ClientEvent, 0)
	return &res
}

// LeaveResponse represents a /sync response for a room which is under the 'leave' key.
type LeaveResponse struct {
	State struct {
		Events []gomatrixserverlib.ClientEvent `json:"events"`
	} `json:"state"`
	Timeline struct {
		Events    []gomatrixserverlib.ClientEvent `json:"events"`
		Limited   bool                            `json:"limited"`
		PrevBatch string                          `json:"prev_batch"`
	} `json:"timeline"`
}

// NewLeaveResponse creates an empty response with initialised arrays.
func NewLeaveResponse() *LeaveResponse {
	res := LeaveResponse{}
	res.State.Events = make([]gomatrixserverlib.ClientEvent, 0)
	res.Timeline.Events = make([]gomatrixserverlib.ClientEvent, 0)
	return &res
}

type KeyChanged struct {
	Changes []string `json:"changed"`
}

func (r *KeyChanged) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *KeyChanged) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type JoinedRoomsResp struct {
	JoinedRooms []string `json:"joined_rooms,omitempty"`
}

func (r *JoinedRoomsResp) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *JoinedRoomsResp) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type StateEventInState struct {
	gomatrixserverlib.ClientEvent
	PrevContent   jsonRaw.RawMessage `json:"prev_content,omitempty"`
	ReplacesState string             `json:"replaces_state,omitempty"`
}

type StateEventInStateResp []StateEventInState

func (r StateEventInStateResp) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r StateEventInStateResp) Decode(input []byte) error {
	return json.Unmarshal(input, &r)
}

type MemberResponse struct {
	Chunk []gomatrixserverlib.ClientEvent `json:"chunk"`
}

func (r *MemberResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *MemberResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type MessageEventResp struct {
	Start string                          `json:"start,omitempty"`
	Chunk []gomatrixserverlib.ClientEvent `json:"chunk"`
	End   string                          `json:"end,omitempty"`
}

func (r *MessageEventResp) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *MessageEventResp) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ContextEventResp struct {
	Start     string                           `json:"start"`
	End       string                           `json:"end"`
	EvsBefore []gomatrixserverlib.ClientEvent  `json:"events_before"`
	Event     gomatrixserverlib.ClientEvent    `json:"event"`
	EvsAfter  []gomatrixserverlib.ClientEvent  `json:"events_after"`
	State     []*gomatrixserverlib.ClientEvent `json:"state"`
}

func (r *ContextEventResp) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ContextEventResp) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReceiptUpdate struct {
	Users  []string `json:"users"`
	Offset int64    `json:"offset"`
	RoomID string   `json:"room_id"`
}

type TypingUpdate struct {
	Type      string   `json:"type,omitempty"`
	RoomID    string   `json:"room_id,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
	DeviceID  string   `json:"device_id,omitempty"`
	RoomUsers []string `json:"users"`
}

type SyncServerRequest struct {
	RequestType      string     `json:"request_type,omitempty"`
	InviteRooms      []SyncRoom `json:"invite_rooms,omitempty"`
	JoinRooms        []SyncRoom `json:"join_rooms,omitempty"`
	LeaveRooms       []SyncRoom `json:"leave_rooms,omitempty"`
	JoinedRooms      []string   `json:"joined_rooms,omitempty"`
	UserID           string     `json:"user_id,omitempty"`
	DeviceID         string     `json:"device_id,omitempty"`
	ReceiptOffset    int64      `json:"receipt_offset,omitempty"`
	MaxReceiptOffset int64      `json:"max_receipt_offset,omitempty"`
	IsHuman          bool       `json:"is_human,omitempty"`
	Limit            int        `json:"limit,omitempty"`
	LoadReady        bool       `json:"-"`
	SyncReady        bool       `json:"-"`
	SyncInstance     uint32     `json:"sync_instance"`
	IsFullSync       bool       `json:"is_full_sync"`
	Reply            string     `json:"-"`
	TraceID          string     `json:"trace_id"`
	Slot             uint32     `json:"slot"`
	RSlot            uint32     `json:"-"`
}

type SyncRoom struct {
	RoomID    string `json:"room_id,omitempty"`
	RoomState string `json:"room_state,omitempty"`
	Start     int64  `json:"start,omitempty"`
	End       int64  `json:"end,omitempty"`
}

type SyncUnreadRequest struct {
	JoinRooms    []string `json:"join_rooms,omitempty"`
	UserID       string   `json:"user_id,omitempty"`
	SyncInstance uint32   `json:"sync_instance"`
	Reply        string
}

type SyncUnreadResponse struct {
	Count int64 `json:"count,omitempty"`
}

type UserTimeLineStream struct {
	Offset     int64  `json:"offset,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	RoomID     string `json:"room_id,omitempty"`
	RoomOffset int64  `json:"room_offset,omitempty"`
	RoomState  string `json:"room_state,omitempty"`
}

type ClientEvents []gomatrixserverlib.ClientEvent

func (es ClientEvents) Len() int { return len(es) }

func (es ClientEvents) Less(i, j int) bool {
	return es[i].Depth < es[j].Depth
}

func (es ClientEvents) Swap(i, j int) { es[i], es[j] = es[j], es[i] }
