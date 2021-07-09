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
	"github.com/finogeeks/ligase/rcsserver/processors"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Result struct {
	resp *pb.HandleEventByRcsRsp
	err  error
}

type Payload struct {
	ctx  context.Context
	req  *gomatrixserverlib.Event
	resp chan Result
}

type Server struct {
	cfg        *config.Dendrite
	proc       *processors.EventProcessor
	grpcServer *grpc.Server

	slot     uint32
	chanSize uint32
	msgChan  []chan Payload
}

func NewServer(
	cfg *config.Dendrite,
	proc *processors.EventProcessor,
) *Server {
	s := &Server{
		cfg:      cfg,
		proc:     proc,
		slot:     64,
		chanSize: 1024,
	}
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("syncserver grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.Rcs.Port))
	if err != nil {
		return errors.New("syncserver grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterRcsServerServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("syncserver grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()

	s.msgChan = make([]chan Payload, s.slot)
	for i := uint32(0); i < s.slot; i++ {
		s.msgChan[i] = make(chan Payload, s.chanSize)
		go s.startWorker(s.msgChan[i])
	}
	return nil
}

func (r *Server) HandleEventByRcs(ctx context.Context, req *pb.Event) (*pb.HandleEventByRcsRsp, error) {
	event := helper.ToEvent(req)

	ch := make(chan Result)
	payload := Payload{ctx, event, ch}

	idx := common.CalcStringHashCode(event.RoomID()) % r.slot
	r.msgChan[idx] <- payload

	result := <-ch
	return result.resp, result.err
}

func (r *Server) startWorker(msgChan chan Payload) {
	for payload := range msgChan {
		resp, err := r.process(payload.ctx, payload.req)
		payload.resp <- Result{resp, err}
	}
}

func (r *Server) process(ctx context.Context, event *gomatrixserverlib.Event) (resp *pb.HandleEventByRcsRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("rcsserver handleEvent panic %v", e)
		}
	}()
	ev, _ := json.Marshal(event)
	log.Infof("rcsserver=====================EventConsumer.handleEvent, RCS Server receive event: %s\n", string(ev))
	var evs []gomatrixserverlib.Event
	if event.Type() == gomatrixserverlib.MRoomCreate {
		evs, err = r.proc.HandleCreate(ctx, event)
	} else if event.Type() == gomatrixserverlib.MRoomMember {
		evs, err = r.proc.HandleMembership(ctx, event)
	} else {
		evs = append(evs, *event)
		err = nil
	}

	resp = &pb.HandleEventByRcsRsp{
		Succeed: true,
	}
	for _, v := range evs {
		resp.Events = append(resp.Events, helper.ToPBEvent(&v))
	}

	if err != nil {
		log.Errorf("Failed to handle event, event=%s, error: %v\n", string(ev), err)
		resp.Succeed = false
	}

	return resp, nil
}
