// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//GetRidsForUser
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package syncapi

import (
	"context"
	"database/sql"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
	_ "github.com/lib/pq"
)

func init() {
	common.Register("syncapi", NewDatabase)
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Same as gomatrixserverlib.ClientEvent but also has the stream position for this event.
type streamEvent struct {
	gomatrixserverlib.ClientEvent
	StreamPosition syncapitypes.StreamPosition
	TransactionID  *roomservertypes.TransactionID
}

// SyncServerDatabase represents a sync server database
type Database struct {
	db              *sql.DB
	idg             *uid.UidGenerator
	topic           string
	underlying      string
	events          outputRoomEventsStatements
	roomstate       currentRoomStateStatements
	keyChange       keyChangeStatements
	stdMsg          stdEventsStatements
	clientData      clientDataStreamStatements
	receiptData     receiptDataStreamStatements
	userReceiptData userReceiptDataStatements
	presenceData    presenceDataStreamStatements
	userTimeLine    userTimeLineStatements
	outputMinStream outputMinStreamStatements
	AsyncSave       bool

	qryDBGauge mon.LabeledGauge
}

// WriteOutputEvents implements OutputRoomEventWriter
func (d *Database) WriteDBEvent(ctx context.Context, update *dbtypes.DBEvent) error {
	span, _ := common.StartSpanFromContext(ctx, d.topic)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, d.topic, d.underlying)
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic,
		&core.TransportPubMsg{
			Keys:    []byte(update.GetEventKey()),
			Obj:     update,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}

func (d *Database) WriteDBEventWithTbl(ctx context.Context, update *dbtypes.DBEvent, tbl string) error {
	span, _ := common.StartSpanFromContext(ctx, d.topic+"_"+tbl)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, d.topic+"_"+tbl, d.underlying)
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic+"_"+tbl,
		&core.TransportPubMsg{
			Keys:    []byte(update.GetEventKey()),
			Obj:     update,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}

type RoomEventVerbose struct {
	Stream int64
	RoomID string
	Event  gomatrixserverlib.ClientEvent
}

// NewSyncServerDatabase creates a new sync server database
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	d := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_syncapi")

	if d.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}

	d.db.SetMaxOpenConns(30)
	d.db.SetMaxIdleConns(30)
	d.db.SetConnMaxLifetime(time.Minute * 3)

	d.topic = topic
	d.underlying = underlying
	d.AsyncSave = useAsync

	schemas := []string{
		d.events.getSchema(),
		d.roomstate.getSchema(),
		d.keyChange.getSchema(),
		d.stdMsg.getSchema(),
		d.clientData.getSchema(),
		d.receiptData.getSchema(),
		d.presenceData.getSchema(),
		d.userReceiptData.getSchema(),
		d.userTimeLine.getSchema(),
		d.outputMinStream.getSchema()}
	for _, sqlStr := range schemas {
		_, err := d.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = d.events.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.roomstate.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.keyChange.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.stdMsg.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.clientData.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.receiptData.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.presenceData.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.userReceiptData.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.userTimeLine.prepare(d.db, d); err != nil {
		return nil, err
	}
	if err := d.outputMinStream.prepare(d.db, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

func (d *Database) SetIDGenerator(idg *uid.UidGenerator) {
	d.idg = idg
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}

// Events lookups a list of event by their event ID.
// Returns a list of events matching the requested IDs found in the database.
// If an event is not found in the database then it will be omitted from the list.
// Returns an error if there was a problem talking with the database.
// Does not include any transaction IDs in the returned events.
func (d *Database) Events(ctx context.Context, eventIDs []string) ([]gomatrixserverlib.ClientEvent, error) {
	streamEvents, err := d.events.selectEvents(ctx, nil, eventIDs)
	if err != nil {
		return nil, err
	}

	// We don't include a device here as we only include transaction IDs in
	// incremental syncs.
	return streamEventsToEvents(nil, streamEvents), nil
}

func (d *Database) StreamEvents(ctx context.Context, eventIDs []string) ([]gomatrixserverlib.ClientEvent, []int64, error) {
	streamEvents, err := d.events.selectEvents(ctx, nil, eventIDs)
	if err != nil {
		return nil, nil, err
	}

	evs := make([]gomatrixserverlib.ClientEvent, len(streamEvents))
	offsets := make([]int64, len(streamEvents))
	for i := 0; i < len(streamEvents); i++ {
		evs[i] = streamEvents[i].ClientEvent
		offsets[i] = int64(streamEvents[i].StreamPosition)

	}

	return evs, offsets, nil
}

// WriteEvent into the database. It is not safe to call this function from multiple goroutines, as it would create races
// when generating the stream position for this event. Returns the sync stream position for the inserted event.
// Returns an error if there was a problem inserting this event.
func (d *Database) WriteEvent(
	ctx context.Context,
	ev *gomatrixserverlib.ClientEvent,
	addStateEvents []gomatrixserverlib.ClientEvent,
	addStateEventIDs, removeStateEventIDs []string,
	transactionID *roomservertypes.TransactionID,
	offset, domainOffset, depth int64, domain string,
	originTs int64,
) error {
	return d.events.insertEvent(ctx, offset, ev, addStateEventIDs, removeStateEventIDs, transactionID, domainOffset, depth, domain, originTs)
}

func (d *Database) InsertEventRaw(
	ctx context.Context,
	id int64, roomId, eventId string, json []byte, addState, removeState []string,
	device, txnID, eventType string, domainOffset, depth int64, domain string,
	originTs int64,
) (err error) {
	return d.events.insertEventRaw(ctx,
		id, roomId, eventId,
		json, addState, removeState,
		device, txnID, eventType, domainOffset, depth, domain, originTs,
	)
}

func (d *Database) UpdateRoomState(
	ctx context.Context,
	event gomatrixserverlib.ClientEvent,
	membership *string,
	streamPos syncapitypes.StreamPosition,
) error {
	return d.roomstate.upsertRoomState(ctx, event, membership, int64(streamPos))
}

func (d *Database) UpdateRoomStateRaw(
	ctx context.Context,
	roomId, eventId string, json []byte,
	eventType, stateKey string,
	membership string, addedAt int64,
) error {
	return d.roomstate.upsertRoomStateRaw(ctx, roomId, eventId, json, eventType, stateKey, membership, addedAt)
}

func (d *Database) UpdateRoomState2(
	ctx context.Context,
	roomId, eventId string, json []byte,
	eventType, stateKey string,
	membership string, addedAt int64,
) error {
	return d.roomstate.upsertRoomStateRaw2(ctx, roomId, eventId, json, eventType, stateKey, membership, addedAt)
}

// GetStateEventsForRoom fetches the state events for a given room.
// Returns an empty slice if no state events could be found for this room.
// Returns an error if there was an issue with the retrieval.
func (d *Database) GetStateEventsForRoom(
	ctx context.Context, roomID string,
) ([]gomatrixserverlib.ClientEvent, []int64, error) {
	return d.roomstate.selectCurrentState(ctx, roomID)
}

func (d *Database) GetStateEventsStreamForRoom(
	ctx context.Context, roomID string,
) ([]gomatrixserverlib.ClientEvent, []int64, error) {
	return d.events.selectRoomStateStream(ctx, roomID, 1)
}

func (d *Database) GetStateEventsStreamForRoomBeforePos(
	ctx context.Context, roomID string, pos int64,
) ([]gomatrixserverlib.ClientEvent, []int64, error) {
	return d.events.selectRoomStateStream(ctx, roomID, pos)
}

func (d *Database) UpdateEvent(
	ctx context.Context, event gomatrixserverlib.ClientEvent, eventID string, eventType string,
	RoomID string,
) error {
	eventJSON, err := json.Marshal(&event)
	if err != nil {
		return err
	}
	return d.events.updateEvent(ctx, eventJSON, eventID, RoomID, eventType)
}

func (d *Database) OnUpdateEvent(ctx context.Context, eventID, roomID string, eventJson []byte, eventType string) error {
	return d.events.updateEventRaw(ctx, eventJson, eventID, roomID, eventType)
}

func (d *Database) SelectEventsByDir(
	ctx context.Context,
	userID, roomID string, dir string, from int64, limit int,
) ([]gomatrixserverlib.ClientEvent, []int64, []int64, error, int64, int64) {
	return d.events.selectEventsByDir(ctx, roomID, dir, from, limit)
}

func (d *Database) SelectEventsByDirRange(
	ctx context.Context,
	userID, roomID string, dir string, from, to int64,
) ([]gomatrixserverlib.ClientEvent, []int64, []int64, error, int64, int64) {
	return d.events.selectEventsByDirRange(ctx, roomID, dir, from, to)
}

func (d *Database) GetRidsForUser(
	ctx context.Context,
	userID string,
) ([]string, []int64, []string, error) {
	return d.roomstate.selectRoomIDsWithMembership(ctx, userID, []string{"join"})
}

func (d *Database) GetFriendShip(
	ctx context.Context,
	roomIDs []string,
) ([]string, error) {
	return d.roomstate.selectRoomJoinedUsers(ctx, roomIDs)
}

func (d *Database) GetInviteRidsForUser(
	ctx context.Context,
	userID string,
) ([]string, []int64, []string, error) {
	return d.roomstate.selectRoomIDsWithMembership(ctx, userID, []string{"invite"})
}


func (d *Database) GetLeaveRidsForUser(
	ctx context.Context,
	userID string,
) ([]string, []int64, []string, error) {
	return d.roomstate.selectRoomIDsWithMembership(ctx, userID, []string{"leave","ban"})
}

// streamEventsToEvents converts streamEvent to Event. If device is non-nil and
// matches the streamevent.transactionID device then the transaction ID gets
// added to the unsigned section of the output event.
func streamEventsToEvents(device *authtypes.Device, in []streamEvent) []gomatrixserverlib.ClientEvent {
	out := make([]gomatrixserverlib.ClientEvent, len(in))
	for i := 0; i < len(in); i++ {
		out[i] = in[i].ClientEvent
		if device != nil && in[i].TransactionID != nil {
			if device.UserID == in[i].Sender && device.ID == in[i].TransactionID.DeviceID {
				unsigned := types.Unsigned{}
				unsigned.TransactionID = in[i].TransactionID.TransactionID
				unsignedBytes, err := json.Marshal(unsigned)
				out[i].Unsigned = unsignedBytes
				if err != nil {
					log.Errorf("Failed to add ev:%s transaction ID to event err:%v", out[i].EventID, err)
				}
			}
		}
	}
	return out
}

/*
send to device messaging implementation
del / maxID / select in range / insert
*/

// InsertStdMessage insert std message
func (d *Database) InsertStdMessage(
	ctx context.Context, stdEvent syncapitypes.StdHolder, targetUID, targetDevice, identifier string, offset int64,
) (err error) {
	return d.stdMsg.insertStdEvent(ctx, offset, stdEvent, targetUID, targetDevice, identifier)
}

func (d *Database) OnInsertStdMessage(
	ctx context.Context, id int64, stdEvent syncapitypes.StdHolder, targetUID, targetDevice, identifier string,
) (pos int64, err error) {
	return d.stdMsg.onInsertStdEvent(ctx, id, stdEvent, targetUID, targetDevice, identifier)
}

func (d *Database) DeleteStdMessage(
	ctx context.Context, id int64, targetUID, targetDevice string,
) error {
	return d.stdMsg.deleteStdEvent(ctx, id, targetUID, targetDevice)
}

func (d *Database) OnDeleteStdMessage(
	ctx context.Context, id int64, targetUID, targetDevice string,
) error {
	return d.stdMsg.onDeleteStdEvent(ctx, id, targetUID, targetDevice)
}

func (d *Database) DeleteMacStdMessage(
	ctx context.Context, identifier, targetUID, targetDevice string,
) error {
	return d.stdMsg.deleteMacStdEvent(ctx, identifier, targetUID, targetDevice)
}

func (d *Database) OnDeleteMacStdMessage(
	ctx context.Context, identifier, targetUID, targetDevice string,
) error {
	return d.stdMsg.onDeleteMacStdEvent(ctx, identifier, targetUID, targetDevice)
}

func (d *Database) DeleteDeviceStdMessage(
	ctx context.Context, targetUID, targetDevice string,
) error {
	return d.stdMsg.deleteDeviceStdEvent(ctx, targetUID, targetDevice)
}

func (d *Database) OnDeleteDeviceStdMessage(
	ctx context.Context, targetUID, targetDevice string,
) error {
	return d.stdMsg.onDeleteDeviceStdEvent(ctx, targetUID, targetDevice)
}

func (d *Database) GetHistoryStdStream(
	ctx context.Context,
	targetUserID,
	targetDeviceID string,
	limit int64,
) ([]types.StdEvent, []int64, error) {
	return d.stdMsg.selectHistoryStream(ctx, targetUserID, targetDeviceID, limit)
}

func (d *Database) GetHistoryEvents(
	ctx context.Context,
	roomid string,
	limit int,
) (events []gomatrixserverlib.ClientEvent, offsets []int64, err error) {
	return d.events.selectHistoryEvents(ctx, roomid, limit)
}

func (d *Database) GetRoomLastOffsets(
	ctx context.Context,
	roomIDs []string,
) (map[string]int64, error) {
	return d.events.selectRoomLastOffsets(ctx, roomIDs)
}

func (d *Database) GetJoinRoomOffsets(
	ctx context.Context,
	eventIDs []string,
)([]int64,[]string,[]string,error){
	return d.events.selectEventsByEvents(ctx, eventIDs)
}

func (d *Database) GetRoomReceiptLastOffsets(
	ctx context.Context,
	roomIDs []string,
) (map[string]int64, error) {
	return d.receiptData.selectRoomLastOffsets(ctx, roomIDs)
}

func (d *Database) GetUserMaxReceiptOffset(
	ctx context.Context,
	userID string,
) (offsets int64, err error) {
	return d.receiptData.selectUserMaxPos(ctx, userID)
}

func (d *Database) UpsertClientDataStream(
	ctx context.Context, userID, roomID, dataType, streamType string,
) (syncapitypes.StreamPosition, error) {
	id, err := d.idg.Next()
	if err != nil {
		return syncapitypes.StreamPosition(-1), err
	}

	pos, err := d.clientData.insertClientDataStream(ctx, id, userID, roomID, dataType, streamType)
	return syncapitypes.StreamPosition(pos), err
}

func (d *Database) OnUpsertClientDataStream(
	ctx context.Context, id int64, userID, roomID, dataType, streamType string,
) (syncapitypes.StreamPosition, error) {
	pos, err := d.clientData.onInsertClientDataStream(ctx, id, userID, roomID, dataType, streamType)
	return syncapitypes.StreamPosition(pos), err
}

func (d *Database) GetHistoryClientDataStream(
	ctx context.Context,
	userID string,
	limit int,
) (streams []types.ActDataStreamUpdate, offset []int64, err error) {
	return d.clientData.selectHistoryStream(ctx, userID, limit)
}

func (d *Database) UpsertReceiptDataStream(
	ctx context.Context, offset, evtOffset int64, roomID, content string,
) error {
	err := d.receiptData.insertReceiptDataStream(ctx, offset, evtOffset, roomID, content)
	return err
}

func (d *Database) OnUpsertReceiptDataStream(
	ctx context.Context, id, evtOffset int64, roomID, content string,
) error {
	err := d.receiptData.onInsertReceiptDataStream(ctx, id, evtOffset, roomID, content)
	return err
}

func (d *Database) GetHistoryReceiptDataStream(
	ctx context.Context, roomID string,
) (streams []types.ReceiptStream, offset []int64, err error) {
	return d.receiptData.selectHistoryStream(ctx, roomID)
}

func (d *Database) UpsertUserReceiptData(
	ctx context.Context, roomID, userID, content string, evtOffset int64,
) error {
	err := d.userReceiptData.insertUserReceiptData(ctx, roomID, userID, content, evtOffset)
	return err
}

func (d *Database) OnUpsertUserReceiptData(
	ctx context.Context, roomID, userID, content string, evtOffset int64,
) (syncapitypes.StreamPosition, error) {
	pos, err := d.userReceiptData.onInsertUserReceiptData(ctx, roomID, userID, content, evtOffset)
	return syncapitypes.StreamPosition(pos), err
}

func (d *Database) GetUserHistoryReceiptData(
	ctx context.Context, roomID, userID string,
) (evtOffset int64, content []byte, err error) {
	return d.userReceiptData.selectHistoryStream(ctx, roomID, userID)
}

func (d *Database) InsertKeyChange(ctx context.Context, changedUserID string, offset int64) error {
	return d.keyChange.insertKeyStream(ctx, offset, changedUserID)
}

func (d *Database) OnInsertKeyChange(ctx context.Context, id int64, changedUserID string) error {
	return d.keyChange.onInsertKeyStream(ctx, id, changedUserID)
}

func (d *Database) GetHistoryKeyChangeStream(
	ctx context.Context,
	users []string,
) (streams []types.KeyChangeStream, offset []int64, err error) {
	return d.keyChange.selectHistoryStream(ctx, users)
}

func (d *Database) UpsertPresenceDataStream(
	ctx context.Context, userID, content string,
) (syncapitypes.StreamPosition, error) {
	id, err := d.idg.Next()
	if err != nil {
		return syncapitypes.StreamPosition(-1), err
	}

	pos, err := d.presenceData.insertPresenceDataStream(ctx, id, userID, content)
	return syncapitypes.StreamPosition(pos), err
}

func (d *Database) OnUpsertPresenceDataStream(
	ctx context.Context, id int64, userID, content string,
) (syncapitypes.StreamPosition, error) {
	pos, err := d.presenceData.onInsertPresenceDataStream(ctx, id, userID, content)
	return syncapitypes.StreamPosition(pos), err
}

func (d *Database) GetHistoryPresenceDataStream(
	ctx context.Context, limit, offset int,
) ([]types.PresenceStream, []int64, error) {
	return d.presenceData.selectHistoryStream(ctx, limit, offset)
}

func (d *Database) GetUserPresenceDataStream(
	ctx context.Context, users []string,
) ([]types.PresenceStream, []int64, error) {
	return d.presenceData.selectUserPresenceStream(ctx, users)
}

func (d *Database) SelectTypeEventForward(
	ctx context.Context,
	typ []string, roomID string,
) (events []gomatrixserverlib.ClientEvent, offsets []int64, err error) {
	return d.events.selectTypeEventForward(ctx, typ, roomID)
}

func (d *Database) UpdateSyncMemberEvent(
	ctx context.Context, userID, oldAvatarUrl, newAvatarUrl string,
) error {
	err := d.roomstate.updateMemberEventAvatar(ctx, userID, oldAvatarUrl, newAvatarUrl)
	if err != nil {
		return err
	}
	return d.events.updateMemberEventAvatar(ctx, userID, oldAvatarUrl, newAvatarUrl)
}

func (d *Database) GetAllSyncRooms() ([]string, error) {
	return d.events.selectAllSyncRooms()
}

func (d *Database) SelectUserTimeLineEvents(
	ctx context.Context,
	userID string,
	id int64,
	limit int,
) (events []syncapitypes.UserTimeLineStream, err error) {
	return d.userTimeLine.selectUserTimeLineEvents(ctx, userID, id, limit)
}

func (d *Database) SelectUserTimeLineMinPos(
	ctx context.Context,
	userID string,
) (int64, error) {
	return d.userTimeLine.selectUserTimeLineMinPos(ctx, userID)
}

func (d *Database) InsertUserTimeLine(
	ctx context.Context,
	id int64,
	roomID string,
	evtNID int64,
	userID,
	roomState string,
	ts,
	eventOffset int64,
) (err error) {
	return d.userTimeLine.insertUserTimeLine(ctx, id, roomID, evtNID, userID, roomState, ts, eventOffset)
}

func (d *Database) OnInsertUserTimeLine(
	ctx context.Context,
	id int64,
	roomID string,
	evtNID int64,
	userID,
	roomState string,
	ts,
	eventOffset int64,
) (err error) {
	return d.userTimeLine.onInsertUserTimeLine(ctx, id, roomID, evtNID, userID, roomState, ts, eventOffset)
}

func (d *Database) SelectUserTimeLineHistory(
	ctx context.Context,
	userID string,
	limit int,
) (events []syncapitypes.UserTimeLineStream, err error) {
	return d.userTimeLine.selectUserTimeLineHistory(ctx, userID, limit)
}

func (d *Database) SelectUserTimeLineOffset(
	ctx context.Context,
	userID string,
	roomOffsets []int64,
) (events []syncapitypes.UserTimeLineStream, err error) {
	return d.userTimeLine.selectUserTimeLineOffset(ctx, userID, roomOffsets)
}

func (d *Database) InsertOutputMinStream(
	ctx context.Context,
	id int64,
	roomID string,
) error {
	return d.outputMinStream.insertOutputMinStream(ctx, id, roomID)
}

func (d *Database) OnInsertOutputMinStream(
	ctx context.Context,
	id int64,
	roomID string,
) error {
	return d.outputMinStream.onInsertOutputMinStream(ctx, id, roomID)
}

func (d *Database) SelectOutputMinStream(
	ctx context.Context,
	roomID string,
) (int64, error) {
	return d.outputMinStream.selectOutputMinStream(ctx, roomID)
}

func (d *Database) SelectDomainMaxOffset(
	ctx context.Context,
	roomID string,
) ([]string, []int64, error) {
	return d.events.selectDomainMaxOffset(ctx, roomID)
}

func (d *Database) UpdateSyncEvent(ctx context.Context, domainOffset, originTs int64, domain, roomID, eventID string) error {
	return d.events.UpdateSyncEvent(ctx, domainOffset, originTs, domain, roomID, eventID)
}

func (d *Database) OnUpdateSyncEvent(ctx context.Context, domainOffset, originTs int64, domain, roomID, eventID string) error {
	return d.events.onUpdateSyncEvent(ctx, domainOffset, originTs, domain, roomID, eventID)
}

func (d *Database) GetSyncEvents(ctx context.Context, start, end int64, limit, offset int64) ([][]byte, error) {
	return d.events.selectSyncEvents(ctx, start, end, limit, offset)
}

func (d *Database) GetSyncMsgEventsMigration(ctx context.Context, limit, offset int64) ([]int64, []string, [][]byte, error) {
	return d.events.selectSyncMsgEventsMigration(ctx, limit, offset)
}
func (d *Database) GetSyncMsgEventsTotalMigration(ctx context.Context) (int, int64, error) {
	return d.events.selectSyncMsgEventsTotalMigration(ctx)
}
func (d *Database) UpdateSyncMsgEventMigration(ctx context.Context, id int64, EncryptedEventBytes []byte) error {
	return d.events.updateSyncMsgEventMigration(ctx, id, EncryptedEventBytes)
}

func (d *Database) GetEventRaw(ctx context.Context, eventID string) (int64, []byte, error) {
	return d.events.selectEventRaw(ctx, eventID)
}
func (d *Database) GetMsgEventsByRoomIDMigration(ctx context.Context, roomID string) ([]int64, []string, [][]byte, error) {
	return d.events.selectEventsByRoomIDMigration(ctx, roomID)
}

func (d *Database) GetRoomStateTotal(ctx context.Context) (int, error) {
	return d.roomstate.selectRoomStateTotal(ctx)
}
func (d *Database) UpdateRoomStateWithEventID(ctx context.Context, eventID string, eventBytes []byte) error {
	return d.roomstate.updateRoomStateWithEventID(ctx, eventID, eventBytes)
}
func (d *Database) GetRoomStateWithLimit(ctx context.Context, limit, offset int64) ([]string, [][]byte, error) {
	return d.roomstate.selectRoomStateWithLimit(ctx, limit, offset)
}
func (d *Database) GetRoomStateByEventID(ctx context.Context, eventID string) ([]byte, error) {
	return d.roomstate.selectRoomStateByEventID(ctx, eventID)
}
