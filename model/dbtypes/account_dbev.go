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

package dbtypes

const (
	AccountDataInsertKey int64 = 0
	AccountInsertKey     int64 = 1
	FilterInsertKey      int64 = 2
	ProfileInsertKey     int64 = 3
	RoomTagInsertKey     int64 = 4
	RoomTagDeleteKey     int64 = 5
	ProfileInitKey       int64 = 6
	DisplayNameInsertKey int64 = 7
	AvatarInsertKey      int64 = 8
	UserInfoInsertKey    int64 = 9
	UserInfoInitKey      int64 = 10
	UserInfoDeleteKey    int64 = 11
	AccountMaxKey        int64 = 12
)

func AccountDBEventKeyToStr(key int64) string {
	switch key {
	case AccountDataInsertKey:
		return "AccountDataInsert"
	case AccountInsertKey:
		return "AccountInsert"
	case FilterInsertKey:
		return "FilterInsert"
	case ProfileInsertKey:
		return "ProfileInsert"
	case RoomTagInsertKey:
		return "RoomTagInsert"
	case RoomTagDeleteKey:
		return "RoomTagDelete"
	case ProfileInitKey:
		return "ProfileInit"
	case DisplayNameInsertKey:
		return "DisplayNameInsertKey"
	case AvatarInsertKey:
		return "AvatarInsertKey"
	case UserInfoInsertKey:
		return "UserInfoInsert"
	case UserInfoInitKey:
		return "UserInfoInit"
	case UserInfoDeleteKey:
		return "UserInfoDelete"
	default:
		return "unknown"
	}
}

func AccountDBEventKeyToTableStr(key int64) string {
	switch key {
	case AccountDataInsertKey:
		return "account_data"
	case AccountInsertKey:
		return "account_accounts"
	case FilterInsertKey:
		return "account_filter"
	case ProfileInsertKey, ProfileInitKey, DisplayNameInsertKey, AvatarInsertKey:
		return "account_profiles"
	case RoomTagInsertKey, RoomTagDeleteKey:
		return "room_tags"
	case UserInfoInitKey, UserInfoInsertKey, UserInfoDeleteKey:
		return "user_info"
	default:
		return "unknown"
	}
}

type AccountDBEvent struct {
	AccountDataInsert *AccountDataInsert `json:"account_data_insert,omitempty"`
	AccountInsert     *AccountInsert     `json:"account_insert,omitempty"`
	FilterInsert      *FilterInsert      `json:"filter_insert,omitempty"`
	ProfileInsert     *ProfileInsert     `json:"profile_insert,omitempty"`
	RoomTagInsert     *RoomTagInsert     `json:"room_tag_insert,omitempty"`
	RoomTagDelete     *RoomTagDelete     `json:"room_tag_delete,omitempty"`
	UserInfoInsert    *UserInfoInsert    `json:"user_info_insert,omitempty"`
	UserInfoDelete    *UserInfoDelete    `json:"user_info_delete,omitempty"`
}

type RoomTagInsert struct {
	RoomID  string `json:"room_id"`
	UserID  string `json:"user_id"`
	Tag     string `json:"tag"`
	Content []byte `json:"content"`
}

type RoomTagDelete struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
	Tag    string `json:"tag"`
}

type ProfileInsert struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarUrl   string `json:"avatar_url"`
}

type AccountInsert struct {
	UserID       string `json:"user_id"`
	CreatedTs    int64  `json:"created_ts"`
	PassWordHash string `json:"password_hash"`
	AppServiceID string `json:"app_service_id"`
}

type AccountDataInsert struct {
	UserID  string `json:"user_id"`
	RoomID  string `json:"room_id"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type FilterInsert struct {
	Filter   string `json:"filter"`
	FilterID string `json:"filter_id"`
	UserID   string `json:"user_id"`
}

type UserInfoInsert struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state,omitempty"`
}

type UserInfoDelete struct {
	UserID string `json:"user_id"`
}
