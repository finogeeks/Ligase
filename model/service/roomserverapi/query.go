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
	"context"
	"sort"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/plugins/message/external"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// QueryEventsByIDRequest is a request to QueryEventsByID
type QueryEventsByIDRequest struct {
	// The event IDs to look up.
	EventIDs []string `json:"event_ids"`
}

// QueryEventsByIDResponse is a response to QueryEventsByID
type QueryEventsByIDResponse struct {
	// Copy of the request for debugging.
	EventIDs []string `json:"event_ids"`
	// A list of events with the requested IDs.
	// If the roomserver does not have a copy of a requested event
	// then it will omit that event from the list.
	// If the roomserver thinks it has a copy of the event, but
	// fails to read it from the database then it will fail
	// the entire request.
	// This list will be in an arbitrary order.
	Events []*gomatrixserverlib.Event `json:"events"`
}

// QueryRoomEventByIDRequest is a request to QueryEventsByID
type QueryRoomEventByIDRequest struct {
	EventID string `json:"event_id"`
	RoomID  string `json:"room_id"`
}

// QueryRoomEventByIDResponse is a response to QueryEventsByID
type QueryRoomEventByIDResponse struct {
	// Copy of the request for debugging.
	EventID string                   `json:"event_id"`
	RoomID  string                   `json:"room_id"`
	Event   *gomatrixserverlib.Event `json:"event"`
}

type QueryBackFillEventsRequest gomatrixserverlib.BackfillRequest

// QueryBackFillEventsResponse is a response to QueryBackFillEventsRequest
type QueryBackFillEventsResponse gomatrixserverlib.BackfillResponse

type QueryEventAuthRequest struct {
	EventID string `json:"event_id"`
}

type QueryEventAuthResponse struct {
	AuthEvents []*gomatrixserverlib.Event
}

type QueryJoinRoomsRequest struct {
	UserID string `json:"user_id"`
}

type QueryJoinRoomsResponse struct {
	UserID string   `json:"user_id"`
	Rooms  []string `json:"rooms"`
}

type QueryRoomStateRequest struct {
	RoomID string `json:"room_id"`
}

type QueryRoomStateResponse struct {
	RoomID            string                              `json:"room_id"`
	RoomExists        bool                                `json:"room_exists"`
	Creator           *gomatrixserverlib.Event            `json:"create_ev"`
	JoinRule          *gomatrixserverlib.Event            `json:"join_rule_ev"`
	HistoryVisibility *gomatrixserverlib.Event            `json:"history_visibility_ev"`
	Visibility        *gomatrixserverlib.Event            `json:"visibility_ev"`
	Name              *gomatrixserverlib.Event            `json:"name_ev"`
	Topic             *gomatrixserverlib.Event            `json:"topic_ev"`
	Desc              *gomatrixserverlib.Event            `json:"desc_ev"`
	CanonicalAlias    *gomatrixserverlib.Event            `json:"canonical_alias_ev"`
	Power             *gomatrixserverlib.Event            `json:"power_ev"`
	Alias             *gomatrixserverlib.Event            `json:"alias_ev"`
	Join              map[string]*gomatrixserverlib.Event `json:"join_map"`
	Leave             map[string]*gomatrixserverlib.Event `json:"leave_map"`
	Invite            map[string]*gomatrixserverlib.Event `json:"invite_map"`
	ThirdInvite       map[string]*gomatrixserverlib.Event `json:"third_invite_map"`
	Avatar            *gomatrixserverlib.Event            `json:"avatar_ev"`
	GuestAccess       *gomatrixserverlib.Event            `json:"guest_access"`
}

type RoomserverRpcRequest struct {
	QueryEventsByID     *QueryEventsByIDRequest     `json:"qry_events_by_id,omitempty"`
	QueryRoomEventByID  *QueryRoomEventByIDRequest  `json:"qry_room_events_by_id,omitempty"`
	QueryJoinRooms      *QueryJoinRoomsRequest      `json:"qry_join_rooms,omitempty"`
	QueryRoomState      *QueryRoomStateRequest      `json:"qry_room_state,omitempty"`
	QueryBackFillEvents *QueryBackFillEventsRequest `json:"qry_back_fill,omitempty"`
	QueryEventAuth      *QueryEventAuthRequest      `json:"qry_event_auth,omitempty"`
	Reply               string
}

