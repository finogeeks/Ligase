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
	dbregistry.Register("encrypt_device_key", NewDBEncryptDeviceKeyProcessor, NewCacheEncryptDeviceKeyProcessor)
}

type DBEncryptDeviceKeyProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.EncryptorAPIDatabase
}

func NewDBEncryptDeviceKeyProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBEncryptDeviceKeyProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBEncryptDeviceKeyProcessor) Start() {
	db, err := common.GetDBInstance("encryptoapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to encryptoapi db")
	}
	p.db = db.(model.EncryptorAPIDatabase)
}

func (p *DBEncryptDeviceKeyProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.DeviceKeyInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.DeviceKeyDeleteKey:
		p.processDelete(ctx, inputs)
	case dbtypes.MacDeviceKeyDeleteKey:
		p.processMacDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBEncryptDeviceKeyProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.KeyInsert
		err := p.db.OnInsertDeviceKey(ctx, msg.DeviceID, msg.UserID, msg.KeyInfo, msg.Algorithm, msg.Signature, msg.Identifier)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.DeviceID, msg.UserID, msg.KeyInfo, msg.Algorithm, msg.Signature, msg.Identifier)
		}
	}
	return nil
}

func (p *DBEncryptDeviceKeyProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.DeviceKeyDelete
		err := p.db.OnDeleteDeviceKey(ctx, msg.DeviceID, msg.UserID)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.DeviceID, msg.UserID)
		}
	}
	return nil
}

func (p *DBEncryptDeviceKeyProcessor) processMacDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.MacKeyDelete
		err := p.db.OnDeleteMacDeviceKey(ctx, msg.DeviceID, msg.UserID, msg.Identifier)
		if err != nil {
			log.Error(p.name, "mac delete err", err, msg.DeviceID, msg.UserID, msg.Identifier)
		}
	}
	return nil
}

type CacheEncryptDeviceKeyProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheEncryptDeviceKeyProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheEncryptDeviceKeyProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheEncryptDeviceKeyProcessor) Start() {
}

func (p *CacheEncryptDeviceKeyProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.E2EDBEvents
	switch key {
	case dbtypes.DeviceKeyInsertKey:
		return p.onDeviceKeyInsert(ctx, data.KeyInsert)
	}
	return nil
}

func (p *CacheEncryptDeviceKeyProcessor) onDeviceKeyInsert(ctx context.Context, msg *dbtypes.KeyInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	keyKey := fmt.Sprintf("%s:%s:%s:%s", "device_key", msg.UserID, msg.DeviceID, msg.Algorithm)

	err := conn.Send("hmset", keyKey, "device_id", msg.DeviceID, "user_id", msg.UserID, "key_info", msg.KeyInfo,
		"algorithm", msg.Algorithm, "signature", msg.Signature)
	if err != nil {
		return err
	}
	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "device_key_list", msg.UserID, msg.DeviceID), keyKey, keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}
