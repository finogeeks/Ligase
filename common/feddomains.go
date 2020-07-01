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
	"strconv"
	"sync"
)

type FedDomains struct {
	domains  *sync.Map
	settings *Settings
	loaded   bool
}

func NewFedDomains(settings *Settings) *FedDomains {
	return &FedDomains{domains: new(sync.Map), settings: settings}
}

func (f *FedDomains) LoadCache() {
	infos := f.settings.GetFederationDomains()
	domains := new(sync.Map)
	for _, info := range infos {
		domains.LoadOrStore(info.Domain, info)
		f.loaded = true
	}
	f.domains = domains
}

func (f *FedDomains) OnFedDomainsUpdate(domains []FedDomainInfo) {
	domainMap := new(sync.Map)
	for _, info := range domains {
		domainMap.Store(info.Domain, info)
	}
	f.domains = domainMap
}

func (f *FedDomains) GetDomainInfo(domain string) (FedDomainInfo, bool) {
	if !f.loaded {
		f.LoadCache()
	}
	v, ok := f.domains.Load(domain)
	if !ok {
		return FedDomainInfo{}, false
	}
	return v.(FedDomainInfo), true
}

func (f *FedDomains) GetAllDomainInfos() []FedDomainInfo {
	if !f.loaded {
		f.LoadCache()
	}
	ret := []FedDomainInfo{}
	f.domains.Range(func(k, v interface{}) bool {
		ret = append(ret, v.(FedDomainInfo))
		return true
	})
	return ret
}

func (f *FedDomains) GetDomainHost(domain string) (string, bool) {
	if !f.loaded {
		f.LoadCache()
	}
	info, ok := f.GetDomainInfo(domain)
	if !ok {
		return "", false
	}
	return info.Host + ":" + strconv.Itoa(info.Port), true
}

func (f *FedDomains) GetAllFedDomains() []string {
	if !f.loaded {
		f.LoadCache()
	}
	ret := []string{}
	f.domains.Range(func(k, v interface{}) bool {
		d := v.(FedDomainInfo).Domain
		ret = append(ret, d)
		return true
	})
	return ret
}
