// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package consumers

import (
	"context"
	"encoding/json"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
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
