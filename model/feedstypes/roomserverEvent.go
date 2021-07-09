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

package feedstypes

import (
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type RoomserverEvent struct {
	Ev        *gomatrixserverlib.Event
}

func (e *RoomserverEvent) GetOffset() int64 {
	return e.Ev.EventNID()
}

func (e *RoomserverEvent) GetEv() *gomatrixserverlib.Event {
	return e.Ev
}
