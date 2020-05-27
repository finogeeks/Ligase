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

package fedutil

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

func getMsgType(content map[string]interface{}) (string, bool) {
	value, exists := content["msgtype"]
	if !exists {
		return "", false
	}
	msgtype, ok := value.(string)
	if !ok {
		return "", false
	}
	return msgtype, true
}

func IsMediaEv(content map[string]interface{}) bool {
	msgtype, ok := getMsgType(content)
	if !ok {
		return false
	}
	return msgtype == "m.image" || msgtype == "m.audio" || msgtype == "m.video" || msgtype == "m.file"
}

func DownloadFromNetdisk(domain, destination string, ev *gomatrixserverlib.Event, content map[string]interface{}, fedClient *client.FedClientWrap) {
	cfg := config.GetFedConfig().Kafka.Producer.DownloadMedia
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Underlying,
		cfg.Name,
		&core.TransportPubMsg{
			Topic: cfg.Topic,
			Keys:  []byte(ev.RoomID()),
			Obj:   ev,
		})
}
