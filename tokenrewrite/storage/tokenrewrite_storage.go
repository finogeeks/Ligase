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

package storage

import (
	"context"
	"database/sql"
	"time"
)

type TokenRewriteDataBase struct {
	db      *sql.DB
	users   usersStatements
	devices devicesStatements
	tokens  tokensStatements
}

func NewTokenRewriteDataBase(dataSourceName string) (*TokenRewriteDataBase, error) {
	var d TokenRewriteDataBase
	var err error
	if d.db, err = sql.Open("postgres", dataSourceName); err != nil {
		return nil, err
	}
	d.db.SetMaxOpenConns(30)
	d.db.SetMaxIdleConns(30)
	d.db.SetConnMaxLifetime(time.Minute * 3)
	if err = d.users.prepare(d.db); err != nil {
		return nil, err
	}
	if err = d.devices.prepare(d.db); err != nil {
		return nil, err
	}
	if err = d.tokens.prepare(d.db); err != nil {
		return nil, err
	}
	return &d, nil
}

func (d *TokenRewriteDataBase) UpsertUser(
	ctx context.Context, userID string,
) error {
	return d.users.upsertUser(ctx, userID)
}

func (d *TokenRewriteDataBase) UpsertDevice(
	ctx context.Context, userID, deviceID, displayName string,
) error {
	return d.devices.upsertDevice(ctx, userID, deviceID, displayName)
}

func (d *TokenRewriteDataBase) UpsertToken(
	ctx context.Context, id int64, userID, deviceID, token string,
) error {
	return d.tokens.upsertToken(ctx, id, userID, deviceID, token)
}
