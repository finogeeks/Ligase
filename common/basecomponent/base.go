// Copyright 2017 New Vector Ltd
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

package basecomponent

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/roomserver/rpc"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	_ "github.com/finogeeks/ligase/storage/implements"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/gorilla/mux"
)

// BaseDendrite is a base for creating new instances of dendrite. It parses
// command line flags and config, and exposes methods for creating various
// resources. All errors are handled by logging then exiting, so all methods
// should only be used during start up.
// Must be closed when shutting down.
type BaseDendrite struct {
	ComponentName string
	tracerCloser  io.Closer

	// APIMux should be used to register new public matrix api endpoints
	APIMux     *mux.Router
	Cfg        *config.Dendrite
	RedisCache service.Cache
}

// NewBaseDendrite creates a new instance to be used by a component.
// The componentName is used for logging purposes, and should be a friendly name
// of the compontent running, e.g. "SyncAPI"
func NewBaseDendrite(cfg *config.Dendrite, componentName string) *BaseDendrite {
	logCfg := new(log.LogConfig)
	logCfg.Level = cfg.Log.Level
	logCfg.Files = cfg.Log.Files
	/*for idx,f := range logCfg.Files {
		fs := strings.Split(f,"/")
		ls := len(fs)
		logCfg.Files[idx] = "/Users/chenzr/workspace/code/finochat/" + fs[ls-3] +  "/log/" + componentName + ".log"
	}*/
	logCfg.WriteToStdout = cfg.Log.WriteToStdout
	logCfg.ZapConfig.MaxSize = cfg.Log.ZapConfig.MaxSize
	logCfg.ZapConfig.MaxBackups = cfg.Log.ZapConfig.MaxBackups
	logCfg.ZapConfig.MaxAge = cfg.Log.ZapConfig.MaxAge
	logCfg.ZapConfig.LocalTime = cfg.Log.ZapConfig.LocalTime
	logCfg.ZapConfig.Compress = cfg.Log.ZapConfig.Compress
	logCfg.ZapConfig.JsonFormat = cfg.Log.ZapConfig.JsonFormat
	logCfg.ZapConfig.BtEnabled = cfg.Log.ZapConfig.BtEnabled
	logCfg.ZapConfig.BtLevel = cfg.Log.ZapConfig.BtLevel
	logCfg.ZapConfig.FieldSeparator = cfg.Log.ZapConfig.FieldSeparator
	log.Setup(logCfg)

	//add pid-file
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir, _ = filepath.Abs(filepath.Dir("."))
		logDir = logDir + "/log"
	}
	_ = os.Mkdir(logDir, os.ModePerm)
	f, _ := os.OpenFile(filepath.Join(logDir, componentName+".pid"), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	// f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	f.WriteString(strconv.Itoa(os.Getpid())) // faster than fmt.Sprintf
	f.Close()

	closer, err := cfg.SetupTracing("Dendrite" + componentName)
	if err != nil {
		log.Errorf("failed to start opentracing err:%v", err)
	}

	return &BaseDendrite{
		ComponentName: componentName,
		tracerCloser:  closer,
		Cfg:           cfg,
		APIMux:        mux.NewRouter(),
	}
}

// Close implements io.Closer
func (b *BaseDendrite) Close() error {
	return b.tracerCloser.Close()
}

// CreateRsRPCCli returns the AliasAPI and QueryAPI to hit
// the roomserver over tcp.
func (b *BaseDendrite) CreateRsRPCCli(rpcClient rpcService.RpcClient) roomserverapi.RoomserverRPCAPI {
	return rpc.NewRoomserverRpcClient(b.Cfg, rpcClient, nil, nil, nil)
}

// CreateDeviceDB creates a new instance of the device database. Should only be
// called once per component.
func (b *BaseDendrite) CreateDeviceDB() model.DeviceDatabase {
	db, err := common.GetDBInstance("devices", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to devices db err:%v", err)
	}

	return db.(model.DeviceDatabase)
}

// CreateAccountsDB creates a new instance of the accounts database. Should only
// be called once per component.
func (b *BaseDendrite) CreateAccountsDB() model.AccountsDatabase {
	db, err := common.GetDBInstance("accounts", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to accounts db err:%v", err)
	}

	return db.(model.AccountsDatabase)
}

func (b *BaseDendrite) CreateRoomDB() model.RoomServerDatabase {
	db, err := common.GetDBInstance("roomserver", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to room db err:%v", err)
	}

	rdb := db.(model.RoomServerDatabase)
	idg, _ := uid.NewDefaultIdGenerator(b.Cfg.Matrix.InstanceId)
	rdb.SetIDGenerator(idg)
	return rdb
}

func (b *BaseDendrite) CreatePushApiDB() model.PushAPIDatabase {
	db, err := common.GetDBInstance("pushapi", b.Cfg)

	if err != nil {
		log.Panicf("failed to connect to push api db err:%v", err)
	}

	return db.(model.PushAPIDatabase)
}

