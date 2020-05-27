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

package content

import (
	"context"
	"database/sql"
)

const mediaDownloadSchema = `
CREATE TABLE IF NOT EXISTS content_media_download (
    room_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
	event TEXT NOT NULL,
	finished bool NOT NULL,
    CONSTRAINT content_media_download_unique UNIQUE (room_id, event_id)
);
`

const insertMediaDownloadSQL = "" +
	"INSERT INTO content_media_download (room_id, event_id, event, finished)" +
	" VALUES ($1, $2, $3, FALSE)"

const updateMediaDownloadSQL = "" +
	"UPDATE content_media_download SET finished = $3 WHERE room_id = $1 AND event_id = $2"

const selectMediaDownloadSQL = "" +
	"SELECT room_id, event_id, event FROM content_media_download WHERE finished = FALSE"

type mediaDownloadStatements struct {
	insertMediaDownloadStmt *sql.Stmt
	updateMediaDownloadStmt *sql.Stmt
	selectMediaDownloadStmt *sql.Stmt
}

func (s *mediaDownloadStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(mediaDownloadSchema)
	if err != nil {
		return err
	}
	if s.insertMediaDownloadStmt, err = db.Prepare(insertMediaDownloadSQL); err != nil {
		return
	}
	if s.updateMediaDownloadStmt, err = db.Prepare(updateMediaDownloadSQL); err != nil {
		return
	}
	if s.selectMediaDownloadStmt, err = db.Prepare(selectMediaDownloadSQL); err != nil {
		return
	}
	return
}

func (s *mediaDownloadStatements) insertMediaDownload(
	ctx context.Context,
	roomID, eventID, event string,
) error {
	_, err := s.insertMediaDownloadStmt.ExecContext(ctx, roomID, eventID, event)
	return err
}

func (s *mediaDownloadStatements) updateMediaDownload(ctx context.Context, roomID, eventID string, finished bool) error {
	_, err := s.updateMediaDownloadStmt.ExecContext(ctx, roomID, eventID, finished)
	return err
}

func (s *mediaDownloadStatements) selectMediaDownload(
	ctx context.Context,
) (roomIDs, eventIDs, events []string, err error) {
	var roomID string
	var eventID string
	var event string
	rows, err := s.selectMediaDownloadStmt.QueryContext(ctx)
	if err != nil {
		return
	}
	for rows.Next() {
		err = rows.Scan(&roomID, &eventID, &event)
		if err != nil {
			return
		}
		roomIDs = append(roomIDs, roomID)
		eventIDs = append(eventIDs, eventID)
		events = append(events, event)
	}
	return
}
