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
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"
)

type DeviceDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)

	WriteDBEvent(ctx context.Context, update *dbtypes.DBEvent) error

	RecoverCache()

	CreateDevice(
		ctx context.Context, userID, deviceID, deviceType string,
		displayName *string, isHuman bool, identifier *string, specifiedTime int64,
	) (dev *authtypes.Device, returnErr error)

	InsertDevice(
		ctx context.Context, userID string, deviceID *string,
		displayName *string, deviceType, identifier string,
	) error

	OnInsertDevice(
		ctx context.Context, userID string, deviceID *string,
		displayName *string, createdTs, lastActiveTs int64, deviceType, identifier string,
	) error

	OnInsertMigDevice(
		ctx context.Context, access_token, mig_access_token, deviceID, userID string,
	) error

	RemoveDevice(
		ctx context.Context, deviceID, userID string, createTs int64,
	) error

	OnDeleteDevice(
		ctx context.Context, deviceID, userID string, createTs int64,
	) error

	GetDeviceTotal(
		ctx context.Context,
	) (int, error)

	CreateMigDevice(
		ctx context.Context, userID, deviceID, token, migToken string,
	) error

	RemoveMigDevice(
		ctx context.Context, deviceID, userID string,
	) error

	RemoveAllUserMigDevices(
		ctx context.Context, userID string,
	) error

	UpdateDeviceActiveTs(
		ctx context.Context, deviceID, userID string, lastActiveTs int64,
	) error

	OnUpdateDeviceActiveTs(
		ctx context.Context, deviceID, userID string, lastActiveTs int64,
	) error

	SelectUnActiveDevice(
		ctx context.Context, lastActiveTs int64, limit, offset int,
	) ([]string, []string, []string, int, error)

	CheckDevice(
		ctx context.Context, identifier, userID string,
	) (string, string, int64, error)

	LoadSimpleFilterData(ctx context.Context, f *filter.SimpleFilter) bool

	LoadFilterData(ctx context.Context, key string, f *filter.Filter) bool
}
