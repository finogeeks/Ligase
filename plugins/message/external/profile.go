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

// GET /_matrix/client/r0/profile/{userID}
type GetProfileRequest struct {
	UserID string `json:"userID"`
}

type GetProfileResponse struct {
	AvatarURL    string `json:"avatar_url"`
	DisplayName  string `json:"displayname"`
	Status       string `json:"status,omitempty"`
	StatusMsg    string `json:"status_msg,omitempty"`
	ExtStatusMsg string `json:"ext_status_msg,omitempty"`
}

// GET /_matrix/client/r0/profile/{userID}/avatar_url
type GetAvatarURLRequest struct {
	UserID string `json:"user_id"`
}

type GetAvatarURLResponse struct {
	AvatarURL string `json:"avatar_url"`
}

// PUT /_matrix/client/r0/profile/{userID}/avatar_url
type PutAvatarURLRequest struct {
	UserID    string `json:"user_id"`
	AvatarURL string `json:"avatar_url"`
}

// GET /_matrix/client/r0/profile/{userID}/displayname
type GetDisplayNameRequest struct {
	UserID string `json:"user_id"`
}

type GetDisplayNameResponse struct {
	DisplayName string `json:"displayname"`
}

// PUT /_matrix/client/r0/profile/{userID}/displayname
type PutDisplayNameRequest struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"displayname"`
}

// GET /_matrix/client/r0/account/whoami
type GetAccountWhoAmI struct {
	UserID string `json:"user_id"`
}

// GET /_matrix/client/r0/profiles
type GetProfilesRequest struct {
	UserIDs []string `json:"user_ids"`
}

type GetProfilesResponse struct {
	Profiles []ProfileItem `json:"profiles"`
}

type ProfileItem struct {
	UserID      string `json:"user_id"`
	AvatarURL   string `json:"avatar_url"`
	DisplayName string `json:"displayname"`
}
