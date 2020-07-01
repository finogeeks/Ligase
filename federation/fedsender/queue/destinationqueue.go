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

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
)

// destinationQueue is a queue of events for a single destination.
// It is responsible for sending the events to the destination and
// ensures that only one request is in flight to a given destination
// at a time.
type destinationQueue struct {
	// client     *gomatrixserverlib.FederationClient
	fedClient  *client.FedClientWrap
	origin     gomatrixserverlib.ServerName
	domain     string
	feddomains *common.FedDomains
	// destination string
	// The running mutex protects running, sentCounter, lastTransactionIDs and
	// pendingEvents, pendingEDUs.
	runningMutex       sync.Mutex
	running            bool
	sentCounter        *int64
	lastTransactionIDs []gomatrixserverlib.TransactionID
	needSendMissEv     bool
	states             []*gomatrixserverlib.Event
	stateEvents        []*gomatrixserverlib.Event
	backfillEvents     []gomatrixserverlib.Event
	backfillFailSize   int32
	pendingEvents      []*gomatrixserverlib.Event
	pendingEDUs        []*gomatrixserverlib.EDU
	recItem            *RecordItem
	retryingSize       int32
	rpcCli             roomserverapi.RoomserverRPCAPI
	db                 model.FederationDatabase
}

// Send event adds the event to the pending queue for the destination.
// If the queue is empty then it starts a background goroutine to
// start sending events to that destination.
func (oq *destinationQueue) SendEvent(ev *gomatrixserverlib.Event) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()
	oq.pendingEvents = append(oq.pendingEvents, ev)
	if !oq.running {
		oq.running = true
		go oq.backgroundSend()
	}
}

func (oq *destinationQueue) SendStates(evs []*gomatrixserverlib.Event) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()
	oq.stateEvents = evs
	if !oq.running {
		oq.running = true
		go oq.backgroundSend()
	}
}

func (oq *destinationQueue) sendBackfillEvents(evs []gomatrixserverlib.Event) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()
	oq.backfillEvents = append(oq.backfillEvents, evs...)
	if !oq.running {
		oq.running = true
		go oq.backgroundSend()
	}
}

// SendEDU adds the EDU event to the pending queue for the destination.
// If the queue is empty then it starts a background goroutine to
// start sending event to that destination.
func (oq *destinationQueue) SendEDU(e *gomatrixserverlib.EDU) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()
	oq.pendingEDUs = append(oq.pendingEDUs, e)
	if !oq.running {
		oq.running = true
		go oq.backgroundSend()
	}
}

func (oq *destinationQueue) backgroundSend() {
	for {
		destination, ok := oq.feddomains.GetDomainHost(oq.domain)
		if !ok {
			time.Sleep(time.Second * 10)
			continue
		}
		t, eventID, decrementSize := oq.next(destination)
		if t == nil {
			// If the queue is empty then stop processing for this destination.
			// TODO: Remove this destination from the queue map.
			return
		}

		// TODO: handle retries.
		// TODO: blacklist uncooperative servers.

		oq.mustSend(t, eventID, decrementSize)
	}
}

func (oq *destinationQueue) mustSend(t *gomatrixserverlib.Transaction, eventID string, decrementSize int32) {
	retryTimes := 0
	for {
		destination, ok := oq.feddomains.GetDomainHost(oq.domain)
		if !ok {
			retryTimes++
			if retryTimes < 30 {
				time.Sleep(time.Second * time.Duration(retryTimes) * 10)
			} else {
				time.Sleep(time.Second * 300)
			}
			continue
		} else {
			retryTimes = 0
		}
		t.Destination = gomatrixserverlib.ServerName(destination)
		// _, err := oq.client.SendTransaction(context.TODO(), *t)
		_, err := oq.fedClient.SendTransaction(context.TODO(), *t)
		if err != nil {
			if strings.Contains(err.Error(), "Backfill not finished") {
				log.Infof("sending transaction, destination: %v, backfill not finished", t.Destination)
			} else {
				log.Errorf("problem sending transaction, destination: %v, error: %v", t.Destination, err)
			}
			retryTimes++
			if retryTimes < 10 {
				time.Sleep(time.Second * 3)
			} else if retryTimes < 30 {
				time.Sleep(time.Second * time.Duration(retryTimes) * 10)
			} else {
				time.Sleep(time.Second * 300)
			}
		} else {
			// 落地最后成功 eventID
			pendingDiff := -decrementSize
			if pendingDiff != 0 {
				recItem := oq.recItem
				atomic.AddInt32(&recItem.PendingSize, pendingDiff)
				atomic.AddInt32(&recItem.SendTimes, 1)
				recItem.EventID = eventID
				err = oq.db.UpdateSendRecordPendingSizeAndEventID(context.TODO(), recItem.RoomID, oq.domain, pendingDiff, eventID)
				if err != nil {
					log.Errorf("UpdateSendRecordPendingSizeAndEventID %s %d %s error %v", recItem.RoomID, pendingDiff, eventID, err)
				}
			}
			break
		}
	}
}

