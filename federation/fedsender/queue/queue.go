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
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type RecordItem struct {
	RoomID string
	Domain string

	EventID     string
	SendTimes   int32
	PendingSize int32
}

// OutgoingQueues is a collection of queues for sending transactions to other
// matrix servers
type OutgoingQueues struct {
	cfg         *config.Dendrite
	origin      gomatrixserverlib.ServerName
	sentCounter *int64
	// client     *gomatrixserverlib.FederationClient
	fedClient  *client.FedClientWrap
	queues     sync.Map
	rpcCli     roomserverapi.RoomserverRPCAPI
	db         model.FederationDatabase
	feddomains *common.FedDomains
}

// NewOutgoingQueues makes a new OutgoingQueues
func NewOutgoingQueues(
	origin gomatrixserverlib.ServerName,
	sentCounter *int64,
	fedClient *client.FedClientWrap,
	rpcCli roomserverapi.RoomserverRPCAPI,
	db model.FederationDatabase,
	cfg *config.Dendrite,
	feddomains *common.FedDomains,
) *OutgoingQueues {
	return &OutgoingQueues{
		cfg:         cfg,
		origin:      origin,
		sentCounter: sentCounter,
		fedClient:   fedClient,
		rpcCli:      rpcCli,
		db:          db,
		feddomains:  feddomains,
	}
}

// SendEvent sends an event to the destinations
func (oqs *OutgoingQueues) SendEvent(
	ev *gomatrixserverlib.Event,
	origin gomatrixserverlib.ServerName,
	destinations []gomatrixserverlib.ServerName,
	roomID string,
	recItems map[string]*RecordItem,
) error {
	if ev == nil {
		return nil
	}
	// if origin != oqs.origin {
	// 	// TODO: Support virtual hosting; gh issue #577.
	// 	return fmt.Errorf(
	// 		"sendevent: unexpected server to send as: got %q expected %q",
	// 		origin, oqs.origin,
	// 	)
	// }

	// Remove our own server from the list of destinations.
	destinations = filterDestinations(oqs.origin, destinations)

	// log.Infof("Sending event destinations: %v, event: %s", destinations, ev.EventID())
	for _, domain := range destinations {
		oq := oqs.getDestinationQueue(string(domain), roomID, true, recItems[string(domain)])
		if oq == nil {
			log.Warnf("OutgoingQueues.SendEvent connector not in config file, domain: %s", domain)
			continue
		}
		oq.SendEvent(ev)
	}

	return nil
}

// SendStates sends init state events to the destinations
func (oqs *OutgoingQueues) SendStates(
	evs []*gomatrixserverlib.Event,
	origin gomatrixserverlib.ServerName,
	destinations []gomatrixserverlib.ServerName,
	roomID string,
	recItems map[string]*RecordItem,
) error {
	if len(evs) <= 0 {
		return nil
	}
	// if origin != oqs.origin {
	// 	// TODO: Support virtual hosting; gh issue #577.
	// 	return fmt.Errorf(
	// 		"sendevent: unexpected server to send as: got %q expected %q",
	// 		origin, oqs.origin,
	// 	)
	// }

	// Remove our own server from the list of destinations.
	destinations = filterDestinations(oqs.origin, destinations)

	// log.Infof("Sending event destinations: %v, event: %s", destinations, ev.EventID())
	for _, domain := range destinations {
		oq := oqs.getDestinationQueue(string(domain), roomID, true, recItems[string(domain)])
		if oq == nil {
			log.Warnf("OutgoingQueues.SendStates connector not in config file, domain: %s", domain)
			continue
		}
		oq.SendStates(evs)
	}

	return nil
}

func (oqs *OutgoingQueues) RetrySendBackfillEvs(recordItem *RecordItem) {
	oq := oqs.getDestinationQueue(recordItem.Domain, recordItem.RoomID, true, recordItem)
	if oq == nil {
		log.Warnf("OutgoingQueues.RetrySendBackfillEvs connector not in config file, domain: %s", recordItem.Domain)
		return
	}
	oq.RetrySendBackfillEvs()
}

func (oqs *OutgoingQueues) getDestinationQueue(domain, roomID string, create bool, recItem *RecordItem) *destinationQueue {
	key := domain + ":" + roomID
	var oq *destinationQueue
	val, ok := oqs.queues.Load(key)
	if (!ok || val == nil) && create {
		//destination, _ := oqs.feddomains.GetDomainHost(domain)
		oq = &destinationQueue{
			origin:      oqs.origin,
			domain:      domain,
			sentCounter: oqs.sentCounter,
			feddomains:  oqs.feddomains,
			fedClient:   oqs.fedClient,
			rpcCli:      oqs.rpcCli,
			db:          oqs.db,
			recItem:     recItem,
		}
		val, loaded := oqs.queues.LoadOrStore(key, oq)
		if loaded {
			oq = val.(*destinationQueue)
		}
	} else if val != nil {
		oq = val.(*destinationQueue)
	}
	return oq
}

// SendEDU sends an EDU event to the destinations
func (oqs *OutgoingQueues) SendEDU(
	e *gomatrixserverlib.EDU,
	origin gomatrixserverlib.ServerName,
	destinations []gomatrixserverlib.ServerName,
	roomID string,
	recItems map[string]*RecordItem,
) error {
	if e == nil {
		return nil
	}
	// if origin != oqs.origin {
	// 	// TODO: Support virtual hosting; gh issue #577.
	// 	return fmt.Errorf(
	// 		"sendevent: unexpected server to send as: got %q expected %q",
	// 		origin, oqs.origin,
	// 	)
	// }

	// Remove our own server from the list of destinations.
	destinations = filterDestinations(oqs.origin, destinations)

	if len(destinations) > 0 {
		log.Infof("Sending EDU event, destinations: %v, edu_type: %s", destinations, e.Type)
	}

	for _, domain := range destinations {
		var recItem *RecordItem
		if roomID != "" {
			recItem = recItems[string(domain)]
		}
		oq := oqs.getDestinationQueue(string(domain), roomID, true, recItem)
		if oq == nil {
			log.Warnf("OutgoingQueues.SendEDU connector not in config file, domain: %s", domain)
			continue
		}
		oq.SendEDU(e)
	}

	return nil
}

// filterDestinations removes our own server from the list of destinations.
// Otherwise we could end up trying to talk to ourselves.
func filterDestinations(origin gomatrixserverlib.ServerName, destinations []gomatrixserverlib.ServerName) []gomatrixserverlib.ServerName {
	var result []gomatrixserverlib.ServerName
	for _, destination := range destinations {
		if destination == origin {
			continue
		}
		result = append(result, destination)
	}
	return result
}
