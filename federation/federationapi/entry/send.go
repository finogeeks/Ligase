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
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/federation/fedutil"
	"github.com/finogeeks/ligase/federation/model/repos"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/pkg/errors"
)

func init() {
	Register(model.CMD_FED_SEND, Send)
}

type InviteWaitItem struct {
	Event    *gomatrixserverlib.Event
	IsDelete int32
}

var (
	inviteWait sync.Map
)

func Send(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	retMsg := &model.GobMessage{
		Body: []byte{},
	}

	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}

	trans := gomatrixserverlib.Transaction{}
	err := json.Unmarshal(msg.Body, &trans)
	if err != nil {
		log.Errorf("api send msg error: %v", err)
		return retMsg, fmt.Errorf("api send msg error: %v", err)
	}
	log.Infof("api send recv trans: %s", msg.Body)

	pdus := trans.PDUs
	if len(pdus) > 0 {
		ev := pdus[0]
		roomID := ev.RoomID()
		var lastlastPdu *gomatrixserverlib.Event
		if v, ok := inviteWait.Load(roomID); ok {
			item := v.(*InviteWaitItem)
			if atomic.CompareAndSwapInt32(&item.IsDelete, 0, 1) {
				lastlastPdu = item.Event
			}
		}
		inviteWait.Delete(roomID)
		if ev.Type() == gomatrixserverlib.MRoomCreate {
			log.Infof("api handle send check m.room.create event duplicate %s", roomID)
			var joinedRoomsVal *repos.JoinRoomsData
			joinedRoomsVal, err = joinRoomsRepo.GetData(ctx, roomID)
			if err != nil {
				log.Errorf("api handle send get join rooms err %v", err)
				return retMsg, err
			}
			if joinedRoomsVal.HasJoined {
				return retMsg, nil
			}
			joinedRoomsVal.HasJoined = true
			joinedRoomsVal.EventID = ev.EventID()
			joinRoomsRepo.AddData(ctx, joinedRoomsVal)
		}

		sort.Slice(pdus, func(i, j int) bool { return pdus[i].DomainOffset() < pdus[j].DomainOffset() })

		downloadMedia(pdus, fedClient)

		joinedRoomsVal, _ := joinRoomsRepo.GetData(ctx, roomID)
		recvOffsetMap := joinedRoomsVal.RecvOffsetsMap

		domain, _ := common.DomainFromID(ev.EventID())
		eventOffset := recvOffsetMap[domain]
		if eventOffset == nil || eventOffset.Offset == 0 {
			idMap, _ := backfillRepo.GetFinishedDomains(ctx, roomID)
			if idMap != nil {
				if v, ok := idMap[domain]; ok {
					eventOffset = &repos.JoinedRoomFinishedEventOffset{EventID: v.StartEventID, Offset: v.StartOffset}
				}
			}
		}
		if lastlastPdu != nil {
			pdus = append(pdus, *lastlastPdu)
			for i := len(pdus) - 1; i > 0; i-- {
				pdus[i] = pdus[i-1]
			}
			pdus[0] = *lastlastPdu
			log.Infof("api handle send process add last last invite event")
		}

		if eventOffset == nil {
			eventOffset = &repos.JoinedRoomFinishedEventOffset{}
		}
		getMissingEvents(ctx, eventOffset, domain, pdus)

		for i := range pdus {
			pdus[i].SetDepth(0)
		}

		lastPdu := pdus[len(pdus)-1]
		if lastPdu.Type() == gomatrixserverlib.MRoomMember {
			if membership, _ := lastPdu.Membership(); membership == "invite" {
				// 等待一定时间，如果是autojoin的话join事件一起过来
				pdus = pdus[1:]
				inviteWait.Store(roomID, &InviteWaitItem{
					Event:    &lastPdu,
					IsDelete: 0,
				})
				go func() {
					time.Sleep(time.Second * 2)
					if v, ok := inviteWait.Load(roomID); ok {
						inviteWait.Delete(roomID)
						item := v.(*InviteWaitItem)
						if atomic.CompareAndSwapInt32(&item.IsDelete, 0, 1) {
							log.Infof("api handle send process invite event after wait timeout for room: %s", roomID)
							rawEvent := roomserverapi.RawEvent{}
							rawEvent.Kind = roomserverapi.KindNew
							rawEvent.BulkEvents.Events = []gomatrixserverlib.Event{lastPdu}
							rawEvent.Trust = true
							rawEvent.RoomID = roomID
							rpcCli.InputRoomEvents(ctx, &rawEvent)

							lastOffset := lastPdu.DomainOffset()
							lastEventID := lastPdu.EventID()
							recvOffsetMap[domain] = &repos.JoinedRoomFinishedEventOffset{lastEventID, lastOffset}
							joinRoomsRepo.UpdateData(ctx, joinedRoomsVal)
						}
					}
				}()
			}
		}
		if len(pdus) > 0 {
			log.Infof("api handle send process event for room: %s", roomID)
			rawEvent := roomserverapi.RawEvent{}
			rawEvent.Kind = roomserverapi.KindNew
			rawEvent.BulkEvents.Events = pdus
			rawEvent.Trust = true
			rpcCli.InputRoomEvents(ctx, &rawEvent)

			lastOffset := pdus[len(pdus)-1].DomainOffset()
			lastEventID := pdus[len(pdus)-1].EventID()
			recvOffsetMap[domain] = &repos.JoinedRoomFinishedEventOffset{lastEventID, lastOffset}
			joinRoomsRepo.UpdateData(ctx, joinedRoomsVal)
		}
	}
	for _, edu := range trans.EDUs {
		switch edu.Type {
		case "profile":
			rpcCli.ProcessProfile(&edu)
		case "receipt":
			rpcCli.ProcessReceipt(&edu)
		case "typing":
			rpcCli.ProcessTyping(&edu)
		}
	}
	return retMsg, err
}

