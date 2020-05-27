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

package fedsender

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/federation/federationapi/rpc"
	"github.com/finogeeks/ligase/federation/fedsender/queue"
	fedrepos "github.com/finogeeks/ligase/federation/model/repos"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const (
	kGetPendingCount = 10
)

type ServerName = gomatrixserverlib.ServerName

type FedSendMsg struct {
	partition int32
	domain    string
	roomID    string
	ev        *gomatrixserverlib.Event
	edu       *gomatrixserverlib.EDU
}

type FederationSender struct {
	cfg        *config.Fed
	rsRepo     *repos.RoomServerCurStateRepo
	recRepo    *fedrepos.SendRecRepo
	queuesMap  sync.Map
	domainMap  sync.Map
	fedRpcCli  roomserverapi.RoomserverRPCAPI
	feddomains *common.FedDomains
	chanSize   uint32
	msgChan    []chan common.ContextMsg

	fakePartition int32

	assignedRoomDomain sync.Map
}

func NewFederationSender(cfg *config.Fed, rpcClient *common.RpcClient, feddomains *common.FedDomains) *FederationSender {
	fedRpcCli := rpc.NewFederationRpcClient(cfg, rpcClient, nil, nil, nil)
	sender := &FederationSender{
		cfg:        cfg,
		fedRpcCli:  fedRpcCli,
		feddomains: feddomains,
		chanSize:   16,
	}

	sender.msgChan = make([]chan common.ContextMsg, sender.chanSize)
	for i := 0; i < len(sender.msgChan); i++ {
		sender.msgChan[i] = make(chan common.ContextMsg, 32)
		go sender.startWorker(sender.msgChan[i])
	}
	return sender
}

func (c *FederationSender) SetRsRepo(repo *repos.RoomServerCurStateRepo) {
	c.rsRepo = repo
}

func (c *FederationSender) SetRecRepo(repo *fedrepos.SendRecRepo) {
	c.recRepo = repo
}

func (c *FederationSender) Start() {
	span, ctx := common.StartSobSomSpan(context.Background(), "FederationSender.Start")
	defer span.Finish()

	c.fakePartition = c.recRepo.GenerateFakePartition(ctx)
	if c.fakePartition == 0 {
		panic("fedsender get fake partition error")
	}

	oqs := c.getQueue(domain.FirstDomain) // TODO: cjw
	pendingRoomIDs, pendingDomains := c.recRepo.GetPenddingRooms(ctx, c.fakePartition, kGetPendingCount)
	if len(pendingRoomIDs) > 0 && len(pendingDomains) > 0 {
		for i := 0; i < len(pendingRoomIDs); i++ {
			oqs.RetrySend(ctx, pendingRoomIDs[i], pendingDomains[i])
		}
	}

	go func() {
		for {
			time.Sleep(time.Second * 10)
			ctx := context.TODO()
			c.assignedRoomDomain.Range(func(k, v interface{}) bool {
				key := k.(string)
				ss := strings.Split(key, "|")
				if len(ss) != 2 {
					log.Errorf("invalid assign room domain %s", key)
					return true
				}
				c.recRepo.ExpireRoomPartition(ctx, ss[0], ss[1])
				return true
			})
		}
	}()
}

func (c *FederationSender) AddConsumer(domain string) error {
	underlying := c.cfg.Kafka.Consumer.SenderInput.Underlying
	val, ok := common.GetTransportMultiplexer().GetNode(underlying)
	if ok {
		tran := val.(core.ITransport)
		name := domain
		topic := c.cfg.Kafka.Consumer.SenderInput.Topic + "." + domain

		_, ok := common.GetTransportMultiplexer().GetChannel(underlying, name)
		if !ok {
			c.domainMap.Store(topic, domain)
			tran.AddChannel(core.CHANNEL_SUB, name, topic, c.cfg.Kafka.Consumer.SenderInput.Group, &c.cfg.Kafka.Consumer.SenderInput)
			val, ok := common.GetTransportMultiplexer().GetChannel(underlying, name)
			if ok {
				log.Infof("fed-send add channel name:%s topic:%s", name, topic)
				channel := val.(core.IChannel)
				channel.SetHandler(c)
				if common.GetTransportMultiplexer().PreStartChannel(underlying, name) == false {
					log.Errorf("fed-send pre-start channel name:%s topic:%s fail", name, topic)
					return errors.New("addConsumer pre-start channel fail")
				}
				channel.Start()
				return nil
			}
		}
	}

	return errors.New("addConsumer can't find transport " + underlying)
}

func (c *FederationSender) getQueue(domain string) *queue.OutgoingQueues {
	v, ok := c.queuesMap.Load(domain)
	if !ok {
		if !common.CheckValidDomain(domain, c.cfg.GetServerName()) {
			return nil
		}
		fedClient, _ := client.GetFedClient(domain)
		queues := queue.NewOutgoingQueues(
			ServerName(domain),
			fedClient,
			c.fedRpcCli,
			c.cfg,
			c.feddomains,
			c.rsRepo,
			c.recRepo,
			c,
		)
		v, _ = c.queuesMap.LoadOrStore(domain, queues)
	}
	return v.(*queue.OutgoingQueues)
}

func (c *FederationSender) getDomainByTopic(topic string) (string, bool) {
	v, ok := c.domainMap.Load(topic)
	if !ok {
		return "", false
	}
	return v.(string), true
}

