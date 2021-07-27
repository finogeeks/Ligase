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
	dbregistry.Register("encrypt_algorithm", NewDBEncryptAlgorithmProcessor, NewCacheEncryptAlgorithmProcessor)
}

type DBEncryptAlgorithmProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.EncryptorAPIDatabase
}

func NewDBEncryptAlgorithmProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBEncryptAlgorithmProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBEncryptAlgorithmProcessor) Start() {
	db, err := common.GetDBInstance("encryptoapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to encryptoapi db")
	}
	p.db = db.(model.EncryptorAPIDatabase)
}

func (p *DBEncryptAlgorithmProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.AlInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.DeviceAlDeleteKey:
		p.processDelete(ctx, inputs)
	case dbtypes.MacDeviceAlDeleteKey:
		p.processMacDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBEncryptAlgorithmProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.AlInsert
		err := p.db.OnInsertAl(ctx, msg.UserID, msg.DeviceID, msg.Algorithm, msg.Identifier)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.UserID, msg.DeviceID, msg.Algorithm, msg.Identifier)
		}
	}
	return nil
}

func (p *DBEncryptAlgorithmProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.DeviceKeyDelete
		err := p.db.OnDeleteAl(ctx, msg.UserID, msg.DeviceID)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.UserID, msg.DeviceID)
		}
	}
	return nil
}

func (p *DBEncryptAlgorithmProcessor) processMacDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.MacKeyDelete
		err := p.db.OnDeleteMacAl(ctx, msg.UserID, msg.DeviceID, msg.Identifier)
		if err != nil {
			log.Error(p.name, "mac delete err", err, msg.UserID, msg.DeviceID, msg.Identifier)
		}
	}
	return nil
}

type CacheEncryptAlgorithmProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheEncryptAlgorithmProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheEncryptAlgorithmProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheEncryptAlgorithmProcessor) Start() {
}

func (p *CacheEncryptAlgorithmProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.E2EDBEvents
	switch key {
	case dbtypes.AlInsertKey:
		return p.onAlInsert(ctx, data.AlInsert)
	}
	return nil
}

func (p *CacheEncryptAlgorithmProcessor) onAlInsert(ctx context.Context, msg *dbtypes.AlInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	err := conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "algorithm", msg.UserID, msg.DeviceID),
		"device_id", msg.DeviceID, "user_id", msg.UserID, "algorithms", msg.Algorithm)
	if err != nil {
		return err
	}

	return conn.Flush()
}
