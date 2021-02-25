// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package consumers

import (
	"context"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// OutputRoomEventConsumer consumes events that originated in the room server.
type OutputRoomEventConsumer struct {
	channel  core.IChannel
	db       model.PublicRoomAPIDatabase
	rsRpcCli roomserverapi.RoomserverRPCAPI
}

// NewOutputRoomEventConsumer creates a new OutputRoomEventConsumer. Call Start() to begin consuming from room servers.
func NewOutputRoomEventConsumer(
	cfg *config.Dendrite,
	store model.PublicRoomAPIDatabase,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
) *OutputRoomEventConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputRoomEventPublicRooms.Underlying,
		cfg.Kafka.Consumer.OutputRoomEventPublicRooms.Name,
	)

	if ok {
		channel := val.(core.IChannel)
		s := &OutputRoomEventConsumer{
			channel:  channel,
			db:       store,
			rsRpcCli: rsRpcCli,
		}
		channel.SetHandler(s)
		return s
	}

	return nil
}

// Start consuming from room servers
func (s *OutputRoomEventConsumer) Start() error {
	//s.channel.Start()
	return nil
}

// OnMessage is called when the sync server receives a new event from the room server output log.
func (s *OutputRoomEventConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var output roomserverapi.OutputEvent
	if err := json.Unmarshal(data, &output); err != nil {
		// If the message was invalid, log it and move on to the next message in the stream
		log.Errorw("publicroomsapi: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	if output.Type != roomserverapi.OutputTypeNewRoomEvent {
		return
	}

	ev := output.NewRoomEvent.Event
	log.Infow("publicroomapi received event from roomserver", log.KeysAndValues{"event_id", ev.EventID, "room_id", ev.RoomID, "type", ev.Type})
	if s.isCare(&ev) {
		s.db.UpdateRoomFromEvent(context.TODO(), ev)
	}
}

func (s *OutputRoomEventConsumer) isCare(ev *gomatrixserverlib.ClientEvent) bool {
	switch ev.Type {
	case "m.room.create":
		return true
	case "m.room.member":
		return true
	case "m.room.aliases":
		return true
	case "m.room.canonical_alias":
		return true
	case "m.room.name":
		return true
	case "m.room.topic":
		return true
	case "m.room.desc":
		return true
	case "m.room.avatar":
		return true
	case "m.room.history_visibility":
		return true
	case "m.room.visibility":
		return true
	case "m.room.guest_access":
		return true
	default:
		return false
	}
}
