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

type KeyDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)

	FetcherName() string

	FetchKeys(
		ctx context.Context,
		requests map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.Timestamp,
	) (map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult, error)

	StoreKeys(
		ctx context.Context,
		keyMap map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult,
	) error

	InsertRootCA(ctx context.Context, rootCA string) error
	SelectAllCerts(ctx context.Context) (string, string, string, string, error)
	UpsertCert(ctx context.Context, serverCert, serverKey string) error
	UpsertCRL(ctx context.Context, CRL string) error
}
