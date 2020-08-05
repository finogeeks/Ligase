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

package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/clientapi/threepid"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

var errMissingUserID = errors.New("'user_id' must be supplied")

// SendMembership implements PUT /rooms/{roomID}/(join|kick|ban|unban|leave|invite)
// by building a m.room.member event then sending it to the room server
func SendMembership(
	ctx context.Context, r *external.PostRoomsMembershipRequest, accountDB model.AccountsDatabase, userID string,
	deviceID, roomID, membership string, cfg config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI,
	federation *fed.Federation,
	cache service.Cache,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	tid, err := idg.Next()
	traceId := fmt.Sprintf("%d", tid)
	log.Infof("------- traceId:%s handle SendMembership QueryRoomState send room %s membership:%s user:%s", traceId, roomID, membership, userID)
	var body threepid.MembershipRequest
	if membership != "leave" && len(r.Content) > 0 {
		if err := json.Unmarshal(r.Content, &body); err != nil {
			log.Errorf("------- traceId:%s handle SendMembership The request body could not be decoded into valid JSON room %s membership:%s user:%s", traceId, roomID, membership, userID)
			return http.StatusBadRequest, jsonerror.BadJSON("handle SendMembership The request body could not be decoded into valid JSON. " + err.Error())
		}
	}
	log.Infof("------- traceId:%s handle SendMembership QueryRoomState send room %s membership:%s user:%s body.user:%s", traceId, roomID, membership, userID, body.UserID)
	inviteStored, err := threepid.CheckAndProcessInvite(
		ctx, userID, &body, cfg, rpcCli, cache, membership, roomID, idg, complexCache,
	)
	if err == threepid.ErrMissingParameter {
		return http.StatusBadRequest, jsonerror.BadJSON(err.Error())
	} else if err == threepid.ErrNotTrusted {
		return http.StatusBadRequest, jsonerror.NotTrusted(body.IDServer)
	} else if err == common.ErrRoomNoExists {
		return http.StatusNotFound, jsonerror.NotFound(err.Error())
	} else if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	// If an invite has been stored on an identity server, it means that a
	// m.room.third_party_invite event has been emitted and that we shouldn't
	// emit a m.room.member one.
	if inviteStored {
		return http.StatusOK, &external.PostRoomsMembershipResponse{}
	}

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID

	events := []gomatrixserverlib.Event{}

	err = rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	log.Infof("-------traceId:%s handle SendMembership QueryRoomState recv roomID:%s resp %v, err:%v", traceId, roomID, queryRes, err)
	if err != nil {
		domainID, _ := common.DomainFromID(roomID)
		if membership == "join" && common.CheckValidDomain(domainID, cfg.Matrix.ServerName) == false {
			resp, err := federation.MakeJoin(domainID, roomID, userID, []string{"1"})
			if err != nil {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s make join error: %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}
			builder := resp.JoinEvent
			sendDomain, _ := common.DomainFromID(userID)
			ev, err := common.BuildEvent(&builder, sendDomain, cfg, idg)
			if err != nil {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s make join build event err %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}
			resp0, err := federation.SendJoin(domainID, roomID, ev.EventID(), *ev)
			if err != nil {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s send join err %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}

			events = resp0.StateEvents
			queryRes.InitFromEvents(events)
			log.Infof("traceId:%s handle SendMembership user:%s roomID:%s joinRoom state :%v", traceId, userID, roomID, resp)
		} else if !queryRes.RoomExists {
			return http.StatusBadRequest, jsonerror.NotFound(fmt.Sprintf("room %s not exists", roomID))
		} else {
			log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s  get room state err", traceId, userID, roomID)
			return httputil.LogThenErrorCtx(ctx, err)
		}
	}

	if membership == "forget" {
		if _, ok := queryRes.Join[userID]; ok {
			log.Errorf("traceId:%s handle SendMembership membership:%s, user:%s are currently in the room:%s", traceId, membership, userID, roomID)
			return http.StatusBadRequest, jsonerror.Forbidden(fmt.Sprintf("membership:%s, user:%s are currently in the room:%s", membership, userID, roomID))
		}
		if _, ok := queryRes.Leave[userID]; !ok {
			log.Errorf("traceId:%s handle SendMembership membership:%s You:%s aren't a member of the room:%s and weren't previously a member of the room", traceId, membership, userID, roomID)
			return http.StatusForbidden, jsonerror.Forbidden(fmt.Sprintf("membership:%s user:%s aren't a member of the room:%s and weren't previously a member of the room", membership, userID, roomID))
		}
	} else if membership == "join" {
		joinEv := queryRes.JoinRule
		rule := common.JoinRulesContent{}
		if joinEv == nil {
			return httputil.LogThenErrorCtx(ctx, errors.New("JoinRule nil"))
		}

		if err := json.Unmarshal(joinEv.Content(), &rule); err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}

		if _, ok := queryRes.Join[userID]; ok {
			//重复加入，直接响应成功
			return http.StatusOK, &external.PostRoomsMembershipResponse{
				RoomID: roomID,
			}
		}

		if rule.JoinRule != joinRulePublic {
			if _, ok := queryRes.Invite[userID]; !ok {
				log.Errorf("handle SendMembership traceId:%s membership:%s, user:%s are not invited to this room:%s", traceId, membership, userID, roomID)
				return http.StatusForbidden, jsonerror.Forbidden("You are not invited to this room.")
			}
		}
	} else if membership == "kick" {
		_, ok1 := queryRes.Join[userID]
		_, ok2 := queryRes.Leave[body.UserID]
		//log.Infof("membership:%s, kicker:%s bekicked:%s room:%s",membership,userID,body.UserID, roomID)
		if !ok1 {
			log.Errorf("handle SendMembership traceId:%s membership:%s, kicker:%s aren't a member of the room:%s bekicked:%s ", traceId, membership, userID, roomID, body.UserID)
			return http.StatusForbidden, jsonerror.Forbidden("kicker aren't a member of the room")
		}
		if ok2 {
			log.Errorf("handle SendMembership traceId:%s membership:%s, kickee:%s aren't a member of the room:%s kicker:%s ", traceId, membership, body.UserID, roomID, userID)
			return http.StatusForbidden, jsonerror.Forbidden("kickee aren't a member of the room")
		}
	} else if membership == "dismiss" {
		body.UserID = deviceID
		_, ok1 := queryRes.Join[body.UserID]
		_, ok2 := queryRes.Invite[body.UserID]
		if !ok1 && !ok2 {
			log.Warnf("handle SendMembership traceId:%s membership:%s, dismiss force leave member:%s aren't a member of the room:%s kicker:%s ", traceId, membership, body.UserID, roomID, userID)
			return http.StatusForbidden, jsonerror.Forbidden("dismiss member aren't in the room")
		}
	} else if membership == "leave" {
		_, ok1 := queryRes.Join[userID]
		_, ok2 := queryRes.Invite[userID]
		_, ok3 := queryRes.Leave[userID]
		//log.Infof("membership:%s, user:%s room:%s ",membership,userID,roomID)
		if !ok1 && !ok2 && !ok3 {
			log.Errorf("handle SendMembership traceId:%s membership:%s, user:%s aren't a member of the room:%s and weren't previously a member of the room ", traceId, membership, userID, roomID)
			return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
		} else {
			if ok3 {
				log.Errorf("handle SendMembership traceId:%s membership:%s, user:%s aren't a member of the room:%s and weren't previously a member of the room ", traceId, membership, userID, roomID)
				return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room.")
			}
		}
		domainID, _ := common.DomainFromID(roomID)
		if ok2 && !common.CheckValidDomain(domainID, cfg.Matrix.ServerName) {
			resp, err := federation.MakeLeave(domainID, roomID, userID)
			if err != nil {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s make leave error: %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}
			builder := resp.Event
			sendDomain, _ := common.DomainFromID(userID)
			ev, err := common.BuildEvent(&builder, sendDomain, cfg, idg)
			if err != nil {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s make leave build event err %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}
			resp0, err := federation.SendLeave(domainID, roomID, ev.EventID(), *ev)
			if err != nil {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s send leave err %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}
			if resp0.Code != 200 {
				log.Errorf("traceId:%s handle SendMembership user:%s roomID:%s send leave err %v", traceId, userID, roomID, err)
				return httputil.LogThenErrorCtx(ctx, err)
			}

			log.Infof("traceId:%s handle SendMembership user:%s roomID:%s leave room resp:%s", traceId, userID, roomID, resp)
		}
	}

	domainID, _ := common.DomainFromID(userID)
	event, err := buildMembershipEvent(
		ctx, body, accountDB, userID, membership, roomID, domainID, cfg, rpcCli, &queryRes, cache, idg, traceId, complexCache,
	)
	if err == errMissingUserID {
		log.Errorf("handle SendMembership traceId:%s errMissingUserID membership:%s, user:%s room:%s err:%v", traceId, membership, userID, roomID, err)
		return http.StatusBadRequest, jsonerror.BadJSON(err.Error())
	} else if err == common.ErrRoomNoExists {
		log.Errorf("handle SendMembership traceId:%s ErrRoomNoExists membership:%s, user:%s room:%s err:%v", traceId, membership, userID, roomID, err)
		return http.StatusNotFound, jsonerror.NotFound(err.Error())
	} else if err != nil {
		if membership == "invite" || membership == "join" || membership == "ban" || membership == "unban" || membership == "kick" || membership == "leave" || membership == "dismiss" {
			//log.Errorf("%v", err)
			log.Errorf("handle SendMembership traceId:%s build err membership:%s, user:%s room:%s err:%v", traceId, membership, userID, roomID, err)
			return http.StatusForbidden, jsonerror.Forbidden(err.Error())
		}
		log.Errorf("handle SendMembership traceId:%s build membership:%s, user:%s room:%s err:%v", traceId, membership, userID, roomID, err)
		return httputil.LogThenErrorCtx(ctx, err)
	}

	events = append(events, *event)
	// eventIdx := len(events) - 1
	txnAndDeviceID := &roomservertypes.TransactionID{
		DeviceID: deviceID,
	}

	if membership == "invite" {
		if body.AutoJoin {
			log.Infof("handle SendMembership traceId:%s auto invite membership:%s, user:%s inviter:%s room:%s ", traceId, membership, userID, body.UserID, roomID)
			event, err = buildMembershipEvent(
				ctx, body, accountDB, userID, "join", roomID, domainID, cfg, rpcCli, &queryRes, cache, idg, traceId, complexCache,
			)
			if err != nil {
				log.Errorf("handle SendMembership traceId:%s err auto invite membership:%s, user:%s inviter:%s room:%s ", traceId, membership, userID, body.UserID, roomID)
				return httputil.LogThenErrorCtx(ctx, err)
			}
			events = append(events, *event)
			//bytes, _ := event.MarshalJSON()
			//log.Infof("--------invite autojoin add join ev:%s", string(bytes))
		}
		//federation invite
		// Ask the requesting server to sign the newly created event so we know it
		// acknowledged it

		// inviteeDomain, err := common.DomainFromID(body.UserID)
		// if err != nil {
		// 	return http.StatusBadRequest, jsonerror.BadJSON("invitee Id must be in the form '@localpart:domain'")
		// }

		// if !common.CheckValidDomain(inviteeDomain, cfg.Matrix.ServerName) {
		// 	//TODO federation auto join
		// 	type UnsingedInviteStates struct {
		// 		States []gomatrixserverlib.Event `json:"invite_room_state"`
		// 	}
		// 	inviteStates := UnsingedInviteStates{States: queryRes.GetAllState()}
		// 	inviteEv, err := event.SetUnsigned(inviteStates, false)
		// 	if err != nil {
		// 		return httputil.LogThenErrorCtx(ctx, err)
		// 	}
		// 	resp, err := federation.SendInvite(inviteeDomain, inviteEv)
		// 	if resp.Code != 200 {
		// 		return resp.Code, jsonerror.Unknown("invitee reject from remote server")
		// 	}
		// 	signedEvent := resp.Event
		// 	if err != nil {
		// 		return httputil.LogThenErrorCtx(ctx, err)
		// 	}

		// 	events[eventIdx] = signedEvent
		// }

	}

	rawEvent := roomserverapi.RawEvent{
		RoomID: roomID,
		Kind:   roomserverapi.KindNew,
		TxnID:  txnAndDeviceID,
		Trust:  true,
		BulkEvents: roomserverapi.BulkEvent{
			Events:  events,
			SvrName: domainID,
		},
		Query: []string{"membership", membership},
	}

	_, err = rpcCli.InputRoomEvents(context.Background(), &rawEvent)

	if err != nil {
		log.Errorf("handle SendMembership traceId:%s membership:%s, user:%s room:%s input err:%v", traceId, membership, userID, roomID, err)
		if membership == "invite" && strings.HasPrefix(err.Error(), "SendInvite error: ") {
			errJSON := strings.TrimPrefix(err.Error(), "SendInvite error: ")
			inviteResp := gomatrixserverlib.RespInvite{}
			err := inviteResp.Decode([]byte(errJSON))
			if err == nil {
				return inviteResp.Code, jsonerror.Unknown("invitee reject from remote server")
			}
		}
		if strings.Index(err.Error(), "timeout") >= 0 {
			return http.StatusGatewayTimeout, jsonerror.Timeout(err.Error())
		}
		if membership == "invite" || membership == "join" || membership == "ban" || membership == "unban" || membership == "kick" || membership == "leave" || membership == "dismiss" {
			//log.Errorf("%v", err)
			return http.StatusForbidden, jsonerror.Forbidden(fmt.Sprintf("can't %s.", membership))
		}
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if membership == "join" {
		log.Infof("handle SendMembership traceId:%s join succ membership:%s, user:%s room:%s ", traceId, membership, userID, roomID)
		return http.StatusOK, &external.PostRoomsMembershipResponse{
			RoomID: roomID,
		}
	}
	log.Infof("handle SendMembership traceId:%s succ membership:%s, user:%s room:%s ", traceId, membership, userID, roomID)
	return http.StatusOK, &external.PostRoomsMembershipResponse{}
}

