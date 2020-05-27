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

package common

import (
	"errors"
	"time"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

// ErrRoomNoExists is returned when trying to lookup the state of a room that
// doesn't exist
var ErrRoomNoExists = errors.New("Room does not exist")

// BuildEvent builds a Matrix event using the event builder and roomserver query
// API client provided. If also fills roomserver query API response (if provided)
// in case the function calling FillBuilder needs to use it.
// Returns ErrRoomNoExists if the state of the room could not be retrieved because
// the room doesn't exist
// Returns an error if something else went wrong
func BuildEvent(
	builder *gomatrixserverlib.EventBuilder, domain string, cfg config.Dendrite,
	idg *uid.UidGenerator,
) (*gomatrixserverlib.Event, error) {
	now := time.Now()
	nid, _ := idg.Next()
	event, err := builder.Build(nid, now, gomatrixserverlib.ServerName(domain))
	if err != nil {
		return nil, err
	}

	return &event, nil
}

func IsCreatingDirectRoomEv(ev *gomatrixserverlib.Event) (bool, error) {
	if ev.Type() != gomatrixserverlib.MRoomCreate {
		return false, nil
	}
	cont := CreateContent{}
	err := json.Unmarshal(ev.Content(), &cont)
	if err != nil {
		log.Errorf("Failed to unmarshal CreateContent: %v\n", err)
		return false, err
	}
	if cont.IsDirect != nil && *(cont.IsDirect) == true {
		return true, nil
	}
	return false, nil
}

func IsStateEv(ev *gomatrixserverlib.Event) bool {
	switch ev.Type() {
	case "m.room.create", "m.room.member", "m.room.power_levels":
		return true
	case "m.room.join_rules", "m.room.history_visibility", "m.room.visibility":
		return true
	case "m.room.name", "m.room.topic", "m.room.pinned_events", "m.room.desc":
		return true
	case "m.room.aliases", "m.room.canonical_alias", "m.room.avatar":
		return true
	case "m.room.encryption":
		return true
	case "m.room.third_party_invite", "m.room.guest_access":
		return true
	default:
		return false
	}
}

func IsStateClientEv(ev *gomatrixserverlib.ClientEvent) bool {
	switch ev.Type {
	case "m.room.create", "m.room.member", "m.room.power_levels":
		return true
	case "m.room.join_rules", "m.room.history_visibility", "m.room.visibility":
		return true
	case "m.room.name", "m.room.topic", "m.room.pinned_events", "m.room.desc":
		return true
	case "m.room.aliases", "m.room.canonical_alias", "m.room.avatar":
		return true
	case "m.room.encryption":
		return true
	case "m.room.third_party_invite", "m.room.guest_access":
		return true
	default:
		return false
	}
}

func IsExtEvent(ev *gomatrixserverlib.ClientEvent) bool {
	if ev.Type == "m.room._ext.leave" || ev.Type == "m.room._ext.enter" {
		return true
	}
	return false
}
