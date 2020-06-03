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

import (
	"context"

	"github.com/finogeeks/ligase/plugins/message/external"

	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/types"
)

type RCSServerDatabase interface {
	SetIDGenerator(idg *uid.UidGenerator)
	InsertFriendship(
		ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
		fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
		fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string) error
	GetFriendshipByRoomID(ctx context.Context, roomID string) (*types.RCSFriendship, error)
	GetFriendshipByFcIDAndToFcID(ctx context.Context, fcID, toFcID string) (*types.RCSFriendship, error)
	UpdateFriendshipByRoomID(
		ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
		fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
		fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string) error
	DeleteFriendshipByRoomID(ctx context.Context, roomID string) error
	NotFound(error) bool

	// These function is used for api.
	GetFriendshipsByFcIDOrToFcID(ctx context.Context, userID string) ([]external.Friendship, error)
	GetFriendshipsByFcIDOrToFcIDWithBot(ctx context.Context, userID string) ([]external.Friendship, error)
	GetFriendshipByFcIDOrToFcID(ctx context.Context, fcID, toFcID string) (*external.GetFriendshipResponse, error)
}
