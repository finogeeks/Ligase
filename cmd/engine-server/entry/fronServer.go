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
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/bgmgr"
	"github.com/finogeeks/ligase/clientapi"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rcsserver"
	"github.com/finogeeks/ligase/roomserver/consumers"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"

	//"github.com/finogeeks/ligase/common/keydb"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/encryptoapi"
	"github.com/finogeeks/ligase/publicroomsapi"
	"github.com/finogeeks/ligase/pushapi"
	"github.com/finogeeks/ligase/roomserver"
	"github.com/finogeeks/ligase/storage/implements/keydb"

	fed "github.com/finogeeks/ligase/federation/fedreq"
)

func StartFrontServer(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	cache := base.PrepareCache()
	serverConfDB := base.CreateServerConfDB()
	inst, err := genServerInstanceID("front", cache, serverConfDB)
	if err != nil {
		log.Panicf("get front instance err:%v", err)
	}
	base.Cfg.Matrix.InstanceId = inst
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
	addConsumer(transportMultiplexer, kafka.Consumer.OutputRoomEventPublicRooms, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.InputRoomEvent, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.DismissRoom, base.Cfg.MultiInstance.Instance)

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()

	idg, _ := uid.NewDefaultIdGenerator(base.Cfg.Matrix.InstanceId)
	rpcClient := common.NewRpcClient(base.Cfg.Nats.Uri, idg)
	rpcClient.Start(true)

	domain.GetDomainMngInstance(cache, serverConfDB, base.Cfg.Matrix.ServerName, base.Cfg.Matrix.ServerFromDB, idg)
	base.CheckDomainCfg()
	accountDB := base.CreateAccountsDB()
	deviceDB := base.CreateDeviceDB()
	keyDB := base.CreateKeyDB()
	federation := base.CreateFederationClient()
	keyRing := keydb.CreateKeyRing(federation.Client, keyDB)
	syncDB := base.CreateSyncDB()
	presenceDB := base.CreatePresenceDB()

	newFederation := fed.NewFederation(base.Cfg, rpcClient)

	_, rsRpcCli, roomDB := roomserver.SetupRoomServerComponent(base, true, rpcClient, cache, newFederation)

	tokenFilter := filter.GetFilterMng().Register("device", deviceDB)
	tokenFilter.Load()

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
	pushapi.SetupPushAPIComponent(base, cache, rpcClient)
	encryptDB := encryptoapi.SetupEncryptApi(base, cache, rpcClient, federation, idg)
	clientapi.SetupClientAPIComponent(base, deviceDB, cache, accountDB, newFederation, &keyRing, rsRpcCli, encryptDB, syncDB, presenceDB, roomDB, rpcClient, tokenFilter, idg, complexCache, serverConfDB)
	publicRoomsDB := base.CreatePublicRoomApiDB()
	publicroomsapi.SetupPublicRoomsAPIComponent(base, rpcClient, rsRpcCli, publicRoomsDB)
	bgmgr.SetupBgMgrComponent(deviceDB, cache, encryptDB, syncDB, rpcClient, tokenFilter, base.Cfg.DeviceMng.ScanUnActive, base.Cfg.DeviceMng.KickUnActive)
	rcsserver.SetupRCSServerComponent(base, rpcClient)
}

func genServerInstanceID(serverName string, cache service.Cache, db model.ConfigDatabase) (int, error) {
	lockKey := types.LOCK_INSTANCE_PREFIX + serverName
	token, err := cache.Lock(lockKey, adapter.GetDistLockCfg().LockInstance.Timeout, adapter.GetDistLockCfg().LockInstance.Wait)
	if err != nil {
		log.Errorf("lock key:%s token:%s err:%v", lockKey, token, err)
	}
	defer func(start int64) {
		err := cache.UnLock(lockKey, token, adapter.GetDistLockCfg().LockInstance.Force)
		if err != nil {
			log.Errorf("unlock key:%s token:%s err:%v", lockKey, token, err)
		}
		log.Infof("dist lock key:%s token:%s spend:%d us", lockKey, token, time.Now().UnixNano()/1000-start)
	}(time.Now().UnixNano() / 1000)

	inst, err := db.SelectServerInstance(context.TODO(), serverName)
	if err != nil {
		return int(inst), err
	}
	next := 0
	if inst >= 10000 {
		next = 0
	} else {
		next = int(inst) + 1
	}
	err = db.UpsertServerInstance(context.TODO(), int64(next), serverName)
	log.Infof("get instance from db is:%d", inst)
	return int(inst), err
}
