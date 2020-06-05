// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/federation/client"
	fedrepos "github.com/finogeeks/ligase/federation/model/repos"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

// destinationQueue is a queue of events for a single destination.
// It is responsible for sending the events to the destination and
// ensures that only one request is in flight to a given destination
// at a time.
type destinationQueue struct {
	fedClient  *client.FedClientWrap
	rsRepo     *repos.RoomServerCurStateRepo
	recRepo    *fedrepos.SendRecRepo
	origin     gomatrixserverlib.ServerName
	roomID     string
	domain     string
	feddomains *common.FedDomains
	// The running mutex protects running, sentCounter, lastTransactionIDs and
	// pendingEvents, pendingEDUs.
	runningMutex       sync.Mutex
	running            bool
	sentCounter        int
	lastTransactionIDs []gomatrixserverlib.TransactionID
	pendingEvents      []*gomatrixserverlib.Event
	pendingEDUs        []*gomatrixserverlib.EDU
	rpcCli             roomserverapi.RoomserverRPCAPI
	partitionProcessor PartitionProcessor
}

func (oq *destinationQueue) RetrySend(ctx context.Context) {
}

// Send event adds the event to the pending queue for the destination.
// If the queue is empty then it starts a background goroutine to
// start sending events to that destination.
func (oq *destinationQueue) SendEvent(ctx context.Context, partition int32, ev *gomatrixserverlib.Event) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()
	oq.pendingEvents = append(oq.pendingEvents, ev)
	if !oq.running {
		oq.running = true
		go oq.backgroundSend(ctx)
	}
}

// SendEDU adds the EDU event to the pending queue for the destination.
// If the queue is empty then it starts a background goroutine to
// start sending event to that destination.
func (oq *destinationQueue) SendEDU(ctx context.Context, partition int32, e *gomatrixserverlib.EDU) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()
	oq.pendingEDUs = append(oq.pendingEDUs, e)
	if !oq.running {
		oq.running = true
		go oq.backgroundSend(ctx)
	}
}

func (oq *destinationQueue) backgroundSend(ctx context.Context) {
	retryTimes := 0
	for {
		log.Debugf("fed send start baground send %s %s", oq.roomID, oq.domain)
		var recItem *fedrepos.RecordItem
		if oq.roomID != "" {
			var ok bool
			recItem, ok = oq.partitionProcessor.HasAssgined(ctx, oq.roomID, oq.domain)
			log.Debugf("fed send get has rec item %s %s %v, %t", oq.roomID, oq.domain, recItem, ok)
			if !ok {
				recItem, ok = oq.partitionProcessor.AssignRoomPartition(ctx, oq.roomID, oq.domain, time.Second*60, time.Millisecond*100)
				log.Debugf("fed send get assign rec item %s %s %v, %t", oq.roomID, oq.domain, recItem, ok)
				if !ok {
					oq.partitionProcessor.OnRoomDomainRelease(ctx, string(oq.origin), oq.roomID, oq.domain)
					return
				}
			}
			log.Debugf("fed send room %s target %s pending %d", oq.roomID, oq.domain, recItem.PendingSize)
		}

		destination, ok := oq.feddomains.GetDomainHost(oq.domain)
		if !ok {
			time.Sleep(time.Second * 10)
			if oq.roomID != "" {
				oq.partitionProcessor.UnassignRoomPartition(ctx, oq.roomID, oq.domain)
			}
			continue
		}
		t, eventID, decrementSize, usePDUSize, useEDUSize := oq.next(ctx, recItem, destination)
		if (t == nil || (len(t.PDUs) == 0 && len(t.EDUs) == 0)) && !oq.running {
			oq.partitionProcessor.OnRoomDomainRelease(ctx, string(oq.origin), oq.roomID, oq.domain)
			return
		}
		failed := false
		if t == nil || (len(t.PDUs) == 0 && len(t.EDUs) == 0) {
			failed = true
		} else {
			log.Debugf("fed send begin room %s target %s size %d desrSize %d useSize %d", oq.roomID, oq.domain, len(t.PDUs), decrementSize, usePDUSize)
			err := oq.trySend(t)
			if err != nil {
				log.Errorf("fedsender err room:%s domain:%s %v", oq.roomID, oq.domain, err)
				failed = true
			} else {
				// 落地最后成功
				oq.pendingEvents = oq.pendingEvents[usePDUSize:]
				if len(oq.pendingEvents) == 0 {
					oq.pendingEvents = nil
				}
				oq.pendingEDUs = oq.pendingEDUs[useEDUSize:]
				if len(oq.pendingEDUs) == 0 {
					oq.pendingEDUs = nil
				}
				if recItem != nil {
					domainOffset := recItem.DomainOffset
					if len(t.PDUs) != 0 {
						update := t.PDUs[len(t.PDUs)-1].DomainOffset()
						if update+1 > domainOffset {
							domainOffset = update + 1
						}
						log.Debugf("fed send update domain offset ", oq.roomID, oq.domain, eventID, domainOffset, decrementSize, recItem.PendingSize)
						oq.recRepo.UpdateDomainOffset(ctx, oq.roomID, oq.domain, eventID, domainOffset, decrementSize)
					}
				}
			}
		}
		if failed {
			second := retryTimes + 5
			if second > 600 {
				second = 600
			}
			if oq.roomID != "" {
				oq.partitionProcessor.UnassignRoomPartition(ctx, oq.roomID, oq.domain)
			}
			time.Sleep(time.Second * time.Duration(second))
			retryTimes++
		} else {
			retryTimes = 0
		}
	}
}

