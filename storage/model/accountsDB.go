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

	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"

	_ "github.com/lib/pq"
)

type AccountsDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)

	RecoverCache()

	WriteDBEvent(ctx context.Context, update *dbtypes.DBEvent) error

	CreateAccount(
		ctx context.Context, userID, plaintextPassword, appServiceID, displayName string,
	) (*authtypes.Account, error)

	CreateAccountWithCheck(
		ctx context.Context, oldAccount *authtypes.Account, userID, plaintextPassword, appServiceID, displayName string,
	) (*authtypes.Account, error)

	GetAccount(ctx context.Context, userID string) (*authtypes.Account, error)

	UpsertProfile(ctx context.Context, userID, displayName, avatarURL string) error
	UpsertProfileSync(ctx context.Context, userID, displayName, avatarURL string) error

	UpsertDisplayName(ctx context.Context, userID, displayName string) error
	UpsertDisplayNameSync(ctx context.Context, userID, displayName string) error

	UpsertAvatar(ctx context.Context, userID, avatarURL string) error
	UpsertAvatarSync(ctx context.Context, userID, avatarURL string) error

	InsertAccount(
		ctx context.Context, userID, plaintextPassword, appServiceID, displayName string,
	) (*authtypes.Account, error)

	PutFilter(ctx context.Context, userID, filterID, filter string) error

	AddRoomTag(ctx context.Context, userId, roomID, tag string, content []byte) error

	DeleteRoomTag(ctx context.Context, userId, roomID, tag string) error

	GetAccountsTotal(ctx context.Context) (int, error)
	GetActualTotal(ctx context.Context) (int, error)
	GetRoomTagsTotal(ctx context.Context) (int, error)

	SaveAccountData(ctx context.Context, userID, roomID, dataType, content string) error

	GetAccountDataTotal(ctx context.Context) (int, error)
	OnInsertAccountData(ctx context.Context, userID, roomID, dataType, content string) error
	OnInsertAccount(ctx context.Context, userID, hash, appServiceID string, createdTs int64) error
	OnInsertFilter(ctx context.Context, filter, filterID, userID string) error
	OnUpsertProfile(ctx context.Context, userID, displayName, avatarURL string) error

	OnInitProfile(ctx context.Context, userID, displayName, avatarURL string) error

	OnUpsertDisplayName(ctx context.Context, userID, displayName string) error

	OnUpsertAvatar(ctx context.Context, userID, avatarURL string) error
	OnInsertRoomTag(ctx context.Context, userId, roomID, tag string, content []byte) error
	OnDeleteRoomTag(ctx context.Context, userId, roomID, tag string) error
	GetAllProfile() ([]authtypes.Profile, error)
	GetProfileByUserID(ctx context.Context, userID string) (authtypes.Profile, error)

	UpsertUserInfo(ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int) error
	OnUpsertUserInfo(ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int) error
	OnInitUserInfo(ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int) error
	GetAllUserInfo() ([]authtypes.UserInfo, error)
	DeleteUserInfo(ctx context.Context, userID string) error
	OnDeleteUserInfo(ctx context.Context, userID string) error
}
