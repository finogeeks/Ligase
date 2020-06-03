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
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	fedrepos "github.com/finogeeks/ligase/federation/model/repos"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type PartitionProcessor interface {
	AssignRoomPartition(ctx context.Context, roomID, domain string, retryTime time.Duration, retryInterval time.Duration) (*fedrepos.RecordItem, bool)
	TryAssignRoomPartition(ctx context.Context, roomID, domain string) (*fedrepos.RecordItem, bool)
	UnassignRoomPartition(ctx context.Context, roomID, domain string)
	OnRoomDomainRelease(ctx context.Context, origin, roomID, domain string)
	HasAssgined(ctx context.Context, roomID, domain string) (*fedrepos.RecordItem, bool)
}

// OutgoingQueues is a collection of queues for sending transactions to other
// matrix servers
type OutgoingQueues struct {
	cfg                *config.Fed
	origin             gomatrixserverlib.ServerName
	fedClient          *client.FedClientWrap
	queues             sync.Map
	rpcCli             roomserverapi.RoomserverRPCAPI
	feddomains         *common.FedDomains
	rsRepo             *repos.RoomServerCurStateRepo
	recRepo            *fedrepos.SendRecRepo
	partitionProcessor PartitionProcessor
}

// NewOutgoingQueues makes a new OutgoingQueues
func NewOutgoingQueues(
	origin gomatrixserverlib.ServerName,
	fedClient *client.FedClientWrap,
	rpcCli roomserverapi.RoomserverRPCAPI,
	cfg *config.Fed,
	feddomains *common.FedDomains,
	rsRepo *repos.RoomServerCurStateRepo,
	recRepo *fedrepos.SendRecRepo,
	partitionProcessor PartitionProcessor,
) *OutgoingQueues {
	return &OutgoingQueues{
		cfg:                cfg,
		origin:             origin,
		fedClient:          fedClient,
		rpcCli:             rpcCli,
		feddomains:         feddomains,
		rsRepo:             rsRepo,
		recRepo:            recRepo,
		partitionProcessor: partitionProcessor,
	}
}

func (oqs *OutgoingQueues) getDestinationQueue(roomID, domain string) *destinationQueue {
	var oq *destinationQueue
	key := roomID + ":" + domain
	val, ok := oqs.queues.Load(key)
	if !ok {
		val, _ = oqs.queues.LoadOrStore(key, &destinationQueue{
			origin:             oqs.origin,
			roomID:             roomID,
			domain:             domain,
			feddomains:         oqs.feddomains,
			fedClient:          oqs.fedClient,
			rpcCli:             oqs.rpcCli,
			rsRepo:             oqs.rsRepo,
			recRepo:            oqs.recRepo,
			partitionProcessor: oqs.partitionProcessor,
		})
	}
	oq = val.(*destinationQueue)
	return oq
}

// SendEvent sends an event to the destinations
func (oqs *OutgoingQueues) SendEvent(
	ctx context.Context,
	partition int32,
	ev *gomatrixserverlib.Event,
	domain string,
	roomID string,
) error {
	if ev == nil {
		return nil
	}

	oq := oqs.getDestinationQueue(roomID, domain)
	oq.SendEvent(ctx, partition, ev)

	return nil
}

// SendEDU sends an EDU event to the destinations
func (oqs *OutgoingQueues) SendEDU(
	ctx context.Context,
	partition int32,
	e *gomatrixserverlib.EDU,
	domain string,
	roomID string,
) error {
	if e == nil {
		return nil
	}

	oq := oqs.getDestinationQueue(roomID, domain)
	oq.SendEDU(ctx, partition, e)

	return nil
}

func (oqs *OutgoingQueues) RetrySend(ctx context.Context, roomID, domain string) {
	oq := oqs.getDestinationQueue(roomID, domain)
	oq.RetrySend(ctx)
}

func (oqs *OutgoingQueues) Release(ctx context.Context, roomID, domain string) {
	oqs.queues.Delete(roomID + ":" + domain)
}
