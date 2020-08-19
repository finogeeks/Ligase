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
	"github.com/finogeeks/ligase/bgmng"
	"github.com/finogeeks/ligase/clientapi"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/skunkworks/log"

	// "github.com/finogeeks/ligase/common/keydb"
	"github.com/finogeeks/ligase/common/uid"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/publicroomsapi"
	"github.com/finogeeks/ligase/pushapi"
	"github.com/finogeeks/ligase/roomserver"
)

func StartClientAPIServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.OutputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.InputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.OutputClientData)
	addProducer(transportMultiplexer, kafka.Producer.OutputProfileData)
	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	addProducer(transportMultiplexer, kafka.Producer.OutputRoomFedEvent)

	addConsumer(transportMultiplexer, kafka.Consumer.InputRoomEvent, base.Cfg.MultiInstance.Instance)

	transportMultiplexer.PreStart()
	cache := base.PrepareCache()

	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)

	serverConfDB := base.CreateServerConfDB()
	domain.GetDomainMngInstance(cache, serverConfDB, base.Cfg.Matrix.ServerName, base.Cfg.Matrix.ServerFromDB, idg)
	base.CheckDomainCfg()

	accountDB := base.CreateAccountsDB()
	deviceDB := base.CreateDeviceDB()
	// keyDB := base.CreateKeyDB()
	// federation := base.CreateFederationClient()
	// keyRing := keydb.CreateKeyRing(federation.Client, keyDB)
	encryptDB := base.CreateEncryptApiDB()
	syncDB := base.CreateSyncDB()
	presenceDB := base.CreatePresenceDB()
	roomDB := base.CreateRoomDB()

	rsRpcCli := base.CreateRsRPCCli(rpcClient)

	federation := fed.NewFederation(base.Cfg, rpcClient)

	tokenFilter := filter.GetFilterMng().Register("device", deviceDB)
	tokenFilter.Load()

	settings := common.NewSettings(cache)
	settingConsumer := common.NewSettingConsumer(
		kafka.Consumer.SettingUpdateContent.Underlying,
		kafka.Consumer.SettingUpdateContent.Name,
		settings)
	if err := settingConsumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}
	feddomains := common.NewFedDomains(settings)
	settings.RegisterFederationDomainsUpdateCallback(feddomains.OnFedDomainsUpdate)

	complexCache := common.NewComplexCache(accountDB, cache)
	complexCache.SetDefaultAvatarURL(base.Cfg.DefaultAvatar)

	clientapi.SetupClientAPIComponent(
		base, deviceDB, cache, accountDB, federation, nil,
		rsRpcCli, encryptDB, syncDB, presenceDB, roomDB, rpcClient, tokenFilter, idg, settings, feddomains, complexCache,
	)

	base.SetupAndServeHTTP(string(base.Cfg.Listen.ClientAPI))
}

func StartFederationAPIServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	/*
		accountDB := base.CreateAccountsDB()
		keyDB := base.CreateKeyDB()
		federation := base.CreateFederationClient()
		keyRing := keydb.CreateKeyRing(federation.Client, keyDB)

		alias, input, query := base.CreateHTTPRoomserverAPIs()

		federationapi.SetupFederationAPIComponent(
			base, accountDB, federation, &keyRing,
			alias, input, query, nil,
		)

		base.SetupAndServeHTTP(string(base.Cfg.Listen.FederationAPI))
	*/
}

func StartPublicRoomAPIServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)
	publicRoomsDB := base.CreatePublicRoomApiDB()
	publicroomsapi.SetupPublicRoomsAPIComponent(base, rpcClient, nil, publicRoomsDB)

	base.SetupAndServeHTTP(string(base.Cfg.Listen.PublicRoomsAPI))
}

func StartPushAPIServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	cache := base.PrepareCache()
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)
	pushDB := base.CreatePushApiDB()
	pushDataRepo := repos.NewPushDataRepo(pushDB,base.Cfg)
	pushDataRepo.LoadHistory(context.TODO())
	pushapi.SetupPushAPIComponent(base, cache, rpcClient, pushDataRepo)
}

func StartRoomServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(false)

	cache := base.PrepareCache()
	newFederation := fed.NewFederation(base.Cfg, rpcClient)
	roomserver.SetupRoomServerComponent(base, true, rpcClient, cache, newFederation)
	base.SetupAndServeHTTP(string(base.Cfg.Listen.RoomServer))
}

func StartFederationSender(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	/*
		federation := base.CreateFederationClient()
		_, _, query := base.CreateHTTPRoomserverAPIs()

		federationsender.SetupFederationSenderComponent(base, federation, query)
		base.SetupAndServeHTTP(string(base.Cfg.Listen.FederationSender))
	*/
}

func StartFixDBServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.OutputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.InputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.OutputClientData)
	addProducer(transportMultiplexer, kafka.Producer.OutputProfileData)
	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	addProducer(transportMultiplexer, kafka.Producer.OutputRoomFedEvent)

	transportMultiplexer.PreStart()
	transportMultiplexer.Start()

	roomserver.FixCorruptRooms(base)
}

func StartBgMng(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	deviceDB := base.CreateDeviceDB()
	syncDB := base.CreateSyncDB()
	encryptDB := base.CreateEncryptApiDB()
	serverConfDB := base.CreateServerConfDB()
	cache := base.PrepareCache()
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)
	tokenFilter := filter.GetFilterMng().Register("device", deviceDB)
	tokenFilter.Load()
	bgmng.SetupBgMngComponent(base, deviceDB, cache, encryptDB, syncDB, serverConfDB, rpcClient, tokenFilter, base.Cfg.DeviceMng.ScanUnActive, base.Cfg.DeviceMng.KickUnActive)
}
