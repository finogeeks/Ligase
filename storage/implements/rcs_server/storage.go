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

package rcs_server

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/plugins/message/external"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/types"
)

func init() {
	common.Register("rcsserver", NewDatabase)
}

type Database struct {
	statements statements
	db         *sql.DB
	idg        *uid.UidGenerator
	gauge      mon.LabeledGauge
}

func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	d := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_rcsserver")
	if d.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}

	schemas := []string{d.statements.friendshipStatements.getSchema()}
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

func (d *Database) SetGauge(gauge mon.LabeledGauge) {
	d.gauge = gauge
}

func (d *Database) SetIDGenerator(idg *uid.UidGenerator) {
	d.idg = idg
}

func (d *Database) NotFound(err error) bool {
	return err != nil && err.Error() == sql.ErrNoRows.Error()
}

func (d *Database) InsertFriendship(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string,
) error {
	return d.statements.insertFriendship(
		ctx, ID, roomID, fcID, toFcID, fcIDState, toFcIDState,
		fcIDIsBot, toFcIDIsBot, fcIDRemark, toFcIDRemark,
		fcIDOnceJoined, toFcIDOnceJoined, fcIDDomain, toFcIDDomain, eventID)
}

func (d *Database) GetFriendshipByRoomID(
	ctx context.Context, roomID string,
) (*types.RCSFriendship, error) {
	return d.statements.selectFriendshipByRoomID(ctx, roomID)
}

func (d *Database) GetFriendshipByFcIDAndToFcID(
	ctx context.Context, fcID, toFcID string,
) (*types.RCSFriendship, error) {
	return d.statements.selectFriendshipByFcIDAndToFcID(ctx, fcID, toFcID)
}

func (d *Database) GetFriendshipsByFcIDOrToFcID(
	ctx context.Context, userID string,
) ([]external.Friendship, error) {
	return d.statements.selectFriendshipsByFcIDOrToFcID(ctx, userID)
}

func (d *Database) GetFriendshipsByFcIDOrToFcIDWithBot(
	ctx context.Context, userID string,
) ([]external.Friendship, error) {
	return d.statements.selectFriendshipsByFcIDOrToFcIDWithBot(ctx, userID)
}

func (d *Database) GetFriendshipByFcIDOrToFcID(
	ctx context.Context, fcID, toFcID string,
) (*external.GetFriendshipResponse, error) {
	return d.statements.selectFriendshipByFcIDOrToFcID(ctx, fcID, toFcID)
}

func (d *Database) UpdateFriendshipByRoomID(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string,
) error {
	log.Infow("rcsserver=====================Database.UpdateFriendshipByRoomID",
		log.KeysAndValues{"ID", ID, "roomID", roomID, "fcID", fcID, "toFcID", toFcID,
			"fcIDState", fcIDState, "toFcIDState", toFcIDState,
			"fcIDIsBot", fcIDIsBot, "toFcIDIsBot", toFcIDIsBot,
			"fcIDRemark", fcIDRemark, "toFcIDRemark", toFcIDRemark,
			"fcIDOnceJoined", fcIDOnceJoined, "toFcIDOnceJoined", toFcIDOnceJoined,
			"eventID", eventID})
	return d.statements.updateFriendshipByRoomID(
		ctx, ID, roomID, fcID, toFcID, fcIDState, toFcIDState,
		fcIDIsBot, toFcIDIsBot, fcIDRemark, toFcIDRemark,
		fcIDOnceJoined, toFcIDOnceJoined, fcIDDomain, toFcIDDomain, eventID,
	)
}

func (d *Database) DeleteFriendshipByRoomID(
	ctx context.Context, roomID string,
) error {
	return d.statements.deleteFriendshipByRoomID(ctx, roomID)
}
