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

package content

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/content/download"
	"github.com/finogeeks/ligase/content/repos"
	"github.com/finogeeks/ligase/content/routing"
	_ "github.com/finogeeks/ligase/content/storage/implements"
	"github.com/finogeeks/ligase/content/storage/model"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/client/cert"
	_ "github.com/finogeeks/ligase/plugins"
	"github.com/finogeeks/ligase/skunkworks/log"
	hm "github.com/finogeeks/ligase/skunkworks/monitor/go-client/httpmonitor"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	_ "github.com/finogeeks/ligase/storage/implements"
	models "github.com/finogeeks/ligase/storage/model"
)

type serverCmdPar struct {
	logPorf       *bool
	httpBindAddr  *string
	httpsBindAddr *string
}

var cmdLine = &serverCmdPar{
	httpBindAddr: flag.String("http-address", "", "The HTTP listening port for the server"),
	//httpsBindAddr: flag.String("https-address", "", "The HTTPS listening port for the server"),
	logPorf: flag.Bool("log-porf", true, "log server porfmance"),
}

const usageDoc = `Entry point for content-server.
usage:
        content-server [<flag> ...] <Go file or directory> ...
Flags
		--http-address			http listening port, default 8008
		--log-porf				check log server porformance, default true
`

var defaultListenAddr = ":8008"

func usage() {
	log.Println(usageDoc)
	os.Exit(2)
}

func myNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNotFound)

	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Errorf("url:%s not found", r.URL.String())
		return
	}
	log.Errorf("url:%s not found, content:%s", r.URL.String(), string(requestDump))
}

func listenHTTP(cmd *serverCmdPar) {
	go func() {
		log.Info("Start http listening on ", *cmd.httpBindAddr)
		log.Fatal(http.ListenAndServe(*cmd.httpBindAddr, nil))
	}()
}

