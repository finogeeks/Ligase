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

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type AppServiceDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)
	StoreEvent(
		ctx context.Context,
		appServiceID string,
		event *gomatrixserverlib.ClientEvent,
	) error

	GetEventsWithAppServiceID(
		ctx context.Context,
		appServiceID string,
		limit int,
	) (int, int, []gomatrixserverlib.ClientEvent, bool, error)

	CountEventsWithAppServiceID(
		ctx context.Context,
		appServiceID string,
	) (int, error)

	UpdateTxnIDForEvents(
		ctx context.Context,
		appserviceID string,
		maxID, txnID int,
	) error

	RemoveEventsBeforeAndIncludingID(
		ctx context.Context,
		appserviceID string,
		eventTableID int,
	) error

	GetLatestTxnID(
		ctx context.Context,
	) (int, error)

	GetMsgEventsWithLimit(
		ctx context.Context, limit, offset int64,
	) ([]int64, [][]byte, error)

	GetMsgEventsTotal(
		ctx context.Context,
	) (count int, minID int64, err error)

	UpdateMsgEvent(
		ctx context.Context, id int64, NewBytes []byte,
	) error

	GetEncMsgEventsWithLimit(
		ctx context.Context, limit, offset int64,
	) ([]int64, [][]byte, error)

	GetEncMsgEventsTotal(
		ctx context.Context,
	) (count int, err error)
}
