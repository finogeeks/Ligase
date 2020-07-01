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
	"github.com/finogeeks/ligase/model/dbtypes"
)

type PresenceDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)

	WriteDBEvent(update *dbtypes.DBEvent) error

	RecoverCache()

	UpsertPresences(
		ctx context.Context, userID, status, statusMsg, extStatusMsg string,
	) error

	OnUpsertPresences(
		ctx context.Context, userID, status, statusMsg, extStatusMsg string,
	) error
}
