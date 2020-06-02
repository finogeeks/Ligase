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

// POST /_matrix/client/r0/user_directory/search
type PostUserSearchRequest struct {
	SearchTerm string `json:"search_term"`
	Limit      int    `json:"limit"`
}

type PostUserSearchResponse struct {
	Results []User `json:"result"`
	Limited bool   `json:"limited"`
}

type User struct {
	UserID      string `json:"user_id,omitempty"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

//PUT /_matrix/client/r0/profile/{userId}/displayname
type PutUserProfileDisplayNameRequest struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
}

// GET /_matrix/client/r0/profile/{userId}/displayname
type GetUserProfileDisplayNameRequest struct {
	UserID string `json:"user_id"`
}

//PUT /_matrix/client/r0/profile/{userId}/avatar_url
type PutUserProfileAvatarURLRequest struct {
	UserID    string `json:"user_id"`
	AvatarURL string `json:"avatar_url"`
}

// GET /_matrix/client/r0/profile/{userId}/avatar_url
type GetUserProfileAvatarURLRequest struct {
	UserID string `json:"user_id"`
}

//GET /_matrix/client/r0/profile/{userId}
type GetUserProfileRequest struct {
	UserID string `json:"user_id"`
}
