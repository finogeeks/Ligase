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
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/fedsender"
	"github.com/finogeeks/ligase/federation/fedsync/syncconsumer"
	"github.com/finogeeks/ligase/federation/model/backfilltypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
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
	cfg        *config.Dendrite
	sender     *fedsender.FederationSender
	fedClient  *client.FedClientWrap
	feddomains *common.FedDomains
	backfill   backfilltypes.BackFillProcessor
	grpcServer *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	sender *fedsender.FederationSender,
	fedClient *client.FedClientWrap,
	feddomains *common.FedDomains,
	backfill backfilltypes.BackFillProcessor,
) *Server {
	return &Server{
		cfg:        cfg,
		sender:     sender,
		fedClient:  fedClient,
		feddomains: feddomains,
		backfill:   backfill,
	}
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("fed grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.Fed.Port))
	if err != nil {
		return errors.New("fed grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterFederationServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("fed grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) SendEDU(ctx context.Context, req *pb.SendEDUReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SendEDU err %#v", e)
			err = toErr(e)
		}
	}()
	log.Infof("fed-edu-sender received data data:%+v", req)
	s.sender.SendEDU(&gomatrixserverlib.EDU{
		Type:        req.Type,
		Origin:      req.Origin,
		Destination: req.Destination,
		Content:     req.Content,
	})
	return &pb.Empty{}, nil
}

func (s *Server) GetAliasRoomID(ctx context.Context, req *pb.GetFedAliasRoomIDReq) (result *pb.GetFddAliasRoomIDRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetAliasRoomID err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed GetAliasRoomID from domain: %s dest: %s", req.TargetDomain, destination)

	aliasReq := external.GetDirectoryRoomAliasRequest{RoomAlias: req.RoomAlias}
	resp := syncconsumer.GetAliasRoomID(s.fedClient, &aliasReq, destination)
	return &pb.GetFddAliasRoomIDRsp{
		RoomID:  resp.RoomID,
		Servers: resp.Servers,
	}, nil
}

func (s *Server) GetProfile(ctx context.Context, req *pb.GetProfileReq) (result *pb.GetProfileRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetProfile err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed GetProfile from domain: %s dest: %s", req.TargetDomain, destination)

	profileReq := external.GetProfileRequest{UserID: req.UserID}
	resp := syncconsumer.GetProfile(s.fedClient, &profileReq, destination)
	return &pb.GetProfileRsp{
		AvatarURL:    resp.AvatarURL,
		DisplayName:  resp.DisplayName,
		Status:       resp.Status,
		StatusMsg:    resp.StatusMsg,
		ExtStatusMsg: resp.ExtStatusMsg,
	}, nil
}

func (s *Server) GetAvatar(ctx context.Context, req *pb.GetAvatarReq) (result *pb.GetAvatarRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetAvatar err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed GetAvatar from domain: %s dest: %s", req.TargetDomain, destination)

	profileReq := external.GetProfileRequest{UserID: req.UserID}
	resp := syncconsumer.GetAvatar(s.fedClient, &profileReq, destination)
	return &pb.GetAvatarRsp{
		AvatarURL: resp.AvatarURL,
	}, nil
}

func (s *Server) GetDisplayName(ctx context.Context, req *pb.GetDisplayNameReq) (result *pb.GetDisplayNameRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetDisplayName err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed GetDisplayName from domain: %s dest: %s", req.TargetDomain, destination)

	profileReq := external.GetProfileRequest{UserID: req.UserID}
	resp := syncconsumer.GetDisplayName(s.fedClient, &profileReq, destination)
	return &pb.GetDisplayNameRsp{
		DisplayName: resp.DisplayName,
	}, nil
}

func (s *Server) GetRoomState(ctx context.Context, req *pb.GetRoomStateReq) (result *pb.GetRoomStateRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetRoomState err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed GetRoomState from domain: %s dest: %s", req.TargetDomain, destination)

	stateReq := external.GetFedRoomStateRequest{RoomID: req.RoomID, EventID: req.EventID}
	resp := syncconsumer.GetRoomState(s.fedClient, &stateReq, destination, s.backfill)
	stateResp := pb.GetRoomStateRsp{}
	for _, v := range resp.StateEvents {
		stateResp.StateEvents = append(stateResp.StateEvents, helper.ToPBEvent(&v))
	}
	for _, v := range resp.AuthEvents {
		stateResp.AuthEvents = append(stateResp.AuthEvents, helper.ToPBEvent(&v))
	}
	return &stateResp, nil
}

func (s *Server) Download(req *pb.DownloadReq, stream pb.Federation_DownloadServer) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer Download err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed Download from domain: %s dest: %s", req.TargetDomain, destination)

	downloadReq := external.GetFedDownloadRequest{
		ID:       req.Id,
		FileType: req.FileType,
		MediaID:  req.MediaID,
		Width:    req.FileType,
		Method:   req.Method,
	}
	ch := make(chan struct{}, 1)
	seq := int64(0)
	resp := syncconsumer.Download(s.fedClient, &downloadReq, destination, req.TargetDomain, func(reqID string, data []byte) error {
		seq++
		return stream.Send(&pb.DownloadRsp{Seq: seq, Data: data})
	}, func(reqID string) {
		ch <- struct{}{}
	})
	data, _ := json.Marshal(resp)
	stream.Send(&pb.DownloadRsp{Seq: 0, Data: data})
	<-ch
	return nil
}

