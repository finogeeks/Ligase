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

package handler

import (
	"bytes"
	"context"
	"crypto/tls"
	encJson "encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/storage/model"
	"github.com/pkg/errors"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/noticetypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func NoticeHandler(ctx context.Context, action, targetDomain string, keyDB model.KeyDatabase) (err error) {
	var reqUrl string
	if action == "add" {
		reqUrl = fmt.Sprintf(config.GetConfig().NotaryService.CertUrl, config.GetConfig().Matrix.ServerName[0])
		// reqUrl := fmt.Sprintf(config.GetConfig().NotaryService.CertUrl, "dev.finogeeks.club")
		_, err = DownloadFromNotary(ctx, "cert", reqUrl, keyDB)
	} else if action == "update" {
		if common.CheckValidDomain(targetDomain, config.GetConfig().Matrix.ServerName) {
			reqUrl = fmt.Sprintf(config.GetConfig().NotaryService.CertUrl, config.GetConfig().Matrix.ServerName[0])
			// reqUrl = fmt.Sprintf(config.GetConfig().NotaryService.CertUrl, "dev.finogeeks.club")
			_, err = DownloadFromNotary(ctx, "cert", reqUrl, keyDB)

			reqUrl = fmt.Sprintf(config.GetConfig().NotaryService.CRLUrl, config.GetConfig().Matrix.ServerName[0])
			// reqUrl = fmt.Sprintf(config.GetConfig().NotaryService.CRLUrl, "dev.finogeeks.club")
			_, err = DownloadFromNotary(ctx, "crl", reqUrl, keyDB)
		}

		// cert revoke handler
	} else if action == "delete" {
		// cert revoke handler
	}

	return err
}

func DownloadFromNotary(ctx context.Context, typ string, reqUrl string,
	keyDB model.KeyDatabase) (resp noticetypes.GetCertsResponse, err error) {
	res, err := httpReq("GET", reqUrl, nil)
	if err != nil {
		log.Errorf("notary download, err: %v", err)
		return resp, err
	} else if res.StatusCode != http.StatusOK {
		log.Errorf("notary download, resp: %v", res)
		return resp, err
	}

	err = encJson.NewDecoder(res.Body).Decode(&resp)
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

func httpReq(method, reqUrl string, sendData []byte) (*http.Response, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Second * 15,
		}).DialContext,
		DisableKeepAlives: true, // fd will leak if set it false(default value)
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}

	var content io.Reader
	if sendData != nil {
		content = bytes.NewReader(sendData)
	}
	newReq, err := http.NewRequest(method, reqUrl, content)
	if err != nil {
		return nil, err
	}

	if sendData != nil {
		newReq.Header.Set("Content-Type", "application/json")
	}

	return client.Do(newReq)
}
