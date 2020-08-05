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

package syncserver

import (
	"context"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/syncserver/api"
	"github.com/finogeeks/ligase/syncserver/consumers"
	"github.com/finogeeks/ligase/syncserver/rpc"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// component.
func SetupSyncServerComponent(
	base *basecomponent.BaseDendrite,
	accountDB model.AccountsDatabase,
	cacheIn service.Cache,
	rpcClient *common.RpcClient,
	idg *uid.UidGenerator,
) {
	syncDB := base.CreateSyncDB()
	maxEntries := base.Cfg.Lru.MaxEntries
	gcPerNum := base.Cfg.Lru.GcPerNum
	flushDelay := base.Cfg.FlushDelay
	syncMngChanNum := uint32(64)
	if base.Cfg.SyncMngChanNum > 0 {
		syncMngChanNum = base.Cfg.SyncMngChanNum
	}

	monitor := mon.GetInstance()
	qureyHitCounter := monitor.NewLabeledCounter("syncserver_query_hit", []string{"target", "repo", "func"})

	roomHistory := repos.NewRoomHistoryTimeLineRepo(4, maxEntries, gcPerNum)
	rsCurState := new(repos.RoomCurStateRepo)
	rsTimeline := repos.NewRoomStateTimeLineRepo(4, rsCurState, maxEntries, gcPerNum)

	receiptDataStreamRepo := repos.NewReceiptDataStreamRepo(flushDelay, 100, true)
	receiptDataStreamRepo.SetPersist(syncDB)
	receiptDataStreamRepo.SetMonitor(qureyHitCounter)

	displayNameRepo := repos.NewDisplayNameRepo()
	readCountRepo := repos.NewReadCountRepo(flushDelay)
	readCountRepo.SetCache(cacheIn)
	userReceiptRepo := repos.NewUserReceiptRepo(flushDelay)

	eventReadStreamRepo := repos.NewEventReadStreamRepo()
	eventReadStreamRepo.SetPersist(syncDB)
	eventReadStreamRepo.SetMonitor(qureyHitCounter)

	roomHistory.SetPersist(syncDB)
	roomHistory.SetMonitor(qureyHitCounter)
	roomHistory.SetCache(cacheIn)
	rsCurState.SetPersist(syncDB)

	rsTimeline.SetPersist(syncDB)
	rsTimeline.SetMonitor(qureyHitCounter)

	displayNameRepo.SetPersist(syncDB)
	displayNameRepo.LoadHistory()
	receiptDataStreamRepo.SetRsCurState(rsCurState)
	receiptDataStreamRepo.SetRsTimeline(rsTimeline)
	userReceiptRepo.SetPersist(syncDB)
	userReceiptRepo.SetMonitor(qureyHitCounter)

	settings := common.NewSettings(cacheIn)

	settingConsumer := common.NewSettingConsumer(
		base.Cfg.Kafka.Consumer.SettingUpdateSyncServer.Underlying,
		base.Cfg.Kafka.Consumer.SettingUpdateSyncServer.Name,
		settings)
	if err := settingConsumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}

	complexCache := common.NewComplexCache(accountDB, cacheIn)
	complexCache.SetDefaultAvatarURL(base.Cfg.DefaultAvatar)

	pushConsumer := consumers.NewPushConsumer(cacheIn, rpcClient, complexCache)
	pushConsumer.SetRoomHistory(roomHistory)
	pushConsumer.SetCountRepo(readCountRepo)
	pushConsumer.SetEventRepo(eventReadStreamRepo)
	pushConsumer.SetRoomCurState(rsCurState)
	pushConsumer.SetRsTimeline(rsTimeline)
	pushConsumer.Start()
	feedServer := consumers.NewRoomEventFeedConsumer(base.Cfg, syncDB, pushConsumer, rpcClient, idg)
	feedServer.SetRoomHistory(roomHistory)
	feedServer.SetRsCurState(rsCurState)
	feedServer.SetRsTimeline(rsTimeline)
	feedServer.SetReceiptRepo(receiptDataStreamRepo)
	feedServer.SetDisplayNameRepo(displayNameRepo)
	if err := feedServer.Start(); err != nil {
		log.Panicf("failed to start sync room server consumer err:%v", err)
	}

	profileConsumer := consumers.NewProfileConsumer(base.Cfg)
	profileConsumer.SetDisplayNameRepo(displayNameRepo)
	if err := profileConsumer.Start(); err != nil {
		log.Panicf("failed to start sync profile consumer err:%v", err)
	}

	receiptConsumer := consumers.NewReceiptConsumer(rpcClient, base.Cfg, idg)
	receiptConsumer.SetCountRepo(readCountRepo)
	receiptConsumer.SetReceiptRepo(receiptDataStreamRepo)
	receiptConsumer.SetUserReceiptRepo(userReceiptRepo)
	receiptConsumer.SetRoomHistory(roomHistory)
	receiptConsumer.SetRoomCurState(rsCurState)
	receiptConsumer.SetRsTimeline(rsTimeline)
	if err := receiptConsumer.Start(); err != nil {
		log.Panicf("failed to start sync receipt consumer err:%v", err)
	}

	syncServer := consumers.NewSyncServer(syncDB, syncMngChanNum, 1024, base.Cfg, rpcClient)
	syncServer.SetCache(cacheIn)
	syncServer.SetRoomHistory(roomHistory)
	syncServer.SetRsTimeline(rsTimeline)
	syncServer.SetRsCurState(rsCurState)
	syncServer.SetReceiptDataStreamRepo(receiptDataStreamRepo)
	syncServer.SetUserReceiptDataRepo(userReceiptRepo)
	syncServer.SetReadCountRepo(readCountRepo)
	syncServer.SetDisplayNameRepo(displayNameRepo)
	syncServer.SetSettings(settings)
	syncServer.Start()

	typingRpcConsumer := rpc.NewTypingRpcConsumer(rsCurState, rpcClient, base.Cfg)
	if err := typingRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync typing rpc consumer err:%v", err)
	}

	receiptRpcConsumer := rpc.NewReceiptRpcConsumer(receiptConsumer, rpcClient, base.Cfg)
	if err := receiptRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync receipt rpc consumer err:%v", err)
	}

	syncServerRpcConsumer := rpc.NewSyncServerRpcConsumer(rpcClient, syncServer, base.Cfg)
	if err := syncServerRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync server rpc consumer err:%v", err)
	}

	syncUnreadRpcConsumer := rpc.NewSyncUnreadRpcConsumer(rpcClient, readCountRepo, base.Cfg)
	if err := syncUnreadRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync unread rpc consumer err:%v", err)
	}

	log.Infof("instance:%d,syncserver total:%d", base.Cfg.MultiInstance.Instance, base.Cfg.MultiInstance.SyncServerTotal)
	apiConsumer := api.NewInternalMsgConsumer(*base.Cfg, rpcClient, idg, syncDB, rsCurState, rsTimeline, roomHistory, displayNameRepo, receiptConsumer, settings, cacheIn)
	apiConsumer.Start()
}

