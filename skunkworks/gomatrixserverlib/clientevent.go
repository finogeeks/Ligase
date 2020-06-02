/* Copyright 2017 Vector Creations Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gomatrixserverlib

import (
	"fmt"
)

// EventFormat specifies the format of a client event
type EventFormat int

const (
	// FormatAll will include all client event keys
	FormatAll EventFormat = iota
	// FormatSync will include only the event keys required by the /sync API. Notably, this
	// means the 'room_id' will be missing from the events.
	FormatSync
)

// ClientEvent is an event which is fit for consumption by clients, in accordance with the specification.
type ClientEvent struct {
	Content        rawJSON   `json:"content,omitempty"`
	EventID        string    `json:"event_id,omitempty"`
	EventNID       int64     `json:"event_nid,omitempty"`
	DomainOffset   int64     `json:"domain_offset"`
	Depth          int64     `json:"depth"`
	OriginServerTS Timestamp `json:"origin_server_ts,omitempty"`
	// RoomID is omitted on /sync responses
	RoomID      string  `json:"room_id,omitempty"`
	Sender      string  `json:"sender,omitempty"`
	StateKey    *string `json:"state_key,omitempty"`
	Type        string  `json:"type,omitempty"`
	Redacts     string  `json:"redacts,omitempty"`
	Hint        string  `json:"hint,omitempty"`
	Visible     bool    `json:"visible,omitempty"`
	Unsigned    rawJSON `json:"unsigned,omitempty"`
	EventOffset int64   `json:"event_offset,omitempty"`
}

// ToClientEvents converts server events to client events.
func ToClientEvents(serverEvs []Event, format EventFormat) []ClientEvent {
	evs := make([]ClientEvent, len(serverEvs))
	for i, se := range serverEvs {
		evs[i] = ToClientEvent(se, format)
	}
	return evs
}

// ToClientEvent converts a single server event to a client event.
func ToClientEvent(se Event, format EventFormat) ClientEvent {
	ce := ClientEvent{
		Content:        rawJSON(se.Content()),
		Sender:         se.Sender(),
		Type:           se.Type(),
		StateKey:       se.StateKey(),
		Unsigned:       rawJSON(se.Unsigned()),
		OriginServerTS: se.OriginServerTS(),
		EventID:        se.EventID(),
		EventNID:       se.EventNID(),
		Depth:          se.Depth(),
	}
	if format == FormatAll {
		ce.RoomID = se.RoomID()
	}

	if se.Redacts() != "" {
		ce.Redacts = se.Redacts()
	}
	return ce
}

func (cliEv *ClientEvent) InitFromEvent(e *Event) {
	cliEv.Content = e.fields.Content
	cliEv.EventID = e.fields.EventID
	cliEv.EventNID = e.fields.EventNID
	cliEv.OriginServerTS = e.fields.OriginServerTS
	cliEv.RoomID = e.fields.RoomID
	cliEv.Sender = e.fields.Sender
	cliEv.StateKey = e.fields.StateKey
	cliEv.Type = e.fields.Type
	cliEv.Redacts = e.fields.Redacts
	cliEv.Unsigned = e.fields.Unsigned
	cliEv.DomainOffset = e.fields.DomainOffset
	cliEv.Depth = e.fields.Depth
}

func (cliEv *ClientEvent) SetExtra(visible bool, hint string) {
	cliEv.Visible = visible
	cliEv.Hint = hint
}

// ToContent converts a single server event to a client event.
func ToContent(se Event, format EventFormat) rawJSON {
	return se.Content()
}

func (cliEv *ClientEvent) Membership() (string, error) {
	if cliEv.Type != MRoomMember {
		return "", fmt.Errorf("gomatrixserverlib: not an m.room.member event")
	}
	var content memberContent
	if err := json.Unmarshal(cliEv.Content, &content); err != nil {
		return "", err
	}
	return content.Membership, nil
}
