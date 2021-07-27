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
	"sync"
	"sync/atomic"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/federationapi/rpc"
	"github.com/finogeeks/ligase/federation/fedsender/queue"
	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type ServerName = gomatrixserverlib.ServerName

type FedSendMsg struct {
	domain string
	roomID string
	ev     *gomatrixserverlib.Event
	edu    *gomatrixserverlib.EDU
}

type FederationSender struct {
	cfg         *config.Dendrite
	Repo        *repos.RoomServerCurStateRepo
	queuesMap   sync.Map
	domainMap   sync.Map
	syncedMap   sync.Map
	offset      uint32
	fedRpcCli   roomserverapi.RoomserverRPCAPI
	db          model.FederationDatabase
	feddomains  *common.FedDomains
	recMap      sync.Map
	chanSize    uint32
	msgChan     []chan FedSendMsg
	sentCounter int64
}

func NewFederationSender(cfg *config.Dendrite, rpcCli rpcService.RpcClient, feddomains *common.FedDomains, db model.FederationDatabase) *FederationSender {
	fedRpcCli := rpc.NewFederationRpcClient(cfg, rpcCli, nil, nil, nil)
	sender := &FederationSender{
		cfg:        cfg,
		offset:     0,
		fedRpcCli:  fedRpcCli,
		db:         db,
		feddomains: feddomains,
		chanSize:   16,
	}

	sender.msgChan = make([]chan FedSendMsg, sender.chanSize)
	for i := 0; i < len(sender.msgChan); i++ {
		sender.msgChan[i] = make(chan FedSendMsg, 32)
		go sender.startWorker(sender.msgChan[i])
	}
	for _, domain := range cfg.Matrix.ServerName {
		fedClient, _ := client.GetFedClient(domain)
		queues := queue.NewOutgoingQueues(ServerName(domain), &sender.sentCounter, fedClient, fedRpcCli, db, cfg, feddomains)
		sender.queuesMap.Store(domain, queues)
	}
	return sender
}

func (c *FederationSender) Init() error {
	roomIDs, domains, eventIDs, sendTimeses, pendingSizes, total, err := c.db.SelectAllSendRecord(context.TODO())
	if err != nil {
		log.Errorf("FederationSender.Init error %v", err)
		return err
	}
	for i := 0; i < total; i++ {
		if _, ok := c.feddomains.GetDomainHost(domains[i]); !ok {
			continue
		}
		key := domains[i] + ":" + roomIDs[i]
		c.recMap.Store(key, &queue.RecordItem{
			RoomID:      roomIDs[i],
			Domain:      domains[i],
			EventID:     eventIDs[i],
			SendTimes:   sendTimeses[i],
			PendingSize: pendingSizes[i],
		})
		c.syncedMap.Store(key, true)
	}
	return nil
}

func (c *FederationSender) Start() {
	v, _ := c.queuesMap.Load(domain.FirstDomain)
	queues := v.(*queue.OutgoingQueues)
	c.recMap.Range(func(k, v interface{}) bool {
		recordItem := v.(*queue.RecordItem)
		if recordItem.SendTimes == 0 {
			// 第一次，必须特殊处理，发送state列表即可，由remote自主backfill
			// go c.sendStateEvs(recordItem, "")
		} else if recordItem.PendingSize > 0 {
			// 本域通过向roomserver发起backfill的rpc请求，拿到后续的event，主动发送给remote
			// 不能通过remote自主backfill，因为backfill的处理，会忽略state事件的处理
			queues.RetrySendBackfillEvs(recordItem)
		}
		return true
	})
}

func (c *FederationSender) sendStateEvs(recItem *queue.RecordItem, eventID string) {
	domain := recItem.Domain
	rs := c.Repo.GetRoomStateNoCache(recItem.RoomID)
	pdus := []*gomatrixserverlib.Event{}
	states := rs.GetAllState()
	var sender string
	for i := 0; i < len(states); i++ {
		state := &states[i]
		stateDomain, _ := utils.DomainFromID(state.Sender())
		bytes, _ := json.Marshal(state)
		log.Infof("room:%s type:%s state-domain:%s target-domain:%s ev:%v", state.RoomID(), state.Type(), stateDomain, domain, string(bytes))
		if stateDomain != domain && state.EventID() != eventID {
			pdus = append(pdus, state)
		}
		if state.Type() == "m.room.create" {
			sender = state.Sender()
		}
	}

	senderDomain, _ := utils.DomainFromID(sender)
	origin := ServerName(senderDomain)

	if senderDomain == domain {
		log.Infof("fed-send target domain is sender domain, ignore sending state %s", domain)
		return
	}

	destinations := []ServerName{ServerName(domain)}

	var queues *queue.OutgoingQueues
	if val, ok := c.queuesMap.Load(senderDomain); !ok {
		domain := (c.cfg.GetServerName())[0]
		log.Infof("fed-send state evs send by %s instead of %s", domain, senderDomain)
		val, _ = c.queuesMap.Load(domain)
		queues = val.(*queue.OutgoingQueues)
	} else {
		queues = val.(*queue.OutgoingQueues)
	}

	size := int32(len(pdus))
	var diff int32
	for {
		pendingSize := recItem.PendingSize
		diff = size - pendingSize
		if pendingSize == recItem.PendingSize {
			break
		}
	}
	if diff != 0 {
		atomic.AddInt32(&recItem.PendingSize, diff)
		c.db.UpdateSendRecordPendingSize(context.TODO(), recItem.RoomID, domain, diff)
	}

	queues.SendStates(pdus, origin, destinations, recItem.RoomID, map[string]*queue.RecordItem{domain: recItem})
}

