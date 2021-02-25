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

package fedsender

import (
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"golang.org/x/net/context"
)

type EduSender struct {
	sender  *FederationSender
	cfg     *config.Dendrite
	channel core.IChannel
}

func NewEduSender(
	cfg *config.Dendrite,
) *EduSender {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.EduSenderInput.Underlying,
		cfg.Kafka.Consumer.EduSenderInput.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		eduSender := &EduSender{
			channel: channel,
			cfg:     cfg,
		}
		channel.SetHandler(eduSender)
		return eduSender
	}
	return nil
}

func (e *EduSender) Start() error {
	e.channel.Start()
	return nil
}

func (e *EduSender) SetSender(sender *FederationSender) {
	e.sender = sender
}

func (e *EduSender) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	log.Infof("fed-edu-sender received data topic:%s, data:%s", topic, string(data))
	var output gomatrixserverlib.EDU
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorf("fed-dispatch: message parse failure err:%v", err)
		return
	}
	e.sender.sendEdu(&output)
}
