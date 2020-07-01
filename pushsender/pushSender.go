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

package pushsender

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/pushsender/consumers"
	"github.com/finogeeks/ligase/storage/model"
)

func SetupPushSenderComponent(
	base *basecomponent.BaseDendrite,
	rpcClient *common.RpcClient,
) (model.PushAPIDatabase, *consumers.PushDataConsumer) {
	pushDB := base.CreatePushApiDB()

	pushConsumer := consumers.NewPushDataConsumer(
		base.Cfg, pushDB, rpcClient,
	)
	if err := pushConsumer.Start(); err != nil {
		log.Panicw("failed to start push data consumer", log.KeysAndValues{"error", err})
	}

	return pushDB, pushConsumer
}
