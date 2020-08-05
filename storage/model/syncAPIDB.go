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

	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type SyncAPIDatabase interface {
	//NewDatabase(driver, createAddr, address, topic string, useAsync bool) (interface{}, error)
	GetDB() *sql.DB

	SetIDGenerator(idg *uid.UidGenerator)

	WriteDBEvent(ctx context.Context, update *dbtypes.DBEvent) error

	Events(ctx context.Context, eventIDs []string) ([]gomatrixserverlib.ClientEvent, error)
	StreamEvents(ctx context.Context, eventIDs []string) ([]gomatrixserverlib.ClientEvent, []int64, error)
	WriteEvent(
		ctx context.Context,
		ev *gomatrixserverlib.ClientEvent,
		addStateEvents []gomatrixserverlib.ClientEvent,
		addStateEventIDs, removeStateEventIDs []string,
		transactionID *roomservertypes.TransactionID,
		offset int64, domainOffset, depth int64, domain string,
		originTs int64,
	) (returnErr error)
	InsertEventRaw(
		ctx context.Context,
		id int64, roomId, eventId string, json []byte, addState, removeState []string,
		device, txnID, eventType string, domainOffset, depth int64, domain string,
		originTs int64,
	) (err error)
	UpdateRoomState(
		ctx context.Context,
		event gomatrixserverlib.ClientEvent,
		membership *string,
		streamPos syncapitypes.StreamPosition,
	) error
	UpdateRoomStateRaw(
		ctx context.Context,
		roomId, eventId string, json []byte,
		eventType, stateKey string,
		membership string, addedAt int64,
	) error
	UpdateRoomState2(
		ctx context.Context,
		roomId, eventId string, json []byte,
		eventType, stateKey string,
		membership string, addedAt int64,
	) error
	GetStateEventsForRoom(
		ctx context.Context, roomID string,
	) (stateEvents []gomatrixserverlib.ClientEvent, offset []int64, err error)
	GetStateEventsStreamForRoom(
		ctx context.Context, roomID string,
	) (stateEvents []gomatrixserverlib.ClientEvent, offset []int64, err error)
	GetStateEventsStreamForRoomBeforePos(
		ctx context.Context, roomID string, pos int64,
	) (stateEvents []gomatrixserverlib.ClientEvent, offset []int64, err error)
	UpdateEvent(
		ctx context.Context, event gomatrixserverlib.ClientEvent, eventID string, eventType string,
		RoomID string,
	) error
	OnUpdateEvent(ctx context.Context, eventID, roomID string, eventJson []byte, eventType string) error
	SelectEventsByDir(
		ctx context.Context,
		userID, roomID string, dir string, from int64, limit int,
	) ([]gomatrixserverlib.ClientEvent, []int64, []int64, error, int64, int64)
	SelectEventsByDirRange(
		ctx context.Context,
		userID, roomID string, dir string, from, to int64,
	) ([]gomatrixserverlib.ClientEvent, []int64, []int64, error, int64, int64)
	GetRidsForUser(
		ctx context.Context,
		userID string,
	) (res []string, offsets []int64, events []string, err error)
	GetFriendShip(
		ctx context.Context,
		roomIDs []string,
	) (friends []string, err error)
	GetInviteRidsForUser(
		ctx context.Context,
		userID string,
	) (rids []string, offsets []int64, events []string, err error)
	GetLeaveRidsForUser(
		ctx context.Context,
		userID string,
	) (rids []string, offsets []int64, events []string, err error)
	InsertStdMessage(
		ctx context.Context, stdEvent syncapitypes.StdHolder, targetUID, targetDevice, identifier string, offset int64,
	) (err error)
	OnInsertStdMessage(
		ctx context.Context, id int64, stdEvent syncapitypes.StdHolder, targetUID, targetDevice, identifier string,
	) (pos int64, err error)
	DeleteStdMessage(
		ctx context.Context, id int64, targetUID, targetDevice string,
	) error
	OnDeleteStdMessage(
		ctx context.Context, id int64, targetUID, targetDevice string,
	) error
	DeleteMacStdMessage(
		ctx context.Context, identifier, targetUID, targetDevice string,
	) error
	OnDeleteMacStdMessage(
		ctx context.Context, identifier, targetUID, targetDevice string,
	) error
	DeleteDeviceStdMessage(
		ctx context.Context, targetUID, targetDevice string,
	) error
	OnDeleteDeviceStdMessage(
		ctx context.Context, targetUID, targetDevice string,
	) error
	GetHistoryStdStream(
		ctx context.Context,
		targetUserID,
		targetDeviceID string,
		limit int64,
	) ([]types.StdEvent, []int64, error)
	GetHistoryEvents(
		ctx context.Context,
		roomid string,
		limit int,
	) (events []gomatrixserverlib.ClientEvent, offsets []int64, err error)
	GetRoomLastOffsets(
		ctx context.Context,
		roomIDs []string,
	) (map[string]int64, error)
	GetJoinRoomOffsets(
		ctx context.Context,
		eventIDs []string,
	)([]int64,[]string,[]string,error)
	GetRoomReceiptLastOffsets(
		ctx context.Context,
		roomIDs []string,
	) (map[string]int64, error)
	GetUserMaxReceiptOffset(
		ctx context.Context,
		userID string,
	) (offsets int64, err error)
	UpsertClientDataStream(
		ctx context.Context, userID, roomID, dataType, streamType string,
	) (syncapitypes.StreamPosition, error)
	OnUpsertClientDataStream(
		ctx context.Context, id int64, userID, roomID, dataType, streamType string,
	) (syncapitypes.StreamPosition, error)
	GetHistoryClientDataStream(
		ctx context.Context,
		userID string,
		limit int,
	) (streams []types.ActDataStreamUpdate, offset []int64, err error)
	UpsertReceiptDataStream(
		ctx context.Context, offset, evtOffset int64, roomID, content string,
	) error
	OnUpsertReceiptDataStream(
		ctx context.Context, id, evtOffset int64, roomID, content string,
	) error
	GetHistoryReceiptDataStream(
		ctx context.Context, roomID string,
	) (streams []types.ReceiptStream, offset []int64, err error)
	UpsertUserReceiptData(
		ctx context.Context, roomID, userID, content string, evtOffset int64,
	) error
	OnUpsertUserReceiptData(
		ctx context.Context, roomID, userID, content string, evtOffset int64,
	) (syncapitypes.StreamPosition, error)
	GetUserHistoryReceiptData(
		ctx context.Context, roomID, userID string,
	) (evtOffset int64, content []byte, err error)
	InsertKeyChange(ctx context.Context, changedUserID string, offset int64) error
	OnInsertKeyChange(ctx context.Context, id int64, changedUserID string) error
	GetHistoryKeyChangeStream(
		ctx context.Context,
		users []string,
	) (streams []types.KeyChangeStream, offset []int64, err error)
	UpsertPresenceDataStream(
		ctx context.Context, userID, content string,
	) (syncapitypes.StreamPosition, error)
	OnUpsertPresenceDataStream(
		ctx context.Context, id int64, userID, content string,
	) (syncapitypes.StreamPosition, error)
	GetHistoryPresenceDataStream(
		ctx context.Context, limit, offset int,
	) ([]types.PresenceStream, []int64, error)
	GetUserPresenceDataStream(
		ctx context.Context, users []string,
	) ([]types.PresenceStream, []int64, error)
	SelectTypeEventForward(
		ctx context.Context, typ []string, roomID string,
	) (events []gomatrixserverlib.ClientEvent, offsets []int64, err error)
	UpdateSyncMemberEvent(
		ctx context.Context, userID, oldAvatarUrl, newAvatarUrl string,
	) error
	GetAllSyncRooms() ([]string, error)
	SelectUserTimeLineEvents(
		ctx context.Context,
		userID string,
		id int64,
		limit int,
	) (events []syncapitypes.UserTimeLineStream, err error)
	SelectUserTimeLineMinPos(
		ctx context.Context,
		userID string,
	) (int64, error)
	InsertUserTimeLine(
		ctx context.Context,
		id int64,
		roomID string,
		evtNID int64,
		userID,
		roomState string,
		ts,
		eventOffset int64,
	) (err error)
	OnInsertUserTimeLine(
		ctx context.Context,
		id int64,
		roomID string,
		evtNID int64,
		userID,
		roomState string,
		ts,
		eventOffset int64,
	) (err error)
	SelectUserTimeLineHistory(
		ctx context.Context,
		userID string,
		limit int,
	) (events []syncapitypes.UserTimeLineStream, err error)
	SelectUserTimeLineOffset(
		ctx context.Context,
		userID string,
		roomOffsets []int64,
	) (events []syncapitypes.UserTimeLineStream, err error)
	InsertOutputMinStream(
		ctx context.Context,
		id int64,
		roomID string,
	) error
	OnInsertOutputMinStream(
		ctx context.Context,
		id int64,
		roomID string,
	) error
	SelectOutputMinStream(
		ctx context.Context,
		roomID string,
	) (int64, error)
	SelectDomainMaxOffset(
		ctx context.Context,
		roomID string,
	) ([]string, []int64, error)
	UpdateSyncEvent(ctx context.Context, domainOffset, originTs int64, domain, roomID, eventID string) error
	OnUpdateSyncEvent(ctx context.Context, domainOffset, originTs int64, domain, roomID, eventID string) error
	GetSyncEvents(ctx context.Context, start, end int64, limit, offset int64) ([][]byte, error)
	GetSyncMsgEventsMigration(ctx context.Context, limit, offset int64) ([]int64, []string, [][]byte, error)
	GetSyncMsgEventsTotalMigration(ctx context.Context) (int, int64, error)
	UpdateSyncMsgEventMigration(ctx context.Context, id int64, EncryptedEventBytes []byte) error
	GetEventRaw(ctx context.Context, eventID string) (int64, []byte, error)
	GetMsgEventsByRoomIDMigration(ctx context.Context, roomID string) ([]int64, []string, [][]byte, error)

	GetRoomStateWithLimit(ctx context.Context, limit, offset int64) ([]string, [][]byte, error)
	GetRoomStateTotal(ctx context.Context) (int, error)
	UpdateRoomStateWithEventID(ctx context.Context, eventID string, eventBytes []byte) error
	GetRoomStateByEventID(ctx context.Context, eventID string) ([]byte, error)
}
