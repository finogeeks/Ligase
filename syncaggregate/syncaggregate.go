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

package syncaggregate

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/syncaggregate/api"
	"github.com/finogeeks/ligase/syncaggregate/consumers"
	"github.com/finogeeks/ligase/syncaggregate/rpc"
	"github.com/finogeeks/ligase/syncaggregate/sync"
)

func SetupSyncAggregateComponent(
	base *basecomponent.BaseDendrite,
	cacheIn service.Cache,
	rpcClient *common.RpcClient,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) {
	syncDB := base.CreateSyncDB()
	deviceDB := base.CreateDeviceDB()
	maxEntries := base.Cfg.Lru.MaxEntries
	gcPerNum := base.Cfg.Lru.GcPerNum
	flushDelay := base.Cfg.FlushDelay
	syncMngChanNum := uint32(64)
	if base.Cfg.SyncMngChanNum > 0 {
		syncMngChanNum = base.Cfg.SyncMngChanNum
	}

	monitor := mon.GetInstance()
	queryHitCounter := monitor.NewLabeledCounter("syncaggreate_query_hit", []string{"target", "repo", "func"})

	clientDataStreamRepo := repos.NewClientDataStreamRepo(4, maxEntries, gcPerNum)
	userTimeLine := repos.NewUserTimeLineRepo(idg)

	stdEventStreamRepo := repos.NewSTDEventStreamRepo(base.Cfg, 4, maxEntries, gcPerNum, flushDelay)
	onlineRepo := repos.NewOnlineUserRepo(base.Cfg.StateMgr.StateOffline, base.Cfg.StateMgr.StateOfflineIOS)

	clientDataStreamRepo.SetPersist(syncDB)
	clientDataStreamRepo.SetMonitor(queryHitCounter)

	userTimeLine.SetPersist(syncDB)
	userTimeLine.SetCache(cacheIn)
	userTimeLine.SetMonitor(queryHitCounter)

	presenceStreamRepo := repos.NewPresenceDataStreamRepo(userTimeLine)
	presenceStreamRepo.SetPersist(syncDB)
	presenceStreamRepo.SetMonitor(queryHitCounter)
	presenceStreamRepo.SetCfg(base.Cfg)

	kcRepo := repos.NewKeyChangeStreamRepo(userTimeLine)
	kcRepo.SetSyncDB(syncDB)
	kcRepo.SetCache(cacheIn)
	kcRepo.SetMonitor(queryHitCounter)

	stdEventStreamRepo.SetPersist(syncDB)
	stdEventStreamRepo.SetMonitor(queryHitCounter)

	userDevActiveRepo := repos.NewUserDeviceActiveRepo(base.Cfg.DeviceMng.ScanUnActive, true)
	userDevActiveRepo.SetPersist(deviceDB)

	settings := common.NewSettings(cacheIn)

	settingConsumer := common.NewSettingConsumer(
		base.Cfg.Kafka.Consumer.SettingUpdateSyncAggregate.Underlying,
		base.Cfg.Kafka.Consumer.SettingUpdateSyncAggregate.Name,
		settings)
	if err := settingConsumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}

	accountDataConsumer := consumers.NewActDataConsumer(base.Cfg, syncDB)
	accountDataConsumer.SetClientDataStreamRepo(clientDataStreamRepo)
	if err := accountDataConsumer.Start(); err != nil {
		log.Panicf("failed to start sync account data consumer err:%v", err)
	}

	eventConsumer := consumers.NewEventFeedConsumer(base.Cfg, syncDB,cacheIn)
	eventConsumer.SetUserTimeLine(userTimeLine)
	eventConsumer.SetPresenceStreamRepo(presenceStreamRepo)
	if err := eventConsumer.Start(); err != nil {
		log.Panicf("failed to start sync event consumer err:%v", err)
	}

	profileConsumer := consumers.NewProfileConsumer(base.Cfg, userTimeLine, syncDB, idg, cacheIn)
	profileConsumer.SetPresenceStreamRepo(presenceStreamRepo)
	profileConsumer.SetOnlineUserRepo(onlineRepo)
	if err := profileConsumer.Start(); err != nil {
		log.Panicf("failed to start sync profile consumer err:%v", err)
	}

	typingConsumer := consumers.NewTypingConsumer(50, 10, 50)
	typingConsumer.StartCalculate()

	eventRpcConsumer := rpc.NewEventRpcConsumer(rpcClient, userTimeLine, syncDB, base.Cfg)
	if err := eventRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync event rpc consumer err:%v", err)
	}

	joinedRoomRpcConsumer := rpc.NewJoinedRoomRpcConsumer(rpcClient, userTimeLine, base.Cfg)
	if err := joinedRoomRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync joined room rpc consumer err:%v", err)
	}

	keyChangeRpcConsumer := rpc.NewKeyChangeRpcConsumer(kcRepo, userTimeLine, rpcClient, base.Cfg)
	if err := keyChangeRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync key change rpc consumer err:%v", err)
	}

	keyUpdateRpcConsumer := rpc.NewKeyUpdateRpcConsumer(kcRepo, rpcClient, base.Cfg)
	if err := keyUpdateRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync key update rpc consumer err:%v", err)
	}

	receiptUpdateRpcConsumer := rpc.NewReceiptUpdateRpcConsumer(userTimeLine, rpcClient, base.Cfg)
	if err := receiptUpdateRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync receipt update rpc consumer err:%v", err)
	}

	stdRpcConsumer := rpc.NewStdRpcConsumer(rpcClient, stdEventStreamRepo, cacheIn, syncDB, base.Cfg)
	if err := stdRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync send to device rpc consumer err:%v", err)
	}

	typingRpcConsumer := rpc.NewTypingRpcConsumer(typingConsumer, rpcClient, base.Cfg)
	if err := typingRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync typing rpc consumer err:%v", err)
	}

	unReadRpcConsumer := rpc.NewUnReadRpcConsumer(rpcClient, userTimeLine, base.Cfg)
	if err := unReadRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync unread rpc consumer err:%v", err)
	}

	presenceRpcConsumer := rpc.NewPresenceRpcConsumer(rpcClient, onlineRepo, presenceStreamRepo, base.Cfg)
	if err := presenceRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync presence rpc consumer err:%v", err)
	}

	syncMng := sync.NewSyncMng(syncDB, syncMngChanNum, 1024, base.Cfg, rpcClient)
	syncMng.SetCache(cacheIn)
	syncMng.SetComplexCache(complexCache)
	syncMng.SetOnlineRepo(onlineRepo)
	syncMng.SetUserTimeLine(userTimeLine)
	syncMng.SetTypingConsumer(typingConsumer)
	syncMng.SetClientDataStreamRepo(clientDataStreamRepo)
	syncMng.SetKeyChangeRepo(kcRepo)
	syncMng.SetStdEventStreamRepo(stdEventStreamRepo)
	syncMng.SetPresenceStreamRepo(presenceStreamRepo)
	syncMng.SetUserDeviceActiveTsRepo(userDevActiveRepo)
	syncMng.Start()

	syncRpcConsumer := rpc.NewSyncRpcConsumer(rpcClient, syncMng, base.Cfg)
	if err := syncRpcConsumer.Start(); err != nil {
		log.Panicf("failed to start sync rpc consumer err:%v", err)
	}

	apiConsumer := api.NewInternalMsgConsumer(*base.Cfg, rpcClient, idg, syncMng, userTimeLine, kcRepo, stdEventStreamRepo, syncDB, cacheIn)
	apiConsumer.Start()
}
