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
	"github.com/finogeeks/ligase/bgmng"
	"github.com/finogeeks/ligase/clientapi"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/encryptoapi"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/publicroomsapi"
	"github.com/finogeeks/ligase/rcsserver"
	"github.com/finogeeks/ligase/roomserver"
	"github.com/finogeeks/ligase/roomserver/consumers"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/implements/keydb"
)

func StartFrontServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.OutputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.InputRoomEvent)
	addProducer(transportMultiplexer, kafka.Producer.OutputClientData)
	addProducer(transportMultiplexer, kafka.Producer.OutputProfileData)
	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)
	addProducer(transportMultiplexer, kafka.Producer.OutputRoomFedEvent)
	addProducer(transportMultiplexer, kafka.Producer.SettingUpdate)
	addProducer(transportMultiplexer, kafka.Producer.UserInfoUpdate)
	addProducer(transportMultiplexer, kafka.Producer.DismissRoom)
	addProducer(transportMultiplexer, kafka.Producer.OutputStatic)

	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventPublicRooms, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.InputRoomEvent, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.DismissRoom, 0)

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()
	cache := base.PrepareCache()
	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	serverConfDB := base.CreateServerConfDB()
	domain.GetDomainMngInstance(cache, serverConfDB, base.Cfg.Matrix.ServerName, base.Cfg.Matrix.ServerFromDB, idg)
	base.CheckDomainCfg()
	accountDB := base.CreateAccountsDB()
	deviceDB := base.CreateDeviceDB()
	keyDB := base.CreateKeyDB()
	federation := base.CreateFederationClient()
	keyRing := keydb.CreateKeyRing(federation.Client, keyDB)
	syncDB := base.CreateSyncDB()
	presenceDB := base.CreatePresenceDB()

	rpcCli, err := rpcService.NewRpcClient(base.Cfg.Rpc.Driver, base.Cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", base.Cfg.Rpc.Driver, err)
	}

	newFederation := fed.NewFederation(base.Cfg, rpcCli)

	_, rsRpcCli, roomDB := roomserver.SetupRoomServerComponent(base, true, rpcCli, cache, newFederation)

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

	consumer := consumers.NewDismissRoomConsumer(
		kafka.Consumer.DismissRoom.Underlying,
		kafka.Consumer.DismissRoom.Name,
		rsRpcCli,
		cache,
		accountDB,
		base.Cfg,
		newFederation,
		complexCache,
		idg)
	if err := consumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}
	encryptDB := encryptoapi.SetupEncryptApi(base, cache, rpcCli, federation, idg)
	clientapi.SetupClientAPIComponent(base, deviceDB, cache, accountDB, newFederation, &keyRing, rsRpcCli, encryptDB, syncDB, presenceDB, roomDB, rpcCli, tokenFilter, idg, settings, feddomains, complexCache)
	publicRoomsDB := base.CreatePublicRoomApiDB()
	publicroomsapi.SetupPublicRoomsAPIComponent(base, rpcCli, rsRpcCli, publicRoomsDB)
	bgmng.SetupBgMngComponent(base, deviceDB, cache, encryptDB, syncDB, serverConfDB, rpcCli, tokenFilter, base.Cfg.DeviceMng.ScanUnActive, base.Cfg.DeviceMng.KickUnActive)
	rcsserver.SetupRCSServerComponent(base, rpcCli)
}
