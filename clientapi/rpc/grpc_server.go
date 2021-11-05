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
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func toErr(e interface{}) error {
	if v, ok := e.(error); ok {
		return v
	} else {
		return fmt.Errorf("%#v", e)
	}
}

type Server struct {
	cfg          *config.Dendrite
	idg          *uid.UidGenerator
	accountDB    model.AccountsDatabase
	presenceDB   model.PresenceDatabase
	cache        service.Cache
	complexCache *common.ComplexCache
	rsRpcCli     roomserverapi.RoomserverRPCAPI
	grpcServer   *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	idg *uid.UidGenerator,
	accountDB model.AccountsDatabase,
	presenceDB model.PresenceDatabase,
	cache service.Cache,
	complexCache *common.ComplexCache,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
) *Server {
	s := &Server{
		cfg:          cfg,
		idg:          idg,
		accountDB:    accountDB,
		presenceDB:   presenceDB,
		cache:        cache,
		complexCache: complexCache,
		rsRpcCli:     rsRpcCli,
	}
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("front grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.Front.Port))
	if err != nil {
		return errors.New("front grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterClientapiServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("front grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) UpdateProfile(ctx context.Context, req *pb.UpdateProfileReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer UpdateProfile panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	upDisplayName := true
	upAvatarUrl := true
	upPresence := true
	upUserInfo := true
	oldDisplayName, oldAvatarURL, _ := s.complexCache.GetProfileByUserID(req.UserID)
	if oldDisplayName == req.DisplayName {
		upDisplayName = false
	}
	if oldAvatarURL == req.AvatarUrl {
		upAvatarUrl = false
	}

	oldUserInfo := s.cache.GetUserInfoByUserID(req.UserID)
	if oldUserInfo != nil {
		if oldUserInfo.UserName == req.UserName && oldUserInfo.JobNumber == req.JobNumber &&
			oldUserInfo.Mobile == req.Mobile && oldUserInfo.Landline == req.Landline &&
			oldUserInfo.Email == req.Email && oldUserInfo.State == int(req.State) {
			upUserInfo = false
		}
	}

	if req.Presence != "" {
		oldPresence, ok := s.cache.GetPresences(req.UserID)
		if ok && oldPresence.Status == req.Presence && oldPresence.StatusMsg == req.StatusMsg {
			upPresence = false
		}
	}

	if upDisplayName || upAvatarUrl || upPresence || upUserInfo {
		if upDisplayName {
			s.cache.SetDisplayName(req.UserID, req.DisplayName)
			s.accountDB.UpsertDisplayName(context.TODO(), req.UserID, req.DisplayName)
		}
		if upAvatarUrl {
			s.cache.SetAvatar(req.UserID, req.AvatarUrl)
			s.accountDB.UpsertAvatar(context.TODO(), req.UserID, req.AvatarUrl)
		}
		if upUserInfo {
			s.cache.SetUserInfo(req.UserID, req.UserName, req.JobNumber, req.Mobile, req.Landline, req.Email, int(req.State))
			s.accountDB.UpsertUserInfo(context.TODO(), req.UserID, req.UserName, req.JobNumber, req.Mobile, req.Landline, req.Email, int(req.State))
		}
		if upPresence {
			s.cache.SetPresences(req.UserID, req.Presence, req.StatusMsg, req.ExtStatusMsg)
			s.presenceDB.UpsertPresences(context.TODO(), req.UserID, req.Presence, req.StatusMsg, req.ExtStatusMsg)
		}

		content := types.PresenceJSON{
			AvatarURL:       req.AvatarUrl,
			DisplayName:     req.DisplayName,
			Presence:        req.Presence,
			StatusMsg:       req.StatusMsg,
			ExtStatusMsg:    req.ExtStatusMsg,
			CurrentlyActive: true,
			UserID:          req.UserID,
			LastActiveAgo:   0,
			UserName:        req.UserName,
			JobNumber:       req.JobNumber,
			Mobile:          req.Mobile,
			Landline:        req.Landline,
			Email:           req.Email,
			State:           int(req.State),
		}

		if !upPresence {
			presences, ok := s.cache.GetPresences(req.UserID)
			if ok && presences.UserID != "" {
				content.StatusMsg = presences.StatusMsg
				content.Presence = presences.Status
				content.CurrentlyActive = presences.Status == "online"
				content.ExtStatusMsg = presences.ExtStatusMsg
			}
		}

		data := new(types.ProfileStreamUpdate)
		data.UserID = req.UserID
		data.Presence = content
		data.Ts = time.Now().UnixNano() / 1000000
		common.GetTransportMultiplexer().SendWithRetry(
			s.cfg.Kafka.Producer.OutputProfileData.Underlying,
			s.cfg.Kafka.Producer.OutputProfileData.Name,
			&core.TransportPubMsg{
				Keys: []byte(req.UserID),
				Obj:  data,
			})

		if s.cfg.SendMemberEvent {
			var request roomserverapi.QueryJoinRoomsRequest
			request.UserID = req.UserID
			var response roomserverapi.QueryJoinRoomsResponse
			err := s.rsRpcCli.QueryJoinRooms(context.TODO(), &request, &response)
			if err != nil {
				if err != sql.ErrNoRows {
					log.Errorf("ProfileRpcConsumer processProfile err %v", err)
				}
			} else {
				newProfile := authtypes.Profile{
					DisplayName: req.DisplayName,
					AvatarURL:   req.AvatarUrl,
				}
				go routing.BuildMembershipAndFireEvents(response.Rooms, newProfile, req.UserID, s.cfg, s.rsRpcCli, s.idg)
			}
		}
	}
	return &pb.Empty{}, nil
}
