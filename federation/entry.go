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

package federation

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/client/cert"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/federation/fedbackfill"
	"github.com/finogeeks/ligase/federation/federationapi"
	"github.com/finogeeks/ligase/federation/federationapi/rpc"
	"github.com/finogeeks/ligase/federation/fedmissing"
	"github.com/finogeeks/ligase/federation/fedsender"
	"github.com/finogeeks/ligase/federation/fedsync"
	fedrepos "github.com/finogeeks/ligase/federation/model/repos"
	_ "github.com/finogeeks/ligase/federation/storage/implements"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model/repos"
	_ "github.com/finogeeks/ligase/plugins"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	_ "github.com/finogeeks/ligase/storage/implements"
	"github.com/finogeeks/ligase/storage/model"
)

var (
	configPath    = flag.String("config", "config/fed.yaml", "The path to the config file. For more information, see the config file in this repository.")
	procName      = flag.String("name", "monolith", "Name for the server")
	httpBindAddr  = flag.String("http-address", "", "The HTTP listening port for the server")
	httpsBindAddr = flag.String("https-address", "", "The HTTPS listening port for the server")
)

func Entry() {
	flag.Parse()
	if *configPath == "" {
		log.Fatal("config must be supplied")
	}
	handleSignal()

	if err := config.Load(*configPath); err != nil {
		log.Fatalf("load config failed: %v, exit!", err)
		return
	}

	log.Infof("-------------------------------------")
	log.Infof("Server build:%s", BUILD)
	log.Infof("Server version:%s", VERSION)
	log.Infof("-------------------------------------")

	startFedMonolith()

	if httpBindAddr != nil && *httpBindAddr != "" {
		listenHTTP(*httpBindAddr)
	}

	select {}
}

func addProducer(mult core.IMultiplexer, conf config.ProducerConf) {
	val, ok := common.GetTransportMultiplexer().GetNode(conf.Underlying)
	if ok {
		tran := val.(core.ITransport)
		inst := conf.Inst
		if inst <= 0 {
			inst = adapter.GetKafkaNumProducers()
		}
		if inst <= 1 {
			tran.AddChannel(core.CHANNEL_PUB, conf.Name, conf.Topic, "", &conf)
		} else {
			for i := 0; i < inst; i++ {
				name := conf.Name + strconv.Itoa(i)
				tran.AddChannel(core.CHANNEL_PUB, name, conf.Topic, "", &conf)
			}
		}
	} else {
		log.Errorf("addProducer can't find transport %s", conf.Underlying)
	}
}

func addConsumer(mult core.IMultiplexer, conf config.ConsumerConf) {
	val, ok := common.GetTransportMultiplexer().GetNode(conf.Underlying)
	if ok {
		tran := val.(core.ITransport)
		tran.AddChannel(core.CHANNEL_SUB, conf.Name, conf.Topic, conf.Group, &conf)
	} else {
		log.Errorf("addConsumer can't find transport %s", conf.Underlying)
	}
}