// next creates a new transaction from the pending event queue
// and flushes the queue.
// Returns nil if the queue was empty.
func (oq *destinationQueue) next(ctx context.Context, recItem *fedrepos.RecordItem, destination string) (*gomatrixserverlib.Transaction, string, int32, int, int) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()

	const MAX_SIZE = 50

	edus := make([]gomatrixserverlib.EDU, 0, len(oq.pendingEDUs))
	if len(oq.pendingEDUs) > 0 {
		for i, v := range oq.pendingEDUs {
			edus = append(edus, *v)
			if i > MAX_SIZE {
				break
			}
		}
	}
	if oq.roomID == "" {
		if len(edus) > 0 {
			t := oq.newTransaction(destination, nil, edus)
			return t, "", 0, 0, len(edus)
		} else {
			oq.running = false
			return nil, "", 0, 0, 0
		}
	}

	if oq.roomID != "" && (recItem == nil || recItem.SendTimes == 0) {
		pdus := oq.getStateEvs(ctx)
		if pdus != nil {
			domainOffset := int64(0)
			if recItem == nil {
				if len(oq.pendingEvents) > 0 {
					domainOffset = oq.pendingEvents[0].DomainOffset()
				}
				var err error
				recItem, err = oq.recRepo.StoreRec(ctx, oq.roomID, oq.domain, domainOffset)
				if err != nil {
					log.Errorf("Store send record error %v", err)
					if len(edus) > 0 {
						t := oq.newTransaction(destination, nil, edus)
						return t, "", 0, 0, len(edus)
					} else {
						oq.running = false
						return nil, "", 0, 0, 0
					}
				}
			} else {
				domainOffset = recItem.DomainOffset
			}

			var eventID string
			for i := len(pdus) - 1; i >= 0; i-- {
				ev := pdus[i]
				if domainOffset != 0 && ev.DomainOffset() >= domainOffset {
					pdus = append(pdus[:i], pdus[i+1:]...)
					continue
				}
				eventID = ev.EventID()
			}
			oq.sentCounter += len(pdus)
			t := oq.newTransaction(destination, pdus, edus)
			oq.lastTransactionIDs = []gomatrixserverlib.TransactionID{t.TransactionID}

			data, _ := json.Marshal(pdus)
			log.Debugf("send state room: %s, states: %s", oq.roomID, data)
			return t, eventID, 0, 0, len(edus)
		}
	}

	if recItem != nil && recItem.PendingSize > 0 {
		var pdus []gomatrixserverlib.Event
		getSize := int(recItem.PendingSize)
		useSize := 0
		if len(oq.pendingEvents) > 0 {
			domainOffset := oq.pendingEvents[0].DomainOffset()
			if recItem.DomainOffset == domainOffset {
				getSize = 0
				for i, v := range oq.pendingEvents {
					useSize++
					pdus = append(pdus, *v)
					if i > MAX_SIZE {
						break
					}
				}
			}
		}

		if getSize > MAX_SIZE {
			getSize = MAX_SIZE
		}
		log.Infof("fed send room %s target %s usepending %d getSize %d", oq.roomID, oq.domain, len(pdus), getSize)
		if getSize > 0 {
			getPDUs := oq.getPendingEvs(ctx, recItem.DomainOffset, recItem.EventID, getSize)
			pdus = append(pdus, getPDUs...)
		}
		if len(pdus) == 0 {
			if len(edus) > 0 {
				t := oq.newTransaction(destination, nil, edus)
				return t, "", 0, 0, len(edus)
			} else {
				return nil, "", 0, 0, 0
			}
		}

		t := oq.newTransaction(destination, pdus, edus)
		oq.lastTransactionIDs = []gomatrixserverlib.TransactionID{t.TransactionID}
		oq.sentCounter += len(pdus)

		eventID := pdus[len(pdus)-1].EventID()
		return t, eventID, int32(len(pdus)), useSize, len(edus)
	}
	oq.running = false
	if len(edus) > 0 {
		t := oq.newTransaction(destination, nil, edus)
		return t, "", 0, 0, len(edus)
	} else {
		return nil, "", 0, 0, 0
	}
}