func (b *BaseDendrite) CreatePublicRoomApiDB() model.PublicRoomAPIDatabase {
	db, err := common.GetDBInstance("publicroomapi", b.Cfg)

	if err != nil {
		log.Panicf("failed to connect to public room api db err:%v", err)
	}

	pdb := db.(model.PublicRoomAPIDatabase)
	idg, _ := uid.NewDefaultIdGenerator(b.Cfg.Matrix.InstanceId)
	pdb.SetIDGenerator(idg)
	return pdb
}

func (b *BaseDendrite) CreateApplicationServiceDB() model.AppServiceDatabase {
	db, err := common.GetDBInstance("appservice", b.Cfg)

	if err != nil {
		log.Panicf("failed to connect to push api db err:%v", err)
	}

	return db.(model.AppServiceDatabase)
}

func (b *BaseDendrite) CreateEncryptApiDB() model.EncryptorAPIDatabase {
	db, err := common.GetDBInstance("encryptoapi", b.Cfg)

	if err != nil {
		log.Panicf("failed to connect to encrypt api db err:%v", err)
	}

	return db.(model.EncryptorAPIDatabase)
}

func (b *BaseDendrite) CreateSyncDB() model.SyncAPIDatabase {
	db, err := common.GetDBInstance("syncapi", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to sync db err:%v", err)
	}

	sdb := db.(model.SyncAPIDatabase)
	idg, _ := uid.NewDefaultIdGenerator(b.Cfg.Matrix.InstanceId)
	sdb.SetIDGenerator(idg)
	return sdb
}

func (b *BaseDendrite) CreatePresenceDB() model.PresenceDatabase {
	db, err := common.GetDBInstance("presence", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to presence db err:%v", err)
	}

	return db.(model.PresenceDatabase)
}

func (b *BaseDendrite) PrepareCache() service.Cache {
	if b.RedisCache != nil {
		return b.RedisCache
	}
	b.RedisCache = &cache.RedisCache{}
	err := b.RedisCache.Prepare(b.Cfg.Redis.Uris)
	if err != nil {
		log.Panicf("failed to connect to redis cache err:%v", err)
	}
	return b.RedisCache
}

// CreateKeyDB creates a new instance of the key database. Should only be called
// once per component.
/*func (b *BaseDendrite) CreateKeyDB() *keydb.Database {
	db, err := keydb.NewDatabase(b.Cfg.Database.ServerKey.Driver, b.Cfg.Database.ServerKey.Addresses)
	if err != nil {
		log.Panicf("failed to connect to keys db err:%v", err)
	}

	return db
}
*/
func (b *BaseDendrite) CreateKeyDB() model.KeyDatabase {
	db, err := common.GetDBInstance("serverkey", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to keys db err:%v", err)
	}

	return db.(model.KeyDatabase)
}

func (b *BaseDendrite) CreateServerConfDB() model.ConfigDatabase {
	db, err := common.GetDBInstance("server_conf", b.Cfg)
	if err != nil {
		log.Panicf("failed to connect to serverconf db err:%v", err)
	}

	return db.(model.ConfigDatabase)
}

// CreateFederationClient creates a new federation client. Should only be called
// once per component.
func (b *BaseDendrite) CreateFederationClient() *gomatrixserverlib.FederationClient {
	return gomatrixserverlib.NewFederationClient(
		gomatrixserverlib.ServerName(domain.FirstDomain), b.Cfg.Matrix.KeyID, b.Cfg.Matrix.PrivateKey, "", "", "",
	)
}

// SetupAndServeHTTP sets up the HTTP server to serve endpoints registered on
// ApiMux under /api/ and adds a prometheus handler under /metrics.
func (b *BaseDendrite) SetupAndServeHTTP(addr string) {
	common.SetupHTTPAPI(http.DefaultServeMux, common.WrapHandlerInCORS(b.APIMux))

	log.Infof("Starting %s server on %s", b.ComponentName, addr)

	err := http.ListenAndServe(addr, nil)

	if err != nil {
		log.Fatalf("failed to serve http, err:%v", err)
	}

	log.Infof("Stopped %s server on %s", b.ComponentName, addr)
}

func (b *BaseDendrite) CheckDomainCfg() {
	if !b.Cfg.Matrix.ServerFromDB {
		if len(b.Cfg.Matrix.ServerName) <= 0 {
			log.Panicf("len cfg matrix serverName <= 0 err")
		}
		domain.FirstDomain = b.Cfg.Matrix.ServerName[0]
	} else {
		serverNames := domain.DomainMngInsance.GetDomain()
		if len(serverNames) <= 0 {
			log.Panicf("len matrix serverName <= 0 err")
		}
		domain.FirstDomain = serverNames[0]
	}
}
