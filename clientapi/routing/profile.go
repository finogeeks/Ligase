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
	"database/sql"
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func GetWhoAmi(
	userID string,
) (int, core.Coder) {
	return http.StatusOK, &external.GetAccountWhoAmI{UserID: userID}
}

func getFedProfile(
	ctx context.Context,
	userID string,
	cfg config.Dendrite,
	federation *fed.Federation,
	complexCache *common.ComplexCache,
	domain string,
) (int, *external.GetProfileResponse) {
	avatarURL := cfg.DefaultAvatar
	displayName := ""

	resp, err := federation.LookupProfile(domain, userID)
	if err != nil {
		log.Errorf("get profile from federation error %v", err)
		// set empty result?
	} else {
		avatarURL = resp.AvatarURL
		displayName = resp.DisplayName
		complexCache.SetProfile(ctx, userID, resp.DisplayName, resp.AvatarURL)
	}

	return http.StatusOK, &external.GetProfileResponse{
		AvatarURL:   avatarURL,
		DisplayName: displayName,
	}
}

func checkDomain(ctx context.Context, cfg config.Dendrite, domain string, cache service.Cache, db model.RoomServerDatabase) bool {
	if common.CheckValidDomain(domain, cfg.Matrix.ServerName) {
		return true
	}
	settingKey := "im.federation.domains"
	domains := []common.FedDomainInfo{}
	str, err := cache.GetSettingRaw(settingKey)
	if err != nil {
		log.Warnf("get settingKey:%s from cache err:%v", settingKey, err)
		str, err = db.SelectSettingKey(ctx, settingKey)
		if err != nil {
			log.Warnf("get settingKey:%s from db err:%v", settingKey, err)
			return false
		}
		err = cache.SetSetting(settingKey, str)
		if err != nil {
			log.Warnf("get settingKey:%s set val from db to cache err:%v", settingKey, err)
		}
	}
	if str == "" {
		return false
	}
	err = json.UnmarshalFromString(str, &domains)
	if err != nil {
		log.Warnf("get settingKey:%s UnmarshalFromString err:%v", settingKey, err)
		return false
	}
	for _, d := range domains {
		if d.Domain == domain {
			return true
		}
	}
	return false
}

// GetProfile implements GET /profile/{userID}
func GetProfile(
	ctx context.Context,
	req *external.GetProfileRequest,
	cfg config.Dendrite,
	federation *fed.Federation,
	accountDB model.AccountsDatabase,
	cache service.Cache,
	complexCache *common.ComplexCache,
	roomDB model.RoomServerDatabase,
) (int, core.Coder) {
	userID := req.UserID
	domain, _ := utils.DomainFromID(userID)
	if checkDomain(ctx, cfg, domain, cache, roomDB) == false {
		return http.StatusNotFound, &external.GetProfileResponse{
			AvatarURL:   cfg.DefaultAvatar,
			DisplayName: "",
		}
	}

	displayName, avatarURL, err := complexCache.GetProfileByUserID(ctx, userID)
	if err != nil {
		log.Warnf("getProfileComplex from local domain: %s, error: %v", domain, err)
		return getFedProfile(ctx, userID, cfg, federation, complexCache, domain)
	}

	return http.StatusOK, &external.GetProfileResponse{
		AvatarURL:   avatarURL,
		DisplayName: displayName,
	}
}

// GetAvatarURL implements GET /profile/{userID}/avatar_url
func GetAvatarURL(
	ctx context.Context,
	userID string,
	cfg config.Dendrite,
	accountDB model.AccountsDatabase,
	cache service.Cache,
	federation *fed.Federation,
	complexCache *common.ComplexCache,
	roomDB model.RoomServerDatabase,
) (int, core.Coder) {
	domain, _ := utils.DomainFromID(userID)
	if checkDomain(ctx, cfg, domain, cache, roomDB) == false {
		return http.StatusNotFound, &external.GetAvatarURLResponse{
			AvatarURL: cfg.DefaultAvatar,
		}
	}

	avatarURL, err := complexCache.GetAvatarURL(ctx, userID)
	if err != nil {
		log.Warnf("getAvatarComplex from local domain: %s, error: %v", domain, err)
		code, resp := getFedProfile(ctx, userID, cfg, federation, complexCache, domain)
		return code, &external.GetAvatarURLResponse{
			AvatarURL: resp.AvatarURL,
		}
	}

	return http.StatusOK, &external.GetAvatarURLResponse{
		AvatarURL: avatarURL,
	}
}