func (c *FederationSender) SetRepo(repo *repos.RoomServerCurStateRepo) {
	c.Repo = repo
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
			tran.AddChannel(core.CHANNEL_SUB, name, topic, "name", &c.cfg.Kafka.Consumer.SenderInput)
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

func (c *FederationSender) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	ev := new(gomatrixserverlib.Event)
	if err := json.Unmarshal(data, ev); err != nil {
		log.Errorf("fed-sender: message parse failure err:%v", err)
		return
	}

	log.Infof("fed-sender received data type:%s topic:%s", ev.Type(), topic)

	val, _ := c.domainMap.Load(topic)
	targetDomain := val.(string)

	index := common.CalcStringHashCode(ev.RoomID()) % c.chanSize
	c.msgChan[index] <- FedSendMsg{
		domain: targetDomain,
		roomID: ev.RoomID(),
		ev:     ev,
	}
}

func (c *FederationSender) SendEDU(edu *gomatrixserverlib.EDU) {
	c.sendEdu(edu)
}

func (c *FederationSender) sendEdu(edu *gomatrixserverlib.EDU) {
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

	c.msgChan[idx] <- FedSendMsg{
		domain: edu.Destination,
		roomID: roomID,
		edu:    edu,
	}
}

func (c *FederationSender) startWorker(msgChan chan FedSendMsg) {
	for msg := range msgChan {
		if msg.ev != nil {
			c.processEvent(msg.domain, msg.roomID, msg.ev)
		}
		if msg.edu != nil {
			c.processEdu(msg.domain, msg.roomID, msg.edu)
		}
	}
}

func (c *FederationSender) processEvent(targetDomain, roomID string, ev *gomatrixserverlib.Event) {
	senderDomain, _ := utils.DomainFromID(ev.Sender())
	origin := ServerName(senderDomain)

	val, _ := c.queuesMap.Load(senderDomain)
	queues := val.(*queue.OutgoingQueues)

	// log.Infof("fed-sender received data sender:%s sender-domain:%s item:%v", ev.Sender(), senderDomain, val)

	key := targetDomain + ":" + roomID
	log.Infof("fed-sender received data sender:%s sender-domain:%s key:%s", ev.Sender(), senderDomain, key)

	var recItem *queue.RecordItem
	destinations := []ServerName{ServerName(targetDomain)}
	_, loaded := c.syncedMap.LoadOrStore(key, true)
	if !loaded {
		if err := c.db.InsertSendRecord(context.TODO(), roomID, targetDomain); err != nil {
			log.Errorf("fed-sender insert sending record key: %s error: %v", key, err)
		}
		recItem = &queue.RecordItem{
			RoomID: roomID,
			Domain: targetDomain,
		}
		c.recMap.Store(key, recItem)

		//c.sendStateEvs(recItem, ev.EventID())
		//atomic.AddInt32(&recItem.PendingSize, 1)
		c.db.UpdateSendRecordPendingSize(context.TODO(), roomID, targetDomain, 1)
		queues.SendEvent(ev, origin, destinations, roomID, map[string]*queue.RecordItem{targetDomain: recItem})
	} else {
		v, _ := c.recMap.Load(key)
		recItem = v.(*queue.RecordItem)
		atomic.AddInt32(&recItem.PendingSize, 1)
		c.db.UpdateSendRecordPendingSize(context.TODO(), roomID, targetDomain, 1)
		queues.SendEvent(ev, origin, destinations, roomID, map[string]*queue.RecordItem{targetDomain: recItem})
	}
}

func (c *FederationSender) processEdu(targetDomain, roomID string, edu *gomatrixserverlib.EDU) {
	if val, ok := c.queuesMap.Load(edu.Origin); ok {
		item := val.(*queue.OutgoingQueues)
		if roomID == "" {
			item.SendEDU(edu, ServerName(edu.Origin), []ServerName{ServerName(targetDomain)}, roomID, nil)
		} else {
			key := targetDomain + ":" + roomID
			if val, ok := c.recMap.Load(key); ok {
				recItem := val.(*queue.RecordItem)
				item.SendEDU(edu, ServerName(edu.Origin), []ServerName{ServerName(targetDomain)}, roomID, map[string]*queue.RecordItem{targetDomain: recItem})
			}
		}
	}
}
