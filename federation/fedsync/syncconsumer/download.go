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

package syncconsumer

import (
	"context"
	"io"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/federation/client"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func Download(
	fedClient *client.FedClientWrap,
	request *roomserverapi.FederationEvent,
	destination, domain string,
	rpcCli *common.RpcClient,
) external.GetFedDownloadResponse {
	var req external.GetFedDownloadRequest
	if err := json.Unmarshal(request.Extra, &req); err != nil {
		log.Errorf("federation Download unmarshal error: %v", err)
		return external.GetFedDownloadResponse{
			StatusCode: http.StatusInternalServerError,
		}
	}

	ch := make(chan struct{}, 1)
	var resp *http.Response
	cb := func(response *http.Response) error {
		resp = response
		ch <- struct{}{}
		var data [4096 * 16]byte
		if response != nil && response.Body != nil {
			defer response.Body.Close()
			for {
				n, err := response.Body.Read(data[:])
				if err != nil {
					if err == io.EOF {
						if n > 0 {
							rpcCli.Pub(req.ID, data[:n])
						}
						rpcCli.Pub(req.ID, []byte{})
					} else {
						log.Errorf("federation download read body error %#v", err)
					}
					break
				}
				rpcCli.Pub(req.ID, data[:n])
			}
		}
		return nil
	}
	go func() {
		err := fedClient.Download(context.TODO(),
			destination, domain, req.MediaID, req.Width, req.Method, req.FileType, cb,
		)
		if err != nil {
			log.Errorf("federation Download [%s] error: %v", req.MediaID, err)
		}
	}()
	<-ch

	if resp != nil {
		log.Infof("federation Download [%s] statusCode: %d", req.MediaID, resp.StatusCode)
		return external.GetFedDownloadResponse{
			ID:         req.ID,
			StatusCode: resp.StatusCode,
			Header:     (map[string][]string)(resp.Header),
		}
	} else {
		log.Errorf("federation Download [%s] resp is nil", req.MediaID)
		return external.GetFedDownloadResponse{
			ID:         req.ID,
			StatusCode: 500,
			Header:     nil,
		}
	}
}
