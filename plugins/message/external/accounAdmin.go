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

// GET /_matrix/client/r0/account/3pid
type GetAccount3PIDResponse struct {
	ThreePIDs ThirdPartyIdentifier `json:"threepids"`
}

type ThirdPartyIdentifier struct {
	Medium      string `json:"medium"`
	Address     string `json:"address"`
	ValidatedAt int    `json:"validated_at"`
	AddedAt     int    `json:"added_at"`
}

// ThreePID represents a third-party identifier
type ThreePID struct {
	Address string `json:"address"`
	Medium  string `json:"medium"`
}

type GetThreePIDsResponse struct {
	ThreePIDs []ThreePID `json:"threepids"`
}

//POST /_matrix/client/r0/account/3pid
type PostAccount3PIDRequest struct {
	ThreePIDCreds ThreePidCredentials `json:"three_pid_creds"`
	Bind          bool                `json:"bind"`
}

type ThreePidCredentials struct {
	ClientSecret string `json:"client_secret"`
	IdServer     string `json:"id_server"`
	Sid          string `json:"sid"`
}

//POST /_matrix/client/r0/account/3pid/delete
type PostAccount3PIDDelRequest struct {
	Medium  string `json:"medium"`
	Address string `json:"address"`
}

//POST /_matrix/client/r0/account/3pid/email/requestToken
// same as /_matrix/client/r0/register/email/requestToken
type PostAccount3PIDEmailRequest struct {
	Path         string `json:"path"`
	ClientSecret string `json:"client_secret"`
	Email        string `json:"email"`
	SendAttempt  string `json:"send_attempt"`
	NextLink     string `json:"next_link,omitempty"`
	IdServer     string `json:"id_server"`
}

type PostAccount3PIDEmailResponse struct {
	Sid string `json:"sid"`
}

// POST /_matrix/client/r0/account/3pid/msisdn/requestToken
// same as /register/msisdn/requestToken
type PostAccount3PIDMsisdnRequest struct {
	Path         string `json:"path"`
	ClientSecret string `json:"client_secret"`
	Country      string `json:"country"`
	PhoneNumber  string `json:"phone_number"`
	SendAttempt  string `json:"send_attempt"`
	NextLink     string `json:"next_link,omitempty"`
	IdServer     string `json:"id_server"`
}

type PostAccount3PIDMsisdnResponse struct {
	Sid string `json:"sid"`
}

// GET /_matrix/client/r0/account/whoami
type GetAccountWhoAmIResponse struct {
	UserID string `json:"user_id"`
}
