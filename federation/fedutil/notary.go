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

package fedutil

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/finogeeks/ligase/storage/model"
	"github.com/pkg/errors"

	"github.com/finogeeks/ligase/common/fetch"
	"github.com/finogeeks/ligase/model/noticetypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func DownloadFromNotary(ctx context.Context, typ string, reqUrl string, keyDB model.KeyDatabase) (resp noticetypes.GetCertsResponse, err error) {
	res, err := fetch.HttpsReqUnsafe("GET", reqUrl, nil, nil)
	if err != nil {
		log.Errorf("notary download, err: %v", err)
		return resp, err
	} else if res.StatusCode != http.StatusOK {
		log.Errorf("notary download, resp: %v", res)
		return resp, err
	}

	err = json.Unmarshal(res.Body, &resp)
	if err != nil {
		log.Errorf("notary download, decode failed error:%v", err)
		return resp, err
	}
	if typ == "rootCA" {
		log.Infof("--------------notary download, get rootCA: \n%s", resp.RootCA)

		keyDB.InsertRootCA(ctx, resp.RootCA)
		return resp, nil
	} else if typ == "cert" {
		log.Infof("--------------notary download, get server_cert: \n%s", resp.ServerCert)
		log.Infof("--------------notary download, get server_key: \n%s", resp.ServerKey)

		// upsert cert
		keyDB.UpsertCert(ctx, resp.ServerCert, resp.ServerKey)
		return resp, nil
	} else if typ == "crl" {
		log.Infof("--------------notary download, get CRL: \n%s", resp.CRL)

		keyDB.UpsertCRL(ctx, resp.CRL)
		return resp, nil
	}

	return resp, errors.New("notary download, no handler")
}
