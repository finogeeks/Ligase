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
	PublicRoomInsertKey          int64 = 0
	PublicRoomUpdateKey          int64 = 1
	PublicRoomIncrementJoinedKey int64 = 2
	PublicRoomDecrementJoinedKey int64 = 3
	PublicRoomMaxKey             int64 = 4
)

func PublicRoomDBEventKeyToStr(key int64) string {
	switch key {
	case PublicRoomInsertKey:
		return "PublicRoomInsert"
	case PublicRoomUpdateKey:
		return "PublicRoomUpdate"
	case PublicRoomIncrementJoinedKey:
		return "PublicRoomIncrementJoined"
	case PublicRoomDecrementJoinedKey:
		return "PublicRoomDecrementJoined"
	default:
		return "unknown"
	}
}

func PublicRoomDBEventKeyToTableStr(key int64) string {
	switch key {
	case PublicRoomInsertKey, PublicRoomUpdateKey, PublicRoomIncrementJoinedKey, PublicRoomDecrementJoinedKey:
		return "publicroomsapi_public_rooms"
	default:
		return "unknown"
	}
}

type PublicRoomDBEvent struct {
	PublicRoomInsert *PublicRoomInsert `json:"public_room_insert,omitempty"`
	PublicRoomUpdate *PublicRoomUpdate `json:"public_room_update,omitempty"`
	PublicRoomJoined *string           `json:"public_room_joined,omitempty"`
}

type PublicRoomInsert struct {
	RoomID         string   `json:"room_id"`
	SeqID          int64    `json:"seq_id"`
	JoinedMembers  int64    `json:"joined_members"`
	Aliases        []string `json:"aliases"`
	CanonicalAlias string   `json:"canonical_alias"`
	Name           string   `json:"name"`
	Topic          string   `json:"topic"`
	WorldReadable  bool     `json:"world_readable"`
	GuestCanJoin   bool     `json:"guest_can_join"`
	AvatarUrl      string   `json:"avatar_url"`
	Visibility     bool     `json:"visibility"`
}

type PublicRoomUpdate struct {
	RoomID    string      `json:"room_id"`
	AttrName  string      `json:"attr_name"`
	AttrValue interface{} `json:"attr_value"`
}
