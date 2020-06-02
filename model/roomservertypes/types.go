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

// Package types provides the types that are used internally within the roomserver.
package roomservertypes

// EventTypeNID is a numeric ID for an event type.
type EventTypeNID int64

// EventStateKeyNID is a numeric ID for an event state_key.
type EventStateKeyNID int64

// EventNID is a numeric ID for an event.
type EventNID int64

// RoomNID is a numeric ID for a room.
type RoomNID int64

type MembershipState int64

const (
	MembershipStateLeaveOrBan MembershipState = 1
	MembershipStateInvite     MembershipState = 2
	MembershipStateJoin       MembershipState = 3
	MembershipStateBan        MembershipState = 4
	MembershipStateUnban      MembershipState = 5
)

type TransactionID struct {
	DeviceID      string `json:"device_id"`
	TransactionID string `json:"id"`
	IP            string `json:"ip"`
}

type RoomNIDs struct {
	RoomID  string
	RoomNID int64
}
