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
	dbregistry.Register("syncapi_current_room_state", NewDBSyncapiCurrentRoomStateProcessor, nil)
}

type DBSyncapiCurrentRoomStateProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.SyncAPIDatabase
}

func NewDBSyncapiCurrentRoomStateProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBSyncapiCurrentRoomStateProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBSyncapiCurrentRoomStateProcessor) Start() {
	db, err := common.GetDBInstance("syncapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to syncapi db")
	}
	p.db = db.(model.SyncAPIDatabase)
}

func (p *DBSyncapiCurrentRoomStateProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.SyncRoomStateUpdateKey:
		p.processUpsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBSyncapiCurrentRoomStateProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncRoomStateUpdate
		err := p.db.UpdateRoomStateRaw(ctx, msg.RoomId, msg.EventId, msg.EventJson, msg.Type, msg.EventStateKey, msg.Membership, msg.AddPos)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.RoomId, msg.EventId, msg.EventJson, msg.Type, msg.EventStateKey, msg.Membership, msg.AddPos)
		}
	}
	return nil
}
