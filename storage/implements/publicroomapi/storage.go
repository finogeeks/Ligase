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

package publicroomapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/finogeeks/ligase/model/publicroomstypes"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func init() {
	common.Register("publicroomapi", NewDatabase)
}

// Database represents a public rooms server database.
type Database struct {
	db         *sql.DB
	topic      string
	underlying string
	idg        *uid.UidGenerator
	statements publicRoomsStatements
	AsyncSave  bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase creates a new public rooms server database.
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	public := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_publicroomsapi")

	if public.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}
	public.db.SetMaxOpenConns(30)
	public.db.SetMaxIdleConns(30)
	public.db.SetConnMaxLifetime(time.Minute * 3)

	schemas := []string{public.statements.getSchema()}
	for _, sqlStr := range schemas {
		_, err := public.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = public.statements.prepare(public); err != nil {
		return nil, err
	}

	public.topic = topic
	public.AsyncSave = useAsync
	public.underlying = underlying
	return public, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

func (d *Database) SetIDGenerator(idg *uid.UidGenerator) {
	d.idg = idg
}

// WriteOutputEvents implements OutputRoomEventWriter
func (d *Database) WriteDBEvent(update *dbtypes.DBEvent) error {
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic,
		&core.TransportPubMsg{
			Keys: []byte(update.GetEventKey()),
			Obj:  update,
		})
}

func (d *Database) WriteDBEventWithTbl(update *dbtypes.DBEvent, tbl string) error {
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic+"_"+tbl,
		&core.TransportPubMsg{
			Keys: []byte(update.GetEventKey()),
			Obj:  update,
		})
}

func (d *Database) UpdateRoomFromEvent(
	ctx context.Context, event gomatrixserverlib.ClientEvent,
) error {
	// Process the event according to its type
	switch event.Type {
	case "m.room.create":
		return d.statements.insertNewRoom(ctx, event.RoomID, 0, []string{}, "", "",
			"", false, false, "", false)
	case "m.room.member":
		return d.updateNumJoinedUsers(ctx, event)
	case "m.room.aliases":
		return d.updateRoomAliases(ctx, event)
	case "m.room.canonical_alias":
		var content common.CanonicalAliasContent
		field := &(content.Alias)
		attrName := "canonical_alias"
		return d.updateStringAttribute(ctx, attrName, event, &content, field)
	case "m.room.name":
		var content common.NameContent
		field := &(content.Name)
		attrName := "name"
		return d.updateStringAttribute(ctx, attrName, event, &content, field)
	case "m.room.topic":
		var content common.TopicContent
		field := &(content.Topic)
		attrName := "topic"
		return d.updateStringAttribute(ctx, attrName, event, &content, field)
	case "m.room.desc":
		var content common.DescContent
		field := &(content.Desc)
		attrName := "desc"
		return d.updateStringAttribute(ctx, attrName, event, &content, field)
	case "m.room.avatar":
		var content common.AvatarContent
		field := &(content.URL)
		attrName := "avatar_url"
		return d.updateStringAttribute(ctx, attrName, event, &content, field)
	case "m.room.history_visibility":
		var content common.HistoryVisibilityContent
		field := &(content.HistoryVisibility)
		attrName := "world_readable"
		strForTrue := "world_readable"
		return d.updateBooleanAttribute(ctx, attrName, event, &content, field, strForTrue)
	case "m.room.visibility":
		var content common.VisibilityContent
		field := &(content.Visibility)
		attrName := "visibility"
		strForTrue := "public"
		return d.updateBooleanAttribute(ctx, attrName, event, &content, field, strForTrue)
	case "m.room.guest_access":
		var content common.GuestAccessContent
		field := &(content.GuestAccess)
		attrName := "guest_can_join"
		strForTrue := "can_join"
		return d.updateBooleanAttribute(ctx, attrName, event, &content, field, strForTrue)
	}

	// If the event type didn't match, return with no error
	return nil
}

