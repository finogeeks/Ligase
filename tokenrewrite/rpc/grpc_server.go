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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/tokenrewrite/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	cfg           *config.Dendrite
	idg           *uid.UidGenerator
	staffPersist  *storage.TokenRewriteDataBase
	retailPersist *storage.TokenRewriteDataBase
	grpcServer    *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
) *Server {
	idg, _ := uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)

	staffDB, err := storage.NewTokenRewriteDataBase(cfg.TokenRewrite.StaffDB)
	if err != nil {
		log.Panicf("create staffDB err %v", err)
	}

	retailDB, err := storage.NewTokenRewriteDataBase(cfg.TokenRewrite.RetailDB)
	if err != nil {
		log.Panicf("create retailDB err %v", err)
	}

	s := &Server{
		cfg:           cfg,
		idg:           idg,
		staffPersist:  staffDB,
		retailPersist: retailDB,
	}
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("syncserver grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.TokenWriter.Port))
	if err != nil {
		return errors.New("syncserver grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterTokenWriterServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("syncserver grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) UpdateToken(ctx context.Context, req *pb.UpdateTokenReq) (*pb.Empty, error) {
	log.Infof("start process for %s %s %s", req.UserID, req.DeviceID, req.Token)
	domain, _ := common.DomainFromID(req.UserID)
	id, _ := s.idg.Next()
	switch domain {
	case s.cfg.TokenRewrite.StaffDomain:
		err := s.staffPersist.UpsertDevice(context.TODO(), req.UserID, req.DeviceID, req.DisplayName)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", req.UserID, req.DeviceID, req.Token, err)
		}

		err = s.staffPersist.UpsertUser(context.TODO(), req.UserID)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", req.UserID, req.DeviceID, req.Token, err)
		}

		err = s.staffPersist.UpsertToken(context.TODO(), id, req.UserID, req.DeviceID, req.Token)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", req.UserID, req.DeviceID, req.Token, err)
		}
	case s.cfg.TokenRewrite.RetailDomain:
		err := s.staffPersist.UpsertDevice(context.TODO(), req.UserID, req.DeviceID, req.DisplayName)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", req.UserID, req.DeviceID, req.Token, err)
		}

		err = s.staffPersist.UpsertUser(context.TODO(), req.UserID)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", req.UserID, req.DeviceID, req.Token, err)
		}

		err = s.staffPersist.UpsertToken(context.TODO(), id, req.UserID, req.DeviceID, req.Token)
		if err != nil {
			log.Errorf("TokenRpcConsumer process for %s %s %s err %v", req.UserID, req.DeviceID, req.Token, err)
		}
	}
	return &pb.Empty{}, nil
}
