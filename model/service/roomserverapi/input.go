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

// Package api provides the types that are used to communicate with the roomserver.
package roomserverapi

import (
	"context"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/model/roomservertypes"
)

const (
	// KindOutlier event fall outside the contiguous event graph.
	// We do not have the state for these events.
	// These events are state events used to authenticate other events.
	// They can become part of the contiguous event graph via backfill.
	KindOutlier = 1
	// KindNew event extend the contiguous graph going forwards.
	// They usually don't need state, but may include state if the
	// there was a new event that references an event that we don't
	// have a copy of.
	KindNew = 2
	// KindBackfill event extend the contiguous graph going backwards.
	// They always have state.
	KindBackfill = 3
)

// DoNotSendToOtherServers tells us not to send the event to other matrix
// servers.
const DoNotSendToOtherServers = ""

// InputRoomEvent is a matrix room event to add to the room server database.
// TODO: Implement UnmarshalJSON/MarshalJSON in a way that does something sensible with the event JSON.
type InputRoomEvent struct {
	// Whether this event is new, backfilled or an outlier.
	// This controls how the event is processed.
	Kind int `json:"kind"`
	// The event JSON for the event to add.
	Event gomatrixserverlib.Event `json:"event"`
	// The server name to use to push this event to other servers.
	// Or empty if this event shouldn't be pushed to other servers.
	SendAsServer string `json:"send_as_server"`
	// The transaction ID of the send request if sent by a local user and one
	// was specified
	TransactionID *roomservertypes.TransactionID `json:"transaction_id"`
}

// TransactionID contains the transaction ID sent by a client when sending an
// event, along with the ID of that device.

type RawEvent struct {
	RoomID     string
	Kind       int
	Trust      bool
	Reply      string
	TxnID      *roomservertypes.TransactionID
	BulkEvents BulkEvent
	Query      []string
}

type BulkEvent struct {
	Events  []gomatrixserverlib.Event
	SvrName string
}

type FederationEvent struct {
	ReqPath     string
	Destination string
	Reply       string
	RawEvent    RawEvent
	OutputEvent OutputEvent
	RoomState   []byte
	Extra       []byte
}

// InputInviteEvent is a matrix invite event received over federation without
// the usual context a matrix room event would have. We usually do not have
// access to the events needed to check the event auth rules for the invite.
type InputInviteEvent struct {
	Event gomatrixserverlib.Event `json:"event"`
}

// InputRoomEventsRequest is a request to InputRoomEvents
type InputRoomEventsRequest struct {
	InputRoomEvents   []InputRoomEvent   `json:"input_room_events"`
	InputInviteEvents []InputInviteEvent `json:"input_invite_events"`
}

// InputRoomEventsResponse is a response to InputRoomEvents
type InputRoomEventsResponse struct {
	ErrCode int    `json:"input_room_err_code,omitempty"`
	ErrMsg  string `json:"input_room_err_msg,omitempty"`
	N       int    `json:"n"`
}

// RoomserverInputAPI is used to write events to the room server.
type RoomserverInputAPI interface {
	InputRoomEvents(
		ctx context.Context,
		input *RawEvent,
	) (int, error)
}
