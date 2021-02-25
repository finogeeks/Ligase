package processors

import (
	"context"
	"database/sql"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/encryption"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/dbupdates/processors/sqlutil"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/lib/pq"
)

func init() {
	dbregistry.Register("roomserver_event_json", NewDBRoomserverEventJSONProcessor, nil)
}

type DBRoomserverEventJSONProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverEventJSONProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverEventJSONProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverEventJSONProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverEventJSONProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.EventJsonInsertKey: true,
	}
}

func (p *DBRoomserverEventJSONProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.EventJsonInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}
	return nil
}

func (p *DBRoomserverEventJSONProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		sqlutil.WithTransaction(p.db.GetDB(), func(txn0, txn1 *sql.Tx) error {
			stmt0, err := txn0.Prepare(pq.CopyIn("roomserver_event_json", "event_nid", "event_json"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt0.Close()

			stmt1, err := txn1.Prepare(pq.CopyIn("roomserver_event_json_mirror", "event_nid", "event_json"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt1.Close()

			for _, v := range inputs {
				msg := v.Event.RoomDBEvents.EventJsonInsert
				if encryption.CheckCrypto(msg.EventType) {
					_, err = stmt0.ExecContext(ctx, msg.EventNid, string(encryption.Encrypt(msg.EventJson)))
					if err != nil {
						log.Errorf("bulk insert one error %v", err)
						return err
					}
					if encryption.CheckMirror(msg.EventType) {
						_, err = stmt1.ExecContext(ctx, msg.EventNid, string(msg.EventJson))
						if err != nil {
							log.Errorf("bulk insert one error %v", err)
							return err
						}
					}
				} else {
					_, err = stmt0.ExecContext(ctx, msg.EventNid, string(msg.EventJson))
					if err != nil {
						log.Errorf("bulk insert one error %v", err)
						return err
					}
				}
			}
			_, err = stmt0.ExecContext(ctx)
			if err != nil {
				log.Warnf("bulk insert error %v", err)
				return err
			}
			_, err = stmt1.ExecContext(ctx)
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
			msg := v.Event.RoomDBEvents.EventJsonInsert
			err := p.db.InsertEventJSON(ctx, msg.EventNid, msg.EventJson, msg.EventType)
			if err != nil {
				log.Errorf("insert roomserver_event_json err %v %d %s", err, msg.EventNid, msg.EventJson)
			}
		}
	}
	return nil
}
