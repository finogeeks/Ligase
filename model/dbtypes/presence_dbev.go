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
	PresencesInsertKey int64 = 0
	PresenceMaxKey     int64 = 1
)

func PresenceDBEventKeyToStr(key int64) string {
	switch key {
	case PresencesInsertKey:
		return "PresencesInsert"
	default:
		return "unknown"
	}
}

func PresenceDBEventKeyToTableStr(key int64) string {
	switch key {
	case PresencesInsertKey:
		return "presence_presences"
	default:
		return "unknown"
	}
}

type PresenceDBEvent struct {
	PresencesInsert *PresencesInsert `json:"presences_insert,omitempty"`
}

type PresencesInsert struct {
	UserID       string `json:"user_id"`
	Status       string `json:"status"`
	StatusMsg    string `json:"status_msg"`
	ExtStatusMsg string `json:"ext_status_msg"`
}
