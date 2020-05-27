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
