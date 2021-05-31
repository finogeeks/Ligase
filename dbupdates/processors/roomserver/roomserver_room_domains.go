package processors

import (
	"context"
	"strconv"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbregistry"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func init() {
	dbregistry.Register("roomserver_room_domains", NewDBRoomserverRoomDomainsProcessor, nil)
}

type DBRoomserverRoomDomainsProcessor struct {
	name string
	cfg  *config.Dendrite
	db   model.RoomServerDatabase
}

func NewDBRoomserverRoomDomainsProcessor(
	name string,
	cfg *config.Dendrite,
) dbupdatetypes.DBEventSeqProcessor {
	p := new(DBRoomserverRoomDomainsProcessor)
	p.name = name
	p.cfg = cfg

	return p
}

func (p *DBRoomserverRoomDomainsProcessor) BatchKeys() map[int64]bool {
	return map[int64]bool{
		dbtypes.RoomDomainInsertKey: true,
	}
}

func (p *DBRoomserverRoomDomainsProcessor) Start() {
	db, err := common.GetDBInstance("roomserver", p.cfg)
	if err != nil {
		log.Panicf("failed to connect to room server db")
	}
	p.db = db.(model.RoomServerDatabase)
}

func (p *DBRoomserverRoomDomainsProcessor) Process(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	if len(inputs) == 0 {
		return nil
	}

	switch inputs[0].Event.Key {
	case dbtypes.RoomDomainInsertKey:
		p.processInsert(ctx, inputs)
	default:
		log.Errorf("invalid %s event key %d", p.name, inputs[0].Event.Key)
	}

	return nil
}

func (p *DBRoomserverRoomDomainsProcessor) processInsert(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) error {
	cache := map[string]*dbtypes.RoomDomainInsert{}
	for _, v := range inputs {
		msg := v.Event.RoomDBEvents.RoomDomainInsert
		key := strconv.FormatInt(msg.RoomNid, 10) + "_" + msg.Domain
		if v, ok := cache[key]; ok {
			if v.Offset < msg.Offset {
				cache[key] = msg
			}
		} else {
			cache[key] = msg
		}
	}
	for _, v := range cache {
		err := p.db.RoomDomainsInsertRaw(ctx, v.RoomNid, v.Domain, v.Offset)
		if err != nil {
			log.Error(p.name, "insert err", err, v.RoomNid, v.Domain, v.Offset)
		}
	}
	return nil
}
