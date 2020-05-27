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

package fetch

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type Resp struct {
	Body       []byte
	StatusCode int
}

func HttpsReqUnsafe(method, reqUrl string, sendData []byte, header map[string]string) (Resp, error) {
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
		return Resp{}, err
	}

	if sendData != nil {
		newReq.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		newReq.Header.Set(k, v)
	}

	resp, err := client.Do(newReq)
	if err != nil {
		log.Errorf("HttpsReqUnsafe error %v", err)
		return Resp{}, err
	}
	if resp != nil && resp.Body != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Errorf("resp body close err: %v", err)
			}
		}()
	}

	var body []byte
	if resp != nil && resp.Body != nil {
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return Resp{StatusCode: resp.StatusCode}, err
		}
	}
	return Resp{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}
