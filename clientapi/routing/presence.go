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
	"github.com/finogeeks/ligase/model/authtypes"
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func getPresence(userID string, cache service.Cache, rpcCli *common.RpcClient) (presence, statusMsg, extStatusMsg string) {
	if presences, ok := cache.GetPresences(userID); ok && presences.UserID == userID {
		presence = presences.Status
		statusMsg = presences.StatusMsg
		extStatusMsg = presences.ExtStatusMsg
	}
	if presence == "" {
		req := types.OnlinePresence{UserID: userID}
		data, _ := json.Marshal(&req)
		respData, err := rpcCli.Request(types.PresenceTopicDef, data, 30000)
		if err != nil {
			return
		}
		var resp types.OnlinePresence
		err = json.Unmarshal(respData, &resp)
		if err != nil {
			return
		}

		if resp.Found {
			presence = resp.Presence
			statusMsg = resp.StatusMsg
			extStatusMsg = resp.ExtStatusMsg
		}
	}
	if presence == "" {
		presence = "offline"
	}
	return
}

func UpdatePresenceByID(
	ctx context.Context,
	reqContent *external.PutPresenceRequest,
	presenceDB model.PresenceDatabase,
	cfg *config.Dendrite,
	rpcCli *common.RpcClient,
	cache service.Cache,
	userID, deviceID string,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	if userID != reqContent.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("Can't set presence for others")
	}
	lastPresence := &authtypes.Presences{}
	presence, statusMsg, extStatusMsg := getPresence(userID, cache, rpcCli)
	lastPresence.Status = presence
	lastPresence.StatusMsg = statusMsg
	lastPresence.ExtStatusMsg = extStatusMsg
	log.Infof("get last presence userID:%s Presence:%s StatusMsg:%s ExtStatusMsg:%s", reqContent.UserID, lastPresence.Status, lastPresence.StatusMsg, lastPresence.ExtStatusMsg)

	if reqContent.Presence != nil {
		presence = *reqContent.Presence
	}
	if reqContent.StatusMsg != nil {
		statusMsg = *reqContent.StatusMsg
	}
	if reqContent.ExtStatusMsg != nil {
		extStatusMsg = *reqContent.ExtStatusMsg
	}

	err := presenceDB.UpsertPresences(ctx, reqContent.UserID, presence, statusMsg, extStatusMsg)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	cache.SetPresences(reqContent.UserID, presence, statusMsg, extStatusMsg)
	log.Infof("Set Presences success userID:%s Presence:%s StatusMsg:%s ExtStatusMsg:%s", reqContent.UserID, reqContent.Presence, reqContent.StatusMsg, reqContent.ExtStatusMsg)

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(ctx, reqContent.UserID)
	user_info := cache.GetUserInfoByUserID(reqContent.UserID)

	currentlyActive := false
	if presence == "online" {
		currentlyActive = true
	}
	content := types.PresenceJSON{
		Presence:        presence,
		StatusMsg:       statusMsg,
		ExtStatusMsg:    extStatusMsg,
		CurrentlyActive: currentlyActive,
		UserID:          reqContent.UserID,
		LastActiveAgo:   0,
	}

	content.AvatarURL = avatarURL
	content.DisplayName = displayName

	if user_info != nil {
		content.UserName = user_info.UserName
		content.JobNumber = user_info.JobNumber
		content.Mobile = user_info.Mobile
		content.Landline = user_info.Landline
		content.Email = user_info.Email
		content.State = user_info.State
	}
	content.LastPresence = lastPresence.Status
	content.LastStatusMsg = lastPresence.StatusMsg
	content.LastExtStatusMsg = lastPresence.ExtStatusMsg
	data := new(types.ProfileStreamUpdate)
	data.UserID = reqContent.UserID
	data.Presence = content
	data.DeviceID = deviceID
	data.IsUpdateBase = true
	span, ctx := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputProfileData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputProfileData.Name,
		cfg.Kafka.Producer.OutputProfileData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputProfileData.Underlying,
		cfg.Kafka.Producer.OutputProfileData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(reqContent.UserID),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}
func GetPresenceByID(
	rpcCli *common.RpcClient,
	cache service.Cache,
	federation *fed.Federation,
	cfg *config.Dendrite,
	userID string,
) (int, core.Coder) {
	status := ""
	statusMsg := ""
	extStatusMsg := ""
	if presences, ok := cache.GetPresences(userID); ok && presences.UserID == userID {
		status = presences.Status
		statusMsg = presences.StatusMsg
		extStatusMsg = presences.ExtStatusMsg
	}
	if status == "" {
		req := types.OnlinePresence{UserID: userID}
		data, _ := json.Marshal(&req)
		respData, err := rpcCli.Request(types.PresenceTopicDef, data, 30000)
		if err != nil {
			return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error." + err.Error())
		}
		var resp types.OnlinePresence
		err = json.Unmarshal(respData, &resp)
		if err != nil {
			return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error." + err.Error())
		}

		if resp.Found {
			status = resp.Presence
			statusMsg = resp.StatusMsg
			extStatusMsg = resp.ExtStatusMsg
		}
	}
	if status == "" {
		domain, _ := common.DomainFromID(userID)
		if !common.CheckValidDomain(domain, cfg.Matrix.ServerName) {
			profile, err := federation.LookupProfile(domain, userID)
			if err != nil {
				return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error." + err.Error())
			}
			status = profile.Status
			statusMsg = profile.StatusMsg
			extStatusMsg = profile.ExtStatusMsg
		}
	}
	if status == "" {
		status = "offline"
	}

	currentlyActive := false
	if status == "online" {
		currentlyActive = true
	}

	return http.StatusOK, &external.GetPresenceResponse{
		Presence:        status,
		CurrentlyActive: currentlyActive,
		StatusMsg:       statusMsg,
		ExtStatusMsg:    extStatusMsg,
	}
}
func UpdatePresenceListByID() (int, core.Coder) {
	return http.StatusOK, &external.PresenceJSON{}
}
func GetPresenceListByID() (int, core.Coder) {
	return http.StatusOK, &external.GetPresenceListResponse{}
}
