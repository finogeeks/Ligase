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
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/lib/pq"
)

const stateSnapshotSchema = `
-- The state of a room before an event.
-- Stored as a list of state_block entries stored in a separate table.
-- The actual state is constructed by combining all the state_block entries
-- referenced by state_block_nids together. If the same state key tuple appears
-- multiple times then the entry from the later state_block clobbers the earlier
-- entries.
-- This encoding format allows us to implement a delta encoding which is useful
-- because room state tends to accumulate small changes over time. Although if
-- the list of deltas becomes too long it becomes more efficient to encode
-- the full state under single state_block_nid.
CREATE TABLE IF NOT EXISTS roomserver_state_snapshots (
    -- Local numeric ID for the state.
    state_snapshot_nid bigint PRIMARY KEY ,
    -- Local numeric ID of the room this state is for.
    -- Unused in normal operation, but useful for background work or ad-hoc debugging.
    room_nid bigint NOT NULL,
    -- List of state_block_nids, stored sorted by state_block_nid.
    state_block_nids bigint[] NOT NULL
);
`
const insertStateSQL = "" +
	"INSERT INTO roomserver_state_snapshots (state_snapshot_nid, room_nid, state_block_nids)" +
	" VALUES ($1, $2, $3) ON CONFLICT DO NOTHING"

const selectStateSQL = "" +
	"SELECT room_nid, state_block_nids FROM roomserver_state_snapshots WHERE state_snapshot_nid = $1"

//var state_snap_cache = sync.Map{} //make(map[types.StateSnapshotNID]*stateSnapItem)

type stateSnapshotStatements struct {
	db              *Database
	insertStateStmt *sql.Stmt
	selectStateStmt *sql.Stmt
}

func (s *stateSnapshotStatements) getSchema() string {
	return stateSnapshotSchema
}

func (s *stateSnapshotStatements) prepare(db *sql.DB, d *Database) (err error) {
	s.db = d

	return statementList{
		{&s.insertStateStmt, insertStateSQL},
		{&s.selectStateStmt, selectStateSQL},
	}.prepare(db)
}

func (s *stateSnapshotStatements) insertState(
	ctx context.Context, roomNID int64, stateBlockNIDs []int64,
) (int64, error) {
	id, err := s.db.idg.Next()
	if err != nil {
		return 0, err
	}

	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_ROOM_DB_EVENT
		update.Key = dbtypes.EventStateSnapInsertKey
		update.RoomDBEvents.EventStateSnapInsert = &dbtypes.EventStateSnapInsert{
			StateSnapNid:   id,
			RoomNid:        roomNID,
			StateBlockNids: stateBlockNIDs,
		}
		update.SetUid(id)
		s.db.WriteDBEventWithTbl(&update, "roomserver_state_snapshots")

		return id, nil
	}

	return id, s.insertStateRaw(ctx, id, roomNID, stateBlockNIDs)
}

func (s *stateSnapshotStatements) insertStateRaw(
	ctx context.Context, stateNID int64, roomNID int64, nids []int64,
) (err error) {
	_, err = s.insertStateStmt.ExecContext(ctx, stateNID, roomNID, pq.Int64Array(nids))
	if err != nil {
		log.Errorf("--------insertStateRaw err snapid %d roomnid %d err:%v", stateNID, roomNID, err)
	}
	return
}

func (s *stateSnapshotStatements) selectState(
	ctx context.Context, snapshotNID int64,
) (int64, []int64, error) {
	var roomNID int64
	var stateBlockNIDs pq.Int64Array
	err := s.selectStateStmt.QueryRowContext(ctx, snapshotNID).Scan(&roomNID, &stateBlockNIDs)
	if err != nil {
		return 0, nil, err
	}
	return roomNID, stateBlockNIDs, nil
}