func startFedMonolith() {
	cfg := config.GetFedConfig()

	transportMultiplexer, _ := core.GetMultiplexer("transport", nil)
	for _, v := range cfg.TransportConfs {
		tran, err := core.GetTransport(v.Name, v.Underlying, v)
		if err != nil {
			log.Fatalf("get transport name:%s underlying:%s fail err:%v", v.Name, v.Underlying, err)
		}
		tran.Init(false)
		tran.SetBrokers(v.Addresses)
		transportMultiplexer.AddNode(v.Name, tran)
	}

	common.SetTransportMultiplexer(transportMultiplexer)
	kafka := &cfg.Kafka
	addProducer(transportMultiplexer, kafka.Producer.DispatchOutput)
	addProducer(transportMultiplexer, kafka.Producer.FedAPIOutput)
	addProducer(transportMultiplexer, kafka.Producer.GetMissingEvent)
	addProducer(transportMultiplexer, kafka.Producer.DownloadMedia)
	addProducer(transportMultiplexer, kafka.Producer.InputRoomEvent)
	addConsumer(transportMultiplexer, kafka.Consumer.DispatchInput)
	addConsumer(transportMultiplexer, kafka.Consumer.SenderInput)
	addConsumer(transportMultiplexer, kafka.Consumer.FedAPIInput)
	addConsumer(transportMultiplexer, kafka.Consumer.FedBackFill)
	addConsumer(transportMultiplexer, kafka.Consumer.EduSenderInput)
	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdate)
	addConsumer(transportMultiplexer, kafka.Consumer.GetMissingEvent)
	transportMultiplexer.PreStart()

	// check cert
	kdb, err := common.GetDBInstance("serverkey", &cfg)
	if err != nil {
		log.Panicw("failed to connect to serverkey db", log.KeysAndValues{"error", err})
	}
	keyDB := kdb.(model.KeyDatabase)
	certInfo := cert.NewCert(
		cfg.NotaryService.CliHttpsEnable,
		cfg.NotaryService.RootCAUrl,
		cfg.NotaryService.CertUrl,
		cfg.NotaryService.CRLUrl,
		cfg.GetServerName(),
		keyDB,
	)
	client.SetCerts(certInfo.GetCerts())
	if err = certInfo.Load(); err != nil {
		log.Panicw("failed to load cert", log.KeysAndValues{"error", err})
	}

	cache := &cache.RedisCache{}
	err = cache.Prepare(cfg.Redis.Uris)
	if err != nil {
		log.Panicf("failed to connect to redis cache err:%v", err)
	}

	settings := common.NewSettings(cache)

	idg, _ := uid.NewIdGenerator(0, 0)
	rpcClient := common.NewRpcClient(cfg.GetMsgBusAddress(), idg)
	rpcClient.Start(true)

	fedRpcCli := rpc.NewFederationRpcClient(&cfg, rpcClient, nil, nil, nil)

	settingConsumer := common.NewSettingConsumer(
		cfg.Kafka.Consumer.SettingUpdate.Underlying,
		cfg.Kafka.Consumer.SettingUpdate.Name,
		settings)
	if err := settingConsumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}

	feddomains := common.NewFedDomains(settings)
	settings.RegisterFederationDomainsUpdateCallback(feddomains.OnFedDomainsUpdate)

	fedClient, err := client.GetFedClient(cfg.GetServerName()[0])
	if err != nil {
		log.Panicf(err.Error())
	}
	fedSync := fedsync.NewFederationSync(&cfg, fedClient, feddomains)

	fdb, err := common.GetDBInstance("federation", &cfg)
	if err != nil {
		log.Panicw("failed to connect to federation db", log.KeysAndValues{"error", err})
	}
	fedDB := fdb.(fedmodel.FederationDatabase)

	backfillRepo := fedrepos.NewBackfillRepo(fedDB, cache)
	backfillRepo.LoadHistory()
	joinRoomsRepo := fedrepos.NewJoinRoomsRepo(fedDB)

	rdb, err := common.GetDBInstance("roomserver", &cfg)
	if err != nil {
		log.Panicw("failed to connect to room server db", log.KeysAndValues{"error", err})
	}

	roomserverDB := rdb.(model.RoomServerDatabase)

	backfill := fedbackfill.NewFederationBackFill(&cfg, fedClient, feddomains, backfillRepo)
	val, ok := common.GetTransportMultiplexer().GetChannel(
		kafka.Consumer.FedBackFill.Underlying,
		kafka.Consumer.FedBackFill.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		channel.SetHandler(backfill)
		channel.Start()
	}

	adb, err := common.GetDBInstance("accounts", &cfg)
	if err != nil {
		log.Panicw("failed to connect to room account db", log.KeysAndValues{"error", err})
	}
	accountDB := adb.(model.AccountsDatabase)
	complexCache := common.NewComplexCache(accountDB, cache)

	edb, err := common.GetDBInstance("encryptoapi", &cfg)
	if err != nil {
		log.Panicw("failed to connect to encryptoapi db", log.KeysAndValues{"error", err})
	}
	encrytionDB := edb.(model.EncryptorAPIDatabase)

	publicroomsAPI := rpc.NewFedPublicRoomsRpcClient(&cfg, rpcClient)

	fedAPIEntry := federationapi.NewFederationAPIComponent(&cfg, cache, fedClient, fedDB, keyDB,
		feddomains, fedRpcCli, backfillRepo, joinRoomsRepo, backfill, publicroomsAPI,
		rpcClient, encrytionDB, certInfo, idg, complexCache)

	//subject := fmt.Sprintf("%s.%s", fed.cfg.GetMsgBusReqTopic(), ">")
	//fed.NatsBus.SubRegister(subject, "federation-msgbus")
	val, ok = common.GetTransportMultiplexer().GetChannel(
		kafka.Consumer.FedAPIInput.Underlying,
		kafka.Consumer.FedAPIInput.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		channel.SetHandler(fedAPIEntry)
		channel.Start()
	}

	common.GetTransportMultiplexer().Start()

	fedSync.Setup(backfill)
	fedAPIEntry.Setup()

	monitor := mon.GetInstance()
	queryHitCounter := monitor.NewLabeledCounter("federation_query_hit", []string{"target", "repo", "func"})

	repo := repos.NewRoomServerCurStateRepo(roomserverDB, cache, queryHitCounter)

	cdb, err := common.GetDBInstance("server_conf", &cfg)
	if err != nil {
		log.Panicw("failed to connect to serverconf db", log.KeysAndValues{"error", err})
	}
	serverConfDB := cdb.(model.ConfigDatabase)
	domain.GetDomainMngInstance(cache, serverConfDB, cfg.GetServerName(), cfg.GetServerFromDB(), idg)
	checkDomainCfg(cfg)
	fedAPIEntry.SetRepo(repo)
	sendRecRepo := fedrepos.NewSendRecRepo(fedDB, cache)
	err = sendRecRepo.LoadRooms()
	if err != nil {
		log.Panicw("load send record", log.KeysAndValues{"error", err})
	}

	sender := fedsender.NewFederationSender(&cfg, rpcClient, feddomains)
	sender.SetRsRepo(repo)
	sender.SetRecRepo(sendRecRepo)
	sender.Start()

	dispatch := fedsender.NewFederationDispatch(&cfg)
	dispatch.SetRepo(repo)
	dispatch.SetSender(sender)
	dispatch.Start()

	eduSender := fedsender.NewEduSender(&cfg, rpcClient)
	eduSender.SetSender(sender)
	eduSender.Start()

	getMissingEventProcessor := fedmissing.NewGetMissingEventsProcessor(
		fedClient, fedRpcCli, fedDB, feddomains, &cfg,
	)
	err = getMissingEventProcessor.Start()
	if err != nil {
		log.Panicw("failed to start GetMissingEventsProcessor", log.KeysAndValues{"error", err})
	}
}

func checkDomainCfg(cfg config.Fed) {
	if !cfg.GetServerFromDB() {
		if len(cfg.GetServerName()) <= 0 {
			log.Panicf("len cfg matrix serverName <= 0 err")
		}
		domain.FirstDomain = cfg.Homeserver.ServerName[0]
	} else {
		serverNames := domain.DomainMngInsance.GetDomain()
		if len(serverNames) <= 0 {
			log.Panicf("len matrix serverName <= 0 err")
		}
		domain.FirstDomain = serverNames[0]
	}
}

func handleSignal() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range sig {
			switch s {
			case syscall.SIGINT:
				log.Warnf("quit", s)
				os.Exit(0)
			default:
				log.Warnf("other", s)
			}
		}
	}()
}

func listenHTTP(bindAddr string) {
	go func() {
		log.Info("Start http listening on ", bindAddr)
		log.Fatal(http.ListenAndServe(bindAddr, nil))
	}()
}