func (rs *QueryRoomStateResponse) InitFromEvents(events []gomatrixserverlib.Event) {
	if rs.Join == nil {
		rs.Join = make(map[string]*gomatrixserverlib.Event)
	}
	if rs.Leave == nil {
		rs.Leave = make(map[string]*gomatrixserverlib.Event)
	}
	if rs.Invite == nil {
		rs.Invite = make(map[string]*gomatrixserverlib.Event)
	}
	if rs.ThirdInvite == nil {
		rs.ThirdInvite = make(map[string]*gomatrixserverlib.Event)
	}

	for idx, ev := range events {
		if ev.Type() == "m.room.create" {
			rs.RoomID = ev.RoomID()
			rs.RoomExists = true
			rs.Creator = &events[idx]
		} else if ev.Type() == "m.room.join_rules" {
			rs.JoinRule = &events[idx]
		} else if ev.Type() == "m.room.history_visibility" {
			rs.HistoryVisibility = &events[idx]
		} else if ev.Type() == "m.room.visibility" {
			rs.Visibility = &events[idx]
		} else if ev.Type() == "m.room.name" {
			rs.Name = &events[idx]
		} else if ev.Type() == "m.room.topic" {
			rs.Topic = &events[idx]
		} else if ev.Type() == "m.room.desc" {
			rs.Desc = &events[idx]
		} else if ev.Type() == "m.room.canonical_alias" {
			rs.CanonicalAlias = &events[idx]
		} else if ev.Type() == "m.room.power_levels" {
			rs.Power = &events[idx]
		} else if ev.Type() == "m.room.aliases" {
			rs.Alias = &events[idx]
		} else if ev.Type() == "m.room.avatar" {
			rs.Avatar = &events[idx]
		} else if ev.Type() == "m.room.guest_access" {
			rs.GuestAccess = &events[idx]
		} else if ev.Type() == "m.room.third_party_invite" {
			rs.ThirdInvite[*ev.StateKey()] = &events[idx]
		} else if ev.Type() == "m.room.member" {
			member := external.MemberContent{}
			json.Unmarshal(ev.Content(), &member)
			if member.Membership == "join" {
				rs.Join[*ev.StateKey()] = &events[idx]
			} else if member.Membership == "invite" {
				rs.Invite[*ev.StateKey()] = &events[idx]
			} else {
				rs.Leave[*ev.StateKey()] = &events[idx]
			}
		}
	}

}

func (rs *QueryRoomStateResponse) GetAllState() []gomatrixserverlib.Event {
	var res sortedEventArray
	res = append(res, *rs.Creator)
	if rs.JoinRule != nil {
		res = append(res, *rs.JoinRule)
	}
	if rs.HistoryVisibility != nil {
		res = append(res, *rs.HistoryVisibility)
	}
	if rs.Visibility != nil {
		res = append(res, *rs.Visibility)
	}
	if rs.Name != nil {
		res = append(res, *rs.Name)
	}
	if rs.Topic != nil {
		res = append(res, *rs.Topic)
	}
	if rs.CanonicalAlias != nil {
		res = append(res, *rs.CanonicalAlias)
	}
	if rs.Alias != nil {
		res = append(res, *rs.Alias)
	}
	if rs.Power != nil {
		res = append(res, *rs.Power)
	}
	if rs.Avatar != nil {
		res = append(res, *rs.Avatar)
	}
	if rs.GuestAccess != nil {
		res = append(res, *rs.GuestAccess)
	}
	for _, value := range rs.Join {
		res = append(res, *value)
	}
	for _, value := range rs.Leave {
		res = append(res, *value)
	}
	for _, value := range rs.Invite {
		res = append(res, *value)
	}
	for _, value := range rs.ThirdInvite {
		res = append(res, *value)
	}

	sort.Sort(res)
	return res
}

type sortedEventArray []gomatrixserverlib.Event

func (list sortedEventArray) Len() int {
	return len(list)
}

func (list sortedEventArray) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list sortedEventArray) Less(i, j int) bool {
	ei := list[i].Depth()
	ej := list[j].Depth()

	return ei < ej
}

func (resp *QueryRoomStateResponse) Create() (*gomatrixserverlib.Event, error) {
	return resp.Creator, nil
}

func (resp *QueryRoomStateResponse) JoinRules() (*gomatrixserverlib.Event, error) {
	return resp.JoinRule, nil
}

func (resp *QueryRoomStateResponse) PowerLevels() (*gomatrixserverlib.Event, error) {
	return resp.Power, nil
}

func (resp *QueryRoomStateResponse) Member(stateKey string) (*gomatrixserverlib.Event, error) {
	if val, ok := resp.Join[stateKey]; ok {
		return val, nil
	}

	if val, ok := resp.Leave[stateKey]; ok {
		return val, nil
	}

	if val, ok := resp.Invite[stateKey]; ok {
		return val, nil
	}

	return nil, nil
}

func (resp *QueryRoomStateResponse) ThirdPartyInvite(stateKey string) (*gomatrixserverlib.Event, error) {
	if val, ok := resp.ThirdInvite[stateKey]; ok {
		return val, nil
	}

	return nil, nil
}

func (resp *QueryRoomStateResponse) RoomName() (*gomatrixserverlib.Event, error) {
	return resp.Name, nil
}

func (resp *QueryRoomStateResponse) RoomAvatar() (*gomatrixserverlib.Event, error) {
	return resp.Avatar, nil
}

// RoomserverQueryAPI is used to query information from the room server.
type RoomserverQueryAPI interface {
	// Query a list of events by event ID.
	QueryEventsByID( //fed&pub
		ctx context.Context,
		request *QueryEventsByIDRequest,
		response *QueryEventsByIDResponse,
	) error

	QueryRoomEventByID( //cli
		ctx context.Context,
		request *QueryRoomEventByIDRequest,
		response *QueryRoomEventByIDResponse,
	) error

	QueryJoinRooms( //cli & mig
		ctx context.Context,
		request *QueryJoinRoomsRequest,
		response *QueryJoinRoomsResponse,
	) error

	QueryRoomState( //cli & mig
		ctx context.Context,
		request *QueryRoomStateRequest,
		response *QueryRoomStateResponse,
	) error

	QueryBackFillEvents( //fed
		ctx context.Context,
		request *QueryBackFillEventsRequest,
		response *QueryBackFillEventsResponse,
	) error

	QueryEventAuth( //fed
		ctx context.Context,
		request *QueryEventAuthRequest,
		response *QueryEventAuthResponse,
	) error
}