func (c *FederationSender) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		ctx := msg.Ctx
		data := msg.Msg.(FedSendMsg)
		if data.ev != nil {
			c.processEvent(ctx, data.partition, data.domain, data.roomID, data.ev)
		}
		if data.edu != nil {
			c.processEdu(ctx, data.partition, data.domain, data.roomID, data.edu)
		}
	}
}

func (c *FederationSender) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	ev := new(gomatrixserverlib.Event)
	if err := json.Unmarshal(data, ev); err != nil {
		log.Errorf("fed-sender: message parse failure err:%v", err)
		return
	}

	log.Infof("fed-sender received data type:%s topic:%s", ev.Type(), topic)

	targetDomain, _ := c.getDomainByTopic(topic)

	index := common.CalcStringHashCode(ev.RoomID()) % c.chanSize
	c.msgChan[index] <- common.ContextMsg{
		Ctx: ctx,
		Msg: FedSendMsg{
			partition: partition,
			domain:    targetDomain,
			roomID:    ev.RoomID(),
			ev:        ev,
		},
	}
}

func (c *FederationSender) processEvent(ctx context.Context, partition int32, targetDomain, roomID string, ev *gomatrixserverlib.Event) {
	c.recRepo.IncrPendingSize(ctx, roomID, targetDomain, 1, ev.DomainOffset())

	senderDomain, _ := utils.DomainFromID(ev.Sender())

	log.Infof("fed-sender received data sender:%s sender-domain:%s key:%s", ev.Sender(), senderDomain, targetDomain+":"+roomID)

	oqs := c.getQueue(senderDomain)
	oqs.SendEvent(ctx, partition, ev, targetDomain, roomID)
}

func (c *FederationSender) processEdu(ctx context.Context, partition int32, targetDomain, roomID string, edu *gomatrixserverlib.EDU) {
	oqs := c.getQueue(edu.Origin)
	oqs.SendEDU(ctx, partition, edu, targetDomain, roomID)
}

func (c *FederationSender) sendEdu(ctx context.Context, edu *gomatrixserverlib.EDU) {
	var idx uint32
	roomID := ""
	switch edu.Type {
	case "profile":
		idx = uint32(rand.Int31n(int32(c.chanSize)))
	case "receipt":
		var content types.ReceiptContent
		if err := json.Unmarshal(edu.Content, &content); err != nil {
			log.Errorf("send edu error: %v", err)
			return
		}
		roomID = content.RoomID
		idx = common.CalcStringHashCode(content.RoomID) % uint32(c.chanSize)
	case "typing":
		var content types.TypingContent
		if err := json.Unmarshal(edu.Content, &content); err != nil {
			log.Errorf("send edu error: %v", err)
			return
		}
		roomID = content.RoomID
		idx = common.CalcStringHashCode(content.RoomID) % uint32(c.chanSize)
	}

	c.msgChan[idx] <- common.ContextMsg{
		Ctx: ctx,
		Msg: FedSendMsg{
			partition: 0,
			domain:    edu.Destination,
			roomID:    roomID,
			edu:       edu,
		},
	}
}

func (c *FederationSender) AssignRoomPartition(ctx context.Context, roomID, domain string, retryTime time.Duration, retryInterval time.Duration) (*fedrepos.RecordItem, bool) {
	beginTime := time.Now()
	if retryInterval < time.Millisecond*50 {
		retryInterval = time.Millisecond * 50
	}
	recItem, ok := c.recRepo.AssignRoomPartition(ctx, roomID, domain, c.fakePartition)
	for !ok {
		if beginTime.Add(retryTime).Before(time.Now()) {
			break
		}
		time.Sleep(retryInterval)
		recItem, ok = c.recRepo.AssignRoomPartition(ctx, roomID, domain, c.fakePartition)
	}
	if ok {
		c.assignedRoomDomain.Store(roomID+"|"+domain, 1)
	}
	return recItem, ok
}

func (c *FederationSender) TryAssignRoomPartition(ctx context.Context, roomID, domain string) (*fedrepos.RecordItem, bool) {
	recItem, ok := c.recRepo.AssignRoomPartition(ctx, roomID, domain, c.fakePartition)
	if ok {
		c.assignedRoomDomain.Store(roomID+"|"+domain, 1)
	}
	return recItem, ok
}

func (c *FederationSender) UnassignRoomPartition(ctx context.Context, roomID, domain string) {
	c.assignedRoomDomain.Delete(roomID + "|" + domain)
	c.recRepo.UnassignRoomPartition(ctx, roomID, domain)
}

func (c *FederationSender) OnRoomDomainRelease(ctx context.Context, origin, roomID, domain string) {
	oqs := c.getQueue(origin)
	oqs.Release(ctx, roomID, domain)
	c.recRepo.UnassignRoomPartition(ctx, roomID, domain)
	c.assignedRoomDomain.Delete(roomID + "|" + domain)
}

func (c *FederationSender) HasAssgined(ctx context.Context, roomID, domain string) (*fedrepos.RecordItem, bool) {
	_, ok := c.assignedRoomDomain.Load(roomID + "|" + domain)
	if !ok {
		return nil, ok
	}
	recItem := c.recRepo.GetRec(ctx, roomID, domain)
	return recItem, ok
}
