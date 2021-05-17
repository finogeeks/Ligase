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

package proxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/proxy/bridge"
	"github.com/finogeeks/ligase/proxy/consumers"
	"github.com/finogeeks/ligase/proxy/handler"
	"github.com/finogeeks/ligase/proxy/routing"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/consul"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

var (
	Certs sync.Map
)

// SetupClientAPIComponent sets up and registers HTTP handlers for the ClientAPI
// component.
func SetupProxy(
	base *basecomponent.BaseDendrite,
	cache service.Cache,
	rpcCli *common.RpcClient,
	rpcClient rpc.RpcClient,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	tokenFilter *filter.SimpleFilter,
) {
	bridge.SetupBridge(base.Cfg)

	if base.Cfg.Rpc.Driver == "nats" {
		tokenFilterConsumer := consumers.NewFilterTokenConsumer(rpcCli, tokenFilter)
		tokenFilterConsumer.Start()

		verifyTokenConsumer := consumers.NewVerifyTokenConsumer(rpcCli, tokenFilter, cache, base.Cfg)
		verifyTokenConsumer.Start()
	} else {
		grpcServer := consumers.NewServer(base.Cfg, tokenFilter, cache)
		if err := grpcServer.Start(); err != nil {
			log.Panicf("failed to start proxy rpc server err:%v", err)
		}
	}

	if base.Cfg.Rpc.Driver == "grpc_with_consul" {
		if base.Cfg.Rpc.ConsulURL == "" {
			log.Panicf("grpc_with_consul consul url is null")
		}
		consulTag := base.Cfg.Rpc.Proxy.ConsulTagPrefix + "0"
		c := consul.NewConsul(base.Cfg.Rpc.ConsulURL, consulTag, base.Cfg.Rpc.Proxy.ServerName, base.Cfg.Rpc.Proxy.Port)
		c.Init()
	}

	settings := common.NewSettings(cache)
	settingConsumer := common.NewSettingConsumer(
		base.Cfg.Kafka.Consumer.SetttngUpdateProxy.Underlying,
		base.Cfg.Kafka.Consumer.SetttngUpdateProxy.Name,
		settings)
	if err := settingConsumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}
	feddomains := common.NewFedDomains(settings)
	settings.RegisterFederationDomainsUpdateCallback(feddomains.OnFedDomainsUpdate)

	// check cert
	keyDB := base.CreateKeyDB()
	//if err := loadCert(keyDB); err != nil {
	//	log.Panicf("proxy load certs failed, err: %v", err)
	//}

	routing.Setup(
		base.APIMux, *base.Cfg, cache, rpcCli, rpcClient, rsRpcCli, tokenFilter, feddomains, keyDB,
	)
}

func loadCert(keyDB model.KeyDatabase) error {
	if config.GetConfig().NotaryService.CliHttpsEnable == false &&
		config.GetConfig().NotaryService.SrvHttpsEnable == false {
		log.Infof("--------------notary service is disable")
		return nil
	}

	rootCA, serverCert, serverKey, _, _ := keyDB.SelectAllCerts(context.TODO())

	// debug
	/*
		rootCAData, err := ioutil.ReadFile("/Users/joey/go/src/dendrite/dendrite/myca/root/ca.crt")
		if err != nil {
			return err
		}
		host := config.GetConfig().Matrix.ServerName[0]
		serverCertData, err := ioutil.ReadFile("/Users/joey/go/src/dendrite/dendrite/myca/" + host + ".crt")
		if err != nil {
			return err
		}
		serverKeyData, err := ioutil.ReadFile("/Users/joey/go/src/dendrite/dendrite/myca/" + host + ".key")
		if err != nil {
			return err
		}
		rootCA := string(rootCAData)
		serverCert := string(serverCertData)
		serverKey := string(serverKeyData)
	*/

	if rootCA == "" {
		reqUrl := config.GetConfig().NotaryService.RootCAUrl
		resp, err := handler.DownloadFromNotary("rootCA", reqUrl, keyDB)
		if err != nil {
			return err
		}
		rootCA = resp.RootCA
	}
	if serverCert == "" || serverKey == "" {
		reqUrl := fmt.Sprintf(config.GetConfig().NotaryService.CertUrl, config.GetConfig().Matrix.ServerName[0])
		// reqUrl := fmt.Sprintf(config.GetConfig().NotaryService.CertUrl, "dev.finogeeks.club") // debug
		resp, err := handler.DownloadFromNotary("cert", reqUrl, keyDB)
		if err != nil {
			return err
		}
		serverCert = resp.ServerCert
		serverKey = resp.ServerKey
	}

	Certs.Store("rootCA", rootCA)
	Certs.Store("serverCert", serverCert)
	Certs.Store("serverKey", serverKey)

	log.Infof("load cert succeed")

	return nil
}
