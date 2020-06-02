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

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync/atomic"
)

type passwordRequest struct {
	User               string  `json:"user"`
	Password           string  `json:"password"`
	DeviceID           string  `json:"device_id"`
	InitialDisplayName *string `json:"initial_device_display_name"`
	IsHuman            *bool   `json:"is_human"`
}

type loginResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	DeviceID    string `json:"device_id"`
}

func buildLoginURL(host string, port int) string {
	return "http://" + host + ":" + strconv.Itoa(port) + "/_matrix/client/r0/login"
}

func doLogin(seed int, url string, parseRes bool) string {
	defer wg.Done()
	var loginReq passwordRequest
	name := "test" + strconv.Itoa(seed)
	loginReq.User = "@" + name + ":dendrite"
	loginReq.Password = "1111"
	loginReq.DeviceID = name
	loginReq.InitialDisplayName = &name

	content, _ := json.Marshal(loginReq)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(content))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, _ := http.DefaultClient.Do(req)
	if res != nil && res.StatusCode == 200 {
		atomic.AddUint32(&sucess, 1)
	} else {
		atomic.AddUint32(&fail, 1)
	}
	defer res.Body.Close()

	if parseRes {
		var resp loginResponse
		body, _ := ioutil.ReadAll(res.Body)
		json.Unmarshal(body, &resp)
		return resp.AccessToken
	}

	return ""
}
