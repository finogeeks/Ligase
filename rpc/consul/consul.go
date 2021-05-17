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

package consul

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	uuid "github.com/satori/go.uuid"
)

const (
	checkPort = "9091"
)

type Consul struct {
	CheckPort    string
	NodeId       string
	ConsulClient *api.Client
	IsRegSucc    bool
	IsStop       bool

	consulAddr string
	conulTag   string
	serverName string
	grpcPort   int
}

func NewConsul(consulAddr, conulTag, serverName string, grpcPort int) *Consul {
	var m Consul

	m.CheckPort = checkPort
	m.NodeId = uuid.NewV4().String()
	m.IsRegSucc = false
	m.IsStop = false

	m.consulAddr = consulAddr
	m.conulTag = conulTag
	m.serverName = serverName
	m.grpcPort = grpcPort

	var err error
	cfg := api.DefaultConfig()
	cfg.Address = consulAddr
	m.ConsulClient, err = api.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	return &m
}

func (m *Consul) Init() {
	m.InitCheck() //初始化check接口

	time.Sleep(1 * time.Second)
	m.RegisterServer()         //注册服务
	go m.WatchService()        //服务是否发生change
	go m.SyncServiceHealth(60) //主动拉取健康状态
}

func (m *Consul) InitCheck() {
	http.HandleFunc("/"+m.NodeId+"/check", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("success"))
	})

	go http.ListenAndServe(":"+m.CheckPort, nil)
}

func (m *Consul) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func (m *Consul) DisRegisterServer() {
	m.IsStop = true
	err := m.ConsulClient.Agent().ServiceDeregister(m.NodeId)
	if err != nil {
		log.Errorf("ServiceDeregister service:%s err:%s", m.NodeId, err.Error())
	} else {
		log.Debugln("ServiceDeregister service:%s succ", m.NodeId)
	}
}

func (m *Consul) RegisterServer() {
	if m.IsStop {
		return
	}
	registration := new(api.AgentServiceRegistration)
	registration.ID = m.NodeId
	registration.Name = m.serverName
	registration.Port = m.grpcPort
	registration.Tags = []string{m.conulTag}
	registration.Address = m.GetLocalIP()
	log.Debugln("localIP=" + registration.Address)
	registration.Check = &api.AgentServiceCheck{
		CheckID:                        m.NodeId,
		HTTP:                           "http://" + registration.Address + ":" + m.CheckPort + "/" + m.NodeId + "/check",
		Timeout:                        "5s",
		Interval:                       "30s",
		DeregisterCriticalServiceAfter: "35s", //check失败后5秒删除本服务
	}

	err := m.ConsulClient.Agent().ServiceRegister(registration)
	if err != nil {
		log.Errorf("ServiceRegister err:" + err.Error())
		if m.IsRegSucc == false {
			panic(err)
		}
	}

	m.IsRegSucc = true
	log.Debugln("Consul RegisterServer success")
}

func (m *Consul) WatchService() {
	var notifyCh = make(chan struct{})

	watchContent := `{"type":"service", "service":"` + m.serverName + `", "tag":"` + m.conulTag + `"}`
	plan := m.WatchParse(watchContent)
	plan.Handler = func(idx uint64, raw interface{}) {
		log.Debugln("service change...")
		if raw == nil {
			return // ignore
		}
		log.Debugln("do something...")
		notifyCh <- struct{}{}
	}

	go func() {
		if err := plan.Run(m.consulAddr); err != nil {
			panic(err)
		}
	}()
	defer plan.Stop()

	for {
		select {
		case <-notifyCh:
			m.RegisterAgain()
		}
	}
}

func (m *Consul) SyncServiceHealth(gap int) {
	for {
		if m.IsStop {
			break
		}
		time.Sleep(time.Duration(gap) * time.Second)
		isHealth := false
		services, _, err := m.ConsulClient.Health().Service(m.serverName, m.conulTag, true, &api.QueryOptions{WaitIndex: 0})
		if err != nil {
			log.Errorf("error retrieving instances from Consul: %s", err.Error())
		}

		for _, s := range services {
			if s.Service.ID == m.NodeId {
				isHealth = true
				break
			}
		}

		if !isHealth {
			m.RegisterServer()
		}
	}
}

func (m *Consul) WatchParse(q string) *watch.Plan {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(q), &params); err != nil {
		log.Errorln("Unmarshal err:" + err.Error())
		return nil
	}

	plan, err := watch.Parse(params)
	if err != nil {
		panic(err)
	}

	return plan
}

func (m *Consul) RegisterAgain() {
	if !m.CheckServiceIsOK() {
		m.RegisterServer()
	}
}

func (m *Consul) CheckServiceIsOK() bool {
	services, _ := m.ConsulClient.Agent().Services()

	isOK := false
	if len(services) > 0 {
		if value, ok := services[m.NodeId]; ok {
			log.Debugln("service=" + interfaceToJsonString(value))
			if value.Service == m.serverName {
				if value.Weights.Passing == 1 {
					isOK = true
				}
			}
		}
	}

	return isOK
}

func interfaceToJsonString(obj interface{}) string {
	jsonBytes, err := json.Marshal(obj)
	if err == nil {
		return string(jsonBytes)
	}

	return ""
}
