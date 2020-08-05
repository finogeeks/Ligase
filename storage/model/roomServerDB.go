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
	"database/sql"

	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type RoomServerDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)
	GetDB() *sql.DB

	SetIDGenerator(idg *uid.UidGenerator)

	WriteDBEvent(ctx context.Context, update *dbtypes.DBEvent) error

	RecoverCache()

	StoreEvent(
		ctx context.Context, event *gomatrixserverlib.Event, roomNID, stateNID int64,
		refId string, refHash []byte,
	) error
	AssignRoomNID(
		ctx context.Context, roomID string,
	) (int64, error)
	RoomInfo(
		ctx context.Context, roomID string,
	) (int64, int64, int64, error)
	EventNIDs(
		ctx context.Context, eventIDs []string,
	) (map[string]int64, error)
	EventNID(
		ctx context.Context, eventID string,
	) (int64, error)
	Events(
		ctx context.Context, eventNIDs []int64,
	) ([]*gomatrixserverlib.Event, []int64, error)
	AddState(
		ctx context.Context,
		roomNID int64,
		StateBlockNIDs []int64,
	) (int64, error)
	SelectState(ctx context.Context, snapshotNID int64) (int64, []int64, error)
	RoomNID(ctx context.Context, roomID string) (roomservertypes.RoomNID, error)
	SetRoomAlias(ctx context.Context, alias string, roomID string) error
	GetRoomIDFromAlias(ctx context.Context, alias string) (string, error)
	GetAliasesFromRoomID(ctx context.Context, roomID string) ([]string, error)
	RemoveRoomAlias(ctx context.Context, alias string) error
	SetToInvite(
		ctx context.Context, roomNID int64,
		targetUser, senderUserID, eventID string, json []byte,
		eventNID int64, pre, roomID string,
	) error
	SetToJoin(
		ctx context.Context, roomNID int64,
		targetUser, senderUserID string,
		eventNID int64, pre, roomID string,
	) error
	SetToLeave(
		ctx context.Context, roomNID int64,
		targetUser, senderUserID string,
		eventNID int64, pre, roomID string,
	) error
	SetToForget(
		ctx context.Context, roomNID int64,
		targetUser string, eventNID int64,
		pre, roomID string,
	) error
	GetRoomStates(
		ctx context.Context, roomID string,
	) ([]*gomatrixserverlib.Event, []int64, error)
	EventsCount(ctx context.Context) (count int, err error)
	FixCorruptRooms()
	InsertEventJSON(ctx context.Context, eventNID int64, eventJSON []byte, eventType string) error
	InsertEvent(ctx context.Context, eventNID int64,
		roomNID int64, eventType string, eventStateKey string,
		eventID string, referenceSHA256 []byte, authEventNIDs []int64,
		depth int64, stateNID int64, refId string, refHash []byte,
		offset int64, domain string) error
	InsertInvite(ctx context.Context, eventId string, roomNid int64, target, sender string, content []byte) error
	InviteUpdate(ctx context.Context, roomNid int64, target string) error
	MembershipInsert(ctx context.Context, roomNid int64, target, roomID string, membership, eventNID int64) error
	MembershipUpdate(ctx context.Context, roomNid int64, target, sender string, membership, eventNID, version int64) error
	MembershipForgetUpdate(ctx context.Context, roomNid int64, target string, forgetNid int64) error
	InsertRoomNID(ctx context.Context, roomNID int64, roomID string) error
	UpdateLatestEventNIDs(
		ctx context.Context, roomNID int64, eventNIDs []int64,
		lastEventSentNID int64, stateSnapshotNID int64, version int64, depth int64,
	) error
	InsertStateRaw(
		ctx context.Context, stateNID int64, roomNID int64, nids []int64,
	) (err error)
	RoomExists(
		ctx context.Context, roomId string,
	) (exists bool, err error)
	LatestRoomEvent(
		ctx context.Context, roomId string,
	) (eventId string, err error)
	EventCountForRoom(
		ctx context.Context, roomId string,
	) (count int, err error)

	EventCountAll(
		ctx context.Context,
	) (count int, err error)
	SetLatestEvents(
		ctx context.Context, roomNID int64, lastEventNIDSent, currentStateSnapshotNID, depth int64,
	) error
	LoadFilterData(ctx context.Context, key string, f *filter.Filter) bool
	GetUserRooms(
		ctx context.Context, uid string,
	) ([]string, []string, []string, error)
	AliaseInsertRaw(ctx context.Context, aliase, roomID string) error
	AliaseDeleteRaw(ctx context.Context, aliase string) error
	GetAllRooms(ctx context.Context, limit, offset int) ([]roomservertypes.RoomNIDs, error)
	GetRoomEvents(ctx context.Context, roomNID int64) ([][]byte, error)
	GetRoomEventsWithLimit(ctx context.Context, roomNID int64, limit, offset int) ([]int64, [][]byte, error)

	SaveRoomDomainsOffset(ctx context.Context, room_nid int64, domain string, offset int64) error
	RoomDomainsInsertRaw(ctx context.Context, room_nid int64, domain string, offset int64) error
	GetRoomDomainsOffset(ctx context.Context, room_nid int64) ([]string, []int64, error)
	BackFillNids(ctx context.Context, roomNID int64, domain string, eventNid int64, limit int, dir string) ([]int64, error)
	SelectEventNidForBackfill(ctx context.Context, roomNID int64, domain string) (int64, error)
	SelectRoomEventsByDomainOffset(ctx context.Context, roomNID int64, domain string, domainOffset int64, limit int) ([]int64, error)
	UpdateRoomEvent(ctx context.Context, eventNID, roomNID, depth, domainOffset int64, domain string) error
	OnUpdateRoomEvent(ctx context.Context, eventNID, roomNID, depth, domainOffset int64, domain string) error
	UpdateRoomDepth(ctx context.Context, depth, roomNid int64) error
	OnUpdateRoomDepth(ctx context.Context, depth, roomNid int64) error
	SettingsInsertRaw(ctx context.Context, settingKey string, val string) error
	SaveSettings(ctx context.Context, settingKey string, val string) error
	SelectSettingKey(ctx context.Context, settingKey string) (string, error)
	SelectRoomMaxDomainOffsets(ctx context.Context, roomNID int64) (domains, eventIDs []string, offsets []int64, err error)
	SelectEventStateSnapshotNID(ctx context.Context, eventID string) (int64, error)
	SelectRoomStateNIDByStateBlockNID(ctx context.Context, roomNID int64, stateBlockNID int64) ([]int64, []string, []string, []string, error)

	GetMsgEventsMigration(ctx context.Context, limit, offset int64) ([]int64, [][]byte, error)
	GetMsgEventsTotalMigration(ctx context.Context) (int, int64, error)
	UpdateMsgEventMigration(ctx context.Context, id int64, EncryptedEventBytes []byte) error
	GetRoomEventByNID(ctx context.Context, eventNID int64) ([]byte, error)
}
