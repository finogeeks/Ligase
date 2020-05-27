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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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

	return data, err
}
