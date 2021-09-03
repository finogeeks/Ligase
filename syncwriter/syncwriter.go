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

package syncwriter

import (
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/model/repos"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/syncwriter/api"
	"github.com/finogeeks/ligase/syncwriter/consumers"
	"context"
)

func SetupSyncWriterComponent(
	base *basecomponent.BaseDendrite,
	rpcCli rpcService.RpcClient,
) {
	syncDB := base.CreateSyncDB()
	roomDB := base.CreateRoomDB()
	maxEntries := base.Cfg.Lru.MaxEntries
	gcPerNum := base.Cfg.Lru.GcPerNum

	monitor := mon.GetInstance()
	qureyHitCounter := monitor.NewLabeledCounter("syncwriter_query_hit", []string{"target", "repo", "func"})
	cache := base.PrepareCache()
	roomHistory := repos.NewRoomHistoryTimeLineRepo(4, maxEntries, gcPerNum, "sync_writer")
	rsCurState := repos.NewRoomCurStateRepo(base)
	rsTimeline := repos.NewRoomStateTimeLineRepo(4, rsCurState, maxEntries, gcPerNum, "sync_writer")
	displayNameRepo := repos.NewDisplayNameRepo()
	roomHistory.SetCfg(base.Cfg)
	roomHistory.SetRoomPersist(roomDB)
	roomHistory.SetPersist(syncDB)
	roomHistory.SetMonitor(qureyHitCounter)
	roomHistory.LoadAllDomainMaxStream(context.Background())
	roomHistory.SetCache(cache)
	rsCurState.SetPersist(syncDB)

	rsTimeline.SetPersist(syncDB)
	rsTimeline.SetMonitor(qureyHitCounter)

	displayNameRepo.SetPersist(syncDB)
	displayNameRepo.LoadHistory()

	eventServer := consumers.NewRoomEventConsumer(base.Cfg, syncDB)
	eventServer.SetRoomHistory(roomHistory)
	eventServer.SetRsCurState(rsCurState)
	eventServer.SetRsTimeline(rsTimeline)
	eventServer.SetDisplayNameRepo(displayNameRepo)
	if err := eventServer.Start(); err != nil {
		log.Panicf("failed to start sync room server consumer err:%v", err)
	}

	apiConsumer := api.NewInternalMsgConsumer(*base.Cfg, rpcCli, rsCurState, rsTimeline, roomHistory)
	apiConsumer.Start()
}
