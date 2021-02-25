// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package keydb

import (
	"context"
	"database/sql"
	"time"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

// A Database implements gomatrixserverlib.KeyDatabase and is used to store
// the public keys for other matrix servers.
type Database struct {
	statements serverKeyStatements
}

// NewDatabase prepares a new key database.
// It creates the necessary tables if they don't already exist.
// It prepares all the SQL statements that it will use.
// Returns an error if there was a problem talking to the database.
func NewDatabase(driver, address string) (*Database, error) {
	db, err := sql.Open(driver, address)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(30)
	db.SetConnMaxLifetime(time.Minute * 3)

	d := &Database{}
	if _, err = db.Exec(d.statements.getSchema()); err != nil {
		return nil, err
	}

	if err = d.statements.prepare(db); err != nil {
		return nil, err
	}
	return d, nil
}

// FetcherName implements KeyFetcher
func (d Database) FetcherName() string {
	return "KeyDatabase"
}

// FetchKeys implements gomatrixserverlib.KeyDatabase
func (d *Database) FetchKeys(
	ctx context.Context,
	requests map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.Timestamp,
) (map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult, error) {
	return d.statements.bulkSelectServerKeys(ctx, requests)
}

// StoreKeys implements gomatrixserverlib.KeyDatabase
func (d *Database) StoreKeys(
	ctx context.Context,
	keyMap map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult,
) error {
	// TODO: Inserting all the keys within a single transaction may
	// be more efficient since the transaction overhead can be quite
	// high for a single insert statement.
	var lastErr error
	for request, keys := range keyMap {
		if err := d.statements.upsertServerKeys(ctx, request, keys); err != nil {
			// Rather than returning immediately on error we try to insert the
			// remaining keys.
			// Since we are inserting the keys outside of a transaction it is
			// possible for some of the inserts to succeed even though some
			// of the inserts have failed.
			// Ensuring that we always insert all the keys we can means that
			// this behaviour won't depend on the iteration order of the map.
			lastErr = err
		}
	}
	return lastErr
}
