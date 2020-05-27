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

package domain

import (
	"context"
	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"sync"
)

var onceDomainMng sync.Once
var DomainMngInsance *DomainMng
var FirstDomain string

type DomainMng struct {
	cache        service.Cache
	servernameDB model.ConfigDatabase
	domains      []string
	fromDb       bool
	idg          *uid.UidGenerator
}

func GetDomainMngInstance(
	cache service.Cache,
	servernameDB model.ConfigDatabase,
	domains []string,
	fromDb bool,
	idg *uid.UidGenerator,
) *DomainMng {
	onceDomainMng.Do(func() {
		DomainMngInsance = &DomainMng{
			cache:        cache,
			servernameDB: servernameDB,
			domains:      domains,
			fromDb:       fromDb,
			idg:          idg,
		}
		adapter.SetDomainCfg(DomainMngInsance.domains)
	})
	return DomainMngInsance
}

func (dm *DomainMng) GetDomain() []string {
	if !dm.fromDb {
		return dm.domains
	}
	domains, err := dm.cache.GetDomains()
	if err != nil || len(domains) <= 0 {
		if err != nil {
			log.Warnf("get domains from cache err:%v", err)
		}
		domains, err = dm.servernameDB.SelectServerNames(context.TODO())
		if err != nil {
			log.Errorf("get domains from database err:%v", err)
			//use last domains
			return dm.domains
		} else {
			//db is empty too
			flag := false
			if len(domains) <= 0 {
				domains = dm.domains
				flag = true
			}
			log.Info("recover domains to cache and recover database if empty")
			for _, domain := range domains {
				dm.cache.AddDomain(domain)
				if flag {
					log.Warnf("domain in db is empty")
					nid, _ := dm.idg.Next()
					dm.servernameDB.UpsertServerName(context.TODO(), nid, domain)
				}
			}
		}
	}
	//update to lastest
	dm.domains = domains
	adapter.SetDomainCfg(domains)
	return domains
}

func CheckValidDomain(domain string, domains []string) bool {
	checkDomains := []string{}
	if DomainMngInsance == nil || !DomainMngInsance.fromDb {
		checkDomains = domains
	} else {
		checkDomains = DomainMngInsance.GetDomain()
	}
	for _, val := range checkDomains {
		if val == domain {
			return true
		}
	}
	return false
}