func FixSyncCorruptRooms(
	base *basecomponent.BaseDendrite,
	typ, fixRoom string,
) {
	span, ctx := common.StartSobSomSpan(context.Background(), "FixSyncCorruptRooms")
	defer span.Finish()
	syncDB := base.CreateSyncDB()
	types := []string{
		"m.room.create",
		"m.room.member",
		"m.room.power_levels",
		"m.room.join_rules",
		"m.room.history_visibility",
		"m.room.visibility",
		"m.room.name",
		"m.room.topic",
		"m.room.desc",
		"m.room.pinned_events",
		"m.room.aliases",
		"m.room.canonical_alias",
		"m.room.avatar",
		"m.room.encryption",
	}

	if fixRoom == "*" {
		rooms, err := syncDB.GetAllSyncRooms()
		if err != nil {
			log.Errorf("FixSyncCorruptRooms get all sync rooms fail err:%v", err)
			return
		}
		for _, roomID := range rooms {
			if typ == "*" {
				fixSyncRoom(ctx, types, roomID, syncDB)
			} else {
				fixSyncRoom(ctx, []string{typ}, roomID, syncDB)
			}
		}
	} else {
		if typ == "*" {
			fixSyncRoom(ctx, types, fixRoom, syncDB)
		} else {
			fixSyncRoom(ctx, []string{typ}, fixRoom, syncDB)
		}
	}
	log.Infof("FixSyncCorruptRooms end")
}

func fixSyncRoom(
	ctx context.Context,
	fixType []string,
	roomID string,
	syncDB model.SyncAPIDatabase,
) {
	log.Infof("start fix sync room %s type %s", roomID, fixType)

	evs, ids, err := syncDB.SelectTypeEventForward(ctx, fixType, roomID)
	if err != nil {
		log.Errorf("fixSyncRoom fail roomID %s type %s err:%v", roomID, fixType, err)
		return
	}

	//time.Sleep(time.Millisecond * 50)

	for idx, ev := range evs {
		roomID := ev.RoomID
		offset := ids[idx]
		target := ""
		if ev.StateKey != nil {
			target = *ev.StateKey
		}

		memberShip := ""
		if ev.Type == "m.room.member" {
			member := external.MemberContent{}
			json.Unmarshal(ev.Content, &member)
			memberShip = member.Membership
		}

		evBytes, err := json.Marshal(ev)
		if err != nil {
			log.Errorf("fixSyncRoom marshal fail roomID %s type %s err:%v", roomID, fixType, err)
			break
		}

		syncDB.UpdateRoomState2(nil, roomID, ev.EventID, evBytes, ev.Type, target, memberShip, offset)
	}
	log.Infof("finish fix sync room %s type %s", roomID, fixType)
}
