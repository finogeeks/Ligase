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

package entry

import (
	"context"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type FedDBUpdate struct {
	RoomNID int64
	RoomID  string
}

type UpdateDBForFed struct {
	roomDB   model.RoomServerDatabase
	syncDB   model.SyncAPIDatabase
	msgChan  []chan *FedDBUpdate
	chanSize uint32
}

func StartUpdateDBForFed(
	base *basecomponent.BaseDendrite,
) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka
	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}
	transportMultiplexer.PreStart()
	transportMultiplexer.Start()

	log.Infof("start process db update for federation")
	roomDB := base.CreateRoomDB()
	syncDB := base.CreateSyncDB()

	s := &UpdateDBForFed{
		roomDB:   roomDB,
		syncDB:   syncDB,
		chanSize: 4,
	}
	s.Start()

	exists := true
	limit := 10000
	offset := 0
	for exists {
		rooms, err := roomDB.GetAllRooms(context.TODO(), limit, offset)
		if err != nil {
			log.Errorf("ProcessGoEvents.getRooms err %v", err)
			exists = false
		}
		if len(rooms) == 0 {
			exists = false
			break
		}
		for _, room := range rooms {
			up := &FedDBUpdate{
				RoomID:  room.RoomID,
				RoomNID: room.RoomNID,
			}
			idx := common.CalcStringHashCode(room.RoomID) % s.chanSize
			s.msgChan[idx] <- up
		}
		offset = offset + limit
	}
	log.Infof("finish process db update for federation")
}

func (s *UpdateDBForFed) Start() {
	s.msgChan = make([]chan *FedDBUpdate, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *FedDBUpdate, 512)
		go s.startWorker(s.msgChan[i])
	}
}

func (s *UpdateDBForFed) startWorker(msgChan chan *FedDBUpdate) {
	for data := range msgChan {
		s.processRoom(data.RoomNID, data.RoomID)
	}
}

func (s *UpdateDBForFed) processRoom(
	roomNID int64,
	roomID string,
) {
	log.Infof("start process db update for federation roomID %s roomNID %d", roomID, roomNID)
	exists := true
	limit := 10000
	offset := 0
	depth := int64(0)
	domainMap := new(sync.Map)

	for exists {
		nids, events, err := s.roomDB.GetRoomEventsWithLimit(context.TODO(), roomNID, limit, offset)
		if err != nil {
			log.Errorf("processRoom.GetRoomEventsWithLimit err %v", err)
			exists = false
		}
		if len(events) == 0 {
			exists = false
			break
		}
		for index, jsonStr := range events {
			evNid := nids[index]
			depth = depth + 1
			ev, err := gomatrixserverlib.NewEventFromTrustedJSON(jsonStr, false)
			if err != nil {
				log.Errorw("json string convert error", log.KeysAndValues{"json_string", string(jsonStr), "err", err})
				continue
			}
			domain, _ := common.DomainFromID(ev.Sender())
			var domainOffset int64
			if val, ok := domainMap.Load(domain); !ok {
				domainOffset = 1
			} else {
				domainOffset = val.(int64) + 1
			}
			domainMap.Store(domain, domainOffset)

			s.roomDB.UpdateRoomEvent(context.TODO(), evNid, roomNID, depth, domainOffset, domain)
			s.syncDB.UpdateSyncEvent(context.TODO(), domainOffset, int64(ev.OriginServerTS()), domain, roomID, ev.EventID())
		}

		offset = offset + limit
	}

	s.roomDB.UpdateRoomDepth(context.TODO(), depth, roomNID)

	domainMap.Range(func(key, value interface{}) bool {
		s.roomDB.SaveRoomDomainsOffset(context.TODO(), roomNID, key.(string), value.(int64))
		return true
	})

	log.Infof("finish process db update for federation roomID %s roomNID %d", roomID, roomNID)
}
