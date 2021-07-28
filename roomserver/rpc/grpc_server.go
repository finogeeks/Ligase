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
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/roomservertypes"
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
	input      roomserverapi.RoomserverInputAPI
	grpcServer *grpc.Server

	chanSize uint32
	msgChan  []chan Payload
}

func NewServer(
	cfg *config.Dendrite,
	Proc roomserverapi.RoomserverQueryAPI,
	aliasProc roomserverapi.RoomserverAliasAPI,
	input roomserverapi.RoomserverInputAPI,
) *Server {
	s := &Server{
		cfg:       cfg,
		Proc:      Proc,
		aliasProc: aliasProc,
		input:     input,
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
		Join:              make(map[string]*pb.Event, len(response.Join)),
		Leave:             make(map[string]*pb.Event, len(response.Leave)),
		Invite:            make(map[string]*pb.Event, len(response.Invite)),
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

type Result struct {
	resp *pb.InputRoomEventsRsp
	err  error
}

type Payload struct {
	ctx  context.Context
	req  *pb.InputRoomEventsReq
	resp chan Result
}

func (s *Server) InputRoomEvents(ctx context.Context, req *pb.InputRoomEventsReq) (*pb.InputRoomEventsRsp, error) {
	ch := make(chan Result)
	payload := Payload{ctx, req, ch}

	log.Infow("rpc received data from client api server", log.KeysAndValues{
		"roomid", req.RoomID, "crc64", common.CalcStringHashCode(req.RoomID),
		"len", len(req.BulkEvents.Events),
	})

	for _, event := range req.BulkEvents.Events {
		log.Infof("rpc room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID, event.EventID, event.DomainOffset, event.OriginServerTS, event.Depth)
	}

	idx := common.CalcStringHashCode(req.RoomID) % s.chanSize
	s.msgChan[idx] <- payload

	result := <-ch
	return result.resp, result.err
}

func (r *Server) startWorker(msgChan chan Payload) {
	for payload := range msgChan {
		resp, err := r.process(payload.ctx, payload.req)
		payload.resp <- Result{resp, err}
	}
}

func (r *Server) process(ctx context.Context, req *pb.InputRoomEventsReq) (resp *pb.InputRoomEventsRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("rcsserver handleEvent panic %v", e)
		}
	}()
	log.Infof("------------------------client-api processEvent start roomId:%s len:%d", req.RoomID, len(req.BulkEvents.Events))
	begin := time.Now()
	last := begin
	var respResult roomserverapi.InputRoomEventsResponse

	//log.Infof("------------------------client-api processEvent start ev:%v", input.BulkEvents.Events)
	for _, event := range req.BulkEvents.Events {
		log.Infof("processEvent input room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID, event.EventID, event.DomainOffset, event.OriginServerTS, event.Depth)
	}
	rawEvent := &roomserverapi.RawEvent{
		RoomID: req.RoomID,
		Kind:   int(req.Kind),
		Trust:  req.Trust,
		Query:  req.Query,
	}
	if req.TxnID != nil {
		rawEvent.TxnID = &roomservertypes.TransactionID{
			DeviceID:      req.TxnID.DeviceID,
			TransactionID: req.TxnID.TransactionID,
			IP:            req.TxnID.Ip,
		}
	}
	if req.BulkEvents != nil {
		rawEvent.BulkEvents.SvrName = req.BulkEvents.SvrName
		for _, v := range req.BulkEvents.Events {
			rawEvent.BulkEvents.Events = append(rawEvent.BulkEvents.Events, *helper.ToEvent(v))
		}
	}
	n, err := r.input.InputRoomEvents(context.TODO(), rawEvent)
	respResult.N = n
	if err != nil {
		return &pb.InputRoomEventsRsp{
			ErrCode: -1,
			ErrMsg:  err.Error(),
		}, err
	}

	log.Infof("------------------------client-api processEvent use %v", time.Now().Sub(last))
	last = time.Now()

	return &pb.InputRoomEventsRsp{
		ErrCode: 0,
		ErrMsg:  "",
		N:       int32(n),
	}, nil
}
