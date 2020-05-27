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

const (
	RCSFriendshipTypeBot = "bot"
)

type GetFriendshipsRequest struct {
	Type string `json:"type"`
}

type GetFriendshipsResponse struct {
	Friendships []Friendship `json:"friendships"`
}

type Friendship struct {
	FcID   string `json:"fcid"`
	ToFcID string `json:"toFcid"`
	RoomID string `json:"roomId"`
	IsBot  bool   `json:"isBot"`
	Remark string `json:"remark"`
}

type GetFriendshipRequest struct {
	FcID   string `json:"fcid"`
	ToFcID string `json:"toFcid"`
}

type GetFriendshipResponse struct {
	FcID        string `json:"fcid"`
	ToFcID      string `json:"to_fcid"`
	RoomID      string `json:"room_id"`
	FcIDState   string `json:"fcid_state"`
	ToFcIDState string `json:"to_fcid_state"`
}
