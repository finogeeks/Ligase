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

package entry

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/finogeeks/ligase/common/localExporter"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/encryption"
	"github.com/finogeeks/ligase/common/lifecycle"
	"github.com/finogeeks/ligase/core"
	_ "github.com/finogeeks/ligase/plugins"
	"github.com/finogeeks/ligase/proxy"
	_ "github.com/finogeeks/ligase/rpc/grpc"
	"github.com/finogeeks/ligase/skunkworks/log"
	hm "github.com/finogeeks/ligase/skunkworks/monitor/go-client/httpmonitor"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/gchaincl/sqlhooks"
	"github.com/lib/pq"
)

type serverCmdPar struct {
	maxCore        *bool
	useSQLHook     *bool
	logPorf        *bool
	procName       *string
	httpBindAddr   *string
	httpsBindAddr  *string
	certFile       *string
	keyFile        *string
	fixType        *string
	fixRoom        *string
	evRecoverStart *int64
	evRecoverEnd   *int64
}

var cmdLine = &serverCmdPar{
	procName: flag.String("name", "cache-loader-server", "Name for the server"),
	maxCore:  flag.Bool("max-core", true, "use max core"),

	//monolith-server/sync-server/front-server/migration-server/push-api-server
	httpBindAddr:   flag.String("http-address", "", "The HTTP listening port for the server"),
	httpsBindAddr:  flag.String("https-address", "", "The HTTPS listening port for the server"),
	useSQLHook:     flag.Bool("use-sql-hook", false, "use sql porfmance hook"),
	certFile:       flag.String("tls-cert", "", "The PEM formatted X509 certificate to use for TLS"),
	keyFile:        flag.String("tls-key", "", "The PEM private key to use for TLS"),
	logPorf:        flag.Bool("log-porf", true, "log server porfmance"),
	fixType:        flag.String("fix-type", "", "fix event type"),
	fixRoom:        flag.String("fix-room", "*", "fix room name"),
	evRecoverStart: flag.Int64("ev-recover-start", -1, "events recover start time stamp"),
	evRecoverEnd:   flag.Int64("ev-recover-end", -1, "events recover end time stamp"),
}

const usageDoc = `Entry point for finogeeks homeserver.
usage:
        engine-server [<flag> ...] <Go file or directory> ...
Flags
        --name     				proc name for the engine to start, shoud be one of:
								cache-loader/front-server/persist-server/sync-server/
								monolith-server/push-sender/migration-server
		--max-core  			check use the max cpu core, default true
		--http-address			http listening port, default 8008
		--use-sql-hook   		check use the sql hook, default true
		--tls-cert   			pem file for ssh
		--tls-key   			key file for ssl
		--https-address			https listening port, default 8448
		--log-porf				check log server porformance, default true

		--ev-recover-start		for events-recover service, events recover start time stamp
		--ev-recover-end		for events-recover service, events recover end time stamp
`

var defaultListenAddr = ":8008"

func usage() {
	log.Println(usageDoc)
	os.Exit(2)
}

func setUseMaxCore() {
	ncpu := runtime.NumCPU()
	runtime.GOMAXPROCS(ncpu)
}

