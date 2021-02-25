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
	"github.com/finogeeks/ligase/common/uid"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/publicroomsapi"
	"github.com/finogeeks/ligase/pushapi"
	"github.com/finogeeks/ligase/roomserver"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
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

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()
	cache := base.PrepareCache()

	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)

	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}

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

	rsRpcCli := base.CreateRsRPCCli(rpcCli)

	federation := fed.NewFederation(base.Cfg, rpcCli)

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
		rsRpcCli, encryptDB, syncDB, presenceDB, roomDB, rpcCli, tokenFilter, idg, settings, feddomains, complexCache,
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
	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}
	publicRoomsDB := base.CreatePublicRoomApiDB()
	publicroomsapi.SetupPublicRoomsAPIComponent(base, rpcCli, nil, publicRoomsDB)

	base.SetupAndServeHTTP(string(base.Cfg.Listen.PublicRoomsAPI))
}

func StartPushAPIServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	cache := base.PrepareCache()
	pushDB := base.CreatePushApiDB()
	pushDataRepo := repos.NewPushDataRepo(pushDB, base.Cfg)
	pushDataRepo.LoadHistory(context.TODO())
	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}

	pushapi.SetupPushAPIComponent(base, cache, rpcCli, pushDataRepo)
}

func StartRoomServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}

	cache := base.PrepareCache()
	newFederation := fed.NewFederation(base.Cfg, rpcCli)
	roomserver.SetupRoomServerComponent(base, true, rpcCli, cache, newFederation)
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

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

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
	tokenFilter := filter.GetFilterMng().Register("device", deviceDB)
	tokenFilter.Load()
	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}
	bgmng.SetupBgMngComponent(base, deviceDB, cache, encryptDB, syncDB, serverConfDB, rpcCli, tokenFilter, base.Cfg.DeviceMng.ScanUnActive, base.Cfg.DeviceMng.KickUnActive)
}
