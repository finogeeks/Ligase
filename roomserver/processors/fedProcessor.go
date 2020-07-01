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

package processors

import (
	"context"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	//log "github.com/finogeeks/ligase/skunkworks/log"
)

type FedProcessor struct {
	Alias AliasProcessor
}

func (r *FedProcessor) OnRoomEvent(
	event *gomatrixserverlib.Event,
) error {
	if event.Type() == "m.room.aliases" {
		var request roomserverapi.SetRoomAliasRequest
		var response roomserverapi.SetRoomAliasResponse
		var payload roomAliasesContent

		err := json.Unmarshal(event.Content(), &payload)
		if err != nil {
			return err
		}

		for _, alias := range payload.Aliases {
			request.Alias = alias
			request.RoomID = event.RoomID()
			request.UserID = event.Sender()
			err = r.Alias.AllocRoomAlias(context.TODO(), &request, &response)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
