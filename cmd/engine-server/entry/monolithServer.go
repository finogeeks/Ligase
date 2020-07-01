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
	"github.com/finogeeks/ligase/appservice"
	"github.com/finogeeks/ligase/bgmng"
	"github.com/finogeeks/ligase/cachewriter"
	"github.com/finogeeks/ligase/clientapi"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/rcsserver"
	"github.com/finogeeks/ligase/skunkworks/log"

	// "github.com/finogeeks/ligase/common/keydb"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/dbwriter"
	"github.com/finogeeks/ligase/encryptoapi"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/proxy"
	"github.com/finogeeks/ligase/publicroomsapi"
	"github.com/finogeeks/ligase/pushapi"
	"github.com/finogeeks/ligase/pushsender"
	"github.com/finogeeks/ligase/roomserver"
	"github.com/finogeeks/ligase/storage/implements/keydb"
	"github.com/finogeeks/ligase/syncaggregate"
	"github.com/finogeeks/ligase/syncserver"
	"github.com/finogeeks/ligase/syncwriter"
)

func StartMonolithServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)

	addProducer(transportMultiplexer, kafka.Producer.OutputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.InputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.OutputClientData)
	addProducer(transportMultiplexer, kafka.Producer.OutputProfileData)
	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	addProducer(transportMultiplexer, kafka.Producer.OutputRoomFedEvent)
	addProducer(transportMultiplexer, kafka.Producer.FedEduUpdate)
	addProducer(transportMultiplexer, kafka.Producer.FedBridgeOut)
	addProducer(transportMultiplexer, kafka.Producer.FedBridgeOutHs)
	addProducer(transportMultiplexer, kafka.Producer.DeviceStateUpdate)
	addProducer(transportMultiplexer, kafka.Producer.SettingUpdate)
	addProducer(transportMultiplexer, kafka.Producer.UserInfoUpdate)
	addProducer(transportMultiplexer, kafka.Producer.DismissRoom)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventPublicRooms, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventAppservice, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncServer, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncWriter, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.InputRoomEvent, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputClientData, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputProfileSyncServer, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.OutputProfileSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.CacheUpdates, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.DBUpdates, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.FedBridgeOutRes, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdateSyncAggregate, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdateSyncServer, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.SetttngUpdateProxy, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.DismissRoom, base.Cfg.MultiInstance.Instance)

	transportMultiplexer.PreStart()
	cache := base.PrepareCache()

	/*
		idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
		rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
		rpcClient.Start(true)
	*/

	serverConfDB := base.CreateServerConfDB()
	domain.GetDomainMngInstance(cache, serverConfDB, base.Cfg.Matrix.ServerName, base.Cfg.Matrix.ServerFromDB, idg)
	base.CheckDomainCfg()

	accountDB := base.CreateAccountsDB()
	deviceDB := base.CreateDeviceDB()
	keyDB := base.CreateKeyDB()
	federation := base.CreateFederationClient()
	keyRing := keydb.CreateKeyRing(federation.Client, keyDB)
	cachewriter.SetupCacheWriterComponent(base)
	syncDB := base.CreateSyncDB()
	presenceDB := base.CreatePresenceDB()

	tokenFilter := filter.GetFilterMng().Register("device", deviceDB)
	tokenFilter.Load()
	newTokenFilter := filter.NewSimpleFilter(deviceDB)
	newTokenFilter.Load()

	newFederation := fed.NewFederation(base.Cfg, rpcClient)

	_, rsRpcCli, roomDB := roomserver.SetupRoomServerComponent(base, true, rpcClient, cache, newFederation)
	dbwriter.SetupDBWriterComponent(base)

	pushsender.SetupPushSenderComponent(base, rpcClient)

	encryptDB := encryptoapi.SetupEncryptApi(base, cache, rpcClient, federation, idg)
	pushapi.SetupPushAPIComponent(base, cache, rpcClient)

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

	clientapi.SetupClientAPIComponent(base, deviceDB, cache, accountDB, newFederation, &keyRing, rsRpcCli, encryptDB, syncDB, presenceDB, roomDB, rpcClient, tokenFilter, idg, settings, feddomains, complexCache)

	syncserver.SetupSyncServerComponent(base, accountDB, cache, rpcClient, idg)
	//federationapi.SetupFederationAPIComponent(base, accountDB, federation, &keyRing, alias, input, query, cache)
	//federationsender.SetupFederationSenderComponent(base, federation, query)

	publicRoomsDB := base.CreatePublicRoomApiDB()
	publicroomsapi.SetupPublicRoomsAPIComponent(base, rpcClient, rsRpcCli, publicRoomsDB)
	appservice.SetupApplicationServiceComponent(base)
	StartCacheLoader(base, cmd)
	//pushDB := base.CreatePushApiDB()
	//roomServerDB := base.CreateRoomDB()
	//migration.SetupMigrationComponent(base, accountDB, deviceDB, pushDB, roomServerDB, idg, syncDB)
	//tokenrewrite.SetupTokenRewrite(rpcClient, base.Cfg)
	syncwriter.SetupSyncWriterComponent(base)
	syncaggregate.SetupSyncAggregateComponent(base, cache, rpcClient, idg, complexCache)
	proxy.SetupProxy(base, cache, rpcClient, rsRpcCli, newTokenFilter)
	bgmng.SetupBgMngComponent(base, deviceDB, cache, encryptDB, syncDB, serverConfDB, rpcClient, tokenFilter, base.Cfg.DeviceMng.ScanUnActive, base.Cfg.DeviceMng.KickUnActive)
	rcsserver.SetupRCSServerComponent(base, rpcClient)
}
