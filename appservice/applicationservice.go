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

package appservice

import (
	"github.com/finogeeks/ligase/appservice/consumers"
	"github.com/finogeeks/ligase/appservice/types"
	"github.com/finogeeks/ligase/appservice/workers"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/skunkworks/log"
	"sync"
)

// SetupApplicationServiceComponent sets up
// 因为roomserver 的api实现不全 要查房间别名的时候需要databse
// 正常应该使用api访问来实现
func SetupApplicationServiceComponent(base *basecomponent.BaseDendrite) {
	applicationServiceDB := base.CreateApplicationServiceDB()
	roomserverDB := base.CreateRoomDB()

	// Wrap application services in a type that relates the application service and
	// a sync.Cond object that can be used to notify workers when there are new
	// events to be sent out.
	// 每一个appservice 对应 一个 worker
	workerStates := make([]types.ApplicationServiceWorkerState, len(base.Cfg.Derived.ApplicationServices))
	for i, appservice := range base.Cfg.Derived.ApplicationServices {
		m := sync.Mutex{}
		ws := types.ApplicationServiceWorkerState{
			AppService: appservice,
			Cond:       sync.NewCond(&m),
		}
		workerStates[i] = ws
	}

	consumer := consumers.NewOutputRoomEventConsumer(base.Cfg, applicationServiceDB, roomserverDB,
		workerStates)

	if err := consumer.Start(); err != nil {
		log.Panicw("failed to start room server consumer", log.KeysAndValues{"error", err})
	}

	// Create application service transaction workers
	if err := workers.SetupTransactionWorkers(applicationServiceDB, workerStates); err != nil {
		log.Panicw("failed to start app service transaction workers", log.KeysAndValues{"error", err})
	}
}