func (oq *destinationQueue) newTransaction(destination string, pdus []gomatrixserverlib.Event, edus []gomatrixserverlib.EDU) *gomatrixserverlib.Transaction {
	var t gomatrixserverlib.Transaction
	now := gomatrixserverlib.AsTimestamp(time.Now())
	t.TransactionID = gomatrixserverlib.TransactionID(fmt.Sprintf("%d-%d", now, oq.sentCounter))
	t.Origin = oq.origin
	t.Destination = gomatrixserverlib.ServerName(destination)
	t.OriginServerTS = now
	t.PreviousIDs = oq.lastTransactionIDs
	t.PDUs = pdus
	t.EDUs = edus
	if t.PreviousIDs == nil {
		t.PreviousIDs = []gomatrixserverlib.TransactionID{}
	}

	return &t
}

func (oq *destinationQueue) trySend(t *gomatrixserverlib.Transaction) error {
	_, err := oq.fedClient.SendTransaction(context.TODO(), *t)
	if err != nil {
		if strings.Contains(err.Error(), "Backfill not finished") {
			log.Infof("sending transaction, destination: %v, backfill not finished", t.Destination)
		} else {
			log.Errorf("problem sending transaction, destination: %v, error: %v", t.Destination, err)
		}
		return err
	}
	return nil
}

func (oq *destinationQueue) getStateEvs(ctx context.Context) []gomatrixserverlib.Event {
	rs := oq.rsRepo.GetRoomState(ctx, oq.roomID)
	states := rs.GetAllState()
	pdus := make([]gomatrixserverlib.Event, 0, len(states))
	var sender string
	for i := 0; i < len(states); i++ {
		state := &states[i]
		stateDomain, _ := common.DomainFromID(state.Sender())
		log.Infof("room:%s type:%s evntID:%s state-domain:%s target-domain:%s", state.RoomID(), state.Type(), state.EventID(), stateDomain, oq.domain)
		if state.Type() == "m.room.create" {
			sender = state.Sender()
		}
		pdus = append(pdus, *state)
	}
	if len(pdus) == 0 {
		return nil
	}

	senderDomain, _ := utils.DomainFromID(sender)
	if senderDomain == oq.domain {
		log.Infof("fed-send target domain is sender domain, ignore sending state %s", oq.domain)
		return nil
	}
	return pdus
}

func (oq *destinationQueue) getPendingEvs(ctx context.Context, domainOffset int64, eventID string, limit int) []gomatrixserverlib.Event {
	request := roomserverapi.QueryEventsByDomainOffsetRequest{
		RoomID:       oq.roomID,
		Domain:       string(oq.origin),
		DomainOffset: domainOffset,
		EventID:      eventID,
		Limit:        limit,
	}
	if domainOffset == 0 && eventID != "" {
		request.UseEventID = true
	}
	var response roomserverapi.QueryEventsByDomainOffsetResponse
	err := oq.rpcCli.QueryEventsByDomainOffset(ctx, &request, &response)
	if err != nil {
		log.Errorf("query events by domainOffset error %v", err)
		return nil
	}
	if response.Error != "" {
		log.Errorf("query events by domainOffset error by roomserver %s", response.Error)
		return nil
	}

	// TODO: cjw check loss
	return response.PDUs
}
