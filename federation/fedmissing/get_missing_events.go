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

package fedmissing

import (
	"container/list"
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/skunkworks/util/cas"
)

type GetMissingEventsProcessor struct {
	fedClient *client.FedClientWrap
	rpcCli    roomserverapi.RoomserverRPCAPI
	db        model.FederationDatabase
	feddomain *common.FedDomains

	mutex    cas.Mutex
	missings list.List
}

func NewGetMissingEventsProcessor(
	fedClient *client.FedClientWrap,
	rpcCli roomserverapi.RoomserverRPCAPI,
	db model.FederationDatabase,
	feddomain *common.FedDomains,
	cfg *config.Dendrite,
) *GetMissingEventsProcessor {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.GetMissingEvent.Underlying,
		cfg.Kafka.Consumer.GetMissingEvent.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		p := &GetMissingEventsProcessor{
			fedClient: fedClient,
			rpcCli:    rpcCli,
			db:        db,
			feddomain: feddomain,
		}
		channel.SetHandler(p)

		return p
	}
	return nil
}

func (p *GetMissingEventsProcessor) Start() error {
	roomIDs, eventIDs, limits, err := p.db.SelectMissingEvents(context.TODO())
	if err != nil {
		return err
	}

	for i := 0; i < len(roomIDs); i++ {
		info := model.GetMissingEventsInfo{
			RoomID:  roomIDs[i],
			EventID: eventIDs[i],
			Limit:   limits[i],
		}
		p.missings.PushBack(info)
	}

	go p.startProcessor()

	return nil
}

func (p *GetMissingEventsProcessor) startProcessor() {
	for {
		info, ok := p.pop()
		if !ok {
			time.Sleep(time.Millisecond * 50)
			continue
		}

		p.backfillEvents(info)
	}
}

func (p *GetMissingEventsProcessor) pop() (model.GetMissingEventsInfo, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	elem := p.missings.Front()
	if elem == nil {
		return model.GetMissingEventsInfo{}, false
	}
	p.missings.Remove(elem)

	return elem.Value.(model.GetMissingEventsInfo), true
}

func (p *GetMissingEventsProcessor) push(info model.GetMissingEventsInfo) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.missings.PushBack(info)
	return
}

func (p *GetMissingEventsProcessor) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var info model.GetMissingEventsInfo
	json.Unmarshal(data, &info)
	p.db.InsertMissingEvents(context.TODO(), info.RoomID, info.EventID, info.Limit)
	p.push(info)
}

func (p *GetMissingEventsProcessor) backfillEvents(info model.GetMissingEventsInfo) {
	eventID := info.EventID
	domain, err := common.DomainFromID(eventID)
	if err != nil {
		log.Errorf("GetMissEvents eventID domain wrong %s", eventID)
		return
	}
	host, ok := p.feddomain.GetDomainHost(domain)
	if !ok {
		log.Errorf("GetMissEvents domain host not found, domain: %s", domain)
		return
	}
	roomID := info.RoomID
	limit := info.Limit
	log.Infof("GetMissingEvents backfill roomID: %s, eventID: %s, limit: %d", roomID, eventID, limit)
	minEventID := eventID
	maxEventID := eventID
	forwardFinished := false
	backwardFinished := false
	backfilPDUs := []gomatrixserverlib.Event{}
	if limit > 25 {
		limit = 25
	}
	for i := 0; i < info.Limit/limit+3; i++ {
		var resp gomatrixserverlib.BackfillResponse
		if !backwardFinished {
			resp, err = p.fedClient.Backfill(context.TODO(), gomatrixserverlib.ServerName(host), domain, roomID, limit*2, []string{minEventID}, "b")
			if err != nil {
				log.Errorf("GetMissingEvents backfill error %v", err)
				return
			}
			if len(resp.PDUs) == 0 {
				backwardFinished = true
			}
			minOffset := int64(math.MaxInt64)
			for _, v := range resp.PDUs {
				if v.DomainOffset() < info.DomainOffset && v.DomainOffset() >= info.DomainOffset-int64(info.Limit) {
					backfilPDUs = append(backfilPDUs, v)
					if minOffset > v.DomainOffset() {
						minOffset = v.DomainOffset()
						minEventID = v.EventID()
					}
				}
			}
			if len(backfilPDUs) == info.Limit {
				break
			}
		}

		if !forwardFinished {
			resp, err = p.fedClient.Backfill(context.TODO(), gomatrixserverlib.ServerName(host), domain, roomID, limit, []string{maxEventID}, "f")
			if err != nil {
				log.Errorf("GetMissingEvents backfill error %v", err)
				return
			}
			if len(resp.PDUs) == 0 {
				forwardFinished = true
			}
			maxOffset := int64(math.MinInt64)
			for _, v := range resp.PDUs {
				if v.DomainOffset() < info.DomainOffset && v.DomainOffset() >= info.DomainOffset-int64(info.Limit) {
					backfilPDUs = append(backfilPDUs, v)
					if maxOffset < v.DomainOffset() {
						maxOffset = v.DomainOffset()
						maxEventID = v.EventID()
					}
				}
			}
			if len(backfilPDUs) == info.Limit {
				break
			}
		}
		if forwardFinished && backwardFinished {
			break
		}
	}
	for i := range backfilPDUs {
		backfilPDUs[i].SetDepth(-1)
	}
	sort.Slice(backfilPDUs, func(i, j int) bool { return backfilPDUs[i].EventNID() < backfilPDUs[j].EventNID() })
	_, err = p.rpcCli.InputRoomEvents(context.TODO(), &roomserverapi.RawEvent{
		RoomID: roomID,
		Kind:   roomserverapi.KindBackfill,
		Trust:  true,
		BulkEvents: roomserverapi.BulkEvent{
			Events: backfilPDUs,
		},
	})

	if err != nil {
		log.Errorf("GetMissingEvents insert error: %v, info: %v", err, info)
		p.push(info)
	} else {
		log.Debugf("GetMissingEvents finished: %s, %s, %d", info.RoomID, info.EventID, info.Limit)
		p.db.UpdateMissingEvents(context.TODO(), info.RoomID, info.EventID, true)
	}
}
