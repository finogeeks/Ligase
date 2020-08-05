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

package rpc

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/nats-io/go-nats"
)

type ProfileRpcConsumer struct {
	rpcClient *common.RpcClient
	rsRpcCli  roomserverapi.RoomserverRPCAPI
	chanSize  uint32
	//msgChan      []chan *types.ProfileContent
	msgChan      []chan common.ContextMsg
	cfg          *config.Dendrite
	idg          *uid.UidGenerator
	accountDB    model.AccountsDatabase
	presenceDB   model.PresenceDatabase
	cache        service.Cache
	complexCache *common.ComplexCache
	serverConfDB model.ConfigDatabase
}

func NewProfileRpcConsumer(
	rpcClient *common.RpcClient,
	cfg *config.Dendrite,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	idg *uid.UidGenerator,
	accountDB model.AccountsDatabase,
	presenceDB model.PresenceDatabase,
	cache service.Cache,
	complexCache *common.ComplexCache,
) *ProfileRpcConsumer {
	s := &ProfileRpcConsumer{
		rpcClient:    rpcClient,
		chanSize:     4,
		cfg:          cfg,
		rsRpcCli:     rsRpcCli,
		idg:          idg,
		accountDB:    accountDB,
		presenceDB:   presenceDB,
		cache:        cache,
		complexCache: complexCache,
	}

	return s
}

func (s *ProfileRpcConsumer) GetCB() common.MsgHandlerWithContext {
	return s.cb
}

func (s *ProfileRpcConsumer) GetTopic() string {
	return types.ProfileUpdateTopicDef
}

func (s *ProfileRpcConsumer) Clean() {
}

func (s *ProfileRpcConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result types.ProfileContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc profile cb error %v", err)
		return
	}

	idx := common.CalcStringHashCode(result.UserID) % s.chanSize
	ctxMsg := common.ContextMsg{Ctx: ctx, Msg: &result}
	s.msgChan[idx] <- ctxMsg
}

func (s *ProfileRpcConsumer) startWorker(msgChan chan common.ContextMsg) {
	for data := range msgChan {
		s.processProfile(data.Ctx, data.Msg.(*types.ProfileContent))
	}
}

func (s *ProfileRpcConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}
	s.rpcClient.ReplyGrpWithContext(s.GetTopic(), types.PROFILE_RPC_GROUP, s.cb)
	return nil
}

func (s *ProfileRpcConsumer) processProfile(ctx context.Context, profile *types.ProfileContent) {
	upDisplayName := true
	upAvatarUrl := true
	upPresence := true
	upUserInfo := true
	oldDisplayName, oldAvatarURL, _ := s.complexCache.GetProfileByUserID(ctx, profile.UserID)
	if oldDisplayName == profile.DisplayName {
		upDisplayName = false
	}
	if oldAvatarURL == profile.AvatarUrl {
		upAvatarUrl = false
	}

	oldUserInfo := s.cache.GetUserInfoByUserID(profile.UserID)
	if oldUserInfo != nil {
		if oldUserInfo.UserName == profile.UserName && oldUserInfo.JobNumber == profile.JobNumber &&
			oldUserInfo.Mobile == profile.Mobile && oldUserInfo.Landline == profile.Landline &&
			oldUserInfo.Email == profile.Email {
			upUserInfo = false
		}
	}

	if profile.Presence != "" {
		oldPresence, ok := s.cache.GetPresences(profile.UserID)
		if ok && oldPresence.Status == profile.Presence && oldPresence.StatusMsg == profile.StatusMsg {
			upPresence = false
		}
	}

	if upDisplayName || upAvatarUrl || upPresence || upUserInfo {
		if upDisplayName {
			s.cache.SetDisplayName(profile.UserID, profile.DisplayName)
			s.accountDB.UpsertDisplayName(ctx, profile.UserID, profile.DisplayName)
		}
		if upAvatarUrl {
			s.cache.SetAvatar(profile.UserID, profile.AvatarUrl)
			s.accountDB.UpsertAvatar(ctx, profile.UserID, profile.AvatarUrl)
		}
		if upUserInfo {
			s.cache.SetUserInfo(profile.UserID, profile.UserName, profile.JobNumber, profile.Mobile, profile.Landline, profile.Email, profile.State)
			s.accountDB.UpsertUserInfo(ctx, profile.UserID, profile.UserName, profile.JobNumber, profile.Mobile, profile.Landline, profile.Email, profile.State)
		}
		if upPresence {
			s.cache.SetPresences(profile.UserID, profile.Presence, profile.StatusMsg, profile.ExtStatusMsg)
			s.presenceDB.UpsertPresences(ctx, profile.UserID, profile.Presence, profile.StatusMsg, profile.ExtStatusMsg)
		}

		content := types.PresenceJSON{
			AvatarURL:       profile.AvatarUrl,
			DisplayName:     profile.DisplayName,
			Presence:        profile.Presence,
			StatusMsg:       profile.StatusMsg,
			ExtStatusMsg:    profile.ExtStatusMsg,
			CurrentlyActive: true,
			UserID:          profile.UserID,
			LastActiveAgo:   0,
			UserName:        profile.UserName,
			JobNumber:       profile.JobNumber,
			Mobile:          profile.Mobile,
			Landline:        profile.Landline,
			Email:           profile.Email,
			State:           profile.State,
		}

		if !upPresence {
			presences, ok := s.cache.GetPresences(profile.UserID)
			if ok && presences.UserID != "" {
				content.StatusMsg = presences.StatusMsg
				content.Presence = presences.Status
				content.CurrentlyActive = presences.Status == "online"
				content.ExtStatusMsg = presences.ExtStatusMsg
			}
		}

		data := new(types.ProfileStreamUpdate)
		data.UserID = profile.UserID
		data.Presence = content
		span, _ := common.StartSpanFromContext(ctx, s.cfg.Kafka.Producer.OutputProfileData.Name)
		defer span.Finish()
		common.ExportMetricsBeforeSending(span, s.cfg.Kafka.Producer.OutputProfileData.Name,
			s.cfg.Kafka.Producer.OutputProfileData.Underlying)
		common.GetTransportMultiplexer().SendWithRetry(
			s.cfg.Kafka.Producer.OutputProfileData.Underlying,
			s.cfg.Kafka.Producer.OutputProfileData.Name,
			&core.TransportPubMsg{
				Keys:    []byte(profile.UserID),
				Obj:     data,
				Headers: common.InjectSpanToHeaderForSending(span),
			})

		if s.cfg.SendMemberEvent {
			var request roomserverapi.QueryJoinRoomsRequest
			request.UserID = profile.UserID
			var response roomserverapi.QueryJoinRoomsResponse
			err := s.rsRpcCli.QueryJoinRooms(ctx, &request, &response)
			if err != nil {
				if err != sql.ErrNoRows {
					log.Errorf("ProfileRpcConsumer processProfile err %v", err)
				}
			} else {
				newProfile := authtypes.Profile{
					DisplayName: profile.DisplayName,
					AvatarURL:   profile.AvatarUrl,
				}
				go routing.BuildMembershipAndFireEvents(ctx, response.Rooms, newProfile, profile.UserID, s.cfg, s.rsRpcCli, s.idg)
			}
		}
	}
}
