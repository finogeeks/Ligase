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
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
	fed "github.com/finogeeks/ligase/federation/fedreq"
)

func UpdatePresenceByID(
	ctx context.Context,
	reqContent *external.PutPresenceRequest,
	presenceDB model.PresenceDatabase,
	cfg *config.Dendrite,
	cache service.Cache,
	userID, deviceID string,
	complexCache *common.ComplexCache,
) (int, core.Coder) {
	if userID != reqContent.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("Can't set presence for others")
	}

	err := presenceDB.UpsertPresences(ctx, reqContent.UserID, reqContent.Presence, reqContent.StatusMsg, reqContent.ExtStatusMsg)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	cache.SetPresences(reqContent.UserID, reqContent.Presence, reqContent.StatusMsg, reqContent.ExtStatusMsg)
	log.Infof("Set Presences success %s %s %s %s", reqContent.UserID, reqContent.Presence, reqContent.StatusMsg, reqContent.ExtStatusMsg)

	displayName, avatarURL, _ := complexCache.GetProfileByUserID(reqContent.UserID)
	user_info := cache.GetUserInfoByUserID(reqContent.UserID)

	currentlyActive := false
	if reqContent.Presence == "online" {
		currentlyActive = true
	}
	content := types.PresenceJSON{
		Presence:        reqContent.Presence,
		StatusMsg:       reqContent.StatusMsg,
		ExtStatusMsg:    reqContent.ExtStatusMsg,
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
	}

	data := new(types.ProfileStreamUpdate)
	data.IsMasterHndle = true
	data.UserID = reqContent.UserID
	data.Presence = content
	data.IsUpdateStauts = true
	data.DeviceID = deviceID
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputProfileData.Underlying,
		cfg.Kafka.Producer.OutputProfileData.Name,
		&core.TransportPubMsg{
			Keys: []byte(reqContent.UserID),
			Obj:  data,
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
