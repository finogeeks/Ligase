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
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

// GetUserInfo implements GET /user_info/{userID}
func GetUserInfo(
	ctx context.Context,
	req *external.GetUserInfoRequest,
	cfg config.Dendrite,
	federation *fed.Federation,
	accountDB model.AccountsDatabase,
	cache service.Cache,
) (int, core.Coder) {
	userID := req.UserID

	user_info := cache.GetUserInfoByUserID(userID)
	if user_info != nil && user_info.UserID != "" {
		return http.StatusOK, &external.GetUserInfoResponse{
			UserName:  user_info.UserName,
			JobNumber: user_info.JobNumber,
			Mobile:    user_info.Mobile,
			Landline:  user_info.Landline,
			Email:     user_info.Email,
			State:     user_info.State,
		}
	}
	log.Infof("mssing user_info cache: %s, user_info: %#v", userID, user_info)

	if user_info == nil || user_info.UserID == "" {
		domain, _ := utils.DomainFromID(userID)
		if !common.CheckValidDomain(domain, cfg.Matrix.ServerName) {
			resp, err := federation.LookupUserInfo(domain, userID)
			if err != nil {
				log.Errorf("get user_info from federation error %v", err)
			} else {
				log.Infof("get user_info from federation succeed: %s, resp: %#v", userID, resp)
				cache.SetUserInfo(userID, resp.UserName, resp.JobNumber, resp.Mobile, resp.Landline, resp.Email,resp.State)
				accountDB.UpsertUserInfo(ctx, userID, resp.UserName, resp.JobNumber, resp.Mobile, resp.Landline, resp.Email,resp.State)
				return http.StatusOK, &external.GetUserInfoResponse{
					UserName:  resp.UserName,
					JobNumber: resp.JobNumber,
					Mobile:    resp.Mobile,
					Landline:  resp.Landline,
					Email:     resp.Email,
					State:     resp.State,
				}
			}
		}
	}

	log.Warnf("get user_info failed, return empty: %s", userID)
	return http.StatusOK, &external.GetUserInfoResponse{
		UserName:  "",
		JobNumber: "",
		Mobile:    "",
		Landline:  "",
		Email:     "",
		State:     0,
	}
}

// AddUserInfo implements POST /user_info/{userID}
func AddUserInfo(
	ctx context.Context, r *external.PostUserInfoRequest,
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

	log.Infof("AddUserInfo %s, %v", userID, r)

	oldUserInfo := cache.GetUserInfoByUserID(userID)
	if oldUserInfo != nil {
		if oldUserInfo.UserName == r.UserName && oldUserInfo.JobNumber == r.JobNumber &&
			oldUserInfo.Mobile == r.Mobile && oldUserInfo.Landline == r.Landline &&
			oldUserInfo.Email == r.Email && oldUserInfo.State == r.State {
			return http.StatusOK, nil
		}
	}

	err := cache.SetUserInfo(userID, r.UserName, r.JobNumber, r.Mobile, r.Landline, r.Email, r.State)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	err = accountDB.UpsertUserInfo(ctx, userID, r.UserName, r.JobNumber, r.Mobile, r.Landline, r.Email, r.State)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	content := types.PresenceJSON{
		UserName:  r.UserName,
		JobNumber: r.JobNumber,
		Mobile:    r.Mobile,
		Landline:  r.Landline,
		Email:     r.Email,
		State:     r.State,
		//Presence:        "online",
		CurrentlyActive: true,
		UserID:          userID,
		LastActiveAgo:   0,
	}

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)
	content.DisplayName = displayName
	content.AvatarURL = avatarURL

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

	// we use definition of profile to report user_info event
	data := new(types.ProfileStreamUpdate)
	data.UserID = userID
	data.Presence = content

	span1, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputProfileData.Name)
	defer span1.Finish()
	common.ExportMetricsBeforeSending(span1, cfg.Kafka.Producer.OutputProfileData.Name,
		cfg.Kafka.Producer.OutputProfileData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputProfileData.Underlying,
		cfg.Kafka.Producer.OutputProfileData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(userID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span1),
		})

	// Will be consumed by finstore.
	span2, _ := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.UserInfoUpdate.Name)
	defer span2.Finish()
	common.ExportMetricsBeforeSending(span2, cfg.Kafka.Producer.UserInfoUpdate.Name,
		cfg.Kafka.Producer.UserInfoUpdate.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.UserInfoUpdate.Underlying,
		cfg.Kafka.Producer.UserInfoUpdate.Name,
		&core.TransportPubMsg{
			Keys:    []byte(userID),
			Obj:     r,
			Headers: common.InjectSpanToHeaderForSending(span2),
		})

	return http.StatusOK, nil
}

