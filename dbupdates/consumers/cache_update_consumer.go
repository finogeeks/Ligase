package consumers

import (
	"context"
	"encoding/json"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/dbtypes"
)

type CacheUpdateConsumer struct {
	name      string
	Cfg       *config.Dendrite
	processor dbupdatetypes.CacheProcessor
}

func newCacheUpdateConsumer(
	name string,
	cfg *config.Dendrite,
	processor dbupdatetypes.CacheProcessor,
) *CacheUpdateConsumer {
	c := new(CacheUpdateConsumer)
	c.name = name
	c.Cfg = cfg
	c.processor = processor
	return c
}

func (c *CacheUpdateConsumer) Start() error {
	c.processor.Start()
	return nil
}

func (c *CacheUpdateConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var output dbtypes.DBEvent
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("dbevent: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	c.processor.Process(ctx, dbupdatetypes.CacheInput{Event: &output})
}
