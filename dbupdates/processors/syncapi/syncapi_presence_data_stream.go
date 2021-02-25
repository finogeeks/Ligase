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
	dbregistry.Register("syncapi_presence_data_stream", NewDBSyncapiPresenceDataStreamProcessor, nil)
}

type DBSyncapiPresenceDataStreamProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.SyncAPIDatabase
}

func NewDBSyncapiPresenceDataStreamProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBSyncapiPresenceDataStreamProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBSyncapiPresenceDataStreamProcessor) Start() {
	db, err := common.GetDBInstance("syncapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to syncapi db")
	}
	p.db = db.(model.SyncAPIDatabase)
}

func (p *DBSyncapiPresenceDataStreamProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.SyncPresenceInsertKey:
		p.processUpsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBSyncapiPresenceDataStreamProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.SyncDBEvents.SyncPresenceInsert
		_, err := p.db.OnUpsertPresenceDataStream(ctx, msg.ID, msg.UserID, msg.Content)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.ID, msg.UserID, string(msg.Content))
		}
	}
	return nil
}
