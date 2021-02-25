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

package roomserver

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/model/dbtypes"
)

const roomdomainsSchema = `
CREATE TABLE IF NOT EXISTS roomserver_room_domains (
    room_nid bigint NOT NULL ,
    domain TEXT NOT NULL ,
    offsets bigint NOT NULL DEFAULT 0,
	CONSTRAINT roomserver_room_domains_unique UNIQUE (room_nid, domain)
);
`

const upsertRoomDomainsSQL = "" +
	"INSERT INTO roomserver_room_domains (room_nid, domain, offsets) VALUES ($1, $2, $3)" +
	" ON CONFLICT ON CONSTRAINT roomserver_room_domains_unique" +
	" DO UPDATE SET offsets = $3"

const selectRoomDomainsSQL = "" +
	"SELECT domain, offsets FROM roomserver_room_domains WHERE room_nid = $1"

type roomDomainsStatements struct {
	db                    *Database
	upsertRoomDomainsStmt *sql.Stmt
	selectRoomDomainsStmt *sql.Stmt
}

func (s *roomDomainsStatements) getSchema() string {
	return roomdomainsSchema
}

func (s *roomDomainsStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d
	if prepare_with_create {
		_, err = db.Exec(roomdomainsSchema)
		if err != nil {
			return
		}
	}

	return statementList{
		{&s.upsertRoomDomainsStmt, upsertRoomDomainsSQL},
		{&s.selectRoomDomainsStmt, selectRoomDomainsSQL},
	}.prepare(db)
}

func (s *roomDomainsStatements) insertRoomDomains(
	ctx context.Context, room_nid int64, domain string, offset int64,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.RoomDomainInsertKey
		update.RoomDBEvents.RoomDomainInsert = &dbtypes.RoomDomainInsert{
			RoomNid: room_nid,
			Domain:  domain,
			Offset:  offset,
		}
		update.SetUid(room_nid)
		s.db.WriteDBEventWithTbl(&update, "roomserver_room_domains")
		return nil
	}

	return s.insertRoomDomainsRaw(ctx, room_nid, domain, offset)
}

func (s *roomDomainsStatements) insertRoomDomainsRaw(
	ctx context.Context, room_nid int64, domain string, offset int64,
) error {
	_, err := s.upsertRoomDomainsStmt.ExecContext(ctx, room_nid, domain, offset)
	return err
}

func (s *roomDomainsStatements) selectRoomDomains(
	ctx context.Context, room_nid int64,
) ([]string, []int64, error) {
	rows, err := s.selectRoomDomainsStmt.QueryContext(ctx, room_nid)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() // nolint: errcheck
	var domains []string
	var offsets []int64
	var domain string
	var offset int64
	for rows.Next() {
		if err = rows.Scan(&domain, &offset); err != nil {
			return nil, nil, err
		}
		domains = append(domains, domain)
		offsets = append(offsets, offset)
	}
	return domains, offsets, err
}
