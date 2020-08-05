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

import jsonRaw "encoding/json"

//POST /_matrix/client/r0/createRoom
type PostCreateRoomRequest struct {
	Visibility      string                 `json:"visibility"`
	RoomAliasName   string                 `json:"room_alias_name"`
	Name            string                 `json:"name"`
	Topic           string                 `json:"topic"`
	Desc            string                 `json:"desc"`
	Invite          []string               `json:"invite"`
	Invite3pid      []Invite3pid           `json:"invite_3pid"`
	RoomVersion     string                 `json:"room_version"`
	CreationContent map[string]interface{} `json:"creation_content"`
	InitialState    []StateEvent           `json:"initial_state"`
	Preset          string                 `json:"preset"`
	IsDirect        bool                   `json:"is_direct"`
	AutoJoin        bool                   `json:"auto_join"`

	RoomID       string `json:"room_id"`
	GuestCanJoin bool   `json:"guest_can_join"`
}

type Invite3pid struct {
	IDServer string   `json:"id_server"`
	Medium   string   `json:"medium"`
	Address  []string `json:"address"`
}

type PostCreateRoomResponse struct {
	RoomID    string `json:"room_id"`
	RoomAlias string `json:"room_alias,omitempty"` // in synapse not spec
}

//PUT /_matrix/client/r0/directory/room/{roomAlias}
type PutDirectoryRoomAliasRequest struct {
	RoomAlias string `json:"roomAlias"`
	RoomID    string `json:"room_id"`
}

//GET /_matrix/client/r0/directory/room/{roomAlias}
type GetDirectoryRoomAliasRequest struct {
	RoomAlias string `json:"roomAlias"`
}

type GetDirectoryRoomAliasResponse struct {
	RoomID  string   `json:"room_id"`
	Servers []string `json:"servers"`
}

//DELETE /_matrix/client/r0/directory/room/{roomAlias}
type DelDirectoryRoomAliasRequest struct {
	RoomAlias string `json:"roomAlias"`
}

// GET /_matrix/client/r0/joined_rooms
type GetJoinedRoomsResponse struct {
	JoinedRooms []string `json:"joined_rooms,omitempty"`
}

//POST /_matrix/client/r0/rooms/{roomId}/invite
type PostRoomsInviteRequest struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
}

//POST /_matrix/client/r0/rooms/{roomId}/join
type PostRoomsMembershipRequest struct {
	RoomID     string `json:"room_id"`
	Membership string `json:"membership"`
	Content    []byte `json:"content,omitempty"`
}

type PostRoomsMembershipResponse struct {
	RoomID string `json:"room_id,omitempty"`
}

type ThirdPartySigned struct {
	MxID       string     `json:"mxid,omitempty"`
	Sender     string     `json:"sender,omitempty"`
	Token      string     `json:"token,omitempty"`
	Signatures Signatures `json:"signatures,omitempty"`
}

type Signatures struct {
	Signatures map[string]map[string]string
}

// POST /_matrix/client/r0/join/{roomIdOrAlias}
type PostRoomsJoinByAliasRequest struct {
	RoomID  string `json:"room_id"`
	Content []byte `json:"content"`
}

type PostRoomsJoinByAliasResponse struct {
	RoomID string `json:"room_id"`
}

//POST /_matrix/client/r0/rooms/{roomId}/leave
type PostRoomsLeaveRequest struct {
	RoomID string `json:"room_id"`
}

//POST /_matrix/client/r0/rooms/{roomId}/forget
type PostRoomsForgetRequest struct {
	RoomID string `json:"room_id"`
}

//POST /_matrix/client/r0/rooms/{roomId}/kick
type PostRoomsKickRequest struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
	Reason string `json:"reason"`
}

//POST /_matrix/client/r0/rooms/{roomId}/ban
type PostRoomsBanRequest struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
	Reason string `json:"reason"`
}

//POST /_matrix/client/r0/rooms/{roomId}/unban
type PostRoomsUnbanRequest struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
}

// GET /_matrix/client/r0/directory/list/room/{roomId}
type GetDirectoryRoomRequest struct {
	RoomID string `json:"room_id"`
}

type GetDirectoryRoomResponse struct {
	Visibility string `json:"visibility"`
}

// PUT /_matrix/client/r0/directory/list/room/{roomId}
type PutDirectoryRoomRequest struct {
	RoomID     string `json:"room_id"`
	Visibility string `json:"visibility"`
}