func getMissingEvents(ctx context.Context, eventOffset *repos.JoinedRoomFinishedEventOffset, domain string, pdus []gomatrixserverlib.Event) {
	sort.Slice(pdus, func(i, j int) bool { return pdus[i].DomainOffset() < pdus[j].DomainOffset() })

	roomID := pdus[0].RoomID()
	if eventOffset.Offset != 0 && eventOffset.Offset != pdus[0].DomainOffset()-1 && eventOffset.Offset < pdus[0].DomainOffset() {
		limit := int(pdus[0].DomainOffset() - eventOffset.Offset - 1)
		log.Debugf("fed-api send find missing event at first sent, roomID: %s, eventID1: %s, offset1: %d, eventID2: %s, offset2: %d", roomID, pdus[0].EventID(), pdus[0].DomainOffset(), eventOffset.EventID, eventOffset.Offset)
		backfillEvents(ctx, roomID, pdus[0].EventID(), limit, pdus[0].DomainOffset())
	}

	for i := 1; i < len(pdus); i++ {
		v0 := pdus[i-1]
		v1 := pdus[i]
		if v0.DomainOffset() != v1.DomainOffset()-1 && v0.DomainOffset() < v1.DomainOffset() {
			limit := int(v1.DomainOffset() - v0.DomainOffset() - 1)
			log.Debugf("fed-api send find missing event, roomID: %s, eventID1: %s, offset1: %d, eventID2: %s, offset2: %d", roomID, v1.EventID(), v1.DomainOffset(), v0.EventID(), v1.DomainOffset())
			backfillEvents(ctx, roomID, v1.EventID(), limit, v1.DomainOffset())
		}
	}
}

func backfillEvents(ctx context.Context, roomID, eventID string, limit int, domainOffset int64) error {
	info := fedmodel.GetMissingEventsInfo{
		RoomID:       roomID,
		EventID:      eventID,
		Limit:        limit,
		DomainOffset: domainOffset,
	}
	span, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.GetMissingEvent.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.GetMissingEvent.Name,
		cfg.Kafka.Producer.GetMissingEvent.Underlying)
	return common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.GetMissingEvent.Underlying,
		cfg.Kafka.Producer.GetMissingEvent.Name,
		&core.TransportPubMsg{
			Topic:   cfg.Kafka.Producer.GetMissingEvent.Topic,
			Keys:    []byte(roomID),
			Obj:     info,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}

func downloadMedia(pdus []gomatrixserverlib.Event, fedClient *client.FedClientWrap) {
	for _, ev := range pdus {
		if ev.Type() != "m.room.message" {
			continue
		}
		domain, _ := utils.DomainFromID(ev.Sender())
		if common.CheckValidDomain(domain, config.GetFedConfig().GetServerName()) {
			continue
		}
		destination, ok := feddomains.GetDomainHost(domain)
		if !ok {
			log.Errorf("ReciverConsumer.downloadMedia domain: %s", domain)
			continue
		}
		var content map[string]interface{}
		err := json.Unmarshal(ev.Content(), &content)
		if err != nil {
			continue
		}
		if fedutil.IsMediaEv(content) {
			fedutil.DownloadFromNetdisk(domain, destination, &ev, content, fedClient)
		}
	}
}