func useSQLHook() {
	sql.Register("postgres_hook", sqlhooks.Wrap(&pq.Driver{}, &common.Hooks{}))
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

func loadDefault(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	if cmd.procName != nil {
		if cmd.httpBindAddr == nil || cmd.httpBindAddr != nil && *cmd.httpBindAddr == "" && (*cmd.procName == "monolith-server" || *cmd.procName == "push-api-server" ||
			*cmd.procName == "sync-server" || *cmd.procName == "front-server" || *cmd.procName == "migration-server") {
			cmd.httpBindAddr = &defaultListenAddr
		}
	}
}

//todo use etcd or else
func getInstanceId() int {
	hostName := os.Getenv("HOSTNAME")
	log.Infof("start server hostname:%s", hostName)
	if hostName != "" {
		res := strings.Split(hostName, "-") //rancher
		if len(res) == 3 {
			instance, err := strconv.Atoi(res[2])
			if err == nil {
				log.Infof("getInstanceId hostName: %s, instance:%d", hostName, instance)
				return instance
			}
		}
	}
	log.Infof("instance set to default:%d", -1)
	return -1
}

func checkProcName(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	if cmd.procName == nil {
		name := os.Getenv("SERVER_NAME")
		if name != "" {
			cmd.procName = &name
		}
	}

	//base.Cfg.Matrix.InstanceId = getInstanceId()
	getInstanceId()
	if cmd.procName == nil {
		usage()
	}

	switch *cmd.procName {
	case "cache-loader":
		StartCacheLoader(base, cmd)
	case "front-server":
		StartFrontServer(base, cmd)
	case "persist-server":
		StartPersistServer(base, cmd)
	case "sync-server":
		StartSyncServer(base, cmd)
	case "sync-monolith":
		StartSyncMonolith(base, cmd)
	case "monolith-server":
		StartMonolithServer(base, cmd)
	case "push-sender":
		StartPushSender(base, cmd)
	case "app-service":
		StartAppService(base, cmd)
	//for internal
	case "client-api-server":
		StartClientAPIServer(base, cmd)
	case "federation-api-server":
		StartFederationAPIServer(base, cmd)
	case "public-room-api-server":
		StartPublicRoomAPIServer(base, cmd)
	case "push-api-server":
		StartPushAPIServer(base, cmd)
	case "room-server":
		StartRoomServer(base, cmd)
	case "federation-sender":
		StartFederationSender(base, cmd)
	case "fix-db":
		StartFixDBServer(base, cmd)
	case "fix-sync-db":
		StartFixSyncDBServer(base, cmd)
	case "api-gw":
		StartApiGateWay(base, cmd)
	case "token-rewrite":
		StartTokenRewrite(base, cmd)
	case "sync-writer":
		StartSyncWriter(base, cmd)
	case "sync-aggregate":
		StartSyncAggregate(base, cmd)
	case "db-update-for-fed":
		StartUpdateDBForFed(base)
	case "bgmng-server":
		StartBgMng(base, cmd)
	case "profile-recover":
		StartProfileRecover(base, cmd)
	default:
		usage()
	}
}

func listenHTTP(cmd *serverCmdPar) {
	go func() {
		log.Info("Start http listening on ", *cmd.httpBindAddr)
		log.Fatal(http.ListenAndServe(*cmd.httpBindAddr, nil))
	}()
}

func listenHTTPS(cmd *serverCmdPar) {
	go func() {
		log.Info("Start https listening on ", *cmd.httpsBindAddr)

		rootCA, ok := proxy.Certs.Load("rootCA")
		if !ok {
			log.Panicf("cannot find rootCA")
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(rootCA.(string)))

		s := &http.Server{
			Addr:    *cmd.httpsBindAddr,
			Handler: nil,
			TLSConfig: &tls.Config{
				ClientCAs:  pool,
				ClientAuth: tls.RequireAndVerifyClientCert,
			},
		}

		serverCert, ok := proxy.Certs.Load("serverCert")
		if !ok {
			log.Panicf("cannot find server cert")
		}
		serverKey, ok := proxy.Certs.Load("serverKey")
		if !ok {
			log.Panicf("cannot find server key")
		}
		ioutil.WriteFile("server.crt", []byte(serverCert.(string)), 0644)
		ioutil.WriteFile("server.key", []byte(serverKey.(string)), 0644)

		log.Fatal(s.ListenAndServeTLS("server.crt", "server.key"))
	}()
}

func logSysPorformance(cmd *serverCmdPar) {
	log.Info("sys porformance enabled")
	go func() {
		name := *cmd.procName
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

func handleSignalUSR2() {
	cfg := config.GetConfig()
	logCfg := new(log.LogConfig)
	if cfg.Log.Signaled {
		cfg.Log.Signaled = false
		logCfg.Level = cfg.Log.Level
	} else {
		cfg.Log.Signaled = true
		logCfg.Level = "debug"
	}
	logCfg.Files = cfg.Log.Files
	logCfg.WriteToStdout = cfg.Log.WriteToStdout
	logCfg.ZapConfig.MaxSize = cfg.Log.ZapConfig.MaxSize
	logCfg.ZapConfig.MaxBackups = cfg.Log.ZapConfig.MaxBackups
	logCfg.ZapConfig.MaxAge = cfg.Log.ZapConfig.MaxAge
	logCfg.ZapConfig.LocalTime = cfg.Log.ZapConfig.LocalTime
	logCfg.ZapConfig.Compress = cfg.Log.ZapConfig.Compress
	logCfg.ZapConfig.JsonFormat = cfg.Log.ZapConfig.JsonFormat
	logCfg.ZapConfig.BtEnabled = cfg.Log.ZapConfig.BtEnabled
	logCfg.ZapConfig.BtLevel = cfg.Log.ZapConfig.BtLevel
	log.Setup(logCfg)
}

func handleSignal() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range sig {
			switch s {
			case syscall.SIGINT:
				log.Println("退出", s)
				os.Exit(0)
			case syscall.SIGUSR2:
				handleSignalUSR2()
			default:
				log.Println("other", s)
			}
		}
	}()
}

func decodeLicense(cfg *config.Dendrite) {
	license := encryption.DecryptLicense(cfg.License)
	err := json.Unmarshal([]byte(license), &cfg.LicenseItem)
	if err != nil {
		log.Panic("decode license err:", err)
	}
}

func Entry() {
	if err := lifecycle.RunBeforeStartup(); err != nil {
		panic(err)
	}
	flag.Usage = usage

	if *cmdLine.maxCore == true {
		setUseMaxCore()
	}

	if *cmdLine.useSQLHook == true {
		useSQLHook()
	}

	basecomponent.ParseMonolithFlags()
	cfg := config.GetConfig()
	base := basecomponent.NewBaseDendrite(cfg, *cmdLine.procName)
	base.APIMux.NotFoundHandler = http.HandlerFunc(myNotFound)
	defer base.Close() // nolint: errcheck
	localExporter.InitMon()
	if len(cfg.Log.Files) == 0 {
		log.Warn("Lack of log files")
	} else if cfg.Log.RedirectStderr {
		initPanicFile(cfg.Log.Files[0])
	}

	startGCDebugger(cfg.GcTimer)

	handleSignal()
	loadDefault(base, cmdLine)
	decodeLicense(base.Cfg)
	encryption.Init(base.Cfg.LicenseItem.Encryption, base.Cfg.LicenseItem.Secret, base.Cfg.Encryption.Mirror)
	setUpTransport(base, cmdLine)
	checkProcName(base, cmdLine)

	transportMultiplexer := common.GetTransportMultiplexer()
	transportMultiplexer.Start()

	//StartUpdateDBForFed(base)

	httpHandler := common.WrapHandlerInCORS(base.APIMux)
	http.Handle("/", hm.Wrap2(httpHandler))

	if cmdLine.httpBindAddr != nil && *cmdLine.httpBindAddr != "" {
		listenHTTP(cmdLine)
	}

	if cfg.NotaryService.SrvHttpsEnable {
		listenHTTPS(cmdLine)
	}

	if *cmdLine.logPorf == true {
		logSysPorformance(cmdLine)
	}

	if err := lifecycle.RunAfterStartup(); err != nil {
		panic(err)
	}
	select {}
}

func startGCDebugger(t int64) {
	if t == 0 {
		return
	}
	go func() {
		log.Infof("start gc timer: %d", t)
		for true {
			time.Sleep(time.Duration(t) * time.Second)
			log.Infof("call debug.FreeOSMemory()")
			debug.FreeOSMemory()
		}
	}()
}
