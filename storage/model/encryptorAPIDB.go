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

type EncryptorAPIDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)

	WriteDBEvent(update *dbtypes.DBEvent) error

	RecoverCache()

	InsertDeviceKey(
		ctx context.Context,
		deviceID, userID, keyInfo, al, sig, identifier string,
	) (err error)

	InsertOneTimeKey(
		ctx context.Context,
		deviceID, userID, keyID, keyInfo, al, sig, identifier string,
	) (err error)

	OnInsertDeviceKey(
		ctx context.Context,
		deviceID, userID, keyInfo, al, sig, identifier string,
	) (err error)

	OnInsertOneTimeKey(
		ctx context.Context,
		deviceID, userID, keyID, keyInfo, al, sig, identifier string,
	) (err error)

	DeleteOneTimeKey(
		ctx context.Context,
		deviceID, userID, keyID, algorithm string,
	) (err error)

	OnDeleteOneTimeKey(
		ctx context.Context,
		deviceID, userID, keyID, algorithm string,
	) (err error)

	DeleteMacOneTimeKey(
		ctx context.Context,
		deviceID, userID, identifier string,
	) (err error)

	OnDeleteMacOneTimeKey(
		ctx context.Context,
		deviceID, userID, identifier string,
	) (err error)

	InsertAl(
		ctx context.Context, uid, device, als, identifier string,
	) (err error)

	OnInsertAl(
		ctx context.Context, uid, device, als, identifier string,
	) (err error)

	DeleteDeviceKeys(
		ctx context.Context, deviceID, userID string,
	) error

	DeleteMacKeys(
		ctx context.Context, deviceID, userID, identifier string,
	) error

	OnDeleteAl(
		ctx context.Context,
		userID, deviceID string,
	) error

	OnDeleteMacAl(
		ctx context.Context,
		userID, deviceID, identifier string,
	) error

	OnDeleteDeviceKey(
		ctx context.Context,
		deviceID, userID string,
	) error

	OnDeleteMacDeviceKey(
		ctx context.Context,
		deviceID, userID, identifier string,
	) error

	OnDeleteDeviceOneTimeKey(
		ctx context.Context,
		deviceID, userID string,
	) error

	DeleteDeviceOneTimeKey(
		ctx context.Context, deviceID, userID string,
	) error
}