func (oq *destinationQueue) RetrySendBackfillEvs() {
	oq.retryingSize = oq.recItem.PendingSize
	go oq.startSendBackfillEvs(oq.recItem.PendingSize, oq.recItem.EventID)
}

func (oq *destinationQueue) startSendBackfillEvs(size int32, lastEventID string) {
	log.Infof("retry send backfill events, %v", oq.recItem)

	leftSize := size
	retryTimes := 0
	waitTimes := 0

	sleep := func(err error) {
		retryTimes++
		waitTimes++
		if waitTimes > 10 && err != nil {
			log.Errorf("fed-sender queue backfill retyr-times: %d, roomID: %s, pendingSize: %d, err: %v", waitTimes, oq.recItem.RoomID, size, err)
		}
		sleepSec := waitTimes
		if sleepSec > 60 {
			sleepSec = 60
		}
		time.Sleep(time.Second * time.Duration(sleepSec))
	}

	lastDomainOffset := int64(0)
	for leftSize > 0 {
		batchSize := int(leftSize)
		if batchSize > 50 {
			batchSize = 50
		}
		pdus, err := oq.getBackfillEvs(batchSize, lastEventID, "f")
		if err != nil {
			if waitTimes > 3 {
				break
			}
			sleep(err)
			continue
		}

		log.Infof("fed-sender queue backfill, batchSize: %d, leftSize: %d resp %#v", batchSize, leftSize, pdus)
		lossCount := 0
		if len(pdus) > 0 {
			if lastDomainOffset == 0 {
				lastDomainOffset = pdus[0].DomainOffset() - 1
			}
			for _, pdu := range pdus {
				domainOffset := pdu.DomainOffset()
				if domainOffset != lastDomainOffset+1 {
					log.Errorf("fed retry send events domainOffset wrong, roomID: %s, lastDomainOffset: %d, domainOffset: %d", oq.recItem.RoomID, lastDomainOffset, domainOffset)
					if domainOffset > lastDomainOffset {
						lossCount += int(domainOffset - lastDomainOffset - 1)
					} else {
						log.Errorf("fed retry send events domainOffset not sorted, roomID: %s, lastDomainOffset: %d, domainOffset: %d", oq.recItem.RoomID, lastDomainOffset, domainOffset)
					}
				}
				lastDomainOffset = domainOffset
			}
		}
		if len(pdus) > 0 {
			leftSize -= int32(len(pdus)) + int32(lossCount)
			lastEventID = pdus[len(pdus)-1].EventID()
			oq.sendBackfillEvents(pdus)
		}
		if len(pdus) == 0 && leftSize > 0 {
			log.Errorf("fed retry send events leftSize, roomID: %s, pendingSize: %d, backfillSize: %d", oq.recItem.RoomID, size, size-leftSize)
			if waitTimes > 3 {
				break
			}
			sleep(nil)
			continue
		}
		waitTimes = 0
	}
	if leftSize > 0 {
		log.Errorf("fed retry send events failed. roomID: %s, from: %s, size: %d", oq.recItem.RoomID, lastEventID, leftSize)
		atomic.AddInt32(&oq.recItem.PendingSize, -leftSize)
		oq.db.UpdateSendRecordPendingSizeAndEventID(context.TODO(), oq.recItem.RoomID, oq.domain, -leftSize, oq.recItem.EventID)
		oq.runningMutex.Lock()
		defer oq.runningMutex.Unlock()
		oq.backfillFailSize = leftSize
		oq.retryingSize = 0
	}
}

func (oq *destinationQueue) getBackfillEvs(size int, lastEventID, dir string) ([]gomatrixserverlib.Event, error) {
	var req roomserverapi.QueryBackFillEventsRequest
	var resp roomserverapi.QueryBackFillEventsResponse
	req.EventID = lastEventID
	req.Limit = size
	req.RoomID = oq.recItem.RoomID
	req.Dir = dir
	log.Infof("fed-sender queue query backfill events %#v", req)
	err := oq.rpcCli.QueryBackFillEvents(context.TODO(), &req, &resp)
	if err != nil {
		log.Errorf("Backfill err %v", err)
		return nil, err
	}

	return resp.PDUs, nil
}

