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
)

// SetRoomAliasRequest is a request to SetRoomAlias
type SetRoomAliasRequest struct {
	// ID of the user setting the alias
	UserID string `json:"user_id"`
	// New alias for the room
	Alias string `json:"alias"`
	// The room ID the alias is referring to
	RoomID string `json:"room_id"`
}

// SetRoomAliasResponse is a response to SetRoomAlias
type SetRoomAliasResponse struct {
	// Does the alias already refer to a room?
	AliasExists bool `json:"alias_exists"`
}

// GetAliasRoomIDRequest is a request to GetAliasRoomID
type GetAliasRoomIDRequest struct {
	// Alias we want to lookup
	Alias string `json:"alias"`
}

// GetAliasRoomIDResponse is a response to GetAliasRoomID
type GetAliasRoomIDResponse struct {
	// The room ID the alias refers to
	RoomID string `json:"room_id"`
}

// RemoveRoomAliasRequest is a request to RemoveRoomAlias
type RemoveRoomAliasRequest struct {
	// ID of the user removing the alias
	UserID string `json:"user_id"`
	// The room alias to remove
	Alias string `json:"alias"`
}

// RemoveRoomAliasResponse is a response to RemoveRoomAlias
type RemoveRoomAliasResponse struct{}

type RoomserverAliasRequest struct {
	SetRoomAliasRequest    *SetRoomAliasRequest    `json:"set_room_alias,omitempty"`
	GetAliasRoomIDRequest  *GetAliasRoomIDRequest  `json:"get_room_alias,omitempty"`
	RemoveRoomAliasRequest *RemoveRoomAliasRequest `json:"rem_room_alias,omitempty"`
	AllocRoomAliasRequest  *SetRoomAliasRequest    `json:"alloc_room_alias,omitempty"`
	Reply                  string
}

// RoomserverAliasAPI is used to save, lookup or remove a room alias
type RoomserverAliasAPI interface {
	AllocRoomAlias( //cli
		ctx context.Context,
		req *SetRoomAliasRequest,
		response *SetRoomAliasResponse,
	) error

	// Set a room alias
	SetRoomAlias( //cli
		ctx context.Context,
		req *SetRoomAliasRequest,
		response *SetRoomAliasResponse,
	) error

	// Get the room ID for an alias
	GetAliasRoomID( //fed & cli
		ctx context.Context,
		req *GetAliasRoomIDRequest,
		response *GetAliasRoomIDResponse,
	) error

	// Remove a room alias
	RemoveRoomAlias(
		ctx context.Context,
		req *RemoveRoomAliasRequest,
		response *RemoveRoomAliasResponse,
	) error
}