// updateNumJoinedUsers updates the number of joined user in the database representation
// of a room using a given "m.room.member" Matrix event.
// If the membership property of the event isn't "join", ignores it and returs nil.
// If the remove parameter is set to false, increments the joined members counter in the
// database, if set to truem decrements it.
// Returns an error if the update failed.
func (d *Database) updateNumJoinedUsers(
	ctx context.Context, membershipEvent gomatrixserverlib.ClientEvent,
) error {
	membership, err := membershipEvent.Membership()
	if err != nil {
		return err
	}

	if membership == "invite" {
		return nil
	}

	if membership == "join" {
		return d.statements.incrementJoinedMembersInRoom(ctx, membershipEvent.RoomID)
	}
	return d.statements.decrementJoinedMembersInRoom(ctx, membershipEvent.RoomID)
}

// updateStringAttribute updates a given string attribute in the database
// representation of a room using a given string data field from content of the
// Matrix event triggering the update.
// Returns an error if decoding the Matrix event's content or updating the attribute
// failed.
func (d *Database) updateStringAttribute(
	ctx context.Context, attrName string, event gomatrixserverlib.ClientEvent,
	content interface{}, field *string,
) error {
	if err := json.Unmarshal(event.Content, content); err != nil {
		return err
	}

	return d.statements.updateRoomAttribute(ctx, attrName, *field, event.RoomID)
}

// updateBooleanAttribute updates a given boolean attribute in the database
// representation of a room using a given string data field from content of the
// Matrix event triggering the update.
// The attribute is set to true if the field matches a given string, false if not.
// Returns an error if decoding the Matrix event's content or updating the attribute
// failed.
func (d *Database) updateBooleanAttribute(
	ctx context.Context, attrName string, event gomatrixserverlib.ClientEvent,
	content interface{}, field *string, strForTrue string,
) error {
	if err := json.Unmarshal(event.Content, content); err != nil {
		return err
	}

	var attrValue bool
	if *field == strForTrue {
		attrValue = true
	} else {
		attrValue = false
	}

	return d.statements.updateRoomAttribute(ctx, attrName, attrValue, event.RoomID)
}

// updateRoomAliases decodes the content of a "m.room.aliases" Matrix event and update the list of aliases of
// a given room with it.
// Returns an error if decoding the Matrix event or updating the list failed.
func (d *Database) updateRoomAliases(
	ctx context.Context, aliasesEvent gomatrixserverlib.ClientEvent,
) error {
	var content common.AliasesContent
	if err := json.Unmarshal(aliasesEvent.Content, &content); err != nil {
		return err
	}

	return d.statements.updateRoomAttribute(
		ctx, "aliases", content.Aliases, aliasesEvent.RoomID,
	)
}

func (d *Database) OnUpdateRoomAttribute(
	ctx context.Context, attrName string, attrValue interface{}, roomID string,
) error {
	return d.statements.onUpdateRoomAttribute(ctx, attrName, attrValue, roomID)
}

func (d *Database) OnIncrementJoinedMembersInRoom(
	ctx context.Context, roomID string, n int,
) error {
	return d.statements.onIncrementJoinedMembersInRoom(ctx, roomID, n)
}

func (d *Database) OnDecrementJoinedMembersInRoom(
	ctx context.Context, roomID string,
) error {
	return d.statements.onDecrementJoinedMembersInRoom(ctx, roomID)
}

func (d *Database) OnInsertNewRoom(
	ctx context.Context,
	roomID string,
	seqID,
	joinedMembers int64,
	aliases []string,
	canonicalAlias,
	name,
	topic string,
	worldReadable,
	guestCanJoin bool,
	avatarUrl string,
	visibility bool,
) error {
	return d.statements.onInsertNewRoom(ctx, roomID, seqID, joinedMembers, aliases, canonicalAlias, name, topic, worldReadable, guestCanJoin, avatarUrl, visibility)
}

// CountPublicRooms returns the number of room set as publicly visible on the server.
// Returns an error if the retrieval failed.
func (d *Database) CountPublicRooms(ctx context.Context) (int64, error) {
	return d.statements.countPublicRooms(ctx)
}

// GetPublicRooms returns an array containing the local rooms set as publicly visible, ordered by their number
// of joined members. This array can be limited by a given number of elements, and offset by a given value.
// If the limit is 0, doesn't limit the number of results. If the offset is 0 too, the array contains all
// the rooms set as publicly visible on the server.
// Returns an error if the retrieval failed.
func (d *Database) GetPublicRooms(
	ctx context.Context, offset int64, limit int64, filter string,
) ([]publicroomstypes.PublicRoom, error) {
	return d.statements.selectPublicRooms(ctx, offset, limit, filter)
}