// next creates a new transaction from the pending event queue
// and flushes the queue.
// Returns nil if the queue was empty.
func (oq *destinationQueue) next(destination string) (*gomatrixserverlib.Transaction, string, int32) {
	oq.runningMutex.Lock()
	defer oq.runningMutex.Unlock()

	if oq.stateEvents != nil {
		var eventID string
		pdus := []gomatrixserverlib.Event{}
		for _, ev := range oq.stateEvents {
			pdus = append(pdus, *ev)
			eventID = ev.EventID()
		}
		oq.states = oq.stateEvents
		oq.stateEvents = nil
		oq.needSendMissEv = true

		counter := atomic.AddInt64(oq.sentCounter, int64(len(pdus)))
		t := oq.newTransaction(destination, counter)
		t.PDUs = pdus
		oq.lastTransactionIDs = []gomatrixserverlib.TransactionID{t.TransactionID}

		data, _ := json.Marshal(t.PDUs)
		log.Debugf("send state room: %s, states: %s", oq.recItem.RoomID, data)
		return t, eventID, int32(len(t.PDUs))
	}

	if len(oq.backfillEvents) > 0 {
		pdus := oq.backfillEvents
		if len(pdus) > 50 {
			pdus = oq.backfillEvents[:50]
			oq.backfillEvents = oq.backfillEvents[50:]
		} else {
			oq.backfillEvents = nil
		}

		lossCount := 0
		lastDomainOffset := pdus[0].DomainOffset() - 1
		for _, pdu := range pdus {
			domainOffset := pdu.DomainOffset()
			if domainOffset != lastDomainOffset+1 {
				if domainOffset > lastDomainOffset {
					lossCount += int(domainOffset - lastDomainOffset - 1)
				}
			}
		}

		counter := atomic.AddInt64(oq.sentCounter, int64(len(pdus)))
		t := oq.newTransaction(destination, counter)
		t.PDUs = pdus
		oq.lastTransactionIDs = []gomatrixserverlib.TransactionID{t.TransactionID}

		oq.retryingSize -= int32(len(pdus)) + int32(lossCount)

		eventID := pdus[len(pdus)-1].EventID()
		return t, eventID, int32(len(pdus))
	}

	if oq.retryingSize <= 0 && (len(oq.pendingEvents) != 0 || len(oq.pendingEDUs) != 0) {
		var eventID string
		count := 0

		var pdus []gomatrixserverlib.Event
		for _, ev := range oq.pendingEvents {
			pdus = append(pdus, *ev)
			eventID = ev.EventID()
			count++
			if count >= 50 {
				break
			}
		}
		if count == len(oq.pendingEvents) {
			oq.pendingEvents = nil
		} else {
			oq.pendingEvents = oq.pendingEvents[50:]
		}

		var edus []gomatrixserverlib.EDU
		for _, edu := range oq.pendingEDUs {
			edus = append(edus, *edu)
		}
		oq.pendingEDUs = nil
		counter := atomic.AddInt64(oq.sentCounter, int64(len(pdus)+len(edus)))
		t := oq.newTransaction(destination, counter)
		t.PDUs = pdus
		t.EDUs = edus
		oq.lastTransactionIDs = []gomatrixserverlib.TransactionID{t.TransactionID}

		decrementSize := int32(len(t.PDUs))
		if oq.backfillFailSize > 0 {
			decrementSize += oq.backfillFailSize
			oq.backfillFailSize = 0
		}

		return t, eventID, decrementSize
	}

	oq.running = false
	return nil, "", 0
}

func (oq *destinationQueue) newTransaction(destination string, counter int64) *gomatrixserverlib.Transaction {
	var t gomatrixserverlib.Transaction
	now := gomatrixserverlib.AsTimestamp(time.Now())
	t.TransactionID = gomatrixserverlib.TransactionID(fmt.Sprintf("%d-%d", now, counter))
	t.Origin = oq.origin
	t.Destination = gomatrixserverlib.ServerName(destination)
	t.OriginServerTS = now
	t.PreviousIDs = oq.lastTransactionIDs
	if t.PreviousIDs == nil {
		t.PreviousIDs = []gomatrixserverlib.TransactionID{}
	}

	return &t
}
