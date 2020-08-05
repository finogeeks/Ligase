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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/plugins/message/internals"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

const (
	presetPrivateChat        = "private_chat"
	presetTrustedPrivateChat = "trusted_private_chat"
	presetPublicChat         = "public_chat"
)

const (
	joinRulePublic = "public"
	joinRuleInvite = "invite"
)
const (
	historyVisibilityShared        = "shared"         //only after join can read
	historyVisibilityWorldReadable = "world_readable" //non-join people can read
	historyVisibilityInvited       = "invited"        //if invited can read
	historyVisibilityJoin          = "join"           //if joined can read
)

func validate(r *external.PostCreateRoomRequest) (int, *jsonerror.MatrixError) {
	whitespace := "\t\n\x0b\x0c\r " // https://docs.python.org/2/library/string.html#string.whitespace
	// https://github.com/matrix-org/synapse/blob/v0.19.2/synapse/handlers/room.py#L81
	// Synapse doesn't check for ':' but we will else it will break parsers badly which split things into 2 segments.
	if strings.ContainsAny(r.RoomAliasName, whitespace+":") {
		return http.StatusBadRequest, jsonerror.BadJSON("room_alias_name cannot contain whitespace")
	}
	for _, userID := range r.Invite {
		// TODO: We should put user ID parsing code into gomatrixserverlib and use that instead
		//       (see https://github.com/finogeeks/dendrite/skunkworks/gomatrixserverlib/blob/3394e7c7003312043208aa73727d2256eea3d1f6/eventcontent.go#L347 )
		//       It should be a struct (with pointers into a single string to avoid copying) and
		//       we should update all refs to use UserID types rather than strings.
		// https://github.com/matrix-org/synapse/blob/v0.19.2/synapse/types.py#L92
		if _, _, err := gomatrixserverlib.SplitID('@', userID); err != nil {
			return http.StatusBadRequest, jsonerror.BadJSON("user id must be in the form @localpart:domain")
		}
	}
	switch r.Preset {
	case presetPrivateChat, presetTrustedPrivateChat, presetPublicChat:
		break
	default:
		return http.StatusBadRequest, jsonerror.BadJSON("preset must be any of 'private_chat', 'trusted_private_chat', 'public_chat'")
	}

	return http.StatusOK, nil
}

// CreateRoom implements /createRoom
func CreateRoom(ctx context.Context, r *external.PostCreateRoomRequest, userID string,
	cfg config.Dendrite,
	accountDB model.AccountsDatabase, rpcCli roomserverapi.RoomserverRPCAPI,
	cache service.Cache,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	// TODO (#267): Check room ID doesn't clash with an existing one, and we
	//              probably shouldn't be using pseudo-random strings, maybe GUIDs?
	roomID := ""
	domainID, _ := common.DomainFromID(userID)
	if r.RoomID != "" {
		roomID = r.RoomID
	} else {
		nid, _ := idg.Next()
		roomID = fmt.Sprintf("!%d:%s", nid, domainID)
	}
	return createRoom(ctx, r, userID, cfg, roomID, domainID, accountDB, rpcCli, cache, idg, complexCache)
}

