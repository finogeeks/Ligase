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
	"errors"
	"fmt"
	"net"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	cfg        *config.Dendrite
	Proc       roomserverapi.RoomserverQueryAPI
	aliasProc  roomserverapi.RoomserverAliasAPI
	grpcServer *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	Proc roomserverapi.RoomserverQueryAPI,
	aliasProc roomserverapi.RoomserverAliasAPI,
) *Server {
	s := &Server{
		cfg:       cfg,
		Proc:      Proc,
		aliasProc: aliasProc,
	}
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("roomserver grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.RoomServer.Port))
	if err != nil {
		return errors.New("roomserver grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterRoomServerServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("roomserver grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) QueryEventsByID(ctx context.Context, req *pb.QueryEventsByIDReq) (*pb.QueryEventsByIDRsp, error) {
	var response roomserverapi.QueryEventsByIDResponse
	err := s.Proc.QueryEventsByID(context.Background(), &roomserverapi.QueryEventsByIDRequest{EventIDs: req.EventIDs}, &response)
	if err != nil {
		return nil, err
	}
	result := &pb.QueryEventsByIDRsp{EventIDs: response.EventIDs}
	for _, v := range response.Events {
		result.Events = append(result.Events, helper.ToPBEvent(v))
	}
	return result, nil
}

func (s *Server) QueryRoomEventByID(ctx context.Context, req *pb.QueryRoomEventByIDReq) (*pb.QueryRoomEventByIDRsp, error) {
	var response roomserverapi.QueryRoomEventByIDResponse
	err := s.Proc.QueryRoomEventByID(context.Background(), &roomserverapi.QueryRoomEventByIDRequest{EventID: req.EventID, RoomID: req.RoomID}, &response)
	if err != nil {
		return nil, err
	}
	return &pb.QueryRoomEventByIDRsp{EventID: response.EventID, RoomID: response.RoomID, Event: helper.ToPBEvent(response.Event)}, nil
}

func (s *Server) QueryJoinRooms(ctx context.Context, req *pb.QueryJoinRoomsReq) (*pb.QueryJoinRoomsRsp, error) {
	var response roomserverapi.QueryJoinRoomsResponse
	err := s.Proc.QueryJoinRooms(context.Background(), &roomserverapi.QueryJoinRoomsRequest{UserID: req.UserID}, &response)
	if err != nil {
		return nil, err
	}
	return &pb.QueryJoinRoomsRsp{UserID: response.UserID, Rooms: response.Rooms}, nil
}

func (s *Server) QueryRoomState(ctx context.Context, req *pb.QueryRoomStateReq) (*pb.QueryRoomStateRsp, error) {
	var response roomserverapi.QueryRoomStateResponse
	log.Infof("processQueryRoomState recv request recv data:%s", req.RoomID)
	err := s.Proc.QueryRoomState(context.Background(), &roomserverapi.QueryRoomStateRequest{RoomID: req.RoomID}, &response)
	if err != nil {
		return nil, err
	}

	log.Infof("processQueryRoomState recv process room:%s, data:%v", req.RoomID, response)

	result := &pb.QueryRoomStateRsp{
		RoomID:            response.RoomID,
		RoomExists:        response.RoomExists,
		Creator:           helper.ToPBEvent(response.Creator),
		JoinRule:          helper.ToPBEvent(response.JoinRule),
		HistoryVisibility: helper.ToPBEvent(response.HistoryVisibility),
		Visibility:        helper.ToPBEvent(response.Visibility),
		Name:              helper.ToPBEvent(response.Name),
		Topic:             helper.ToPBEvent(response.Topic),
		Desc:              helper.ToPBEvent(response.Desc),
		CanonicalAlias:    helper.ToPBEvent(response.CanonicalAlias),
		Power:             helper.ToPBEvent(response.Power),
		Alias:             helper.ToPBEvent(response.Alias),
		Avatar:            helper.ToPBEvent(response.Avatar),
		GuestAccess:       helper.ToPBEvent(response.GuestAccess),
	}
	for k, v := range response.Join {
		result.Join[k] = helper.ToPBEvent(v)
	}
	for k, v := range response.Leave {
		result.Leave[k] = helper.ToPBEvent(v)
	}
	for k, v := range response.Invite {
		result.Invite[k] = helper.ToPBEvent(v)
	}
	return result, nil
}

func (s *Server) QueryBackFillEvents(ctx context.Context, req *pb.QueryBackFillEventsReq) (*pb.QueryBackFillEventsRsp, error) {
	var response roomserverapi.QueryBackFillEventsResponse
	err := s.Proc.QueryBackFillEvents(context.Background(), &roomserverapi.QueryBackFillEventsRequest{
		EventID: req.EventID,
		Limit:   int(req.Limit),
		RoomID:  req.RoomID,
		Dir:     req.Dir,
		Domain:  req.Domain,
		Origin:  req.Origin,
	}, &response)
	if err != nil {
		return nil, err
	}

	result := &pb.QueryBackFillEventsRsp{
		Error:          response.Error,
		Origin:         response.Origin,
		OriginServerTs: int64(response.OriginServerTs),
	}
	for _, v := range response.PDUs {
		result.Pdus = append(result.Pdus, helper.ToPBEvent(&v))
	}
	return result, nil
}

func (s *Server) QueryEventAuth(ctx context.Context, req *pb.QueryEventAuthReq) (*pb.QueryEventAuthRsp, error) {
	var response roomserverapi.QueryEventAuthResponse
	s.Proc.QueryEventAuth(context.Background(), &roomserverapi.QueryEventAuthRequest{EventID: req.EventID}, &response)

	result := &pb.QueryEventAuthRsp{}
	for _, v := range response.AuthEvents {
		result.AuthEvents = append(result.AuthEvents, helper.ToPBEvent(v))
	}
	return result, nil
}

func (s *Server) SetRoomAlias(ctx context.Context, req *pb.SetRoomAliasReq) (*pb.SetRoomAliasRsp, error) {
	var resp roomserverapi.SetRoomAliasResponse
	s.aliasProc.SetRoomAlias(ctx, &roomserverapi.SetRoomAliasRequest{
		UserID: req.UserID,
		Alias:  req.Alias,
		RoomID: req.RoomID,
	}, &resp)
	return &pb.SetRoomAliasRsp{AliasExists: resp.AliasExists}, nil
}

func (s *Server) GetAliasRoomID(ctx context.Context, req *pb.GetAliasRoomIDReq) (*pb.GetAliasRoomIDRsp, error) {
	var resp roomserverapi.GetAliasRoomIDResponse
	s.aliasProc.GetAliasRoomID(ctx, &roomserverapi.GetAliasRoomIDRequest{
		Alias: req.Alias,
	}, &resp)
	return &pb.GetAliasRoomIDRsp{RoomID: resp.RoomID}, nil
}

func (s *Server) RemoveRoomAlias(ctx context.Context, req *pb.RemoveRoomAliasReq) (*pb.RemoveRoomAliasRsp, error) {
	var resp roomserverapi.RemoveRoomAliasResponse
	s.aliasProc.RemoveRoomAlias(ctx, &roomserverapi.RemoveRoomAliasRequest{
		UserID: req.Alias,
		Alias:  req.Alias,
	}, &resp)
	return &pb.RemoveRoomAliasRsp{}, nil
}

func (s *Server) AllocRoomAlias(ctx context.Context, req *pb.AllocRoomAliasReq) (*pb.AllocRoomAliasRsp, error) {
	var resp roomserverapi.SetRoomAliasResponse
	s.aliasProc.AllocRoomAlias(ctx, &roomserverapi.SetRoomAliasRequest{
		UserID: req.Alias,
		Alias:  req.Alias,
	}, &resp)
	return &pb.AllocRoomAliasRsp{AliasExists: resp.AliasExists}, nil
}
