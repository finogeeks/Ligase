package processors

import (
	"context"
	"database/sql"
	"fmt"

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
	dbregistry.Register("roomserver_events", NewDBRoomserverEventsProcessor, NewCacheRoomserverEventsProcessor)
}

type DBRoomserverEventsProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverEventsProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverEventsProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverEventsProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverEventsProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.EventInsertKey: true,
	}
}

func (p *DBRoomserverEventsProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.EventInsertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.RoomEventUpdateKey:
		p.processUpdate(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverEventsProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		common.WithTransaction(p.db.GetDB(), func(txn *sql.Tx) error {
			stmt, err := txn.Prepare(pq.CopyIn("roomserver_events", "room_nid", "event_type_id", "event_state_key_id", "event_id", "reference_sha256", "auth_event_nids", "depth", "event_nid", "state_snapshot_nid", "previous_event_id", "previous_reference_sha256", "offsets", "domain", "sent_to_output"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt.Close()

			for _, v := range inputs {
				msg := v.Event.RoomDBEvents.EventInsert
				_, err = stmt.ExecContext(
					ctx, msg.RoomNid, msg.EventType, msg.EventStateKey, msg.EventId,
					msg.RefSha, pq.Int64Array(msg.AuthEventNids), msg.Depth, msg.EventNid,
					msg.StateSnapNid, msg.RefEventId, msg.Sha, msg.Offset, msg.Domain, true,
				)
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
			msg := v.Event.RoomDBEvents.EventInsert
			err := p.db.InsertEvent(ctx, msg.EventNid,
				msg.RoomNid, msg.EventType, msg.EventStateKey, msg.EventId,
				msg.RefSha, msg.AuthEventNids, msg.Depth,
				msg.StateSnapNid, msg.RefEventId, msg.Sha,
				msg.Offset, msg.Domain)
			if err != nil {
				log.Error("insert roomserver_events err", err, msg.EventNid,
					msg.RoomNid, msg.EventType, msg.EventStateKey, msg.EventId,
					msg.RefSha, msg.AuthEventNids, msg.Depth,
					msg.StateSnapNid, msg.RefEventId, msg.Sha,
					msg.Offset, msg.Domain)
			}
		}
	}
	return nil
}

func (p *DBRoomserverEventsProcessor) processUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.RoomEventUpdate
		err := p.db.OnUpdateRoomEvent(ctx, msg.EventNid, msg.RoomNid, msg.Depth, msg.Offset, msg.Domain)
		if err != nil {
			log.Errorf("update roomEvent err", err, msg.EventNid, msg.RoomNid, msg.Depth, msg.Offset, msg.Domain)
		}
	}
	return nil
}

type CacheRoomserverEventsProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheRoomserverEventsProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheRoomserverEventsProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheRoomserverEventsProcessor) Start() {
}

func (p *CacheRoomserverEventsProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.RoomDBEvents
	switch key {
	case dbtypes.EventInsertKey:
		return p.onEventInsert(ctx, *data.EventInsert)
	}
	return nil
}

func (p *CacheRoomserverEventsProcessor) onEventInsert(ctx context.Context, msg dbtypes.EventInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	_, err := conn.Do("hmset", fmt.Sprintf("offsets:%d", msg.RoomNid), "depth", msg.Depth,
		msg.Domain, msg.Offset)
	if err != nil {
		log.Errorf("roomserver_evnents cache update err", err)
	}
	//log.Infof("onEventInsert room:%d set depth %d domain:%s offset:%d", msg.RoomNid, msg.Depth, msg.Domain, msg.Offset)
	return err
}
