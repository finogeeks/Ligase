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
	dbregistry.Register("encrypt_onetime_key", NewDBEncryptOnetimeKeyProcessor, NewCacheEncryptOnetimeKeyProcessor)
}

type DBEncryptOnetimeKeyProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.EncryptorAPIDatabase
}

func NewDBEncryptOnetimeKeyProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBEncryptOnetimeKeyProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBEncryptOnetimeKeyProcessor) Start() {
	db, err := common.GetDBInstance("encryptoapi", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to encryptoapi db")
	}
	p.db = db.(model.EncryptorAPIDatabase)
}

func (p *DBEncryptOnetimeKeyProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.OneTimeKeyInsertKey:
		p.processUpsert(ctx, inputs)
	case dbtypes.OneTimeKeyDeleteKey:
		p.processDelete(ctx, inputs)
	case dbtypes.MacOneTimeKeyDeleteKey:
		p.processMacDelete(ctx, inputs)
	case dbtypes.DeviceOneTimeKeyDeleteKey:
		p.processDeviceDelete(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBEncryptOnetimeKeyProcessor) processUpsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.KeyInsert
		err := p.db.OnInsertOneTimeKey(ctx, msg.DeviceID, msg.UserID, msg.KeyID, msg.KeyInfo, msg.Algorithm, msg.Signature, msg.Identifier)
		if err != nil {
			log.Error(p.name, "upsert err", err, msg.DeviceID, msg.UserID, msg.KeyID, msg.KeyInfo, msg.Algorithm, msg.Signature, msg.Identifier)
		}
	}
	return nil
}

func (p *DBEncryptOnetimeKeyProcessor) processDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.KeyDelete
		err := p.db.OnDeleteOneTimeKey(ctx, msg.DeviceID, msg.UserID, msg.KeyID, msg.Algorithm)
		if err != nil {
			log.Error(p.name, "delete err", err, msg.DeviceID, msg.UserID, msg.KeyID, msg.Algorithm)
		}
	}
	return nil
}

func (p *DBEncryptOnetimeKeyProcessor) processMacDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.MacKeyDelete
		err := p.db.OnDeleteMacOneTimeKey(ctx, msg.DeviceID, msg.UserID, msg.Identifier)
		if err != nil {
			log.Error(p.name, "mac delete err", err, msg.DeviceID, msg.UserID, msg.Identifier)
		}
	}
	return nil
}

func (p *DBEncryptOnetimeKeyProcessor) processDeviceDelete(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	for _, v := range inputs {
		msg := v.Event.E2EDBEvents.DeviceKeyDelete
		err := p.db.OnDeleteDeviceOneTimeKey(ctx, msg.DeviceID, msg.UserID)
		if err != nil {
			log.Error(p.name, "mac delete err", err, msg.DeviceID, msg.UserID)
		}
	}
	return nil
}

type CacheEncryptOnetimeKeyProcessor struct {
	name string
	cfg  *config.Dendrite
	pool dbupdatetypes.Pool
}

func NewCacheEncryptOnetimeKeyProcessor(name string, cfg *config.Dendrite, pool dbupdatetypes.Pool) dbupdatetypes.CacheProcessor {
	p := new(CacheEncryptOnetimeKeyProcessor)
	p.name = name
	p.cfg = cfg
	p.pool = pool
	return p
}

func (p *CacheEncryptOnetimeKeyProcessor) Start() {
}

func (p *CacheEncryptOnetimeKeyProcessor) Process(ctx context.Context, input dbupdatetypes.CacheInput) error {
	key := input.Event.Key
	data := input.Event.E2EDBEvents
	switch key {
	case dbtypes.OneTimeKeyInsertKey:
		return p.onOneTimeKeyInsert(ctx, data.KeyInsert)
	case dbtypes.OneTimeKeyDeleteKey:
		return p.onOneTimeKeyDelete(ctx, data.KeyDelete)
	}
	return nil
}

func (p *CacheEncryptOnetimeKeyProcessor) onOneTimeKeyInsert(ctx context.Context, msg *dbtypes.KeyInsert) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()

	keyKey := fmt.Sprintf("%s:%s:%s:%s:%s", "one_time_key", msg.UserID, msg.DeviceID, msg.KeyID, msg.Algorithm)

	err := conn.Send("hmset", keyKey, "device_id", msg.DeviceID, "user_id", msg.UserID, "key_id", msg.KeyID,
		"key_info", msg.KeyInfo, "algorithm", msg.Algorithm, "signature", msg.Signature)
	if err != nil {
		return err
	}

	err = conn.Send("hmset", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID), keyKey, keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}

func (p *CacheEncryptOnetimeKeyProcessor) onOneTimeKeyDelete(ctx context.Context, msg *dbtypes.KeyDelete) error {
	conn := p.pool.Pool().Get()
	defer conn.Close()
	keyKey := fmt.Sprintf("%s:%s:%s:%s:%s", "one_time_key", msg.UserID, msg.DeviceID, msg.KeyID, msg.Algorithm)

	err := conn.Send("del", keyKey)
	if err != nil {
		return err
	}

	err = conn.Send("hdel", fmt.Sprintf("%s:%s:%s", "one_time_key_list", msg.UserID, msg.DeviceID), keyKey)
	if err != nil {
		return err
	}

	return conn.Flush()
}
