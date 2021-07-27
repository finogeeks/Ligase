package dbupdatetypes

import (
	"context"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/gomodule/redigo/redis"
)

type DBEventDataInput struct {
	Data   []byte
	RawMsg interface{}
	Event  *dbtypes.DBEvent
}

type CacheInput struct {
	Event *dbtypes.DBEvent
}

type DBEventSeqProcessor interface {
	Start()
	Process(ctx context.Context, input []DBEventDataInput) error
}

type KafkaCommiter interface {
	Commit(rawMsg []interface{}) error
}

type DBEventSeqBatch interface {
	BatchKeys() map[int64]bool
}

type CacheProcessor interface {
	Start()
	Process(ctx context.Context, input CacheInput) error
}

type Pool interface {
	Pool() *redis.Pool
}

type ProcessorFactory = func(string, *config.Dendrite) DBEventSeqProcessor
type CacheFactory = func(string, *config.Dendrite, Pool) CacheProcessor
