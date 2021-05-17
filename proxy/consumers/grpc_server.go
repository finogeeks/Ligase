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

package consumers

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	cfg         *config.Dendrite
	tokenFilter *filter.SimpleFilter
	cache       service.Cache
	grpcServer  *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	tokenFilter *filter.SimpleFilter,
	cache service.Cache,
) *Server {

	s := &Server{
		cfg:         cfg,
		tokenFilter: tokenFilter,
		cache:       cache,
	}
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("syncserver grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.Proxy.Port))
	if err != nil {
		return errors.New("syncserver grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterProxyServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("syncserver grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) AddFilterToken(ctx context.Context, req *pb.AddFilterTokenReq) (*pb.Empty, error) {
	s.tokenFilter.Insert(req.UserID, req.DeviceID)
	return &pb.Empty{}, nil
}

func (s *Server) DelFilterToken(ctx context.Context, req *pb.DelFilterTokenReq) (*pb.Empty, error) {
	s.tokenFilter.Delete(req.UserID, req.DeviceID)
	return &pb.Empty{}, nil
}

func (s *Server) VerifyToken(ctx context.Context, req *pb.VerifyTokenReq) (*pb.VerifyTokenRsp, error) {
	device, resErr := common.VerifyToken(req.Token, req.RequestURI, s.cache, *s.cfg, s.tokenFilter)
	if resErr != nil {
		data, _ := json.Marshal(resErr.JSON)
		return &pb.VerifyTokenRsp{Error: string(data)}, nil
	}

	return &pb.VerifyTokenRsp{
		Device: &pb.Device{
			Id:           device.ID,
			UserID:       device.UserID,
			DisplayName:  device.DisplayName,
			DeviceType:   device.DeviceType,
			IsHuman:      device.IsHuman,
			Identifier:   device.Identifier,
			CreateTs:     device.CreateTs,
			LastActiveTs: device.LastActiveTs,
		},
	}, nil
}
