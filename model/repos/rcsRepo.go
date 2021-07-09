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

package repos

import (
	"context"
	"sync"

	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
)

type RCSRepo struct {
	persist model.RCSServerDatabase
	andIDFriendship sync.Map
	orIDFriendship sync.Map
	roomFriendship      sync.Map
}

func NewRCSRepo(persist model.RCSServerDatabase) *RCSRepo {
	r := new(RCSRepo)
	r.persist = persist
	return r
}

func (r *RCSRepo) NotFound(err error) bool {
	return r.persist.NotFound(err)
}

func (r *RCSRepo) InsertFriendship(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string,
) error {
	f := types.RCSFriendship{
		ID: ID,
		RoomID: roomID,
		FcID: fcID,
		ToFcID: toFcID,
		FcIDState: fcIDState,
		ToFcIDState: toFcIDState,
		FcIDIsBot: fcIDIsBot,
		ToFcIDIsBot: toFcIDIsBot,
		FcIDRemark: fcIDRemark,
		ToFcIDRemark: toFcIDRemark,
		FcIDOnceJoined: fcIDOnceJoined,
		ToFcIDOnceJoined: toFcIDOnceJoined,
		FcIDDomain: fcIDDomain,
		ToFcIDDomain: toFcIDDomain,
		EventID: eventID,
	}
	r.andIDFriendship.Store(getID(fcID, toFcID), &f)
	r.roomFriendship.Store(roomID, &f)
	if err := r.persist.InsertFriendship(
		ctx, ID, roomID, fcID, toFcID, fcIDState, toFcIDState,
		fcIDIsBot, toFcIDIsBot, fcIDRemark, toFcIDRemark,
		fcIDOnceJoined, toFcIDOnceJoined, fcIDDomain, toFcIDDomain, eventID,
	); err != nil {
		return err
	}

	return nil
}

func (r *RCSRepo) UpdateFriendshipByRoomID(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string,
) error {
	f := types.RCSFriendship{
		ID: ID,
		RoomID: roomID,
		FcID: fcID,
		ToFcID: toFcID,
		FcIDState: fcIDState,
		ToFcIDState: toFcIDState,
		FcIDIsBot: fcIDIsBot,
		ToFcIDIsBot: toFcIDIsBot,
		FcIDRemark: fcIDRemark,
		ToFcIDRemark: toFcIDRemark,
		FcIDOnceJoined: fcIDOnceJoined,
		ToFcIDOnceJoined: toFcIDOnceJoined,
		FcIDDomain: fcIDDomain,
		ToFcIDDomain: toFcIDDomain,
		EventID: eventID,
	}
	r.andIDFriendship.Store(getID(fcID, toFcID), &f)
	r.roomFriendship.Store(roomID, &f)
	if err := r.persist.UpdateFriendshipByRoomID(
		ctx, ID, roomID, fcID, toFcID, fcIDState, toFcIDState,
		fcIDIsBot, toFcIDIsBot, fcIDRemark, toFcIDRemark,
		fcIDOnceJoined, toFcIDOnceJoined, fcIDDomain, toFcIDDomain, eventID,
	); err != nil {
		return err
	}

	return nil
}


func (r *RCSRepo) GetFriendshipByRoomID(ctx context.Context, roomID string) (*types.RCSFriendship, error) {
	if val, ok := r.roomFriendship.Load(roomID); ok {
		return val.(*types.RCSFriendship), nil
	}

	f, err := r.persist.GetFriendshipByRoomID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	r.andIDFriendship.Store(getID(f.FcID, f.ToFcID), f)
	r.roomFriendship.Store(f.RoomID, f)

	return f, nil
}

func (r *RCSRepo) GetFriendshipByFcIDAndToFcID (ctx context.Context, fcID, toFcID string) (*types.RCSFriendship, error) {
	ID := getID(fcID, toFcID)
	if val, ok := r.andIDFriendship.Load(ID); ok {
		return val.(*types.RCSFriendship), nil
	}

	f, err := r.persist.GetFriendshipByFcIDAndToFcID(ctx, fcID, toFcID)
	if err != nil {
		return nil, err
	}

	r.andIDFriendship.Store(ID, f)
	r.roomFriendship.Store(f.RoomID, f)
	return f, nil
}

func (r *RCSRepo) GetFriendshipByFcIDOrToFcID (ctx context.Context, fcID, toFcID string) (*external.GetFriendshipResponse, error) {
	ID := getID(fcID, toFcID)
	if val, ok := r.orIDFriendship.Load(ID); ok {
		return val.(*external.GetFriendshipResponse), nil
	}

	f, err := r.persist.GetFriendshipByFcIDOrToFcID(ctx, fcID, toFcID)
	if err != nil {
		return nil, err
	}

	r.orIDFriendship.Store(ID, f)
	return f, nil
}


func (r *RCSRepo) DeleteFriendshipByRoomID(ctx context.Context, roomID string) error {
	if err := r.persist.DeleteFriendshipByRoomID(ctx, roomID); err != nil {
		return err
	}

	if val, ok := r.roomFriendship.Load(roomID); ok {
		f := val.(*types.RCSFriendship)
		r.andIDFriendship.Delete(getID(f.FcID, f.ToFcID))
		r.roomFriendship.Store(f.RoomID, f)
	}

	return nil
}

func getID(fcID, toFcID string) string {
	return fcID + toFcID
}