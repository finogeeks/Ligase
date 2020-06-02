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

package processors

import (
	"context"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	dbregistry.Register("publicroomsapi_public_rooms", NewDBPublicroomapiPublicRoomsProcessor, nil)
}

type DBPublicroomapiPublicRoomsProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.PublicRoomAPIDatabase
}

func NewDBPublicroomapiPublicRoomsProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBPublicroomapiPublicRoomsProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBPublicroomapiPublicRoomsProcessor) Start() {
	db, err := common.GetDBInstance("publicroomapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to publicroomapi db")
	}
	p.db = db.(model.PublicRoomAPIDatabase)
}

func (p *DBPublicroomapiPublicRoomsProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.PublicRoomInsertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.PublicRoomUpdateKey:
		p.processUpdate(ctx, inputs)
	case dbtypes.PublicRoomIncrementJoinedKey:
		p.processIncJoined(ctx, inputs)
	case dbtypes.PublicRoomDecrementJoinedKey:
		p.processDecJoined(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBPublicroomapiPublicRoomsProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PublicRoomDBEvents.PublicRoomInsert
		err := p.db.OnInsertNewRoom(ctx, msg.RoomID, msg.SeqID, msg.JoinedMembers, msg.Aliases, msg.CanonicalAlias, msg.Name, msg.Topic,
			msg.WorldReadable, msg.GuestCanJoin, msg.AvatarUrl, msg.Visibility)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.RoomID, msg.SeqID, msg.JoinedMembers, msg.Aliases, msg.CanonicalAlias, msg.Name, msg.Topic,
				msg.WorldReadable, msg.GuestCanJoin, msg.AvatarUrl, msg.Visibility)
		}
	}
	return nil
}

func (p *DBPublicroomapiPublicRoomsProcessor) processUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.PublicRoomDBEvents.PublicRoomUpdate
		err := p.db.OnUpdateRoomAttribute(ctx, msg.AttrName, msg.AttrValue, msg.RoomID)
		if err != nil {
			log.Error(p.name, "update err", err, msg.AttrName, msg.AttrValue, msg.RoomID)
		}
	}
	return nil
}

func (p *DBPublicroomapiPublicRoomsProcessor) processIncJoined(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		roomID := *v.Event.PublicRoomDBEvents.PublicRoomJoined
		err := p.db.OnIncrementJoinedMembersInRoom(ctx, roomID)
		if err != nil {
			log.Error(p.name, "inc joined err", err, roomID)
		}
	}
	return nil
}

func (p *DBPublicroomapiPublicRoomsProcessor) processDecJoined(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		roomID := *v.Event.PublicRoomDBEvents.PublicRoomJoined
		err := p.db.OnDecrementJoinedMembersInRoom(ctx, roomID)
		if err != nil {
			log.Error(p.name, "dec joined err", err, roomID)
		}
	}
	return nil
}
