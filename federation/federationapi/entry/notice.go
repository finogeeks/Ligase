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

package entry

import (
	"context"
	"fmt"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/fedutil"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/noticetypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	dbmodel "github.com/finogeeks/ligase/storage/model"
	"github.com/pkg/errors"
)

func init() {
	Register(model.CMD_FED_NOTARY_NOTICE, NotaryNotice)
}

func NotaryNotice(msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	retMsg := &model.GobMessage{
		Body: []byte{},
	}
	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}

	req := &external.ReqPostNotaryNoticeRequest{}
	req.Decode(msg.Body)

	err := noticeHandler(req.Action, req.TargetDomain, keyDB)
	log.Infof("NotaryNotice, err: %v", err)

	return retMsg, nil
}

func noticeHandler(action, targetDomain string, keyDB dbmodel.KeyDatabase) (err error) {
	log.Infof("noticeHandler, action: %s, targetDomain: %s, servername: %#v", action, targetDomain, cfg.Matrix.ServerName)

	var reqUrl string
	if action == "add" {
		reqUrl = fmt.Sprintf(cfg.NotaryService.CertUrl, cfg.Matrix.ServerName[0])
		_, err = fedutil.DownloadFromNotary("cert", reqUrl, keyDB)
	} else if action == "update" || action == "delete" {
		// update crl
		reqUrl = cfg.NotaryService.CRLUrl
		respCRL, err := fedutil.DownloadFromNotary("crl", reqUrl, keyDB)
		if err == nil {
			certInfo.GetCerts().Store("crl", respCRL.CRL)
		}

		// update/delete if needed
		if common.CheckValidDomain(targetDomain, cfg.Matrix.ServerName) {
			var resp noticetypes.GetCertsResponse
			if action == "update" {
				reqUrl = fmt.Sprintf(cfg.NotaryService.CertUrl, cfg.Matrix.ServerName[0])
				resp, err = fedutil.DownloadFromNotary("cert", reqUrl, keyDB)
				if err == nil {
					certInfo.GetCerts().Store("serverCert", resp.ServerCert)
					certInfo.GetCerts().Store("serverKey", resp.ServerKey)
				}
			} else {
				keyDB.UpsertCert(context.TODO(), "", "")
				certInfo.GetCerts().Store("serverCert", "")
				certInfo.GetCerts().Store("serverKey", "")
			}

			// check cert
			if ok, err := certInfo.CheckCert(respCRL.CRL, resp.ServerCert); !ok {
				// FIXME: should panic here ?
				log.Warnf(err.Error())
			}
			// renew fed client if cert updated
			client.ReNewFedClient(cfg.Matrix.ServerName[0])
		}
	} else {
		return errors.New("cann't find notary service handler")
	}

	return nil
}
