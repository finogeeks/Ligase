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

package common

import (
	"errors"
	"log"
	"sync"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/storage/model"
)

var (
	regMu      sync.RWMutex
	dbMap      sync.Map
	newHandler = make(map[string]func(string, string, string, string, string, bool) (interface{}, error))
	gauge      mon.LabeledGauge
	once       sync.Once
)

//can't use skunkworks log
func Register(name string, f func(string, string, string, string, string, bool) (interface{}, error)) {
	regMu.Lock()
	defer regMu.Unlock()

	log.Printf("DatabaseMng Register: %s\n", name)
	if f == nil {
		log.Panicf("DatabaseMng Register: %s func nil\n", name)
	}

	if _, ok := newHandler[name]; ok {
		log.Panicf("DatabaseMng Register: %s already registered\n", name)
	}

	newHandler[name] = f
}

func GetDBInstance(name string, cfg core.IConfig) (interface{}, error) {
	driver, createAddr, address, underlying, topic, useSync := cfg.GetDBConfig(name)
	f := newHandler[name]
	if f == nil {
		return nil, errors.New("unknown db " + name)
	}

	val, ok := dbMap.Load(name)
	if ok {
		return val, nil
	}

	val, err := f(driver, createAddr, address, underlying, topic, useSync)
	if err != nil {
		return nil, err
	}
	dbMap.Store(name, val)

	dbMon := val.(model.DBMonitor)
	dbMon.SetGauge(GetGaugeInstance())

	return val, err
}

func GetGaugeInstance() mon.LabeledGauge {
	monitor := mon.GetInstance()

	once.Do(func() {
		gauge = monitor.NewLabeledGauge("storage_query_duration_millisecond", []string{"process"})
	})
	return gauge
}
