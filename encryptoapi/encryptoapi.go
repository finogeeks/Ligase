// Copyright 2018 Vector Creations Ltd
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

package encryptoapi

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/encryptoapi/api"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
)

// in order to gain key management capability
// , CMD should involve this invoke into main function
// , a setup need an assemble of i.e configs as base and
// accountDB and deviceDB

// SetupEncryptApi set up to servers
func SetupEncryptApi(
	base *basecomponent.BaseDendrite,
	cache service.Cache,
	rpcClient *common.RpcClient,
	federation *gomatrixserverlib.FederationClient,
	idg *uid.UidGenerator,
) model.EncryptorAPIDatabase {
	encryptionDB := base.CreateEncryptApiDB()
	syncDB := base.CreateSyncDB()
	serverName := base.Cfg.Matrix.ServerName

	apiConsumer := api.NewInternalMsgConsumer(
		*base.Cfg, encryptionDB, syncDB,
		idg, cache, rpcClient, federation, serverName,
	)
	apiConsumer.Start()

	return encryptionDB
}
