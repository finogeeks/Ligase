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

// GET /_matrix/client/r0/login
type GetLoginRequest struct {
	IsAdmin bool `json:"is_admin"`
}
type GetLoginAdminRequest GetLoginRequest

//response
type GetLoginResponse struct {
	Flows []Flow `json:"flows"`
}

type Flow struct {
	Type   string   `json:"type"`
	Stages []string `json:"stages"`
}

//POST /_matrix/client/r0/login
//request
type PostLoginRequest struct {
	RequestType        string         `json:"type"`
	Identifier         UserIdentifier `json:"identifier"`
	User               string         `json:"user"`
	Medium             string         `json:"medium"`
	Address            string         `json:"address"`
	Password           string         `json:"password"`
	Token              string         `json:"token"`
	DeviceID           string         `json:"device_id"`
	InitialDisplayName *string        `json:"initial_device_display_name"`
	IsHuman            *bool          `json:"is_human"`
	IsAdmin            bool           `json:"is_admin"`
}
type PostLoginAdminRequest PostLoginRequest

type UserIdentifier struct {
	userType string `json:"type"`
}

//response
type PostLoginResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	DeviceID    string `json:"device_id"`
}

//POST /_matrix/client/r0/logout

//POST /_matrix/client/r0/logout/all  //not support
