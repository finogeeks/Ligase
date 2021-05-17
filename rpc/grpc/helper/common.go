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

package helper

import (
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func ToDevice(device *pb.Device) *authtypes.Device {
	request := &authtypes.Device{
		ID:           device.Id,
		UserID:       device.UserID,
		DisplayName:  device.DisplayName,
		DeviceType:   device.DeviceType,
		IsHuman:      device.IsHuman,
		Identifier:   device.Identifier,
		CreateTs:     device.CreateTs,
		LastActiveTs: device.LastActiveTs,
	}
	return request
}

func ToEvent(ev *pb.Event) *gomatrixserverlib.Event {
	if ev == nil {
		return nil
	}
	var stateKey *string
	if ev.StateKey != "" || ev.Type == "m.room.member" || ev.Type == "m.room.create" {
		stateKey = &ev.StateKey
	}
	e := gomatrixserverlib.BuildEventsByFields(
		gomatrixserverlib.EventFields{
			RoomID:         ev.RoomID,
			EventID:        ev.EventID,
			EventNID:       ev.EventNID,
			DomainOffset:   ev.DomainOffset,
			Sender:         ev.Sender,
			Type:           ev.Type,
			StateKey:       stateKey,
			Content:        ev.Content,
			Redacts:        ev.Redacts,
			Depth:          ev.Depth,
			Unsigned:       ev.Unsigned,
			OriginServerTS: gomatrixserverlib.Timestamp(ev.OriginServerTS),
			Origin:         gomatrixserverlib.ServerName(ev.Origin),
			RedactsSender:  ev.RedactsSender,
		},
	)
	return e
}

func ToPBEvent(ev *gomatrixserverlib.Event) *pb.Event {
	if ev == nil {
		return nil
	}
	stateKey := ""
	if ev.StateKey() != nil {
		stateKey = *ev.StateKey()
	}
	return &pb.Event{
		RoomID:         ev.RoomID(),
		EventID:        ev.EventID(),
		EventNID:       ev.EventNID(),
		DomainOffset:   ev.DomainOffset(),
		Sender:         ev.Sender(),
		Type:           ev.Type(),
		StateKey:       stateKey,
		Content:        ev.Content(),
		Redacts:        ev.Redacts(),
		Depth:          ev.Depth(),
		Unsigned:       []byte(ev.Unsigned()),
		OriginServerTS: int64(ev.OriginServerTS()),
		Origin:         string(ev.Origin()),
		RedactsSender:  ev.RedactsSender(),
	}
}