// GET /_matrix/client/r0/publicRooms
type GetPublicRoomsRequest struct {
	Limit  int64  `json:"limit"`
	Since  string `json:"since"`
	Filter filter `json:"filter,omitempty"`
}

type filter struct {
	SearchTerms string `json:"generic_search_term,omitempty"`
}

type GetPublicRoomsResponse struct {
	Chunk     []PublicRoomsChunk `json:"chunk"`
	NextBatch string             `json:"next_batch,omitempty"`
	PrevBatch string             `json:"prev_batch,omitempty"`
	Estimate  int64              `json:"total_room_count_estimate,omitempty"`
}

type PublicRoomsChunk struct {
	RoomID           string   `json:"room_id"`
	Aliases          []string `json:"aliases,omitempty"`
	CanonicalAlias   string   `json:"canonical_alias,omitempty"`
	Name             string   `json:"name,omitempty"`
	Topic            string   `json:"topic,omitempty"`
	AvatarURL        string   `json:"avatar_url,omitempty"`
	NumJoinedMembers int64    `json:"num_joined_members"`
	WorldReadable    bool     `json:"world_readable"`
	GuestCanJoin     bool     `json:"guest_can_join"`
}

//  POST /_matrix/client/r0/publicRooms
type PostPublicRoomsRequest struct {
	Limit                int64  `json:"limit"`
	Since                string `json:"since"`
	Filter               filter `json:"filter"`
	IncludeAllNetworks   bool   `json:"include_all_networks"`
	ThirdPartyInstanceID string `json:"third_party_instance_id"`
	Server               string `json:"server"`
}

// response
type PostPublicRoomsResponse struct {
	Chunk     []PublicRoomsChunk `json:"chunk"`
	NextBatch string             `json:"next_batch,omitempty"`
	PrevBatch string             `json:"prev_batch,omitempty"`
	Estimate  int64              `json:"total_room_count_estimate,omitempty"`
}

//  GET /_matrix/client/r0/rooms/{roomID}/joined_members
type GetJoinedMemberRequest struct {
	RoomID string `json:"room_id"`
}

type GetJoinedMemberResponse struct {
	Joined map[string]MemberContent `json:"joined"`
}

// MemberContent is the event content for http://matrix.org/docs/spec/client_server/r0.2.0.html#m-room-member
type MemberContent struct {
	Membership       string    `json:"membership"`
	DisplayName      string    `json:"displayname"`
	AvatarURL        string    `json:"avatar_url"`
	Reason           string    `json:"reason,omitempty"`
	ThirdPartyInvite *TPInvite `json:"third_party_invite,omitempty"`
}

// TPInvite is the "Invite" structure defined at http://matrix.org/docs/spec/client_server/r0.2.0.html#m-room-member
type TPInvite struct {
	DisplayName string         `json:"display_name"`
	Signed      TPInviteSigned `json:"signed"`
}

// TPInviteSigned is the "signed" structure defined at http://matrix.org/docs/spec/client_server/r0.2.0.html#m-room-member
type TPInviteSigned struct {
	MXID       string                       `json:"mxid"`
	Signatures map[string]map[string]string `json:"signatures"`
	Token      string                       `json:"token"`
}

//  GET /_matrix/client/r0/rooms
type GetRoomInfoRequest struct {
	RoomIDs []string `json:"room_ids"`
}

type GetRoomInfoResponse struct {
	Rooms []GetRoomInfo `json:"rooms"`
}

type GetRoomInfo struct {
	Creator     string   `json:"creator"`
	RoomID      string   `json:"room_id"`
	RoomExists  bool     `json:"room_exists"`
	Name        string   `json:"name"`
	AvatarURL   string   `json:"avatar_url"`
	IsDirect    bool     `json:"is_direct"`
	IsChannel   bool     `json:"is_channel"`
	IsFederate  bool     `json:"is_federate"`
	Topic       string   `json:"topic"`
	JoinMembers []string `json:"join_members"`
	PowerLevels jsonRaw.RawMessage `json:"power_levels"`
}

// POST /_matrix/client/unstable/{roomID}/dismiss
type DismissRoomRequest struct {
	RoomID string `json:"roomID"`
	UserID string `json:"userID,omitempty"`
}

type DismissRoomResponse struct {
}