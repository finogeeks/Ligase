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

package rpc

import (
	// "github.com/finogeeks/ligase/skunkworks/log"
	"errors"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	jsoniter "github.com/json-iterator/go"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RpcResponse struct {
	Error   string
	Payload jsoniter.RawMessage
}

func RpcRequest(
	rpcClient *common.RpcClient,
	destination, topic string,
	request interface{},
) ([]byte, error) {
	content := roomserverapi.FederationEvent{
		Destination: string(destination),
	}

	reqContent, err := json.Marshal(&request)
	content.Extra = reqContent

	bytes, err := json.Marshal(content)
	// log.Infof("-------------------- fed rpc request, topic: %s, bytes: %s", topic, string(bytes))
	data, err := rpcClient.Request(topic, bytes, 30000)
	if err != nil {
		return nil, err
	}
	log.Infof("-------------------- fed rpc response, topic: %s, bytes: %s", topic, data)

	resp := RpcResponse{}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Errorf("-------------------- fed rpc response, topic: %s, err: %#v", topic, err)
		return nil, err
	}
	log.Infof("-------------------- fed rpc response, topic: %s, data: %s %s", topic, resp.Error, resp.Payload)
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return resp.Payload, err
}
