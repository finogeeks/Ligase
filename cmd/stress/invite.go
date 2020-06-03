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
	"net/http"
	"strconv"
)

type inviteRequest struct {
	UserID   string `json:"user_id"`
	AutoJoin bool   `json:"auto_join"`
}

func buildInviteURL(roomID, token, host string, port int) string {
	return "http://" + host + ":" + strconv.Itoa(port) + "/_matrix/client/r0/rooms/" + roomID + "/invite?access_token=" + token
}

func doInvite(seed int, url string, autoJoin bool) (*http.Response, error) {
	var crReq inviteRequest
	crReq.UserID = "test" + strconv.Itoa(seed) + ":" + *domain
	crReq.AutoJoin = autoJoin

	content, _ := json.Marshal(crReq)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(content))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	return http.DefaultClient.Do(req)
}