// createRoom implements /createRoom
// nolint: gocyclo
func createRoom(ctx context.Context, r *external.PostCreateRoomRequest,
	userID string, cfg config.Dendrite, roomID, domainID string,
	accountDB model.AccountsDatabase, rpcCli roomserverapi.RoomserverRPCAPI, cache service.Cache,
	idg *uid.UidGenerator, complexCache *common.ComplexCache,
) (int, core.Coder) {
	bs := time.Now().UnixNano()/1000000
	defer func(bs int64, roomID, userID string){
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("userID:%s createRoom roomID:%s spend:%d", userID, roomID, spend)
	}(bs, roomID, userID)
	now := time.Now()
	last := now
	fields := util.GetLogFields(ctx)
	//r.CreationContent = make(map[string]interface{})
	// TODO: apply rate-limit
	if r.Preset == "" {
		r.Preset = presetPrivateChat
	}

	if code, jerr := validate(r); jerr != nil {
		return code, jerr
	}

	// TODO: visibility/presets/raw initial state/creation content
	if r.Visibility == "" {
		r.Visibility = "private"
	}

	// TODO: Create room alias association

	fields = append(fields, log.KeysAndValues{"userID", userID, "roomID", roomID, "isDirect", r.IsDirect, "invite", r.Invite}...)
	log.Infow("Creating new room", fields)

	var roomAlias string
	if r.RoomAliasName != "" {
		roomAlias = fmt.Sprintf("#%s:%s", r.RoomAliasName, domainID)

		aliasReq := roomserverapi.SetRoomAliasRequest{
			Alias:  roomAlias,
			RoomID: roomID,
			UserID: userID,
		}

		var aliasResp roomserverapi.SetRoomAliasResponse
		err := rpcCli.AllocRoomAlias(ctx, &aliasReq, &aliasResp)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}

		if aliasResp.AliasExists {
			log.Debugf("send event check param use %v", time.Now().Sub(now))
			return http.StatusBadRequest, &internals.RespMessage{"Alias already exists"}
		}
	}

	now = time.Now()
	log.Debugf("send event check param use %v", now.Sub(last))
	last = now

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)

	now = time.Now()
	log.Debugf("send event get profile use %v", now.Sub(last))
	last = now

	membershipContent := external.MemberContent{
		Membership:  "join",
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}

	var joinRules, historyVisibility string
	powerContent := common.InitialPowerLevelsContent(userID)
	switch r.Preset {
	case presetPrivateChat:
		joinRules = joinRuleInvite
		historyVisibility = historyVisibilityShared
	case presetTrustedPrivateChat:
		joinRules = joinRuleInvite
		historyVisibility = historyVisibilityShared
		powerContent.UsersDefault = 100
		// TODO If trusted_private_chat, all invitees are given the same power level as the room creator.
	case presetPublicChat:
		joinRules = joinRulePublic
		historyVisibility = historyVisibilityShared
	}

	var builtEvents []gomatrixserverlib.Event

	// send events into the room in order of:
	//  1- m.room.create
	//  2- room creator join member
	//  3- m.room.power_levels
	//  4- m.room.canonical_alias (opt) TODO
	//  5- m.room.join_rules
	//  6- m.room.history_visibility
	//  7- m.room.guest_access (opt)
	//  8- other initial state items
	//  9- m.room.name (opt)
	//  10- m.room.topic (opt)
	//  11- invite events (opt) - with is_direct flag if applicable TODO
	//  12- 3pid invite events (opt) TODO
	//  13- m.room.aliases event for HS (if alias specified) TODO
	// This differs from Synapse slightly. Synapse would vary the ordering of 3-7
	// depending on if those events were in "initial_state" or not. This made it
	// harder to reason about, hence sticking to a strict static ordering.
	// TODO: Synapse has txn/token ID on each event. Do we need to do this here?

	federate := false
	fed, ok := r.CreationContent["m.federate"]
	if ok {
		federate = fed.(bool)
	}
	//r.CreationContent["m.federate"] = federate
	//r.CreationContent["creator"] = userID
	//r.CreationContent["is_direct"] = r.IsDrirect

	createContent := common.CreateContent{Creator: userID, Federate: &federate, IsDirect: &r.IsDirect}

	mapVal, ok := r.CreationContent["enable_watermark"]
	if ok {
		val := mapVal.(bool)
		createContent.EnableWatermark = &val
	}
	mapVal, ok = r.CreationContent["version"]
	if ok {
		val := int(mapVal.(float64))
		createContent.Version = &val
	}
	mapVal, ok = r.CreationContent["is_secret"]
	if ok {
		val := mapVal.(bool)
		createContent.IsSecret = &val
	}
	mapVal, ok = r.CreationContent["enable_favorite"]
	if ok {
		val := mapVal.(bool)
		createContent.EnableFavorite = &val
	}
	mapVal, ok = r.CreationContent["enable_snapshot"]
	if ok {
		val := mapVal.(bool)
		createContent.EnableSnapshot = &val
	}
	mapVal, ok = r.CreationContent["enable_forward"]
	if ok {
		val := mapVal.(bool)
		createContent.EnableForward = &val
	}
	mapVal, ok = r.CreationContent["is_channel"]
	if ok {
		val := mapVal.(bool)
		createContent.IsChannel = &val
	}

	mapVal, ok = r.CreationContent["room_type"]
	if ok {
		val := int(mapVal.(float64))
		createContent.RoomType = &val
	}
	mapVal, ok = r.CreationContent["is_organization_room"]
	if ok {
		val := mapVal.(bool)
		createContent.IsOrganizationRoom = &val
	}
	mapVal, ok = r.CreationContent["is_group_room"]
	if ok {
		val := mapVal.(bool)
		createContent.IsGroupRoom = &val
	}

	eventsToMake := []external.StateEvent{
		{Type: "m.room.create", Content: createContent},
		{Type: "m.room.member", StateKey: userID, Content: membershipContent},
		{Type: "m.room.power_levels", Content: powerContent},
		// TODO: m.room.canonical_alias
		{Type: "m.room.join_rules", Content: common.JoinRulesContent{JoinRule: joinRules}},
		{Type: "m.room.history_visibility", Content: common.HistoryVisibilityContent{HistoryVisibility: historyVisibility}},
		{Type: "m.room.visibility", Content: common.VisibilityContent{Visibility: r.Visibility}},
	}
	if r.GuestCanJoin {
		eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.guest_access", Content: common.GuestAccessContent{GuestAccess: "can_join"}})
	}
	eventsToMake = append(eventsToMake, r.InitialState...)
	if r.Name != "" {
		eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.name", Content: common.NameContent{Name: r.Name}})
	}
	if r.Desc != "" {
		eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.desc", Content: common.DescContent{Desc: r.Desc}})
	}
	if r.Topic != "" {
		eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.topic", Content: common.TopicContent{Topic: r.Topic}})
	}
	if roomAlias != "" {
		eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.aliases", StateKey: domainID, Content: common.AliasesContent{Aliases: []string{roomAlias}}})
	}

	now = time.Now()
	log.Debugf("send event set event use %v", now.Sub(last))
	last = now

	for _, invitor := range r.Invite {

		displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, invitor)

		membershipContent := external.MemberContent{
			Membership:  "invite",
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}

		eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.member", StateKey: invitor, Content: membershipContent, Sender: userID})
		if r.AutoJoin {
			membershipContent = external.MemberContent{
				Membership:  "join",
				DisplayName: displayName,
				AvatarURL:   avatarURL,
			}

			// tip: for join event, the sender should be one of invitees, but we set userID as sender because of federation stuff
			eventsToMake = append(eventsToMake, external.StateEvent{Type: "m.room.member", StateKey: invitor, Content: membershipContent, Sender: userID})
		}
	}
	now = time.Now()
	log.Debugf("send event set invite-event use %v", now.Sub(last))
	last = now
	// TODO: invite events
	// TODO: 3pid invite events
	// TODO: m.room.aliases

	authEvents := gomatrixserverlib.NewAuthEvents(nil)

	for i, e := range eventsToMake {
		depth := i + 1 // depth starts at 1

		if e.Sender == "" {
			e.Sender = userID
		}
		builder := gomatrixserverlib.EventBuilder{
			Sender:   e.Sender,
			RoomID:   roomID,
			Type:     e.Type,
			StateKey: &eventsToMake[i].StateKey,
			Depth:    int64(depth),
		}
		err := builder.SetContent(e.Content)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
		ev, err := common.BuildEvent(&builder, domainID, cfg, idg)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}

		//if err = gomatrixserverlib.Allowed(*ev, &authEvents); err != nil {
		//	return httputil.LogThenErrorCtx(ctx, err)
		//}

		// Add the event to the list of auth events
		builtEvents = append(builtEvents, *ev)
		err = authEvents.AddEvent(ev)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
	}

	now = time.Now()
	log.Debugf("send event set build-event use %v", now.Sub(last))
	last = now

	// send events to the room server
	txnAndDeviceID := &roomservertypes.TransactionID{
		DeviceID: userID,
	}

	rawEvent := roomserverapi.RawEvent{
		RoomID: roomID,
		Kind:   roomserverapi.KindNew,
		TxnID:  txnAndDeviceID,
		Trust:  false,
		BulkEvents: roomserverapi.BulkEvent{
			Events:  builtEvents,
			SvrName: domainID,
		},
		Query: []string{"create_room", ""},
	}

	_, err := rpcCli.InputRoomEvents(ctx, &rawEvent)

	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	now = time.Now()
	log.Debugf("send event set post-sync-event use %v", now.Sub(last))
	last = now

	response := &external.PostCreateRoomResponse{
		RoomID:    roomID,
		RoomAlias: roomAlias,
	}

	return http.StatusOK, response
}
