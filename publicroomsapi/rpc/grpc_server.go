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
	"strconv"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	cfg        *config.Dendrite
	DB         model.PublicRoomAPIDatabase
	grpcServer *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	DB model.PublicRoomAPIDatabase,
) *Server {
	s := &Server{
		cfg: cfg,
		DB:  DB,
	}
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("publicroom grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.PublicRoom.Port))
	if err != nil {
		return errors.New("publicroom grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterPublicRoomServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("publicroom grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) QueryPublicRoomState(ctx context.Context, req *pb.QueryPublicRoomStateReq) (*pb.QueryPublicRoomStateRsp, error) {
	limit := req.Limit
	since := req.Since
	filter := req.Filter
	log.Infof("processQueryPublicRooms recv request recv data, limit:%d, since:%s, filter:%s", limit, since, filter)

	offset := int64(0)
	if len(since) > 0 {
		offset, _ = strconv.ParseInt(since, 10, 64)
	}

	var response pb.QueryPublicRoomStateRsp
	// defer func(response *publicroomsapi.QueryPublicRoomsResponse) {
	// 	s.rpcClient.PubObj(reply, response)
	// }(&response)

	count, err := s.DB.CountPublicRooms(ctx)
	if err != nil {
		log.Errorf("CountPublicRooms err %v", err)
		return nil, err
	}

	publicRooms, err := s.DB.GetPublicRooms(ctx, offset, limit, filter)
	if err != nil {
		log.Errorf("GetPublicRooms err %v", err)
		return nil, err
	}

	if offset > 0 {
		response.PrevBatch = strconv.Itoa(int(offset) - 1)
	}

	for _, v := range publicRooms {
		response.Chunk = append(response.Chunk, helper.ToPublicRoomState(&v))
	}
	response.NextBatch = strconv.FormatInt(offset+limit, 10)
	response.Estimate = count

	log.Infof("processQueryPublicRooms recv process data:%v", response)
	return &response, nil
}
