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

package types

import "github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"

type RCSInputEventContent struct {
	Event gomatrixserverlib.Event
	Reply string `json:"reply"`
}

type RCSOutputEventContent struct {
	Events  []gomatrixserverlib.Event `json:"events"`
	Succeed bool                      `json:"succeed"`
}

type RCSFriendship struct {
	ID               string `json:"_id"`
	RoomID           string `json:"room_id"`
	FcID             string `json:"fcid"`
	ToFcID           string `json:"to_fcid"`
	FcIDState        string `json:"fcid_state"`
	ToFcIDState      string `json:"to_fcid_state"`
	FcIDIsBot        bool   `json:"fcid_is_bot"`
	ToFcIDIsBot      bool   `json:"to_fcid_is_bot"`
	FcIDRemark       string `json:"fcid_remark"`
	ToFcIDRemark     string `json:"to_fcid_remark"`
	FcIDOnceJoined   bool   `json:"fcid_once_joined"`
	ToFcIDOnceJoined bool   `json:"to_fcid_once_joined"`
	FcIDDomain       string `json:"fcid_domain"`
	ToFcIDDomain     string `json:"to_fcid_domain"`
	EventID          string `json:"event_id"`
}