// SetUserInfo implements PUT /user_info/{userID}
func SetUserInfo(
	ctx context.Context, r *external.PutUserInfoRequest,
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

	log.Infof("SetUserInfo %s, %v", userID, r)

	oldUserInfo := cache.GetUserInfoByUserID(userID)
	if oldUserInfo != nil {
		if oldUserInfo.UserName == r.UserName && oldUserInfo.JobNumber == r.JobNumber &&
			oldUserInfo.Mobile == r.Mobile && oldUserInfo.Landline == r.Landline &&
			oldUserInfo.Email == r.Email && oldUserInfo.State == r.State {
			return http.StatusOK, nil
		}
	}

	err := cache.SetUserInfo(userID, r.UserName, r.JobNumber, r.Mobile, r.Landline, r.Email, r.State)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	err = accountDB.UpsertUserInfo(ctx, userID, r.UserName, r.JobNumber, r.Mobile, r.Landline, r.Email, r.State)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	content := types.PresenceJSON{
		UserName:  r.UserName,
		JobNumber: r.JobNumber,
		Mobile:    r.Mobile,
		Landline:  r.Landline,
		Email:     r.Email,
		State:     r.State,
		//Presence:        "online",
		CurrentlyActive: true,
		UserID:          userID,
		LastActiveAgo:   0,
	}

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, userID)
	content.DisplayName = displayName
	content.AvatarURL = avatarURL

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

	// we use definition of profile to report user_info event
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

	/*
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
					AvatarURL:   avatar,
				}
				go BuildMembershipAndFireEvents(response.Rooms, newProfile, userID, cfg, rsRpcCli, idg)
			}
		}
	*/
	return http.StatusOK, nil
}

// DeleteUserInfo implements DELETE /user_info/{userID}
func DeleteUserInfo(
	ctx context.Context,
	req *external.DeleteUserInfoRequest,
	cfg config.Dendrite,
	federation *fed.Federation,
	accountDB model.AccountsDatabase,
	cache service.Cache,
) (int, core.Coder) {
	userID := req.UserID
	log.Infof("DeleteUserInfo %s", userID)

	err := cache.DeleteUserInfo(userID)
	if err != nil {
		log.Errorf("delete user_info cache failed, userID: %s, err: %v", userID, err)
		return httputil.LogThenErrorCtx(ctx, err)
	}
	err = accountDB.DeleteUserInfo(ctx, userID)
	if err != nil {
		log.Errorf("delte user_info db data failed, userID: %s, err: %v", userID, err)
		return httputil.LogThenErrorCtx(ctx, err)
	}

	/*
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
					AvatarURL:   avatar,
				}
				go BuildMembershipAndFireEvents(response.Rooms, newProfile, userID, cfg, rsRpcCli, idg)
			}
		}
	*/
	return http.StatusOK, nil
}

/*
func BuildMembershipAndFireEvents(
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
		err := rpcCli.QueryRoomState(context.TODO(), &queryReq, &queryRes)
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
			_, err = rpcCli.InputRoomEvents(context.Background(), &rawEvent)
			if err != nil {
				log.Errorf("BuildMembershipAndFireEvents fail on PostEvent for roomid:%s user:%s with err:%v", roomID, userID, err)
				continue
			}
		}
	}
}
*/

const MaxReqUserInfoUsers = 50

// GetUserInfoList implements GET /user_info_list
func GetUserInfoList(
	ctx context.Context,
	req *external.GetUserInfoListRequest,
	cfg config.Dendrite,
	federation *fed.Federation,
	accountDB model.AccountsDatabase,
	cache service.Cache,
) (int, core.Coder) {
	userIDs := req.UserIDs

	if len(userIDs) <= 0 {
		return http.StatusNotFound, &external.GetUserInfoListRequest{}
	} else if len(userIDs) > MaxReqUserInfoUsers {
		return http.StatusInternalServerError, jsonerror.NotFound("Error user amount")
	}

	resp := &external.GetUserInfoListResponse{}
	resp.UserInfoList = []external.UserInfoItem{}
	userInfoItem := external.UserInfoItem{}

	reqItem := external.GetUserInfoRequest{}
	for _, user := range userIDs {
		reqItem.UserID = user
		_, coder := GetUserInfo(ctx, &reqItem, cfg, federation, accountDB, cache)

		respItem := coder.(*external.GetUserInfoResponse)
		userInfoItem.UserID = user
		userInfoItem.UserName = respItem.UserName
		userInfoItem.JobNumber = respItem.JobNumber
		userInfoItem.Mobile = respItem.Mobile
		userInfoItem.Landline = respItem.Landline
		userInfoItem.Email = respItem.Email
		userInfoItem.State = respItem.State
		resp.UserInfoList = append(resp.UserInfoList, userInfoItem)
	}
	if len(resp.UserInfoList) == 0 {
		return http.StatusNotFound, resp
	}

	return http.StatusOK, resp
}
