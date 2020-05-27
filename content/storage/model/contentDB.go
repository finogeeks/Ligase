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

import "context"

type MediaDownloadInfo struct {
	RoomID  string
	EventID string
	Event   string
}

type ContentDatabase interface {
	InsertMediaDownload(ctx context.Context, roomID, eventID, event string) error
	UpdateMediaDownload(ctx context.Context, roomID, eventID string, finished bool) error
	SelectMediaDownload(ctx context.Context) (roomIDs, eventIDs, events []string, err error)
}
