package processors

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/lib/pq"
)

func init() {
	dbregistry.Register("syncapi_user_time_line", NewDBSyncapiUserTimelineProcessor, nil)
}

type DBSyncapiUserTimelineProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.SyncAPIDatabase
}

func NewDBSyncapiUserTimelineProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBSyncapiUserTimelineProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBSyncapiUserTimelineProcessor) Start() {
	db, err := common.GetDBInstance("syncapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to syncapi db")
	}
	p.db = db.(model.SyncAPIDatabase)
}

func (p *DBSyncapiUserTimelineProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.SyncUserTimeLineInsertKey: true,
	}
}

func (p *DBSyncapiUserTimelineProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.SyncUserTimeLineInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBSyncapiUserTimelineProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		common.WithTransaction(p.db.GetDB(), func(txn *sql.Tx) error {
			stmt, err := txn.Prepare(pq.CopyIn("syncapi_user_time_line", "id", "room_id", "event_nid", "user_id", "room_state", "ts", "event_offset"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt.Close()

			for _, v := range inputs {
				msg := v.Event.SyncDBEvents.SyncUserTimeLineInsert
				_, err = stmt.ExecContext(ctx, msg.ID, msg.RoomID, msg.EventNID, msg.UserID, msg.RoomState, msg.Ts, msg.EventOffset)
				if err != nil {
					log.Errorf("bulk insert one error %v", err)
					return err
				}
			}
			_, err = stmt.ExecContext(ctx)
			if err != nil {
				log.Warnf("bulk insert error %v", err)
				return err
			}

			success = true
			log.Debugf("bulk insert %s success, len %d", p.name, len(inputs))
			return nil
		})
	}
	if !success {
		if len(inputs) > 1 {
			log.Warnf("not use bulk instert, user normal stmt instead len %d", len(inputs))
		}
		for _, v := range inputs {
			msg := v.Event.SyncDBEvents.SyncUserTimeLineInsert
			err := p.db.OnInsertUserTimeLine(ctx, msg.ID, msg.RoomID, msg.EventNID, msg.UserID, msg.RoomState, msg.Ts, msg.EventOffset)
			if err != nil {
				log.Error(p.name, "insert err", err, msg.ID, msg.RoomID, msg.EventNID, msg.UserID, msg.RoomState, msg.Ts, msg.EventOffset)
			}
		}
	}
	return nil
}
