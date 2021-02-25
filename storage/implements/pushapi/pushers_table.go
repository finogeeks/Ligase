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

package pushapi

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const pushersSchema = `
-- storage data for pushers
CREATE TABLE IF NOT EXISTS pushers (
	user_name TEXT NOT NULL,
	profile_tag TEXT NOT NULL,
	kind TEXT NOT NULL,
	app_id TEXT NOT NULL,
	app_display_name TEXT NOT NULL,
	device_display_name TEXT NOT NULL,
	push_key TEXT NOT NULL,
	push_key_ts BIGINT NOT NULL,
	lang TEXT,
	push_data TEXT,
	device_id TEXT NOT NULL,
	CONSTRAINT pushers_unique UNIQUE (user_name, app_id, push_key)
);

CREATE INDEX IF NOT EXISTS pushers_idx_user ON pushers(user_name);
CREATE INDEX IF NOT EXISTS pushers_idx_push_key ON pushers(app_id, push_key);
`

const deleteUserPushersSQL = "" +
	"DELETE FROM pushers WHERE user_name = $1 and app_id=$2 and push_key=$3"

const insertUserPusherSQL = "" +
	"INSERT INTO pushers(user_name, profile_tag, kind, app_id, app_display_name, device_display_name, push_key, lang, push_data, push_key_ts,device_id)" +
	" VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,$11)" +
	" ON CONFLICT ON CONSTRAINT pushers_unique" +
	" DO UPDATE SET profile_tag = EXCLUDED.profile_tag, kind = EXCLUDED.kind, app_display_name = EXCLUDED.app_display_name," +
	"  device_display_name = EXCLUDED.device_display_name, push_key_ts = EXCLUDED.push_key_ts, lang = EXCLUDED.lang, push_data = EXCLUDED.push_data"

const deletePushersByKeySQL = "" +
	"DELETE FROM pushers WHERE app_id=$1 and push_key=$2"

const deletePushersByKeyOnlySQL = "" +
	"DELETE FROM pushers WHERE push_key=$1"

const recoverPusherSQL = "" +
	"SELECT user_name, profile_tag, kind, app_id, app_display_name, device_display_name, push_key, push_key_ts, lang, push_data, device_id FROM pushers limit $1 offset $2"

type pushersStatements struct {
	db                         *DataBase
	deleteUserPushersStmt      *sql.Stmt
	insertUserPusherStmt       *sql.Stmt
	deletePushersByKeyStmt     *sql.Stmt
	deletePushersByKeyOnlyStmt *sql.Stmt
	recoverPusherStmt          *sql.Stmt
}

func (s *pushersStatements) getSchema() string {
	return pushersSchema
}

func (s *pushersStatements) prepare(d *DataBase) (err error) {
	s.db = d
	if s.deleteUserPushersStmt, err = d.db.Prepare(deleteUserPushersSQL); err != nil {
		return
	}
	if s.insertUserPusherStmt, err = d.db.Prepare(insertUserPusherSQL); err != nil {
		return
	}
	if s.deletePushersByKeyStmt, err = d.db.Prepare(deletePushersByKeySQL); err != nil {
		return
	}
	if s.deletePushersByKeyOnlyStmt, err = d.db.Prepare(deletePushersByKeyOnlySQL); err != nil {
		return
	}
	if s.recoverPusherStmt, err = d.db.Prepare(recoverPusherSQL); err != nil {
		return
	}
	return
}

func (s *pushersStatements) loadPusher(ctx context.Context) ([]pushapitypes.Pusher, error) {
	offset := 0
	limit := 1000
	result := []pushapitypes.Pusher{}
	for {
		pushers := []pushapitypes.Pusher{}
		rows, err := s.recoverPusherStmt.QueryContext(ctx, limit, offset)
		if err != nil {
			log.Errorf("load pusher exec recoverPusherStmt err:%v", err)
			return nil, err
		}
		for rows.Next() {
			var pusher pushapitypes.Pusher
			if err := rows.Scan(&pusher.UserName, &pusher.ProfileTag, &pusher.Kind, &pusher.AppId, &pusher.AppDisplayName, &pusher.DeviceDisplayName,
				&pusher.PushKey, &pusher.PushKeyTs, &pusher.Lang, &pusher.Data, &pusher.DeviceID); err != nil {
				log.Errorf("load pusher scan rows error:%v", err)
				return nil, err
			} else {
				pushers = append(pushers, pusher)
			}
		}
		result = append(result, pushers...)
		if len(pushers) < limit {
			break
		} else {
			offset = offset + limit
		}
	}
	return result, nil
}

