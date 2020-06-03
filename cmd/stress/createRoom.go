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

type createRoomRequest struct {
	Invite          []string               `json:"invite"`
	Name            string                 `json:"name"`
	Visibility      string                 `json:"visibility"`
	Topic           string                 `json:"topic"`
	Preset          string                 `json:"preset"`
	CreationContent map[string]interface{} `json:"creation_content"`
	RoomAliasName   string                 `json:"room_alias_name"`
	GuestCanJoin    bool                   `json:"guest_can_join"`
	IsDrirect       bool                   `json:"is_direct"`
}

func buildCreateRoomURL(host, token string, port int) string {
	return "http://" + host + ":" + strconv.Itoa(port) + "/_matrix/client/r0/createRoom?access_token=" + token
}

func doCreateRoom(seed int, url string) (*http.Response, error) {
	defer wg.Done()
	var crReq createRoomRequest
	name := "test" + strconv.Itoa(seed)
	crReq.Preset = "public_chat"
	crReq.Name = name
	crReq.Topic = name
	crReq.CreationContent = make(map[string]interface{})
	crReq.CreationContent["m.federate"] = false

	content, _ := json.Marshal(crReq)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(content))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	return http.DefaultClient.Do(req)
}