// SetAvatarURL implements PUT /profile/{userID}/avatar_url
func SetAvatarURL(
	ctx context.Context, r *external.PutAvatarURLRequest,
	accountDB model.AccountsDatabase,
	userID string, cfg *config.Dendrite,
	cache service.Cache,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	if userID != r.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("userID does not match the current user")
	}

	log.Infof("SetAvatarURL %s avatar: %s", userID, r.AvatarURL)

	oldDisplayName, oldAvatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)
	if oldAvatarURL == r.AvatarURL {
		return http.StatusOK, nil
	}

	err := complexCache.SetAvatarURL(ctx, userID, r.AvatarURL)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	content := types.PresenceJSON{
		AvatarURL:       r.AvatarURL,
		DisplayName:     oldDisplayName,
		Presence:        "",
		CurrentlyActive: true,
		UserID:          userID,
		LastActiveAgo:   0,
	}

	// add ext user info
	userInfo := cache.GetUserInfoByUserID(userID)
	if userInfo != nil {
		content.UserName = userInfo.UserName
		content.JobNumber = userInfo.JobNumber
		content.Mobile = userInfo.Mobile
		content.Landline = userInfo.Landline
		content.Email = userInfo.Email
		content.State = userInfo.State
	}

	presences, ok := cache.GetPresences(userID)
	if ok && presences.UserID != "" {
		currentlyActive := false
		if presences.Status == "online" {
			currentlyActive = true
		}

		content.StatusMsg = presences.StatusMsg
		content.Presence = presences.Status
		content.CurrentlyActive = currentlyActive
		content.ExtStatusMsg = presences.ExtStatusMsg
	}

	data := new(types.ProfileStreamUpdate)
	data.UserID = userID
	data.Presence = content

	span, ctx := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputProfileData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputProfileData.Name,
		cfg.Kafka.Producer.OutputProfileData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputProfileData.Underlying,
		cfg.Kafka.Producer.OutputProfileData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(userID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	if cfg.SendMemberEvent {
		var request roomserverapi.QueryJoinRoomsRequest
		request.UserID = userID
		var response roomserverapi.QueryJoinRoomsResponse
		err = rsRpcCli.QueryJoinRooms(ctx, &request, &response)
		if err != nil {
			if err != sql.ErrNoRows {
				return httputil.LogThenErrorCtx(ctx, err)
			}
		} else {
			newProfile := authtypes.Profile{
				DisplayName: oldDisplayName,
				AvatarURL:   r.AvatarURL,
			}
			go BuildMembershipAndFireEvents(ctx, response.Rooms, newProfile, userID, cfg, rsRpcCli, idg)
		}
	}

	return http.StatusOK, nil
}

// GetDisplayName implements GET /profile/{userID}/displayname
func GetDisplayName(
	ctx context.Context, userID string, cfg config.Dendrite,
	accountDB model.AccountsDatabase,
	cache service.Cache, federation *fed.Federation,
	complexCache *common.ComplexCache,
	roomDB model.RoomServerDatabase,
) (int, core.Coder) {
	domain, _ := utils.DomainFromID(userID)
	if checkDomain(ctx, cfg, domain, cache, roomDB) == false {
		return http.StatusNotFound, &external.GetDisplayNameResponse{
			DisplayName: "",
		}
	}

	// local check
	displayName, err := complexCache.GetDisplayName(ctx, userID)
	if err != nil {
		log.Warnf("getDisplayNameComplex from local domain: %s, error: %v", domain, err)

		code, resp := getFedProfile(ctx, userID, cfg, federation, complexCache, domain)
		return code, &external.GetDisplayNameResponse{
			DisplayName: resp.DisplayName,
		}
	}

	return http.StatusOK, &external.GetDisplayNameResponse{
		DisplayName: displayName,
	}
}

// SetDisplayName implements PUT /profile/{userID}/displayname
func SetDisplayName(
	ctx context.Context, r *external.PutDisplayNameRequest,
	accountDB model.AccountsDatabase,
	userID string, cfg *config.Dendrite,
	cache service.Cache,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	if userID != r.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("userID does not match the current user")
	}

	log.Infof("SetDisplayName %s displayName: %s", userID, r.DisplayName)

	oldDisplayName, oldAvatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)
	if oldDisplayName == r.DisplayName {
		return http.StatusOK, nil
	}

	log.Infof("++++++++++++++++++++oldDisplayName: %s, oldAvatarURL: %s", oldDisplayName, oldAvatarURL)

	err := complexCache.SetDisplayName(ctx, userID, r.DisplayName)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	content := types.PresenceJSON{
		AvatarURL:       oldAvatarURL,
		DisplayName:     r.DisplayName,
		Presence:        "",
		CurrentlyActive: true,
		UserID:          userID,
		LastActiveAgo:   0,
	}

	// add ext user info
	userInfo := cache.GetUserInfoByUserID(userID)
	if userInfo != nil {
		content.UserName = userInfo.UserName
		content.JobNumber = userInfo.JobNumber
		content.Mobile = userInfo.Mobile
		content.Landline = userInfo.Landline
		content.Email = userInfo.Email
		content.State = userInfo.State
	}

	presences, ok := cache.GetPresences(userID)
	if ok && presences.UserID != "" {
		currentlyActive := false
		if presences.Status == "online" {
			currentlyActive = true
		}

		content.StatusMsg = presences.StatusMsg
		content.Presence = presences.Status
		content.CurrentlyActive = currentlyActive
		content.ExtStatusMsg = presences.ExtStatusMsg
	}

	data := new(types.ProfileStreamUpdate)
	data.UserID = userID
	data.Presence = content

	span, ctx := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputProfileData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputProfileData.Name,
		cfg.Kafka.Producer.OutputProfileData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputProfileData.Underlying,
		cfg.Kafka.Producer.OutputProfileData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(userID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	if cfg.SendMemberEvent {
		var request roomserverapi.QueryJoinRoomsRequest
		request.UserID = userID
		var response roomserverapi.QueryJoinRoomsResponse
		err = rsRpcCli.QueryJoinRooms(ctx, &request, &response)
		if err != nil {
			if err != sql.ErrNoRows {
				return httputil.LogThenErrorCtx(ctx, err)
			}
		} else {
			newProfile := authtypes.Profile{
				DisplayName: r.DisplayName,
				AvatarURL:   oldAvatarURL,
			}
			go BuildMembershipAndFireEvents(ctx, response.Rooms, newProfile, userID, cfg, rsRpcCli, idg)
		}
	}

	return http.StatusOK, nil
}

