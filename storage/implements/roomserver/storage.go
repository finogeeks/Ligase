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
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"

	// Import the postgres database driver.
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	_ "github.com/lib/pq"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	common.Register("roomserver", NewDatabase)
}

// A Database is used to store room events and stream offsets.
type Database struct {
	statements statements
	db         *sql.DB
	topic      string
	underlying string
	idg        *uid.UidGenerator
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

var prepare_with_create = true

// Open a postgres database.
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	d := new(Database)
	var err error

	d.topic = topic
	d.underlying = underlying
	d.AsyncSave = useAsync

	common.CreateDatabase(driver, createAddr, "dendrite_roomserver")
	if d.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	d.db.SetMaxOpenConns(30)
	d.db.SetMaxIdleConns(30)
	d.db.SetConnMaxLifetime(time.Minute * 3)

	schemas := []string{
		d.statements.roomStatements.getSchema(),
		d.statements.eventStatements.getSchema(),
		d.statements.eventJSONStatements.getSchema(),
		d.statements.stateSnapshotStatements.getSchema(),
		d.statements.roomAliasesStatements.getSchema(),
		d.statements.inviteStatements.getSchema(),
		d.statements.membershipStatements.getSchema(),
		d.statements.roomDomainsStatements.getSchema(),
		d.statements.settingsStatements.getSchema()}
	for _, sqlStr := range schemas {
		_, err := d.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = d.statements.prepare(d.db, d); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

func (d *Database) RecoverCache() {
	span, ctx := common.StartSobSomSpan(context.Background(), "Database.RecoverCache")
	defer span.Finish()
	err := d.statements.settingsStatements.recoverSettings(ctx)
	if err != nil {
		log.Errorf("roomserver.recoverSetting error %v", err)
	}

	log.Info("room db load finished")
}

func (d *Database) SetIDGenerator(idg *uid.UidGenerator) {
	d.idg = idg
}

func (d *Database) GetDB() *sql.DB {
	return d.db
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

// StoreEvent implements input.EventDatabase
func (d *Database) StoreEvent(
	ctx context.Context, event *gomatrixserverlib.Event, roomNID, stateNID int64,
	refId string, refHash []byte,
) error {
	eventStateKey := ""
	if event.StateKey() != nil {
		eventStateKey = *event.StateKey()
	}

	domain, _ := utils.DomainFromID(event.Sender())

	eventNID, _, err := d.statements.insertEvent(
		ctx,
		roomNID,
		event.Type(),
		eventStateKey,
		event.EventID(),
		event.EventNID(),
		event.EventReference().EventSHA256,
		event.Depth(),
		stateNID,
		refId,
		refHash,
		event.DomainOffset(),
		domain,
	)

	if err != nil {
		return err
	}

	if err = d.statements.insertEventJSON(ctx, eventNID, event.JSON(), event.Type()); err != nil {
		return err
	}

	return nil
}

func (d *Database) BackFillNids(
	ctx context.Context, roomNID int64, domain string, eventNid int64, limit int, dir string,
) ([]int64, error) {
	var eventNID []int64
	var err error
	if limit > 0 {
		if dir == "f" {
			eventNID, err = d.statements.selectBackFillEvFowradNID(
				ctx,
				roomNID,
				domain,
				eventNid,
				limit,
			)
		} else {
			eventNID, err = d.statements.selectBackFillEvNID(
				ctx,
				roomNID,
				domain,
				eventNid,
				limit,
			)
		}
	} else {
		if dir == "f" {
			eventNID, err = d.statements.selectBackFillEvForwardNIDUnLimited(
				ctx,
				roomNID,
				domain,
				eventNid,
			)
		} else {
			eventNID, err = d.statements.selectBackFillEvNIDUnLimited(
				ctx,
				roomNID,
				domain,
				eventNid,
			)
		}
	}

	if err != nil {
		return nil, err
	}

	return eventNID, nil
}

func (d Database) SelectRoomEventsByDomainOffset(ctx context.Context, roomNID int64, domain string, domainOffset int64, limit int) ([]int64, error) {
	return d.statements.selectRoomEventsByDomainOffset(ctx, roomNID, domain, domainOffset, limit)
}

func (d *Database) SelectEventNidForBackfill(ctx context.Context, roomNID int64, domain string) (int64, error) {
	return d.statements.selectEventNidForBackfill(ctx, roomNID, domain)
}

func (d *Database) SelectRoomMaxDomainOffsets(ctx context.Context, roomNID int64) (domains, eventIDs []string, offsets []int64, err error) {
	return d.statements.selectRoomMaxDomainOffset(ctx, roomNID)
}

func (d *Database) SelectEventStateSnapshotNID(ctx context.Context, eventID string) (int64, error) {
	return d.statements.selectEventStateSnapshotNID(ctx, eventID)
}

func (d *Database) SelectRoomStateNIDByStateBlockNID(ctx context.Context, roomNID int64, stateBlockNID int64) ([]int64, []string, []string, []string, error) {
	return d.statements.selectRoomStateNIDByStateBlockNID(ctx, roomNID, stateBlockNID)
}

func (d *Database) AssignRoomNID(
	ctx context.Context, roomID string,
) (int64, error) {
	return d.statements.insertRoomNID(ctx, roomID)
}

func (d *Database) RoomInfo(
	ctx context.Context, roomID string,
) (int64, int64, int64, error) {
	return d.statements.selectRoomInfo(ctx, roomID)
}

// EventNIDs implements query.RoomserverQueryAPIDatabase
func (d *Database) EventNIDs(
	ctx context.Context, eventIDs []string,
) (map[string]int64, error) {
	return d.statements.bulkLoadEventNIDByID(ctx, eventIDs)
}

func (d *Database) EventNID(
	ctx context.Context, eventID string,
) (int64, error) {
	return d.statements.loadEventNIDByID(ctx, eventID)
}

// Events implements input.EventDatabase
func (d *Database) Events(
	ctx context.Context, eventNIDs []int64,
) ([]*gomatrixserverlib.Event, []int64, error) {
	return d.statements.bulkEvents(ctx, eventNIDs)
}

// AddState implements input.EventDatabase
func (d *Database) AddState(
	ctx context.Context,
	roomNID int64,
	StateBlockNIDs []int64,
) (int64, error) {
	return d.statements.insertState(ctx, roomNID, StateBlockNIDs)
}

func (d *Database) SelectState(
	ctx context.Context,
	snapshotNID int64,
) (int64, []int64, error) {
	return d.statements.selectState(ctx, snapshotNID)
}

// RoomNID implements query.RoomserverQueryAPIDB
func (d *Database) RoomNID(ctx context.Context, roomID string) (roomservertypes.RoomNID, error) {
	nid, _, _, err := d.statements.selectRoomInfo(ctx, roomID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return roomservertypes.RoomNID(nid), err
}

// SetRoomAlias implements alias.RoomserverAliasAPIDB
func (d *Database) SetRoomAlias(ctx context.Context, alias string, roomID string) error {
	return d.statements.insertRoomAlias(ctx, alias, roomID)
}

// GetRoomIDFromAlias implements alias.RoomserverAliasAPIDB
func (d *Database) GetRoomIDFromAlias(ctx context.Context, alias string) (string, error) {
	return d.statements.selectRoomIDFromAlias(ctx, alias)
}

// GetAliasesFromRoomID implements alias.RoomserverAliasAPIDB
func (d *Database) GetAliasesFromRoomID(ctx context.Context, roomID string) ([]string, error) {
	return d.statements.selectAliasesFromRoomID(ctx, roomID)
}

// RemoveRoomAlias implements alias.RoomserverAliasAPIDB
func (d *Database) RemoveRoomAlias(ctx context.Context, alias string) error {
	return d.statements.deleteRoomAlias(ctx, alias)
}

func (d *Database) SetToInvite(
	ctx context.Context, roomNID int64,
	targetUser, senderUserID, eventID string, json []byte,
	eventNID int64, pre, roomID string,
) error {
	_, err := d.statements.insertInviteEvent(
		ctx, eventID, roomNID, targetUser, senderUserID, json,
	)
	if err != nil {
		log.Errorf("SetToInvite insertInviteEvent err:%v", err)
		return err
	}

	if pre == "" {
		return d.statements.insertMembership(ctx, roomNID, targetUser, roomID, int64(roomservertypes.MembershipStateInvite), eventNID)
	}

	return d.statements.updateMembership(
		ctx, roomNID, targetUser, senderUserID,
		roomservertypes.MembershipStateInvite, eventNID,
	)
}

func (d *Database) SetToJoin(
	ctx context.Context, roomNID int64,
	targetUser, senderUserID string,
	eventNID int64, pre, roomID string,
) error {
	if pre == "" {
		return d.statements.insertMembership(ctx, roomNID, targetUser, roomID, int64(roomservertypes.MembershipStateJoin), eventNID)
	}

	if pre == "invite" {
		err := d.statements.updateInviteRetired(
			ctx, roomNID, targetUser,
		)
		if err != nil {
			return err
		}
	}

	return d.statements.updateMembership(
		ctx, roomNID, targetUser, senderUserID,
		roomservertypes.MembershipStateJoin, eventNID,
	)
}

func (d *Database) SetToLeave(
	ctx context.Context, roomNID int64,
	targetUser, senderUserID string,
	eventNID int64, pre, roomID string,
) error {
	err := d.statements.updateInviteRetired(
		ctx, roomNID, targetUser,
	)
	if err != nil {
		return err
	}

	if pre == "" {
		return d.statements.insertMembership(ctx, roomNID, targetUser, roomID, int64(roomservertypes.MembershipStateLeaveOrBan), eventNID)
	}

	return d.statements.updateMembership(
		ctx, roomNID, targetUser, senderUserID,
		roomservertypes.MembershipStateLeaveOrBan, eventNID,
	)
}

func (d *Database) SetToForget(
	ctx context.Context, roomNID int64,
	targetUser string, eventNID int64,
	pre, roomID string,
) error {
	if pre == "" {
		return d.statements.insertMembership(ctx, roomNID, targetUser, roomID, int64(roomservertypes.MembershipStateLeaveOrBan), eventNID)
	}

	return d.statements.updateMembershipForgetNID(
		ctx, roomNID, targetUser, eventNID,
	)
}

func (d *Database) GetRoomStates(
	ctx context.Context, roomID string,
) ([]*gomatrixserverlib.Event, []int64, error) {
	start := time.Now()

	roomNID, err := d.RoomNID(ctx, roomID)
	if err != nil {
		return nil, nil, err
	}
	log.Infof("GetRoomStates get room:%s id:%d", roomID, roomNID)

	nids, err := d.statements.getRoomsStateEvNID(ctx, int64(roomNID))
	if err != nil {
		return nil, nil, err
	}
	log.Infof("GetRoomStates get room:%s nids:%v", roomID, nids)

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetRoomStates").Set(duration)

	events, ids, err := d.statements.bulkEvents(ctx, nids)
	log.Infof("GetRoomStates get room:%s evs:%v", roomID, events)
	return events, ids, err
}

func (d *Database) EventsCount(ctx context.Context) (count int, err error) {

	count, err = d.statements.selectEventsTotal(ctx)

	return count, err
}

func (d *Database) FixCorruptRooms() {
	span, ctx := common.StartSobSomSpan(context.Background(), "Database.FixCorruptRooms")
	defer span.Finish()
	rooms, err := d.statements.eventStatements.getCorruptRooms(ctx)

	version, _ := d.idg.Next()

	log.Errorf("FixCorruptRooms try to fix rooms")
	if err == nil {
		log.Errorf("FixCorruptRooms try to fix rooms")
		for _, nid := range rooms {
			last_snap_nid, err := d.statements.eventStatements.getLastEvent(ctx, nid)
			if err == nil && last_snap_nid == 0 {
				log.Errorf("FixCorruptRooms find room nid:%d latest event snapnid 0", nid)
				last_ev_nid, last_valid_snapid, depth, err := d.statements.eventStatements.getLastConfirmEvent(ctx, nid)
				if err == nil {
					log.Errorf("FixCorruptRooms find room nid:%d latest valid snapnid %d, evnid:%d", nid, last_valid_snapid, last_ev_nid)
					err := d.statements.roomStatements.updateLatestEventNIDsRaw(ctx, nid, []int64{last_ev_nid}, last_ev_nid, last_valid_snapid, version, depth)
					log.Errorf("FixCorruptRooms fix room nid:%d snapnid %d, evnid:%d err:%v", nid, last_valid_snapid, last_ev_nid, err)
				}
			}
		}
	}
	log.Errorf("FixCorruptRooms fix rooms end")
}

type transaction struct {
	ctx context.Context
	txn *sql.Tx
}

// Commit implements types.Transaction
func (t *transaction) Commit() error {
	return t.txn.Commit()
}

// Rollback implements types.Transaction
func (t *transaction) Rollback() error {
	return t.txn.Rollback()
}

func (d *Database) InsertEventJSON(ctx context.Context, eventNID int64, eventJSON []byte, eventType string) error {
	return d.statements.insertEventJSONRaw(ctx, eventNID, eventJSON, eventType)
}

func (d *Database) InsertEvent(ctx context.Context, eventNID int64,
	roomNID int64,
	eventType string,
	eventStateKey string,
	eventID string,
	referenceSHA256 []byte,
	authEventNIDs []int64,
	depth int64,
	stateNID int64,
	refId string,
	refHash []byte,
	offset int64,
	domain string) error {
	return d.statements.insertEventRaw(
		ctx,
		eventNID,
		roomNID,
		eventType,
		eventStateKey,
		eventID,
		referenceSHA256,
		authEventNIDs,
		depth,
		stateNID,
		refId,
		refHash,
		offset,
		domain,
	)
}

func (d *Database) InsertInvite(ctx context.Context, eventId string, roomNid int64, target, sender string, content []byte) error {
	return d.statements.insertInviteEventRaw(ctx, eventId, roomNid, target, sender, content)
}

func (d *Database) InviteUpdate(ctx context.Context, roomNid int64, target string) error {
	return d.statements.updateInviteRetiredRaw(ctx, roomNid, target)
}

func (d *Database) MembershipInsert(ctx context.Context, roomNid int64, target, roomID string, membership, eventNID int64) error {
	return d.statements.membershipInsertdRaw(ctx, roomNid, target, roomID, membership, eventNID)
}

func (d *Database) MembershipUpdate(ctx context.Context, roomNid int64, target, sender string, membership, eventNID, version int64) error {
	return d.statements.membershipUpdateRaw(ctx, roomNid, target, sender, membership, eventNID, version)
}

func (d *Database) MembershipForgetUpdate(ctx context.Context, roomNid int64, target string, forgetNid int64) error {
	return d.statements.membershipForgetUpdateRaw(ctx, roomNid, target, forgetNid)
}

func (d *Database) InsertRoomNID(ctx context.Context, roomNID int64, roomID string) error {
	return d.statements.insertRoomNIDRaw(ctx, roomNID, roomID)
}

func (d *Database) UpdateLatestEventNIDs(
	ctx context.Context,
	roomNID int64,
	eventNIDs []int64,
	lastEventSentNID int64,
	stateSnapshotNID int64,
	version int64,
	depth int64,
) error {
	return d.statements.updateLatestEventNIDsRaw(ctx,
		roomNID,
		eventNIDs,
		lastEventSentNID,
		stateSnapshotNID,
		version,
		depth)
}

func (d *Database) InsertStateRaw(
	ctx context.Context, stateNID int64, roomNID int64, nids []int64,
) (err error) {
	return d.statements.insertStateRaw(ctx, stateNID, roomNID, nids)
}

func (d *Database) RoomExists(
	ctx context.Context, roomId string,
) (exists bool, err error) {
	return d.statements.selectRoomExists(ctx, roomId)
}

func (d *Database) LatestRoomEvent(
	ctx context.Context, roomId string,
) (eventId string, err error) {
	return d.statements.selectLatestEventID(ctx, roomId)
}

func (d *Database) EventCountForRoom(
	ctx context.Context, roomId string,
) (count int, err error) {
	return d.statements.selectRoomEventCount(ctx, roomId)
}

func (d *Database) EventCountAll(
	ctx context.Context,
) (count int, err error) {
	return d.statements.selectEventCount(ctx)
}

func (d *Database) SetLatestEvents(
	ctx context.Context,
	roomNID int64, lastEventNIDSent, currentStateSnapshotNID, depth int64,
) error {
	return d.statements.updateLatestEventNIDs(ctx, roomNID, lastEventNIDSent, currentStateSnapshotNID, depth)
}

func (d *Database) LoadFilterData(ctx context.Context, key string, f *filter.Filter) bool {
	offset := 0
	finish := false
	limit := 500

	if key == "alias" {
		for {
			if finish {
				return true
			}

			log.Infof("LoadFilterData %s limit:%d @offset:%d", key, limit, offset)
			aliases, total, err := d.statements.selectAllAliases(ctx, limit, offset)
			if err != nil {
				log.Errorf("LoadFilterData %s with err %v", key, err)
				return false
			}
			for _, alias := range aliases {
				f.Insert([]byte(alias))
			}

			if total < limit {
				finish = true
			} else {
				offset = offset + limit
			}
		}
	}

	return false
}

func (d *Database) GetUserRooms(
	ctx context.Context, uid string,
) ([]string, []string, []string, error) {
	start := time.Now()

	roomMap, err := d.statements.membershipStatements.selectMembershipsFromTarget(ctx, uid)
	if err != nil {
		return nil, nil, nil, err
	}

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetUserRooms").Set(duration)

	join := []string{}
	invite := []string{}
	leave := []string{}
	for roomId, membership := range roomMap {
		if membership == int64(roomservertypes.MembershipStateJoin) {
			join = append(join, roomId)
		} else if membership == int64(roomservertypes.MembershipStateInvite) {
			invite = append(invite, roomId)
		} else {
			leave = append(leave, roomId)
		}
	}

	return join, invite, leave, nil
}

func (d *Database) AliaseInsertRaw(ctx context.Context, aliase, roomID string) error {
	return d.statements.insertRoomAliasRaw(ctx, aliase, roomID)
}

func (d *Database) AliaseDeleteRaw(ctx context.Context, aliase string) error {
	return d.statements.deleteRoomAliasRaw(ctx, aliase)
}

func (d *Database) GetAllRooms(ctx context.Context, limit, offset int) ([]roomservertypes.RoomNIDs, error) {
	start := time.Now()

	roomNIDs, err := d.statements.getAllRooms(ctx, limit, offset)

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetAllRooms").Set(duration)

	return roomNIDs, err
}

func (d *Database) GetRoomEvents(ctx context.Context, roomNID int64) ([][]byte, error) {
	start := time.Now()
	res, err := d.statements.getRoomEvents(ctx, roomNID)

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetRoomEvents").Set(duration)

	return res, err
}

func (d *Database) GetRoomEventsWithLimit(ctx context.Context, roomNID int64, limit, offset int) ([]int64, [][]byte, error) {
	start := time.Now()

	res1, res2, err := d.statements.getRoomEventsWithLimit(ctx, roomNID, limit, offset)

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetRoomEventsWithLimit").Set(duration)

	return res1, res2, err
}

func (d *Database) RoomDomainsInsertRaw(ctx context.Context, room_nid int64, domain string, offset int64) error {
	return d.statements.insertRoomDomainsRaw(ctx, room_nid, domain, offset)
}

func (d *Database) SaveRoomDomainsOffset(ctx context.Context, room_nid int64, domain string, offset int64) error {
	return d.statements.insertRoomDomains(ctx, room_nid, domain, offset)
}

func (d *Database) GetRoomDomainsOffset(ctx context.Context, room_nid int64) ([]string, []int64, error) {
	start := time.Now()

	res1, res2, err := d.statements.selectRoomDomains(ctx, room_nid)

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetRoomDomainsOffset").Set(duration)

	return res1, res2, err
}

func (d *Database) UpdateRoomEvent(ctx context.Context, eventNID, roomNID, depth, domainOffset int64, domain string) error {
	return d.statements.updateRoomEvent(ctx, eventNID, roomNID, depth, domainOffset, domain)
}

func (d *Database) OnUpdateRoomEvent(ctx context.Context, eventNID, roomNID, depth, domainOffset int64, domain string) error {
	return d.statements.onUpdateRoomEvent(ctx, eventNID, roomNID, depth, domainOffset, domain)
}

func (d *Database) UpdateRoomDepth(ctx context.Context, depth, roomNid int64) error {
	return d.statements.updateRoomDepth(ctx, depth, roomNid)
}

func (d *Database) OnUpdateRoomDepth(ctx context.Context, depth, roomNid int64) error {
	return d.statements.onUpdateRoomDepth(ctx, depth, roomNid)
}

func (d *Database) SettingsInsertRaw(ctx context.Context, settingKey string, val string) error {
	return d.statements.insertSettingRaw(ctx, settingKey, val)
}

func (d *Database) SaveSettings(ctx context.Context, settingKey string, val string) error {
	return d.statements.insertSetting(ctx, settingKey, val)
}

func (d *Database) SelectSettingKey(ctx context.Context, settingKey string) (string, error) {
	return d.statements.selectSettingKey(ctx, settingKey)
}

func (d *Database) GetMsgEventsMigration(ctx context.Context, limit, offset int64) ([]int64, [][]byte, error) {
	return d.statements.selectMsgEventsMigration(ctx, limit, offset)
}
func (d *Database) GetMsgEventsTotalMigration(ctx context.Context) (int, int64, error) {
	return d.statements.selectMsgEventsTotalMigration(ctx)
}
func (d *Database) UpdateMsgEventMigration(ctx context.Context, id int64, EncryptedEventBytes []byte) error {
	return d.statements.updateMsgEventMigration(ctx, id, EncryptedEventBytes)
}

func (d *Database) GetRoomEventByNID(ctx context.Context, eventNID int64) ([]byte, error) {
	return d.statements.selectRoomEventByNID(ctx, eventNID)
}
