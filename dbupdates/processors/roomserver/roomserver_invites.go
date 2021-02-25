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
	dbregistry.Register("roomserver_invites", NewDBRoomserverInvitesProcessor, nil)
}

type DBRoomserverInvitesProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverInvitesProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverInvitesProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverInvitesProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverInvitesProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.EventInviteInsertKey: true,
	}
}

func (p *DBRoomserverInvitesProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.EventInviteInsertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.EventInviteUpdateKey:
		p.processUpdate(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverInvitesProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		common.WithTransaction(p.db.GetDB(), func(txn *sql.Tx) error {
			stmt, err := txn.Prepare(pq.CopyIn("roomserver_invites", "invite_event_id", "room_nid", "target_id", "sender_id", "invite_event_json"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt.Close()

			for _, v := range inputs {
				msg := v.Event.RoomDBEvents.EventInviteInsert
				_, err = stmt.ExecContext(
					ctx, msg.EventId, msg.RoomNid, msg.Target, msg.Sender, string(msg.Content),
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
			msg := v.Event.RoomDBEvents.EventInviteInsert
			err := p.db.InsertInvite(ctx, msg.EventId, msg.RoomNid, msg.Target, msg.Sender, msg.Content)
			if err != nil {
				log.Error(p.name, "insert err", err, msg.EventId, msg.RoomNid, msg.Target, msg.Sender, msg.Content)
			}
		}
	}
	return nil
}

func (p *DBRoomserverInvitesProcessor) processUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.EventInviteUpdate
		err := p.db.InviteUpdate(ctx, msg.RoomNid, msg.Target)
		if err != nil {
			log.Errorf(p.name, "update err", err, msg.RoomNid, msg.Target)
		}
	}
	return nil
}
