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

package clientapi

import (
	"github.com/finogeeks/ligase/clientapi/api"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"

	// "github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/clientapi/rpc"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/storage/model"
)

// SetupClientAPIComponent sets up and registers HTTP handlers for the ClientAPI
// component.
func SetupClientAPIComponent(
	base *basecomponent.BaseDendrite,
	deviceDB model.DeviceDatabase,
	cache service.Cache,
	accountsDB model.AccountsDatabase,
	federation *fed.Federation,
	keyRing *gomatrixserverlib.KeyRing,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	presenceDB model.PresenceDatabase,
	roomDB model.RoomServerDatabase,
	rpcCli *common.RpcClient,
	tokenFilter *filter.Filter,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
	serverConfDB model.ConfigDatabase,
) {
	profileRpcConsumer := rpc.NewProfileRpcConsumer(rpcCli, base.Cfg, rsRpcCli, idg, accountsDB, presenceDB, cache, complexCache)
	profileRpcConsumer.Start()

	apiConsumer := api.NewInternalMsgConsumer(
		base.APIMux, *base.Cfg,
		rsRpcCli, accountsDB, deviceDB,
		federation, *keyRing,
		cache, encryptDB, syncDB, presenceDB,
		roomDB, rpcCli, tokenFilter, complexCache, serverConfDB,
	)
	apiConsumer.Start()
}
