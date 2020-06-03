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

//POST /_matrix/client/r0/register
//request
type AuthDict struct {
	Type     string `json:"type"`
	Session  string `json:"session"`
	Mac      []byte `json:"mac"`
	Response string `json:"response"`
}

type PostRegisterRequest struct {
	Auth               AuthDict `json:"auth"`
	BindEmail          bool     `json:"bind_email"`
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	DeviceID           string   `json:"device_id"`
	InitialDisplayName string   `json:"initial_device_display_name"`
	InhibitLogin       bool     `json:"inhibit_login"`

	Kind        string `json:"kind"`
	Domain      string `json:"domain"`
	AccessToken string `json:"access_token"`

	RemoteAddr string `json:"remote_addr"`

	Admin bool `json:"admin"`
}

// legacyRegisterRequest represents the submitted registration request for v1 API.
type LegacyRegisterRequest struct {
	Password string `json:"password"`
	Username string `json:"user"`
	Admin    bool   `json:"admin"`
	Type     string `json:"type"`
	Mac      []byte `json:"mac"`
}

//response
type RegisterResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	DeviceID    string `json:"device_id"`
}

// Flow represents one possible way that the client can authenticate a request.
// https://matrix.org/docs/spec/client_server/r0.3.0.html#user-interactive-authentication-api
type AuthFlow struct {
	Stages []string `json:"stages"`
}

// http://matrix.org/speculator/spec/HEAD/client_server/unstable.html#user-interactive-authentication-api
type UserInteractiveResponse struct {
	Flows     []AuthFlow             `json:"flows"`
	Completed []string               `json:"completed"`
	Params    map[string]interface{} `json:"params"`
	Session   string                 `json:"session"`
}

//POST /_matrix/client/r0/register/email/requestToken
//request
type PostRegisterEmailRequest struct {
	ClientSecret string `json:"client_secret"`
	Email        string `json:"email"`
	SendAttempt  int    `json:"send_attempt"`
	NextLink     string `json:"next_link"`
	IDServer     string `json:"id_server"`
}

//response
type PostRegisterEmailResponse struct {
	SID string `json:"sid"`
}

//POST /_matrix/client/r0/register/msisdn/requestToken
//request
type PostRegisterMsisdnRequest struct {
	ClientSecret string `json:"client_secret"`
	Country      string `json:"country"`
	PhoneNumber  string `json:"phone_number"`
	SendAttempt  int    `json:"send_attempt"`
	NextLink     string `json:"next_link"`
	IDServer     string `json:"id_server"`
}

//response
type PostRegisterMsisdResponse struct {
	SID string `json:"sid"`
}

//POST /_matrix/client/r0/account/password
//request
type PostAccountPasswordRequest struct {
	NewPassword string   `json:"new_password"`
	Auth        AuthData `json:"auth"`
}

type AuthData struct {
	Type    string `json:"type"`
	Session string `json:"session"`
}

//POST /_matrix/client/r0/account/password/email/requestToken
//request
type PostAccountPasswordEmailRequest struct {
	ClientSecret string `json:"client_secret"`
	Email        string `json:"email"`
	SendAttempt  int    `json:"send_attempt"`
	NextLink     string `json:"next_link"`
	IDServer     string `json:"id_server"`
}

//response
type PostAccountPasswordEmailResponse struct {
	SID string `json:"sid"`
}

//POST /_matrix/client/r0/account/password/msisdn/requestToken
//request
type PostAccountPasswordMsisdnRequest struct {
	ClientSecret string `json:"client_secret"`
	Country      string `json:"country"`
	PhoneNumber  string `json:"phone_number"`
	SendAttempt  int    `json:"send_attempt"`
	NextLink     string `json:"next_link"`
	IDServer     string `json:"id_server"`
}

//response
type PostAccountPasswordMsisdResponse struct {
	SID string `json:"sid"`
}

//POST /_matrix/client/r0/account/deactivate
type PostAccountDeactivateRequest struct {
	Auth AuthData `json:"auth"`
}

// GET /_matrix/client/r0/register/available
type GetRegisterAvail struct {
	UserName string `json:"username"`
}

type GetRegisterAvailResponse struct {
	Available bool `json:"available"`
}
