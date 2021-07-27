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
	dbregistry.Register("roomserver_membership", NewDBRoomserverMembershipProcessor, nil)
}

type DBRoomserverMembershipProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverMembershipProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverMembershipProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverMembershipProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverMembershipProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.EventMembershipInsertKey: true,
	}
}

func (p *DBRoomserverMembershipProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.EventMembershipInsertKey:
		p.processInsert(ctx, inputs)
	case dbtypes.EventMembershipUpdateKey:
		p.processUpdate(ctx, inputs)
	case dbtypes.EventMembershipForgetUpdateKey:
		p.processForgetUpdate(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverMembershipProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	success := false
	if len(inputs) > 1 {
		common.WithTransaction(p.db.GetDB(), func(txn *sql.Tx) error {
			stmt, err := txn.Prepare(pq.CopyIn("roomserver_membership", "room_nid", "target_id", "target_forget_nid", "room_id", "membership_nid", "event_nid"))
			if err != nil {
				log.Errorf("bulk insert prepare error %v", err)
				return err
			}
			defer stmt.Close()

			for _, v := range inputs {
				msg := v.Event.RoomDBEvents.EventMembershipInsert
				_, err = stmt.ExecContext(ctx, msg.RoomNID, msg.Target, -1, msg.RoomID, msg.MembershipNID, msg.EventNID)
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
			msg := v.Event.RoomDBEvents.EventMembershipInsert
			err := p.db.MembershipInsert(ctx, msg.RoomNID, msg.Target, msg.RoomID, msg.MembershipNID, msg.EventNID)
			if err != nil {
				log.Error(p.name, "insert err", err, msg.RoomNID, msg.Target, msg.RoomID, msg.MembershipNID, msg.EventNID)
			}
		}
	}
	return nil
}

func (p *DBRoomserverMembershipProcessor) processUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.EventMembershipUpdate
		err := p.db.MembershipUpdate(ctx, msg.RoomID, msg.Target, msg.Sender, msg.Membership, msg.EventNID, msg.Version)
		if err != nil {
			log.Error(p.name, "update err", err, msg.RoomID, msg.Target, msg.Sender, msg.Membership, msg.EventNID, msg.Version)
		}
	}
	return nil
}

func (p *DBRoomserverMembershipProcessor) processForgetUpdate(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.EventMembershipForgetUpdate
		err := p.db.MembershipForgetUpdate(ctx, msg.RoomID, msg.Target, msg.ForgetID)
		if err != nil {
			log.Error(p.name, "update forget err", err, msg.RoomID, msg.Target, msg.ForgetID)
		}
	}
	return nil
}
