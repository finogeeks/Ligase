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

package model

type Payload interface{}

type MsgType int
type Command int
type ApiType int

const (
	REQUEST MsgType = 1
	REPLY   MsgType = 2
	MESSAGE MsgType = 3
)

// api 对应命令号，按段分配，从1000-9999
const (
	/**************roomserver api***************/
	CMD_HS Command = 1000 + iota
	CMD_HS_SEND
	CMD_HS_ROOM_ALIAS
	CMD_HS_PROFILE
	CMD_HS_AVATARURL
	CMD_HS_DISPLAYNAME
	CMD_HS_INVITE
	CMD_HS_MAKEJOIN
	CMD_HS_SENDJOIN
	CMD_HS_NOTARY_NOTICE
)

const (
	// PUT /_matrix/federation/v1/send/{txnId}
	CMD_FED Command = 6000 + iota
	CMD_FED_SEND
	CMD_FED_ROOM_DIRECTORY
	CMD_FED_PROFILE
	CMD_FED_AVATARURL
	CMD_FED_DISPLAYNAME
	CMD_FED_INVITE
	CMD_FED_MAKEJOIN
	CMD_FED_SENDJOIN
	CMD_FED_ROOM_STATE
	CMD_FED_BACKFILL
	CMD_FED_GET_MISSING_EVENTS
	CMD_FED_NOTARY_NOTICE
	CMD_FED_USER_INFO
	CMD_FED_MAKELEAVE
	CMD_FED_SENDLEAVE
	CMD_FED_EVENT
	CMD_FED_STATE_IDS
	CMD_FED_QUERY_AUTH
	CMD_FED_EVENT_AUTH
	CMD_FED_GET_PUBLIC_ROOMS
	CMD_FED_POST_PUBLIC_ROOMS
	CMD_FED_USER_DEVICES
	CMD_FED_CLIENT_KEYS
	CMD_FED_CLIENT_KEYS_CLAIM
	CMD_FED_EXCHANGE_THIRD_PARTY_INVITE
)

const (
	API_HS  ApiType = 0
	API_FED ApiType = 1
)
