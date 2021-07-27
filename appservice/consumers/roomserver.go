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

package consumers

import (
	"context"

	"github.com/finogeeks/ligase/appservice/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	jsoniter "github.com/json-iterator/go"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// OutputRoomEventConsumer consumes events that originated in the room server.
type OutputRoomEventConsumer struct {
	channel      core.IChannel
	asDB         model.AppServiceDatabase
	rsDB         model.RoomServerDatabase
	workerStates []types.ApplicationServiceWorkerState
}

// NewOutputRoomEventConsumer creates a new OutputRoomEventConsumer. Call Start() to begin consuming from room servers.
func NewOutputRoomEventConsumer(
	cfg *config.Dendrite,
	appserviceDB model.AppServiceDatabase,
	rsDB model.RoomServerDatabase,
	workerStates []types.ApplicationServiceWorkerState,
) *OutputRoomEventConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.OutputRoomEventAppservice.Underlying,
		cfg.Kafka.Consumer.OutputRoomEventAppservice.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		c := &OutputRoomEventConsumer{
			channel:      channel,
			asDB:         appserviceDB,
			rsDB:         rsDB,
			workerStates: workerStates,
		}
		channel.SetHandler(c)
		return c
	}

	return nil
}

// Start consuming from room servers
func (c *OutputRoomEventConsumer) Start() error {
	//c.channel.Start()
	return nil
}

// onMessage is called when the sync server receives a new event from the room server output log.
// It is not safe for this function to be called from multiple goroutines, or else the
// sync stream position may race and be incorrectly calculated.
func (c *OutputRoomEventConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	// Parse out the event JSON
	var output roomserverapi.OutputEvent
	if err := json.Unmarshal(data, &output); err != nil {
		// If the message was invalid, log it and move on to the next message in the stream
		log.Errorw("applicationservice: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	if output.Type != roomserverapi.OutputTypeNewRoomEvent {
		log.Debugw("applicationservice: ignoring unknown output type", log.KeysAndValues{"topic", topic, "type", output.Type})
		return
	}

	ev := &output.NewRoomEvent.Event
	log.Infow("applicationservice received event from roomserver", log.KeysAndValues{"event_id", ev.EventID, "room_id", ev.RoomID, "type", ev.Type})

	// todo 从字段获取missevents
	missingEvents := []gomatrixserverlib.ClientEvent{}

	events := append(missingEvents, *ev)

	// Send event to any relevant application services
	c.filterRoomserverEvents(context.TODO(), events)

	//c.AsHandler.NotifyInterestedAppServices(context.Background(), ev)
}

// filterRoomserverEvents takes in events and decides whether any of them need
// to be passed on to an external application service. It does this by checking
// each namespace of each registered application service, and if there is a
// match, adds the event to the queue for events to be sent to a particular
// application service.
func (s *OutputRoomEventConsumer) filterRoomserverEvents(
	ctx context.Context,
	events []gomatrixserverlib.ClientEvent,
) error {
	for _, ws := range s.workerStates {
		for _, event := range events {
			// Check if this event is interesting to this application service
			if s.appserviceIsInterestedInEvent(ctx, event, ws.AppService) {
				// Queue this event to be sent off to the application service
				if err := s.asDB.StoreEvent(ctx, ws.AppService.ID, &event); err != nil {
					log.Warnw("failed to insert incoming event into appservices database", log.KeysAndValues{"error", err})
				} else {
					// Tell our worker to send out new messages by updating remaining message
					// count and waking them up with a broadcast
					ws.NotifyNewEvents()
				}
			}
		}
	}

	return nil
}

// appserviceIsInterestedInEvent returns a boolean depending on whether a given
// event falls within one of a given application service's namespaces.
func (s *OutputRoomEventConsumer) appserviceIsInterestedInEvent(ctx context.Context, event gomatrixserverlib.ClientEvent, appservice config.ApplicationService) bool {
	// No reason to queue events if they'll never be sent to the application
	// service
	if appservice.URL == "" {
		return false
	}

	// Check Room ID and Sender of the event
	if appservice.IsInterestedInUserID(event.Sender) ||
		appservice.IsInterestedInRoomID(event.RoomID) {
		log.Infow("interest in this event", log.KeysAndValues{"room_id", event.RoomID})
		return true
	}

	if appservice.InterestedAll {
		return true
	}

	// 注意 这里去掉了根据别名过滤的功能，这样就不会去查询roomserver的db了 但是同时根据别名过滤的功能也失效了
	// 暂时没用用到，以后应该让roomserver使用api支持这个功能

	//aliasList, err := s.rsDB.GetAliasesFromRoomID(context.Background(), event.RoomID)
	//// Check all known room aliases of the room the event came from
	//if err == nil {
	//	for _, alias := range aliasList {
	//		if appservice.IsInterestedInRoomAlias(alias) {
	//			return true
	//		}
	//	}
	//} else {
	//	log.WithFields(log.Fields{
	//		"room_id": event.RoomID(),
	//	}).WithError(err).Errorf("Unable to get aliases for room")
	//}

	return false
}
