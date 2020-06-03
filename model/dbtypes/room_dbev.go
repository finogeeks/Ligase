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

package dbtypes

const (
	EventJsonInsertKey      int64 = 0
	EventInsertKey          int64 = 1
	EventRoomInsertKey      int64 = 2
	EventRoomUpdateKey      int64 = 3
	EventStateSnapInsertKey int64 = 4
	EventInviteInsertKey    int64 = 5
	EventInviteUpdateKey    int64 = 6

	EventMembershipInsertKey       int64 = 7
	EventMembershipUpdateKey       int64 = 8
	EventMembershipForgetUpdateKey int64 = 9

	AliasInsertKey int64 = 10
	AliasDeleteKey int64 = 11

	RoomDomainInsertKey int64 = 12
	RoomEventUpdateKey  int64 = 13
	RoomDepthUpdateKey  int64 = 14

	SettingUpsertKey int64 = 15

	EventMaxKey int64 = 16
)

func RoomDBEventKeyToStr(key int64) string {
	switch key {
	case EventJsonInsertKey:
		return "EventJsonInsert"
	case EventInsertKey:
		return "EventInsert"
	case EventRoomInsertKey:
		return "EventRoomInsert"
	case EventRoomUpdateKey:
		return "EventRoomUpdate"
	case EventStateSnapInsertKey:
		return "EventStateSnapInsert"
	case EventInviteInsertKey:
		return "EventInviteInsert"
	case EventInviteUpdateKey:
		return "EventInviteUpdate"
	case EventMembershipInsertKey:
		return "EventMembershipInsert"
	case EventMembershipUpdateKey:
		return "EventMembershipUpdate"
	case EventMembershipForgetUpdateKey:
		return "EventMembershipForgetUpdate"
	case AliasInsertKey:
		return "AliasInsertKey"
	case AliasDeleteKey:
		return "AliasDeleteKey"
	case RoomDomainInsertKey:
		return "RoomDomainInsertKey"
	case RoomEventUpdateKey:
		return "RoomEventUpdateKey"
	case RoomDepthUpdateKey:
		return "RoomDepthUpdateKey"
	case SettingUpsertKey:
		return "SettingUpsertKey"
	default:
		return "unknown"
	}
}

func RoomDBEventKeyToTableStr(key int64) string {
	switch key {
	case EventJsonInsertKey:
		return "event_json"
	case EventInsertKey, RoomEventUpdateKey:
		return "event"
	case EventRoomInsertKey, EventRoomUpdateKey, RoomDepthUpdateKey:
		return "event_room"
	case EventStateSnapInsertKey:
		return "event_state_snap"
	case EventInviteInsertKey, EventInviteUpdateKey:
		return "event_invite"
	case EventMembershipInsertKey, EventMembershipUpdateKey, EventMembershipForgetUpdateKey:
		return "event_member"
	case AliasInsertKey, AliasDeleteKey:
		return "room_aliases"
	case RoomDomainInsertKey:
		return "roomserver_room_domains"
	case SettingUpsertKey:
		return "roomserver_settings"
	default:
		return "unknown"
	}
}

type RoomDBEvent struct {
	EventJsonInsert      *EventJsonInsert      `json:"event_json_insert,omitempty"`
	EventInsert          *EventInsert          `json:"event_insert,omitempty"`
	EventRoomInsert      *EventRoomInsert      `json:"event_room_insert,omitempty"`
	EventRoomUpdate      *EventRoomUpdate      `json:"event_room_update,omitempty"`
	EventStateSnapInsert *EventStateSnapInsert `json:"event_state_snap_insert,omitempty"`
	EventInviteInsert    *EventInviteInsert    `json:"event_invite_insert,omitempty"`
	EventInviteUpdate    *EventInviteUpdate    `json:"event_invite_update,omitempty"`

	EventMembershipInsert       *EventMembershipInsert       `json:"event_member_insert,omitempty"`
	EventMembershipUpdate       *EventMembershipUpdate       `json:"event_member_update,omitempty"`
	EventMembershipForgetUpdate *EventMembershipForgetUpdate `json:"event_member_forget_update,omitempty"`

	AliaseInsert *AliaseInsert `json:"aliase_insert,omitempty"`
	AliaseDelete *AliaseDelete `json:"aliase_delete,omitempty"`

	RoomDomainInsert *RoomDomainInsert `json:"room_domain_insert,omitempty"`
	RoomEventUpdate  *RoomEventUpdate  `json:"room_event_update,omitempty"`
	RoomDepthUpdate  *RoomDepthUpdate  `json:"room_depth_update,omitempty"`

	SettingsInsert *SettingsInsert `json:"setting_insert,omitempty"`
}

