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

package filter

import (
	"encoding/json"
	"fmt"
	"github.com/finogeeks/ligase/skunkworks/log"
	"strings"
	"sync"
)

type SimpleFilter struct {
	loader         SimpleFilterLoader
	repo           sync.Map //key: userId:mac, val: device
	userRepo       sync.Map //key: userId:client_type, val:true
	delRepo        sync.Map //key: userId:mac val:string
	ready          bool
	kickClientType bool
}

type Device struct {
	devMap sync.Map
	//key: userId:mac , val: true
}

func NewSimpleFilter(
	loader SimpleFilterLoader,
	kickClientType bool,
) *SimpleFilter {
	sf := &SimpleFilter{
		loader:         loader,
		ready:          false,
		kickClientType: kickClientType,
	}
	return sf
}

func (sf *SimpleFilter) Load() bool {
	if sf.ready == false {
		if sf.ready == true {
			return true
		}
		if sf.loader.LoadSimpleFilterData(sf) {
			sf.ready = true
			log.Infof("load data from db ok")
		} else {
			log.Errorf("load data from db err")
			return false
		}
	}
	return true
}

func (sf *SimpleFilter) getMac(deviceID string) string {
	index := strings.LastIndex(deviceID, ":")
	if index == -1 {
		return deviceID
	}
	return deviceID[0:index]
}

func (sf *SimpleFilter) isVirtual(mac string) bool {
	return mac == "virtual"
}

func (sf *SimpleFilter) Insert(userId, deviceId, clientType, deviceName string, createTs int64) {
	mac := fmt.Sprintf("%s:%s", userId, sf.getMac(deviceId))
	key := fmt.Sprintf("%s:%s", userId, deviceId)
	var dev *Device
	if val, ok := sf.repo.Load(mac); ok {
		dev = val.(*Device)
		dev.devMap.Store(key, true)
		if !sf.isVirtual(sf.getMac(deviceId)) {
			dev.devMap.Range(func(k interface{}, val interface{}) bool {
				if k != key {
					dev.devMap.Delete(k)
				}
				return true
			})
		}
	} else {
		dev := new(Device)
		dev.devMap.Store(key, true)
		sf.repo.Store(mac, dev)
	}
	log.Infof("insert token: %s", key)
	if clientType == "" || !sf.kickClientType {
		return
	}
	userClientKey := fmt.Sprintf("%s:%s", userId, clientType)
	if val, ok := sf.userRepo.Load(userClientKey); ok {
		kickDeviceId := val.(string)
		sf.Delete(userId, kickDeviceId)
		delInfo := make(map[string]interface{})
		delInfo["deviceName"] = deviceName
		delInfo["loginTime"] = createTs
		strDelInfo, _ := json.Marshal(delInfo)
		sf.delRepo.Store(fmt.Sprintf("%s:%s", userId, kickDeviceId), string(strDelInfo))
	}
	sf.userRepo.Store(userClientKey, deviceId)
}

func (sf *SimpleFilter) Delete(userId, deviceId string) {
	mac := fmt.Sprintf("%s:%s", userId, sf.getMac(deviceId))
	key := fmt.Sprintf("%s:%s", userId, deviceId)
	if val, ok := sf.repo.Load(mac); ok {
		dev := val.(*Device)
		if dev == nil {
			return
		}
		if _, ok := dev.devMap.Load(key); ok {
			dev.devMap.Delete(key)
		}
	}
	log.Infof("delete token: %s", key)
}

func (sf *SimpleFilter) Lookup(userId, deviceId string) bool {
	if !sf.ready {
		return true
	}
	mac := fmt.Sprintf("%s:%s", userId, sf.getMac(deviceId))
	key := fmt.Sprintf("%s:%s", userId, deviceId)
	if val, ok := sf.repo.Load(mac); ok {
		dev := val.(*Device)
		if dev == nil {
			return false
		}
		if _, ok := dev.devMap.Load(key); ok {
			return true
		} else {
			return false
		}
	} else {
		log.Infof("cannot found token: %s", key)
		return false
	}
}

func (sf *SimpleFilter) GetClientTypeKick(userId, deviceId string) string {
	key := fmt.Sprintf("%s:%s", userId, deviceId)
	if v, ok := sf.delRepo.Load(key); ok {
		return v.(string)
	} else {
		return ""
	}
}

func (sf *SimpleFilter) DelClientTypeKick(userId, deviceId string) {
	key := fmt.Sprintf("%s:%s", userId, deviceId)
	sf.delRepo.Delete(key)
}
