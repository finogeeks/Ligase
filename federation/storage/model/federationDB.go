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
)

type BackfillRecord struct {
	RoomID          string
	Domain          string
	Limit           int
	States          string
	Depth           int64
	Finished        bool
	FinishedDomains string
}

type GetMissingEventsInfo struct {
	RoomID       string
	EventID      string
	Limit        int
	DomainOffset int64
}

type FederationDatabase interface {
	SelectJoinedRooms(ctx context.Context, roomID string) (string, string, error)
	UpdateJoinedRoomsRecvOffset(ctx context.Context, roomID string, recvOffset string) error
	InsertJoinedRooms(ctx context.Context, roomID, eventID string) error

	SelectAllSendRecord(ctx context.Context) ([]string, []string, []string, []int32, []int32, []int64, int, error)
	SelectPendingSendRecord(ctx context.Context) ([]string, []string, []string, []int32, []int32, []int64, int, error)
	SelectSendRecord(ctx context.Context, roomID, domain string) (eventID string, sendTimes, pendingSize int32, domainOffset int64, err error)
	InsertSendRecord(ctx context.Context, roomID, domain string, domainOffset int64) error
	UpdateSendRecordPendingSize(ctx context.Context, roomID, domain string, size int32, domainOffset int64) error
	UpdateSendRecordPendingSizeAndEventID(ctx context.Context, room, domain string, size int32, eventID string, domainOffset int64) error

	SelectAllBackfillRecord(ctx context.Context) ([]BackfillRecord, error)
	SelectBackfillRecord(ctx context.Context, roomID string) (BackfillRecord, error)
	InsertBackfillRecord(ctx context.Context, rec BackfillRecord) error
	UpdateBackfillRecordDomainsInfo(ctx context.Context, roomID string, depth int64, finished bool, finishedDomains string, states string) error

	InsertMissingEvents(ctx context.Context, roomID, eventID string, amount int) error
	UpdateMissingEvents(ctx context.Context, roomID, eventID string, finished bool) error
	SelectMissingEvents(ctx context.Context) (roomIDs, eventIDs []string, amounts []int, err error)
}
