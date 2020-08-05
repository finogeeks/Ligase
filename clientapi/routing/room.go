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

package routing

import (
	"context"
	jsonRaw "encoding/json"
	"errors"
	"github.com/finogeeks/ligase/clientapi/threepid"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/storage/model"
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
)

//handle GET /rooms/{roomId}/joined_members
func OnRoomJoinedRequest(
	ctx context.Context,
	userID,
	roomID string,
	rpcCli roomserverapi.RoomserverRPCAPI,
	cache service.Cache,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	err := rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if queryRes.RoomExists == false {
		return http.StatusNotFound, jsonerror.NotFound("Room does not exist")
	}

	user := userID
	if queryRes.Join[user] == nil {
		return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
	}

	resp := &external.GetJoinedMemberResponse{}
	resp.Joined = make(map[string]external.MemberContent)

	for _, ev := range queryRes.Join {
		member := external.MemberContent{}
		if err := json.Unmarshal(ev.Content(), &member); err != nil {
			log.Errorf("get joined member unmarshal event error: %v, content: %v", err, ev.Content())
			continue
		}

		displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, *ev.StateKey())
		member.AvatarURL = avatarURL
		member.DisplayName = displayName

		resp.Joined[*ev.StateKey()] = member
	}

	return http.StatusOK, resp
}

const MaxReqRooms = 50

var (
	ErrRoomDontExist = errors.New("Room does not exist")
	ErrQryRoomState  = errors.New("Query room state failed")
)

//handle GET /rooms
func OnRoomInfoRequest(
	ctx context.Context,
	userID string,
	roomIDs []string,
	rpcCli roomserverapi.RoomserverRPCAPI,
	cache service.Cache,
) (int, core.Coder) {
	if len(roomIDs) <= 0 {
		return http.StatusNotFound, &external.GetRoomInfoResponse{}
	} else if len(roomIDs) > MaxReqRooms {
		return http.StatusInternalServerError, jsonerror.NotFound("Error room amount")
	}

	resp := &external.GetRoomInfoResponse{}
	resp.Rooms = []external.GetRoomInfo{}

	for _, room := range roomIDs {
		roomInfo, _ := QueryRoomInfo(ctx, room, rpcCli)
		if roomInfo != nil {
			resp.Rooms = append(resp.Rooms, *roomInfo)
		}
	}
	if len(resp.Rooms) == 0 {
		return http.StatusNotFound, resp
	}
	return http.StatusOK, resp
}

func QueryRoomInfo(ctx context.Context, roomID string, rpcCli roomserverapi.RoomserverRPCAPI) (
	roomInfo *external.GetRoomInfo, err error) {
	roomInfo = &external.GetRoomInfo{
		RoomID: roomID,
	}

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest

	queryReq.RoomID = roomID
	err = rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		return roomInfo, err
	}
	if queryRes.RoomExists == false {
		roomInfo.RoomExists = false
		return roomInfo, errors.New("Room does not exist")
	}
	roomInfo.RoomExists = true

	createEvent, _ := queryRes.Create()
	if createEvent != nil {
		createContent := common.CreateContent{}
		if err = json.Unmarshal(createEvent.Content(), &createContent); err != nil {
			log.Errorf("QueryRoomInfo unparsable create event content roomID:%s error %v", roomID, err)
		} else {
			roomInfo.Creator = createContent.Creator
			roomInfo.IsDirect = false
			if createContent.IsDirect != nil {
				roomInfo.IsDirect = *createContent.IsDirect
			}
			roomInfo.IsFederate = false
			if createContent.Federate != nil {
				roomInfo.IsFederate = *createContent.Federate
			}
			roomInfo.IsChannel = false
			if createContent.IsChannel != nil {
				roomInfo.IsChannel = *createContent.IsChannel
			}
		}
	}

	nameEvent, _ := queryRes.RoomName()
	if nameEvent != nil {
		nameContent := common.NameContent{}
		if err = json.Unmarshal(nameEvent.Content(), &nameContent); err != nil {
			log.Errorf("QueryRoomInfo unparsable room name event content roomID:%s error %v", roomID, err)
		} else {
			roomInfo.Name = nameContent.Name
		}
	}

	avatarEvent, _ := queryRes.RoomAvatar()
	if avatarEvent != nil {
		avatarContent := common.AvatarContent{}
		if err = json.Unmarshal(avatarEvent.Content(), &avatarContent); err != nil {
			log.Errorf("QueryRoomInfo unparsable room avatar event content roomID:%s error %v", roomID, err)
		} else {
			roomInfo.AvatarURL = avatarContent.URL
		}
	}

	var joinMembers []string
	for userID := range queryRes.Join {
		joinMembers = append(joinMembers, userID)
	}
	roomInfo.JoinMembers = joinMembers
	if queryRes.Topic != nil {
		topicContent := common.TopicContent{}
		if err = json.Unmarshal(queryRes.Topic.Content(), &topicContent); err != nil {
			log.Errorf("QueryRoomInfo unparsable room topic content roomID:%s error %v", roomID, err)
		} else {
			roomInfo.Topic = topicContent.Topic
		}
	}
	powerLevels, _ := queryRes.PowerLevels()
	if powerLevels != nil {
		roomInfo.PowerLevels = (jsonRaw.RawMessage)(powerLevels.Content())
	}
	return roomInfo, nil
}

func DismissRoom(
	ctx context.Context, req *external.DismissRoomRequest, accountDB model.AccountsDatabase,
	ownerID, deviceID, roomID string,
	cfg config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI,
	federation *fed.Federation,
	cache service.Cache,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	if err := rpcCli.QueryRoomState(ctx, &queryReq, &queryRes); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	if plEvent, err := queryRes.PowerLevels(); plEvent != nil {
		plContent := common.PowerLevelContent{}
		if err = json.Unmarshal(plEvent.Content(), &plContent); err != nil {
			log.Errorf("DismissRoom unparsable powerlevel event content user %s roomID:%s error %v", ownerID, roomID, err)
			return http.StatusForbidden, jsonerror.Forbidden(err.Error())
		}
		if power, ok := plContent.Users[ownerID]; !ok || power != 100 {
			log.Errorf("DismissRoom must be administrator user %s roomID:%s", ownerID, roomID)
			return http.StatusForbidden, jsonerror.Forbidden("non-administrator is not allowed to dismiss room")
		}
	}
	msg := external.PostRoomsMembershipRequest{}
	msg.Membership = "dismiss"
	msg.RoomID = req.RoomID
	var body threepid.MembershipRequest
	body.UserID = ownerID
	content, _ := json.Marshal(body)
	msg.Content = content
	// sender leaves the room
	res, _ := SendMembership(ctx, &msg, accountDB, ownerID, ownerID, roomID, "dismiss", cfg, rpcCli, federation, cache, idg, complexCache)
	if res != http.StatusOK {
		return res, nil
	}
	// other members leave the room
	req.UserID = ownerID
	if err := common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.DismissRoom.Underlying,
		cfg.Kafka.Producer.DismissRoom.Name,
		&core.TransportPubMsg{
			Keys: []byte(roomID),
			Obj:  req,
		}); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	return http.StatusOK, &external.DismissRoomResponse{}

}