func logSysPorformance(cmd *serverCmdPar, name string) {
	go func() {
		name = strings.Replace(name, "-", "_", -1)
		monitor := mon.GetInstance()
		t := time.NewTimer(time.Second * time.Duration(60))
		memGauge := monitor.NewLabeledGauge(name+"_mem_gauge", []string{"app", "index"})
		routineGauge := monitor.NewLabeledGauge(name+"_goroutine_gauge", []string{"app"})
		for {
			select {
			case <-t.C:
				state := new(runtime.MemStats)
				runtime.ReadMemStats(state)
				memGauge.WithLabelValues(name, "alloc").Set(float64(state.Alloc / 1000))
				memGauge.WithLabelValues(name, "sys").Set(float64(state.Sys / 1000))
				memGauge.WithLabelValues(name, "alloctime").Set(float64(state.Mallocs))
				memGauge.WithLabelValues(name, "freetime").Set(float64(state.Frees))
				memGauge.WithLabelValues(name, "gc").Set(float64(state.NumGC))
				memGauge.WithLabelValues(name, "heapobjects").Set(float64(state.HeapObjects))
				routineGauge.WithLabelValues(name).Set(float64(runtime.NumGoroutine()))
				log.Infof("mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
					state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
				t.Reset(time.Second * 15)
			}
		}
	}()
}

func setUpTransport(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer, _ := core.GetMultiplexer("transport", nil)
	for _, v := range base.Cfg.TransportConfs {
		tran, err := core.GetTransport(v.Name, v.Underlying, v)
		if err != nil {
			log.Fatalf("get transport name:%s underlying:%s fail", v.Name, v.Underlying)
		}
		tran.Init(*cmd.logPorf)
		tran.SetBrokers(v.Addresses)
		tran.SetStatsInterval(base.Cfg.Kafka.Statistics.ConsumerInterval)
		transportMultiplexer.AddNode(v.Name, tran)
	}

	common.SetTransportMultiplexer(transportMultiplexer)
}

func addProducer(mult core.IMultiplexer, conf config.ProducerConf) {
	val, ok := common.GetTransportMultiplexer().GetNode(conf.Underlying)
	if ok {
		tran := val.(core.ITransport)
		tran.AddChannel(core.CHANNEL_PUB, conf.Name, conf.Topic, "", &conf)
	} else {
		log.Errorf("AddProducer can't find transport %s, topic:%s", conf.Underlying, conf.Topic)
	}
}

func addConsumer(mult core.IMultiplexer, conf config.ConsumerConf, instance uint32) {
	val, ok := common.GetTransportMultiplexer().GetNode(conf.Underlying)
	if ok {
		tran := val.(core.ITransport)
		group := fmt.Sprintf("%s%d", conf.Group, instance)
		tran.AddChannel(core.CHANNEL_SUB, conf.Name, conf.Topic, group, &conf)
	} else {
		log.Errorf("AddConsumer can't find transport %s, topic:%s", conf.Underlying, conf.Topic)
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

func Entry() {
	procName := "content"
	flag.Usage = usage

	basecomponent.ParseMonolithFlags()
	cfg := config.GetConfig()

	base := basecomponent.NewBaseDendrite(cfg, procName)
	base.APIMux.NotFoundHandler = http.HandlerFunc(myNotFound)
	defer base.Close() // nolint: errcheck

	if len(cfg.Log.Files) == 0 {
		log.Warn("Lack of log files")
	} else if cfg.Log.RedirectStderr {
		//initPanicFile(cfg.Log.Files[0])
	}

	log.Infof("-------------------------------------")
	log.Infof("Server build:%s", BUILD)
	log.Infof("Server version:%s", VERSION)
	log.Infof("-------------------------------------")

	handleSignal()

	if cmdLine.httpBindAddr == nil || *cmdLine.httpBindAddr == "" {
		*cmdLine.httpBindAddr = defaultListenAddr
	}

	setUpTransport(base, cmdLine)

	startContentService(base, cmdLine)

	transportMultiplexer := common.GetTransportMultiplexer()
	transportMultiplexer.Start()

	httpHandler := common.WrapHandlerInCORS(base.APIMux)
	http.Handle("/", hm.Wrap2(httpHandler))

	if cmdLine.httpBindAddr != nil && *cmdLine.httpBindAddr != "" {
		listenHTTP(cmdLine)
	}

	if *cmdLine.logPorf == true {
		logSysPorformance(cmdLine, procName)
	}

	select {}
}

func startContentService(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	cfg := base.Cfg
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addConsumer(transportMultiplexer, kafka.Consumer.SettingUpdateContent, base.Cfg.MultiInstance.Instance)
	addConsumer(transportMultiplexer, kafka.Consumer.DownloadMedia, base.Cfg.MultiInstance.Instance)

	transportMultiplexer.PreStart()

	kdb, err := common.GetDBInstance("serverkey", cfg)
	if err != nil {
		log.Panicw("failed to connect to serverkey db", log.KeysAndValues{"error", err})
	}
	keyDB := kdb.(models.KeyDatabase)
	c := cert.NewCert(
		cfg.NotaryService.CliHttpsEnable,
		cfg.NotaryService.RootCAUrl,
		cfg.NotaryService.CertUrl,
		cfg.NotaryService.CRLUrl,
		cfg.Matrix.ServerName,
		keyDB,
	)
	client.SetCerts(c.GetCerts())
	if err = c.Load(); err != nil {
		log.Panicw("failed to load cert", log.KeysAndValues{"error", err})
	}

	downloadStateRepo := repos.NewDownloadStateRepo()

	idg, _ := uid.NewIdGenerator(0, 0)
	rpcCli := common.NewRpcClient(cfg.Nats.Uri, idg)
	rpcCli.Start(true)

	fedClient, err := client.GetFedClient(cfg.Matrix.ServerName[0])
	if err != nil {
		log.Panicf(err.Error())
	}

	cache := &cache.RedisCache{}
	if err := cache.Prepare(cfg.Redis.Uris); err != nil {
		log.Panicf("failed to connect to redis cache err:%v", err)
	}

	settings := common.NewSettings(cache)

	settingConsumer := common.NewSettingConsumer(
		cfg.Kafka.Consumer.SettingUpdateContent.Underlying,
		cfg.Kafka.Consumer.SettingUpdateContent.Name,
		settings)
	if err := settingConsumer.Start(); err != nil {
		log.Panicf("failed to start settings consumer err:%v", err)
	}

	feddomains := common.NewFedDomains(settings)
	settings.RegisterFederationDomainsUpdateCallback(feddomains.OnFedDomainsUpdate)

	cdb, err := common.GetDBInstance("content", cfg)
	if err != nil {
		log.Panicw("failed to connect to content db", log.KeysAndValues{"error", err})
	}
	contentDB := cdb.(model.ContentDatabase)

	downloadConsumer := download.NewConsumer(cfg, feddomains, fedClient, contentDB, downloadStateRepo)
	if err := downloadConsumer.Start(); err != nil {
		log.Panicf("failed to start download consumer err: %v", err)
	}

	common.GetTransportMultiplexer().Start()

	routing.Setup(
		base.APIMux,
		base.Cfg,
		cache,
		feddomains,
		downloadStateRepo,
		rpcCli,
		downloadConsumer,
		idg,
	)
}