func BuildMembershipAndFireEvents(
	ctx context.Context,
	rooms []string,
	newProfile authtypes.Profile, userID string, cfg *config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI,
	idg *uid.UidGenerator,
) {
	for _, roomID := range rooms {
		builder := gomatrixserverlib.EventBuilder{
			Sender:   userID,
			RoomID:   roomID,
			Type:     "m.room.member",
			StateKey: &userID,
		}

		content := external.MemberContent{
			Membership: "join",
		}

		content.DisplayName = newProfile.DisplayName
		content.AvatarURL = newProfile.AvatarURL

		if err := builder.SetContent(content); err != nil {
			log.Errorf("BuildMembershipAndFireEvents fail on SetContent for roomid:%s user:%s with err:%v", roomID, userID, err)
			continue
		}

		var queryRes roomserverapi.QueryRoomStateResponse
		var queryReq roomserverapi.QueryRoomStateRequest
		queryReq.RoomID = roomID
		err := rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
		if err != nil || queryRes.RoomExists == false {
			log.Errorf("BuildMembershipAndFireEvents fail on QueryRoomState for roomid:%s user:%s with err:%v", roomID, userID, err)
			continue
		}

		domain, err := common.DomainFromID(roomID)
		if err != nil {
			log.Errorf("BuildMembershipAndFireEvents fail on common.DomainFromID for roomid:%s user:%s with err:%v", roomID, userID, err)
			continue
		}

		event, err := common.BuildEvent(&builder, domain, *cfg, idg)
		if err != nil {
			log.Errorf("BuildMembershipAndFireEvents fail on BuildEvent for roomid:%s user:%s with err:%v", roomID, userID, err)
			continue
		}

		if err = gomatrixserverlib.Allowed(*event, &queryRes); err != nil {
			log.Errorf("-----buildMembershipEvents fail to check, err:%v, ev:%v", err, *event)
		} else {
			rawEvent := roomserverapi.RawEvent{
				RoomID: roomID,
				Kind:   roomserverapi.KindNew,
				Trust:  true,
				BulkEvents: roomserverapi.BulkEvent{
					Events:  []gomatrixserverlib.Event{*event},
					SvrName: domain,
				},
			}
			_, err = rpcCli.InputRoomEvents(ctx, &rawEvent)
			if err != nil {
				log.Errorf("BuildMembershipAndFireEvents fail on PostEvent for roomid:%s user:%s with err:%v", roomID, userID, err)
				continue
			}
		}
	}
}

const maxReqUsers = 50

// GetProfiles implements GET /profiles
func GetProfiles(
	ctx context.Context,
	req *external.GetProfilesRequest,
	cfg config.Dendrite,
	federation *fed.Federation,
	accountDB model.AccountsDatabase,
	cache service.Cache,
	complexCache *common.ComplexCache,
	roomDB model.RoomServerDatabase,
) (int, core.Coder) {
	userIDs := req.UserIDs

	if len(userIDs) <= 0 {
		return http.StatusNotFound, &external.GetProfilesResponse{}
	} else if len(userIDs) > maxReqUsers {
		return http.StatusInternalServerError, jsonerror.NotFound("Error user amount")
	}

	resp := &external.GetProfilesResponse{}
	resp.Profiles = []external.ProfileItem{}
	profileItem := external.ProfileItem{}

	reqItem := external.GetProfileRequest{}
	for _, user := range userIDs {
		reqItem.UserID = user
		_, coder := GetProfile(ctx, &reqItem, cfg, federation, accountDB, cache, complexCache, roomDB)

		respItem := coder.(*external.GetProfileResponse)
		profileItem.UserID = user
		profileItem.AvatarURL = respItem.AvatarURL
		profileItem.DisplayName = respItem.DisplayName

		resp.Profiles = append(resp.Profiles, profileItem)
	}
	if len(resp.Profiles) == 0 {
		return http.StatusNotFound, resp
	}

	return http.StatusOK, resp
}
