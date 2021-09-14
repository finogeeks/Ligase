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
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/pushapi/routing"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncserver/consumers"
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
	cfg             *config.Dendrite
	syncServer      *consumers.SyncServer
	pushRepo        *repos.PushDataRepo
	receiptConsumer *consumers.ReceiptConsumer
	readCountRepo   *repos.ReadCountRepo
	roomCurState    *repos.RoomCurStateRepo
	rpcCli          rpc.RpcClient
	grpcServer      *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	syncServer *consumers.SyncServer,
	pushRepo *repos.PushDataRepo,
	receiptConsumer *consumers.ReceiptConsumer,
	readCountRepo *repos.ReadCountRepo,
	roomCurState *repos.RoomCurStateRepo,
	rpcCli rpc.RpcClient,
) *Server {
	return &Server{
		cfg:             cfg,
		syncServer:      syncServer,
		pushRepo:        pushRepo,
		receiptConsumer: receiptConsumer,
		readCountRepo:   readCountRepo,
		roomCurState:    roomCurState,
		rpcCli:          rpcCli,
	}
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("syncserver grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.SyncServer.Port))
	if err != nil {
		return errors.New("syncserver grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterSyncServerServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("syncserver grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) SyncLoad(ctx context.Context, req *pb.SyncProcessReq) (rsp *pb.SyncProcessRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SyncLoad panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	request := helper.ToSyncServerRequest(req)
	result, err := s.syncServer.SyncLoad(request)
	if err != nil {
		return nil, err
	}
	rsp = &pb.SyncProcessRsp{
		Ready: result.Ready,
	}
	return rsp, nil
}

func (s *Server) SyncProcess(ctx context.Context, req *pb.SyncProcessReq) (rsp *pb.SyncProcessRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SyncProcess panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	request := helper.ToSyncServerRequest(req)
	result, err := s.syncServer.SyncProcess(request)
	if err != nil {
		return nil, err
	}
	rsp = helper.ToSyncServerRsp(result)

	return rsp, nil
}

func (s *Server) GetPusherByDevice(ctx context.Context, req *pb.GetPusherByDeviceReq) (result *pb.Pushers, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetPusherByDevice panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	pusher := routing.GetPushersByName(req.UserID, s.pushRepo, false, nil)
	return helper.ToPBPushers(&pusher), nil
}

func (s *Server) GetPushRuleByUser(ctx context.Context, req *pb.GetPusherRuleByUserReq) (result *pb.Rules, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetPushRuleByUser panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	rules := routing.GetUserPushRules(req.UserID, s.pushRepo, true, nil)
	return helper.ToPBRules(&rules), nil
}

func (s *Server) GetPushDataBatch(ctx context.Context, req *pb.GetPushDataBatchReq) (result *pb.GetPushDataBatchRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetPushDataBatch panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	resp := pushapitypes.RespPushUsersData{
		Data: make(map[string]pushapitypes.RespPushData),
	}
	for _, user := range req.Users {
		r := pushapitypes.RespPushData{
			Pushers: routing.GetPushersByName(user, s.pushRepo, false, nil),
			Rules:   routing.GetUserPushRules(user, s.pushRepo, false, nil),
		}
		resp.Data[user] = r
	}
	return helper.ToGetPushDataBatchRsp(&resp), nil
}

func (s *Server) GetPusherBatch(ctx context.Context, req *pb.GetPusherBatchReq) (result *pb.GetPusherBatchRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetPusherBatch panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	resp := pushapitypes.RespUsersPusher{
		Data: make(map[string][]pushapitypes.Pusher),
	}
	for _, user := range req.Users {
		pushers, err := s.pushRepo.GetPusher(ctx, user)
		if err == nil {
			resp.Data[user] = pushers
		}
	}
	return helper.ToGetPusherBatchRsp(&resp), nil
}

func (s *Server) OnReceipt(ctx context.Context, req *pb.OnReceiptReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer OnReceipt panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	s.receiptConsumer.OnReceipt(&types.ReceiptContent{
		UserID:      req.DeviceID,
		DeviceID:    req.DeviceID,
		RoomID:      req.RoomID,
		ReceiptType: req.ReceiptType,
		EventID:     req.EventID,
		Source:      "rpc",
	})
	return &pb.Empty{}, nil
}

func (s *Server) OnTyping(ctx context.Context, req *pb.OnTypingReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer OnTyping panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	state := s.roomCurState.GetRoomState(req.RoomID)
	if state != nil {
		update := syncapitypes.TypingUpdate{
			Type:   req.Type,
			UserID: req.UserID,
			RoomID: req.RoomID,
		}
		domainMap := make(map[string]bool)
		state.GetJoinMap().Range(func(key, value interface{}) bool {
			update.RoomUsers = append(update.RoomUsers, key.(string))
			domain, _ := common.DomainFromID(key.(string))
			if !common.CheckValidDomain(domain, s.cfg.Matrix.ServerName) {
				domainMap[domain] = true
			}
			return true
		})

		ctx := context.Background()
		var err error
		if req.Type == "add" {
			err = s.rpcCli.AddTyping(ctx, &update)
		} else {
			err = s.rpcCli.RemoveTyping(ctx, &update)
		}
		if err != nil {
			log.Errorf("TypingRpcConsumer pub typing update error %v", err)
		}

		senderDomain, _ := common.DomainFromID(req.UserID)
		if common.CheckValidDomain(senderDomain, s.cfg.Matrix.ServerName) {
			content, _ := json.Marshal(req)
			for domain := range domainMap {
				edu := gomatrixserverlib.EDU{
					Type:        "typing",
					Origin:      senderDomain,
					Destination: domain,
					Content:     content,
				}
				err := s.rpcCli.SendEduToRemote(ctx, &edu)
				if err != nil {
					log.Errorf("TypingRpcConsumer pub typing edu error %v", err)
				}
			}
		}
	}
	return &pb.Empty{}, nil
}

func (s *Server) OnUnread(ctx context.Context, req *pb.OnUnreadReq) (result *pb.OnUnreadRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer OnUnread panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	count := int64(0)
	for _, roomID := range req.JoinRooms {
		unread, _ := s.readCountRepo.GetRoomReadCount(roomID, req.UserID)
		count = count + unread
	}
	return &pb.OnUnreadRsp{Count: count}, nil
}
