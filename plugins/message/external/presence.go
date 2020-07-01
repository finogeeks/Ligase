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

//PUT /_matrix/client/r0/presence/{userId}/status
type PutPresenceRequest struct {
	UserID       string `json:"user_id"`
	Presence     string `json:"presence"`
	StatusMsg    string `json:"status_msg"`
	ExtStatusMsg string `json:"ext_status_msg"`
}

//GET /_matrix/client/r0/presence/{userId}/status
type GetPresenceRequest struct {
	UserID string `json:"user_id"`
}

type GetPresenceResponse struct {
	Presence        string `json:"presence"`
	LastActiveAgo   int64  `json:"last_active_ago"`
	StatusMsg       string `json:"status_msg"`
	ExtStatusMsg    string `json:"ext_status_msg"`
	CurrentlyActive bool   `json:"currently_active"`
}

//GET /_matrix/client/r0/presence/list/{userId}
type GetPresenceListRequest struct {
	UserID string `json:"user_id"`
}

type GetPresenceListResponse []PresenceJSON

type PostPresenceListRequest struct {
	UserID string   `json:"user_id"`
	Invite []string `json:"invite"`
	Drop   []string `json:"drop"`
}

/* presence types */
type PresenceJSON struct {
	AvatarURL       string `json:"avatar_url"`
	DisplayName     string `json:"displayname"`
	LastActiveAgo   int64  `json:"last_active_ago"`
	Presence        string `json:"presence"` //required
	CurrentlyActive bool   `json:"currently_active"`
	UserID          string `json:"user_id"` //required
	StatusMsg       string `json:"status_msg"`
	ExtStatusMsg    string `json:"ext_status_msg"`
	/*
		UserName  string `json:"user_name"`
		JobNumber string `json:"job_number"`
		Mobile    string `json:"mobile"`
		Landline  string `json:"landline"`
		Email     string `json:"email"`
	*/
}

type PresenceListJSON struct {
	content PresenceJSON `json:"content"`
	typ     string       `json:"type"`
}