func buildMembershipEvent(
	ctx context.Context,
	body threepid.MembershipRequest, accountDB model.AccountsDatabase,
	userID, membership, roomID, domainID string, cfg config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI, queryRes *roomserverapi.QueryRoomStateResponse, cache service.Cache,
	idg *uid.UidGenerator, traceId string, complexCache *common.ComplexCache,
) (*gomatrixserverlib.Event, error) {
	stateKey, reason, err := getMembershipStateKey(body, userID, membership)
	if err != nil {
		log.Errorf("getMembershipStateKey handle SendMembership buildMembershipEvent traceId:%s membership:%s, user:%s room:%s ", traceId, membership, userID, roomID)
		return nil, err
	}

	profile, _ := loadProfile(ctx, stateKey, cfg, accountDB, complexCache)

	builder := gomatrixserverlib.EventBuilder{
		Sender:   userID,
		RoomID:   roomID,
		Type:     "m.room.member",
		StateKey: &stateKey,
	}

	rawMemberShip := membership

	// "unban" or "kick" isn't a valid membership value, change it to "leave"
	if membership == "unban" || membership == "kick" || membership == "dismiss" {
		membership = "leave"
	}

	content := external.MemberContent{
		Membership: membership,
		Reason:     reason,
	}

	if profile != nil {
		content.DisplayName = profile.DisplayName
		content.AvatarURL = profile.AvatarURL
	}

	if err = builder.SetContent(content); err != nil {
		log.Errorf("setcontent handle SendMembership traceId:%s membership:%s buildMembershipEvent builder.SetContent for user %s roomID:%s error %v", traceId, membership, userID, roomID, err)
		return nil, err
	}

	e, err := common.BuildEvent(&builder, domainID, cfg, idg)
	if err == nil {
		if membership == "join" && body.AutoJoin {
			log.Infof("buildevent join and autojoin handle SendMembership traceId:%s membership:%s buildMembershipEvent builder.SetContent for user %s roomID:%s error %v", traceId, membership, userID, roomID, err)
			return e, nil
		}
	}

	createEvent, _ := queryRes.Create()
	createContent := common.CreateContent{}
	if err = json.Unmarshal(createEvent.Content(), &createContent); err != nil {
		log.Errorf("buildMembershipEvent unparsable create event content traceId:%s membership:%s user %s roomID:%s error %v", traceId, membership, userID, roomID, err)
		return nil, err
	}

	// Special case: leaved user may send invite event in a direct room,
	// just skip auth for convenience.
	if createContent.IsDirect != nil && *createContent.IsDirect == true {
		return e, nil
	}

	if err = gomatrixserverlib.Allowed(*e, queryRes); err == nil {
		if rawMemberShip == "leave" {
			if plEvent, err := queryRes.PowerLevels(); plEvent != nil {
				plContent := common.PowerLevelContent{}
				if err = json.Unmarshal(plEvent.Content(), &plContent); err != nil {
					log.Errorf("buildMembershipEvent unparsable powerlevel event content traceId:%s membership:%s user %s roomID:%s error %v", traceId, membership, userID, roomID, err)
					return nil, err
				}

				if power, ok := plContent.Users[userID]; ok && power == 100 && len(queryRes.Join) > 1 {
					log.Errorf("buildMembershipEvent administrator is not allowed to leave before changing power traceId:%s membership:%s user %s roomID:%s", traceId, membership, userID, roomID)
					return nil, errors.New("administrator is not allowed to leave before changing power")
				}
			}
		}
	}

	return e, err
}