func (s *Server) GetUserInfo(ctx context.Context, req *pb.GetUserInfoReq) (result *pb.GetUserInfoRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetUserInfo err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed GetUserInfo from domain: %s dest: %s", req.TargetDomain, destination)

	userInfoReq := external.GetUserInfoRequest{UserID: req.UserID}
	resp := syncconsumer.GetUserInfo(s.fedClient, &userInfoReq, destination)
	return &pb.GetUserInfoRsp{
		UserName:  resp.UserName,
		JobNumber: resp.JobNumber,
		Mobile:    resp.Mobile,
		Landline:  resp.Landline,
		Email:     resp.Email,
		State:     int32(resp.State),
	}, nil
}

func (s *Server) MakeJoin(ctx context.Context, req *pb.MakeJoinReq) (result *pb.MakeJoinRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer MakeJoin err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed MakeJoin from domain: %s dest: %s", req.TargetDomain, destination)

	joinReq := external.GetMakeJoinRequest{
		RoomID: req.RoomID,
		UserID: req.UserID,
		Ver:    req.Ver,
	}
	resp := syncconsumer.MakeJoin(s.fedClient, &joinReq, destination)

	return &pb.MakeJoinRsp{
		JoinEvent: helper.ToPBEventBuilder(&resp.JoinEvent),
	}, nil
}

func (s *Server) SendJoin(ctx context.Context, req *pb.SendJoinReq) (result *pb.SendJoinRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SendJoin err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed SendJoin from domain: %s dest: %s", req.TargetDomain, destination)

	joinReq := external.PutSendJoinRequest{
		RoomID:  req.RoomID,
		EventID: req.EventID,
		Event:   *helper.ToEvent(req.Event),
	}
	resp := syncconsumer.SendJoin(s.fedClient, &joinReq, destination, s.backfill)
	joinResp := pb.SendJoinRsp{}
	for _, v := range resp.StateEvents {
		joinResp.StateEvents = append(joinResp.StateEvents, helper.ToPBEvent(&v))
	}
	for _, v := range resp.AuthEvents {
		joinResp.AuthEvents = append(joinResp.AuthEvents, helper.ToPBEvent(&v))
	}
	return &joinResp, nil
}

func (s *Server) MakeLeave(ctx context.Context, req *pb.MakeLeaveReq) (result *pb.MakeLeaveRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer MakeLeave err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed MakeLeave from domain: %s dest: %s", req.TargetDomain, destination)

	leaveReq := external.GetMakeLeaveRequest{
		RoomID: req.RoomID,
		UserID: req.UserID,
	}
	resp := syncconsumer.MakeLeave(s.fedClient, &leaveReq, destination)
	return &pb.MakeLeaveRsp{
		Event: helper.ToPBEventBuilder(&resp.Event),
	}, nil
}

func (s *Server) SendLeave(ctx context.Context, req *pb.SendLeaveReq) (result *pb.SendLeaveRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SendLeave err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed SendLeave from domain: %s dest: %s", req.TargetDomain, destination)

	joinReq := external.PutSendLeaveRequest{
		RoomID:  req.RoomID,
		EventID: req.EventID,
		Event:   *helper.ToEvent(req.Event),
	}
	resp := syncconsumer.SendLeave(s.fedClient, &joinReq, destination, s.backfill)
	return &pb.SendLeaveRsp{Code: int32(resp.Code)}, nil
}

func (s *Server) SendInvite(ctx context.Context, req *pb.SendInviteReq) (result *pb.SendInviteRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SendInvite err %#v", e)
			err = toErr(e)
		}
	}()
	destination, ok := s.feddomains.GetDomainHost(req.TargetDomain)
	if !ok {
		log.Errorf("FedSync processRequest invalid destination %s", req.TargetDomain)
		return nil, errors.New("FedSync processRequest invalid destination " + req.TargetDomain)
	}
	log.Debugf("fed SendInvite from domain: %s dest: %s", req.TargetDomain, destination)

	event := helper.ToEvent(req.Event)
	resp := syncconsumer.SendInvite(s.fedClient, event, destination)
	return &pb.SendInviteRsp{
		Code:  int32(resp.Code),
		Event: helper.ToPBEvent(&resp.Event),
	}, nil
}