func (s *pushersStatements) recoverPusher() error {
	limit := 1000
	offset := 0
	exists := true
	for exists {
		exists = false
		rows, err := s.recoverPusherStmt.QueryContext(context.TODO(), limit, offset)
		if err != nil {
			return err
		}
		offset = offset + limit
		exists, err = s.processRecover(rows)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *pushersStatements) processRecover(rows *sql.Rows) (exists bool, err error) {
	defer rows.Close()
	for rows.Next() {
		exists = true
		var pusherInsert dbtypes.PusherInsert
		if err1 := rows.Scan(&pusherInsert.UserID, &pusherInsert.ProfileTag, &pusherInsert.Kind, &pusherInsert.AppID, &pusherInsert.AppDisplayName, &pusherInsert.DeviceDisplayName,
			&pusherInsert.PushKey, &pusherInsert.PushKeyTs, &pusherInsert.Lang, &pusherInsert.Data, &pusherInsert.DeviceID); err1 != nil {
			log.Errorf("load pushers error: %v", err1)
			if err == nil {
				err = err1
			}
			continue
		}
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PusherInsertKey
		update.IsRecovery = true
		update.PushDBEvents.PusherInsert = &pusherInsert
		update.SetUid(int64(common.CalcStringHashCode64(pusherInsert.PushKey)))
		err2 := s.db.WriteDBEventWithTbl(&update, "pushers")
		if err2 != nil {
			log.Errorf("update pushers cache error: %v", err2)
			if err == nil {
				err = err2
			}
			continue
		}
	}
	return
}

func (s *pushersStatements) deletePushers(
	ctx context.Context, userID, appID, pushKey string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PusherDeleteKey
		update.PushDBEvents.PusherDelete = &dbtypes.PusherDelete{
			UserID:  userID,
			AppID:   appID,
			PushKey: pushKey,
		}
		update.SetUid(int64(common.CalcStringHashCode64(pushKey)))
		return s.db.WriteDBEventWithTbl(&update, "pushers")
	}

	return s.onDeletePushers(ctx, userID, appID, pushKey)
}

func (s *pushersStatements) onDeletePushers(
	ctx context.Context, userID, appID, pushKey string,
) (err error) {
	stmt := s.deleteUserPushersStmt
	_, err = stmt.ExecContext(ctx, userID, appID, pushKey)
	return
}

func (s *pushersStatements) deletePushersByKey(
	ctx context.Context, appID, pushKey string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PusherDeleteByKeyKey
		update.PushDBEvents.PusherDeleteByKey = &dbtypes.PusherDeleteByKey{
			AppID:   appID,
			PushKey: pushKey,
		}
		update.SetUid(int64(common.CalcStringHashCode64(pushKey)))
		return s.db.WriteDBEventWithTbl(&update, "pushers")
	}

	return s.onDeletePushersByKey(ctx, appID, pushKey)
}

func (s *pushersStatements) onDeletePushersByKey(
	ctx context.Context, appID, pushKey string,
) (err error) {
	stmt := s.deletePushersByKeyStmt
	_, err = stmt.ExecContext(ctx, appID, pushKey)
	return
}

func (s *pushersStatements) deletePushersByKeyOnly(
	ctx context.Context, pushKey string,
) (err error) {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PusherDeleteByKeyOnlyKey
		update.PushDBEvents.PusherDeleteByKeyOnly = &dbtypes.PusherDeleteByKeyOnly{
			PushKey: pushKey,
		}
		update.SetUid(int64(common.CalcStringHashCode64(pushKey)))
		return s.db.WriteDBEventWithTbl(&update, "pushers")
	}

	return s.onDeletePushersByKeyOnly(ctx, pushKey)
}

func (s *pushersStatements) onDeletePushersByKeyOnly(
	ctx context.Context, pushKey string,
) (err error) {
	stmt := s.deletePushersByKeyOnlyStmt
	_, err = stmt.ExecContext(ctx, pushKey)
	return
}

func (s *pushersStatements) insertPusher(
	ctx context.Context, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey string, pushKeyTs int64, lang string, data []byte, deviceID string,
) error {
	if s.db.AsyncSave == true {
		var update dbtypes.DBEvent
		update.Category = dbtypes.CATEGORY_PUSH_DB_EVENT
		update.Key = dbtypes.PusherInsertKey
		update.PushDBEvents.PusherInsert = &dbtypes.PusherInsert{
			UserID:            userID,
			ProfileTag:        profileTag,
			Kind:              kind,
			AppID:             appID,
			AppDisplayName:    appDisplayName,
			DeviceDisplayName: deviceDisplayName,
			PushKey:           pushKey,
			PushKeyTs:         pushKeyTs,
			Lang:              lang,
			Data:              data,
			DeviceID:          deviceID,
		}
		update.SetUid(int64(common.CalcStringHashCode64(pushKey)))
		return s.db.WriteDBEventWithTbl(&update, "pushers")
	}

	return s.onInsertPusher(ctx, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey, pushKeyTs, lang, data, deviceID)

}

func (s *pushersStatements) onInsertPusher(
	ctx context.Context, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey string, pushKeyTs int64, lang string, data []byte, deviceID string,
) error {
	stmt := s.insertUserPusherStmt
	_, err := stmt.ExecContext(ctx, userID, profileTag, kind, appID, appDisplayName, deviceDisplayName, pushKey, lang, data, pushKeyTs, deviceID)
	return err
}
