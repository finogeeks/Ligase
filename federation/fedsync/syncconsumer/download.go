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

	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func Download(
	fedClient *client.FedClientWrap,
	req *external.GetFedDownloadRequest,
	destination, domain string,
	onData func(string, []byte) error,
	onFinished func(string),
) external.GetFedDownloadResponse {
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
							err = onData(req.ID, data[:n])
							if err != nil {
								return err
							}
						}
						onFinished(req.ID)
					} else {
						log.Errorf("federation download read body error %#v", err)
					}
					break
				}
				err = onData(req.ID, data[:n])
				if err != nil {
					return err
				}
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
