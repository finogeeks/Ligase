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

package routing

import (
	"context"
	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"net/http"
)

func OnUpdateServerNameCfg(
	ctx context.Context,
	req *external.ServerNameCfgRequest,
	serverConfDB model.ConfigDatabase,
	cache service.Cache,
	idg *uid.UidGenerator,
	cfg config.Dendrite,
) (int, core.Coder) {
	if !cfg.Matrix.ServerFromDB {
		return http.StatusInternalServerError, jsonerror.BadJSON("servername can only change when cfg set domain load from db")
	}
	if req.Type == "add" {
		OnAddServerNames(ctx, cache, serverConfDB, req.ServerNames, idg)
	} else if req.Type == "del" {
		OnDelServerNames(ctx, cache, serverConfDB, req.ServerNames)
	} else {
		return http.StatusBadRequest, jsonerror.BadJSON("servername err req type")
	}
	return http.StatusOK, nil
}

func OnAddServerNames(ctx context.Context, cache service.Cache, serverConfDB model.ConfigDatabase, serverNames []string, idg *uid.UidGenerator) {
	for _, serverName := range serverNames {
		nid, _ := idg.Next()
		err := cache.AddDomain(serverName)
		if err != nil {
			log.Errorf("add domain:%s to cache err:%v", serverName, err)
		}
		err = serverConfDB.UpsertServerName(ctx, nid, serverName)
		if err != nil {
			log.Errorf("add domain:%s to database err:%v", serverName, err)
		}
	}
	domains, err := getServerNameCfg(serverConfDB, cache)
	if err == nil && len(domains) > 0 {
		adapter.SetDomainCfg(domains)
	}
}

//current has not del
func OnDelServerNames(ctx context.Context, cache service.Cache, serverConfDB model.ConfigDatabase, serverNames []string) {

}

func getServerNameCfg(
	serverConfDB model.ConfigDatabase,
	cache service.Cache,
) ([]string, error) {
	domains, err := cache.GetDomains()
	if err != nil || len(domains) <= 0 {
		if err != nil {
			log.Warnf("get domains from cache err:%v", err)
		}
		domains, err = serverConfDB.SelectServerNames(context.TODO())
		if err != nil {
			log.Errorf("get domains from database err:%v", err)
			return nil, err
		} else {
			log.Info("recover domains from database to cache")
			for _, domain := range domains {
				cache.AddDomain(domain)
			}
		}
	}
	return domains, nil
}

func OnGetServerNameCfg(
	serverConfDB model.ConfigDatabase,
	cache service.Cache,
	cfg config.Dendrite,
) (int, core.Coder) {
	if !cfg.Matrix.ServerFromDB {
		return http.StatusOK, &external.GetServerNamesResponse{
			ServerNames: cfg.Matrix.ServerName,
		}
	}
	domains, err := getServerNameCfg(serverConfDB, cache)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.BadJSON(err.Error())
	}
	return http.StatusOK, &external.GetServerNamesResponse{
		ServerNames: domains,
	}
}