// loadProfile lookups the profile of a given user from the database and returns
// it if the user is local to this server, or returns an empty profile if not.
// Returns an error if the retrieval failed or if the first parameter isn't a
// valid Matrix ID.
func loadProfile(
	ctx context.Context, userID string, cfg config.Dendrite, accountDB model.AccountsDatabase, complexCache *common.ComplexCache,
) (*authtypes.Profile, error) {
	domain, err := common.DomainFromID(userID)
	if err != nil {
		return nil, err
	}
	var profile *authtypes.Profile
	profile = &authtypes.Profile{}
	if common.CheckValidDomain(domain, cfg.Matrix.ServerName) {
		displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)

		profile.DisplayName = displayName
		profile.AvatarURL = avatarURL
		profile.UserID = userID
	}

	return profile, nil
}

// getMembershipStateKey extracts the target user ID of a membership change.
// For "join" and "leave" this will be the ID of the user making the change.
// For "ban", "unban", "kick" and "invite" the target user ID will be in the JSON request body.
// In the latter case, if there was an issue retrieving the user ID from the request body,
// returns a JSONResponse with a corresponding error code and message.
func getMembershipStateKey(
	body threepid.MembershipRequest, userID, membership string,
) (stateKey string, reason string, err error) {
	if membership == "ban" || membership == "unban" || membership == "kick" || membership == "invite" {
		// If we're in this case, the state key is contained in the request body,
		// possibly along with a reason (for "kick" and "ban") so we need to parse
		// it
		if body.UserID == "" {
			err = errMissingUserID
			return
		}

		stateKey = body.UserID
		reason = body.Reason
	} else {
		stateKey = userID
		if body.AutoJoin == true && body.UserID != "" {
			stateKey = body.UserID
		}
	}

	return
}
