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

package extra

import (
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func ExpandSyncData(repo *repos.RoomCurStateRepo, device *authtypes.Device, displayNameRepo *repos.DisplayNameRepo, res *syncapitypes.SyncServerResponse) {
	ExpandHints(repo, device, displayNameRepo, res)
}

func ExpandMessages(event *gomatrixserverlib.ClientEvent, userID string, repo *repos.RoomCurStateRepo, displayNameRepo *repos.DisplayNameRepo) {
	device := &authtypes.Device{
		UserID:  userID,
		IsHuman: true,
	}
	ExpandEventHint(event, device, repo, displayNameRepo)
}
