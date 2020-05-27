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

import "github.com/finogeeks/ligase/skunkworks/gomatrix"

//POST /_matrix/client/r0/user/{userId}/filter
type PostUserFilterRequest struct {
	UserID      string              `json:"user_id"`
	AccountData gomatrix.FilterPart `json:"account_data,omitempty"`
	EventFields []string            `json:"event_fields,omitempty"`
	EventFormat string              `json:"event_format,omitempty"`
	Presence    gomatrix.FilterPart `json:"presence,omitempty"`
	Room        struct {
		AccountData  gomatrix.FilterPart `json:"account_data,omitempty"`
		Ephemeral    gomatrix.FilterPart `json:"ephemeral,omitempty"`
		IncludeLeave bool                `json:"include_leave,omitempty"`
		NotRooms     []string            `json:"not_rooms,omitempty"`
		Rooms        []string            `json:"rooms,omitempty"`
		State        gomatrix.FilterPart `json:"state,omitempty"`
		Timeline     gomatrix.FilterPart `json:"timeline,omitempty"`
	} `json:"room,omitempty"`
}

type EventFilter struct {
	Limit      int      `json:"limit,omitempty"`
	NotSenders []string `json:"not_senders,omitempty"`
	NotTypes   []string `json:"not_types,omitempty"`
	Senders    []string `json:"senders,omitempty"`
	Types      []string `json:"types,omitempty"`
}

type RoomEventFilter struct {
	EventFilter
	NotRooms    []string `json:"not_rooms,omitempty"`
	Rooms       []string `json:"rooms,omitempty"`
	ContainsURL bool     `json:"contains_url,omitempty"`
}

type PostUserFilterResponse struct {
	FilterID string `json:"filter_id"`
}

//GET /_matrix/client/r0/user/{userId}/filter/{filterId}
type GetUserFilterRequest struct {
	UserID   string `json:"user_id"`
	FilterID string `json:"filter_id"`
}

type GetUserFilterResponse struct {
	AccountData gomatrix.FilterPart `json:"account_data,omitempty"`
	EventFields []string            `json:"event_fields,omitempty"`
	EventFormat string              `json:"event_format,omitempty"`
	Presence    gomatrix.FilterPart `json:"presence,omitempty"`
	Room        struct {
		AccountData  gomatrix.FilterPart `json:"account_data,omitempty"`
		Ephemeral    gomatrix.FilterPart `json:"ephemeral,omitempty"`
		IncludeLeave bool                `json:"include_leave,omitempty"`
		NotRooms     []string            `json:"not_rooms,omitempty"`
		Rooms        []string            `json:"rooms,omitempty"`
		State        gomatrix.FilterPart `json:"state,omitempty"`
		Timeline     gomatrix.FilterPart `json:"timeline,omitempty"`
	} `json:"room,omitempty"`
}
