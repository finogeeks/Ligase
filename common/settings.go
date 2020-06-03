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
	"context"
	"strconv"
	"sync"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type FedDomainInfo struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

type SettingCache interface {
	GetSetting(settingKey string) (int64, error)
	GetSettingRaw(settingKey string) (string, error)
	SetSetting(settingKey string, val string) error
}

type Settings struct {
	cache    SettingCache
	settings sync.Map
	mutex    sync.Mutex

	fedDomainsUpdateCb []func([]FedDomainInfo)
}

func NewSettings(cache SettingCache) *Settings {
	return &Settings{cache: cache}
}

func (c *Settings) UpdateSetting(key, val string) {
	if key == "" {
		return
	}
	c.settings.Store(key, val)
	log.Infof("Settings %s = %s", key, val)
	if key == "im.federation.domains" {
		feddomains := c.GetFederationDomains()
		for _, cb := range c.fedDomainsUpdateCb {
			if cb != nil {
				cb(feddomains)
			}
		}
	}
}

func (c *Settings) getSetting(key string) (string, bool) {
	v, ok := c.settings.Load(key)
	if !ok {
		s, err := c.cache.GetSettingRaw(key)
		if err != nil {
			return "", false
		}
		v, _ = c.settings.LoadOrStore(key, s)
	}
	return v.(string), true
}

func (c *Settings) getSettingInt64(key string) (int64, bool) {
	s, ok := c.getSetting(key)
	if !ok {
		return 0, ok
	}
	i, _ := strconv.ParseInt(s, 0, 64)
	return i, ok
}

func (c *Settings) GetMessageVisilibityTime() int64 {
	v, _ := c.getSettingInt64("im.setting.messageVisibilityTime")
	return v
}

func (c *Settings) GetAutoLogoutTime() int64 {
	v, _ := c.getSettingInt64("im.setting.autoLogoutTime")
	return v
}

func (c *Settings) GetFederationDomains() (ret []FedDomainInfo) {
	s, ok := c.getSetting("im.federation.domains")
	if !ok {
		return nil
	}
	json.UnmarshalFromString(s, &ret)
	return
}

func (c *Settings) RegisterFederationDomainsUpdateCallback(cb func([]FedDomainInfo)) {
	c.mutex.Lock()
	c.mutex.Unlock()
	c.fedDomainsUpdateCb = append(c.fedDomainsUpdateCb, cb)
}

type SettingsConsumer struct {
	settings *Settings
}

func NewSettingConsumer(underlying, name string, settings *Settings) *SettingsConsumer {
	val, ok := GetTransportMultiplexer().GetChannel(underlying, name)
	if ok {
		channel := val.(core.IChannel)
		c := &SettingsConsumer{settings}
		channel.SetHandler(c)

		return c
	}

	return nil
}

func (c *SettingsConsumer) Start() error {
	return nil
}

func (c *SettingsConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var req external.ReqPutSettingRequest
	err := json.Unmarshal(data, &req)
	log.Infof("SettingsConsumer OnMessage topic: %s, partition: %d, data: %s", topic, partition, string(data))
	if err != nil {
		log.Errorf("SettingsConsumer unmarshal error %v", err)
		return
	}
	c.settings.UpdateSetting(req.SettingKey, req.Content)
}
