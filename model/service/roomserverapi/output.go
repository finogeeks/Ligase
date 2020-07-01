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

package roomserverapi

import (
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/model/roomservertypes"
)

// An OutputType is a type of roomserver output.
type OutputType string

const (
	// OutputTypeNewRoomEvent indicates that the event is an OutputNewRoomEvent
	OutputTypeNewRoomEvent OutputType = "new_room_event"
	// OutputTypeNewInviteEvent indicates that the event is an OutputNewInviteEvent
	OutputTypeNewInviteEvent OutputType = "new_invite_event"
	// OutputTypeRetireInviteEvent indicates that the event is an OutputRetireInviteEvent
	OutputTypeRetireInviteEvent OutputType = "retire_invite_event"
	OutputBackfillRoomEvent     OutputType = "back_fill_event"
)

// An OutputEvent is an entry in the roomserver output kafka log.
// Consumers should check the type field when consuming this event.
type OutputEvent struct {
	// What sort of event this is.
	Type OutputType `json:"type"`
	// The content of event with type OutputTypeNewRoomEvent
	NewRoomEvent *OutputNewRoomEvent `json:"new_room_event,omitempty"`
	// The content of event with type OutputTypeNewInviteEvent
	NewInviteEvent *OutputNewInviteEvent `json:"new_invite_event,omitempty"`
	// The content of event with type OutputTypeRetireInviteEvent
	RetireInviteEvent *OutputRetireInviteEvent `json:"retire_invite_event,omitempty"`
}

// An OutputNewRoomEvent is written when the roomserver receives a new event.
// It contains the full matrix room event and enough information for a
// consumer to construct the current state of the room and the state before the
// event.
//
// When we talk about state in a matrix room we are talking about the state
// after a list of events. The current state is the state after the latest
// event IDs in the room. The state before an event is the state after its
// prev_events.
type OutputNewRoomEvent struct {
	// The Event.
	Event gomatrixserverlib.ClientEvent `json:"event"`
	// The state event IDs that were added to the state of the room by this event.
	// Together with RemovesStateEventIDs this allows the receiver to keep an up to date
	// view of the current state of the room.
	AddsStateEventIDs []string `json:"adds_state_event_ids"`
	// The state event IDs that were removed from the state of the room by this event.
	RemovesStateEventIDs []string `json:"removes_state_event_ids"`
	// The server name to use to push this event to other servers.
	// Or empty if this event shouldn't be pushed to other servers.
	//
	// This is used by the federation sender component. We need to tell it what
	// event it needs to send because it can't tell on its own. Normally if an
	// event was created on this server then we are responsible for sending it.
	// However there are a couple of exceptions. The first is that when the
	// server joins a remote room through another matrix server, it is the job
	// of the other matrix server to send the event over federation. The second
	// is the reverse of the first, that is when a remote server joins a room
	// that we are in over federation using our server it is our responsibility
	// to send the join event to other matrix servers.
	//
	// We encode the server name that the event should be sent using here to
	// future proof the API for virtual hosting.
	SendAsServer string `json:"send_as_server"`
	// The transaction ID of the send request if sent by a local user and one
	// was specified
	TransactionID *roomservertypes.TransactionID `json:"transaction_id"`
	Joined        []string                       `json:"joined"`
}

// An OutputNewInviteEvent is written whenever an invite becomes active.
// Invite events can be received outside of an existing room so have to be
// tracked separately from the room events themselves.
type OutputNewInviteEvent struct {
	// The "m.room.member" invite event.
	Event gomatrixserverlib.ClientEvent `json:"event"`
}

// An OutputRetireInviteEvent is written whenever an existing invite is no longer
// active. An invite stops being active if the user joins the room or if the
// invite is rejected by the user.
type OutputRetireInviteEvent struct {
	// The ID of the "m.room.member" invite event.
	EventID string
	// The target user ID of the "m.room.member" invite event that was retired.
	TargetUserID string
	// Optional event ID of the event that replaced the invite.
	// This can be empty if the invite was rejected locally and we were unable
	// to reach the server that originally sent the invite.
	RetiredByEventID string
	// The "membership" of the user after retiring the invite. One of "join"
	// "leave" or "ban".
	Membership string
}
