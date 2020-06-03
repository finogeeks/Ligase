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
