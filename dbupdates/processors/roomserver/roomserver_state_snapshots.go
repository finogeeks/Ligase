package processors

import (
	"context"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	dbregistry.Register("roomserver_state_snapshots", NewDBRoomserverStateSanpshotsProcessor, nil)
}

type DBRoomserverStateSanpshotsProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverStateSanpshotsProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverStateSanpshotsProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverStateSanpshotsProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverStateSanpshotsProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.EventStateSnapInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverStateSanpshotsProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.EventStateSnapInsert
		err := p.db.InsertStateRaw(ctx, msg.StateSnapNid, msg.RoomNid, msg.StateBlockNids)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.StateSnapNid, msg.RoomNid, msg.StateBlockNids)
		}
	}
	return nil
}
