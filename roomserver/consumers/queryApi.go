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
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/roomserver/processors"
	"github.com/finogeeks/ligase/roomserver/rpc"
	"github.com/finogeeks/ligase/storage/model"
)

// consumes events that originated in the client api server.
type QueryConsumer struct {
	grpcServer *rpc.Server
}

func NewQueryConsumer(
	cfg *config.Dendrite,
	db model.RoomServerDatabase,
	repo *repos.RoomServerCurStateRepo,
	umsRepo *repos.RoomServerUserMembershipRepo,
	alias *processors.AliasProcessor,
	rs *processors.RoomQryProcessor,
	input *processors.EventsProcessor,
) *QueryConsumer {
	consumer := new(QueryConsumer)
	consumer.grpcServer = rpc.NewServer(cfg, rs, alias, input)

	return consumer
}

func (s *QueryConsumer) Start() (err error) {
	if s.grpcServer != nil {
		err = s.grpcServer.Start()
	}
	return
}