type RoomDepthUpdate struct {
	RoomNid int64 `json:"room_nid"`
	Depth   int64 `json:"depth"`
}

type RoomEventUpdate struct {
	RoomNid  int64  `json:"room_nid"`
	EventNid int64  `json:"event_nid"`
	Depth    int64  `json:"depth"`
	Offset   int64  `json:"offset"`
	Domain   string `json:"domain"`
}

type EventJsonInsert struct {
	EventNid  int64  `json:"event_nid"`
	EventJson []byte `json:"event_json"`
	EventType string `json:"event_type"`
}

type EventInsert struct {
	RoomNid       int64   `json:"room_nid"`
	EventType     string  `json:"event_type"`
	EventStateKey string  `json:"event_state_key"`
	EventId       string  `json:"event_id"`
	RefSha        []byte  `json:"reference_sha256"`
	AuthEventNids []int64 `json:"auth_event_nids"`
	Depth         int64   `json:"depth"`
	EventNid      int64   `json:"event_nid"`
	StateSnapNid  int64   `json:"state_snapshot_nid"`
	RefEventId    string  `json:"ref_event_id"`
	Sha           []byte  `json:"reference_sha256"`
	Offset        int64   `json:"offset"`
	Domain        string  `json:"domain"`
}

type EventRoomInsert struct {
	RoomNid int64  `json:"room_nid"`
	RoomId  string `json:"room_id"`
}

type EventRoomUpdate struct {
	LatestEventNids  []int64 `json:"latest_event_nids"`
	LastEventSentNid int64   `json:"last_event_sent_nid"`
	StateSnapNid     int64   `json:"state_snapshot_nid"`
	RoomNid          int64   `json:"room_nid"`
	Version          int64   `json:"version"`
	Depth            int64   `json:"depth"`
}

type EventStateSnapInsert struct {
	StateSnapNid   int64   `json:"state_snapshot_nid"`
	RoomNid        int64   `json:"room_nid"`
	StateBlockNids []int64 `json:"state_block_nids"`
}

type EventInviteInsert struct {
	RoomNid int64  `json:"room_nid"`
	EventId string `json:"event_id"`
	Target  string `json:"target"`
	Sender  string `json:"sender"`
	Content []byte `json:"content"`
}

type EventInviteUpdate struct {
	RoomNid int64  `json:"room_nid"`
	Target  string `json:"target"`
}

type EventMembershipInsert struct {
	RoomNID       int64  `json:"room_nid"`
	Target        string `json:"target_id"`
	RoomID        string `json:"room_id"`
	MembershipNID int64  `json:"membership_nid"`
	EventNID      int64  `json:"event_nid"`
}

type EventMembershipUpdate struct {
	RoomID     int64  `json:"room_nid"`
	Target     string `json:"target_id"`
	Sender     string `json:"sender_id"`
	Membership int64  `json:"membership_nid"`
	EventNID   int64  `json:"event_nid"`
	Version    int64  `json:"version"`
}

type EventMembershipForgetUpdate struct {
	RoomID   int64  `json:"room_nid"`
	Target   string `json:"target_id"`
	ForgetID int64  `json:"target_forget_nid"`
}

type AliaseInsert struct {
	Alias  string `json:"alias"`
	RoomID string `json:"room_id"`
}

type AliaseDelete struct {
	Alias string `json:"alias"`
}

type RoomDomainInsert struct {
	RoomNid int64  `json:"room_nid"`
	Domain  string `json:"domain"`
	Offset  int64  `json:"offset"`
}

type SettingsInsert struct {
	SettingKey string `json:"setting_key"`
	Val        string `json:"val"`
}
