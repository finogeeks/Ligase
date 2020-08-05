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

package entry

import (
	"context"
	"encoding/json"
	"github.com/finogeeks/ligase/federation/model/repos"
	"strings"

	"github.com/finogeeks/ligase/federation/client"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/pkg/errors"
)

func init() {
	Register(model.CMD_FED_INVITE, Invite)
}

func Invite(ctx context.Context, msg *model.GobMessage, cache service.Cache, rpcCli roomserverapi.RoomserverRPCAPI, fedClient *client.FedClientWrap, db fedmodel.FederationDatabase) (*model.GobMessage, error) {
	log.Infof("Enter Invite......")

	retMsg := &model.GobMessage{
		Body: []byte{},
	}

	if msg == nil {
		return retMsg, errors.New("msg from connector is nil")
	}

	var event gomatrixserverlib.Event
	json.Unmarshal(msg.Body, &event)

	type InviteStateEv struct {
		States []gomatrixserverlib.Event `json:"invite_room_state"`
	}
	var inviteRoomState InviteStateEv
	err := json.Unmarshal(event.Unsigned(), &inviteRoomState)
	if err != nil {
		return retMsg, errors.New("invite event usnigned invalid " + string(event.Unsigned()))
	}

	// TODO: check if can invite user

	resp := gomatrixserverlib.RespInvite{}
	resp.Code = 200

	roomID := event.RoomID()
	rs := rsRepo.GetRoomState(ctx, roomID)
	if rs == nil {
		log.Infof("invite event: %v", event)
		log.Infof("invite states: %s", event.Unsigned())

		states := inviteRoomState.States
		//states = append(states, event)
		err = backfillProc.AddRequest(ctx, states, false)
		if err != nil {
			if strings.Contains(err.Error(), "backfill finished") {
				resp.Code = 200
				// rawEvent := roomserverapi.RawEvent{
				// 	RoomID: event.RoomID(),
				// 	Kind:   roomserverapi.KindNew,
				// 	Trust:  true,
				// 	BulkEvents: roomserverapi.BulkEvent{
				// 		Events: []gomatrixserverlib.Event{event},
				// 	},
				// }

				// _, err := rpcCli.InputRoomEvents(context.TODO(), &rawEvent)
				// if err != nil {
				// 	log.Errorf("invite input event err: %v", err)
				// 	resp.Code = 500
				// 	//return retMsg, err
				// }
			} else {
				log.Errorf("api invite add backfill request err: %v", err)
				resp.Code = 500
			}
		} else {
			resp.Code = 200
			resp.Event = event

			var joinedRoomsVal *repos.JoinRoomsData
			joinedRoomsVal, err = joinRoomsRepo.GetData(ctx, event.RoomID())
			if err != nil {
				log.Warnf("api handle send get join rooms err %v", err)
				retMsg.Body, _ = json.Marshal(&resp)
				return retMsg, nil
			}
			if joinedRoomsVal.HasJoined {
				retMsg.Body, _ = json.Marshal(&resp)
				return retMsg, nil
			}
			joinedRoomsVal.HasJoined = true
			joinedRoomsVal.EventID = event.EventID()
			joinRoomsRepo.AddData(ctx, joinedRoomsVal)

			rawEvent := roomserverapi.RawEvent{
				RoomID: event.RoomID(),
				Kind:   roomserverapi.KindNew,
				Trust:  true,
				BulkEvents: roomserverapi.BulkEvent{
					Events: states, //[]gomatrixserverlib.Event{states[0]},
				},
			}

			_, err := rpcCli.InputRoomEvents(context.TODO(), &rawEvent)
			if err != nil {
				log.Errorf("invite input create event error %s", err.Error())
				// return retMsg, err
			}
		}
	}

	resp.Event = event

	retMsg.Body, _ = json.Marshal(&resp)

	return retMsg, nil
}
