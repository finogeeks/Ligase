package processors

import (
	"context"
	"fmt"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	dbregistry.Register("roomserver_settings", NewDBRoomserverSettingProcessor, NewCacheRoomserverSettingProcessor)
}

type DBRoomserverSettingProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverSettingProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverSettingProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverSettingProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverSettingProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.SettingUpsertKey:
		p.processUpsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverSettingProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.SettingsInsert
		err := p.db.SettingsInsertRaw(ctx, msg.SettingKey, msg.Val)
		if err != nil {
			log.Error(p.name, "insert err", err, msg.SettingKey, msg.Val)
		}
	}
	return nil
}

type CacheRoomserverSettingProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheRoomserverSettingProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheRoomserverSettingProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheRoomserverSettingProcessor) Start() {
}

func (p *CacheRoomserverSettingProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.RoomDBEvents
	switch key {
	case dbtypes.SettingUpsertKey:
		return p.onSettingUpsert(ctx, data.SettingsInsert)
	}
	return nil
}

func (p *CacheRoomserverSettingProcessor) onSettingUpsert(ctx context.Context, msg *dbtypes.SettingsInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	pusherKey := fmt.Sprintf("%s:%s", "setting", msg.SettingKey)

	err := conn.Send("set", pusherKey, msg.Val)
	if err != nil {
		return err
	}

	return conn.Flush()
}
