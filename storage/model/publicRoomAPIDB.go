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

	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/dbtypes"
	types "github.com/finogeeks/ligase/model/publicroomstypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type PublicRoomAPIDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)
	SetIDGenerator(*uid.UidGenerator)
	WriteDBEvent(update *dbtypes.DBEvent) error

	UpdateRoomFromEvent(
		ctx context.Context, event gomatrixserverlib.ClientEvent,
	) error
	OnIncrementJoinedMembersInRoom(
		ctx context.Context, roomID string, n int,
	) error
	OnDecrementJoinedMembersInRoom(
		ctx context.Context, roomID string,
	) error
	OnInsertNewRoom(
		ctx context.Context,
		roomID string,
		seqID,
		joinedMembers int64,
		aliases []string,
		canonicalAlias,
		name,
		topic string,
		worldReadable,
		guestCanJoin bool,
		avatarUrl string,
		visibility bool,
	) error
	CountPublicRooms(ctx context.Context) (int64, error)
	GetPublicRooms(
		ctx context.Context, offset int64, limit int64, filter string,
	) ([]types.PublicRoom, error)
	OnUpdateRoomAttribute(
		ctx context.Context, attrName string, attrValue interface{}, roomID string,
	) error
}
