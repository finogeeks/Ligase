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

import (
	types "github.com/finogeeks/ligase/model/publicroomstypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

//  GET /_matrix/federation/v1/state/{roomId}
type GetFedRoomStateRequest struct {
	RoomID  string `json:"room_id"`
	EventID string `json:"event_id,omitempty"`
}

//GET /_matrix/federation/v1/backfill/{roomId}
type GetFedBackFillRequest struct {
	RoomID      string `json:"roomId"`
	BackFillIds string `json:"v,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Dir         string `json:"dir"`
	Domain      string `json:"domain"`
	Origin      string `json:"origin"`
}

//  GET /_matrix/federation/v1/media/download/{serverName}/{mediaId}
type GetFedDownloadRequest struct {
	ID       string `json:"id"`
	FileType string `json:"fileType"`
	MediaID  string `json:"mediaID"`
	Width    string `json:"width,omitempty"`
	Method   string `json:"method,omitempty"`
}
type GetFedDownloadResponse struct {
	ID         string              `json:"id"`
	StatusCode int                 `json:"statusCode"`
	Header     map[string][]string `json:"header"`
}

type GetFedDirectoryRequest struct {
	RoomAlias string `json:"room_alias"`
}

type GetFedDirectoryResponse struct {
	RoomID  string   `json:"room_id"`
	Servers []string `json:"servers"`
}

type FedJoinEvent struct {
	Sender         string `json:"sender"`
	Origin         string `json:"origin"`
	OriginServerTS int64  `json:"origin_server_ts"`
	Type           string `json:"type"`
	StateKey       string `json:"state_key"`
	Content        struct {
		Membership string `json:"membership"`
	} `json:"content"`
}

type GetMakeJoinRequest struct {
	RoomID string   `json:"roomId"`
	UserID string   `json:"userId"`
	Ver    []string `json:"ver"`
}

type PutSendJoinRequest struct {
	RoomID  string                  `json:"roomId"`
	EventID string                  `json:"eventId"`
	Event   gomatrixserverlib.Event `json:"event"`
}

type GetMakeLeaveRequest struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

type PutSendLeaveRequest struct {
	RoomID  string                  `json:"roomId"`
	EventID string                  `json:"eventId"`
	Event   gomatrixserverlib.Event `json:"event"`
}

type PutSendJoinResponseRoomState struct {
	Origin    string                    `json:"origin"`
	AuthChain []gomatrixserverlib.Event `json:"auth_chain"`
	State     []gomatrixserverlib.Event `json:"state"`
}

type PutFedInviteRequest struct {
	RoomID  string                  `json:"roomID"`
	EventID string                  `json:"eventID"`
	Event   gomatrixserverlib.Event `json:"event"`
}

type GetMissingEventsRequest struct {
	RoomID         string   `json:"roomId"`
	Limit          int      `json:"limit"`
	MinDepth       int64    `json:"min_depth"`
	EarliestEvents []string `json:"earliest_events"`
	LatestEvents   []string `json:"latest_events"`
}

type GetMissingEventsResponse struct {
	Events []gomatrixserverlib.Event `json:"events"`
}

type GetEventAuthRequest struct {
	RoomID  string `json:"roomId"`
	EventID string `json:"eventId"`
}

type GetEventAuthResponse struct {
	AuthChain []gomatrixserverlib.Event `json:"auth_chain"`
}

type PostQueryAuthRequest struct {
	RoomID    string                    `json:"roomId"`
	EventID   string                    `json:"eventId"`
	AuthChain []gomatrixserverlib.Event `json:"auth_chain"`
	Missing   []string                  `json:"missing,omitempty"`
	Rejects   map[string]struct {
		Reason string `json:"reason"`
	} `json:"rejects,omitempty"`
}

type PostQueryAuthResponse struct {
	AuthChain []gomatrixserverlib.Event `json:"auth_chain"`
	Missing   []string                  `json:"missing"`
	Rejects   map[string]struct {
		Reason string `json:"reason"`
	} `json:"rejects"`
}

type GetEventRequest struct {
	EventID string `json:"eventId"`
}

type GetEventResponse struct {
	Origin         string                    `json:"origin"`
	OriginServerTS int64                     `json:"origin_server_ts"`
	PDUs           []gomatrixserverlib.Event `json:"pdus"`
}

type GetStateIDsRequest struct {
	RoomID  string `json:"roomId"`
	EventID string `json:"event_id"`
}

type GetStateIDsResponse struct {
	AuthChainIDs []string `json:"auth_chain_ids"`
	PduIds       []string `json:"pdu_ids"`
}

type GetFedPublicRoomsRequest struct {
	Limit                int64  `json:"limit"`
	Since                string `json:"since"`
	IncludeAllNetworks   bool   `json:"include_all_networks"`
	ThirdPartyInstanceID string `json:"third_party_instance_id"`
}

type GetFedPublicRoomsResponse struct {
	Chunk     []types.PublicRoom `json:"chunk"`
	NextBatch string             `json:"next_batch,omitempty"`
	PrevBatch string             `json:"prev_batch,omitempty"`
	Estimate  int64              `json:"total_room_count_estimate,omitempty"`
}

type PostFedPublicRoomsRequest struct {
	Limit                int64  `json:"limit"`
	Since                string `json:"since"`
	Filter               string `json:"filter"`
	IncludeAllNetworks   bool   `json:"include_all_networks"`
	ThirdPartyInstanceID string `json:"third_party_instance_id"`
}

type PostFedPublicRoomsResponse struct {
	Chunk     []types.PublicRoom `json:"chunk"`
	NextBatch string             `json:"next_batch,omitempty"`
	PrevBatch string             `json:"prev_batch,omitempty"`
	Estimate  int64              `json:"total_room_count_estimate,omitempty"`
}

type PostQueryClientKeysRequest struct {
	DeviceKeys map[string][]string `json:"device_keys"`
}

type PostQueryClientKeysResponse struct {
	DeviceKeys map[string]map[string]DeviceKeys `json:"device_keys"`
}

type PostClaimClientKeysRequest struct {
	OneTimeKeys map[string]map[string]string `json:"one_time_keys"`
}

type PostClaimClientKeysResponse struct {
	OneTimeKeys map[string]map[string]map[string]interface{} `json:"one_time_keys"`
}
