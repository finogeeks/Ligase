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
	"strconv"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncaggregate/consumers"
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
	cfg            *config.Dendrite
	keyChangeRepo  *repos.KeyChangeStreamRepo
	keyFilter      *filter.Filter
	presenceRepo   *repos.PresenceDataStreamRepo
	userTimeLine   *repos.UserTimeLineRepo
	typingConsumer *consumers.TypingConsumer
	grpcServer     *grpc.Server
}

func NewServer(
	cfg *config.Dendrite,
	keyChangeRepo *repos.KeyChangeStreamRepo,
	presenceRepo *repos.PresenceDataStreamRepo,
	userTimeLine *repos.UserTimeLineRepo,
	typingConsumer *consumers.TypingConsumer,
) *Server {
	s := &Server{
		cfg:            cfg,
		keyChangeRepo:  keyChangeRepo,
		presenceRepo:   presenceRepo,
		userTimeLine:   userTimeLine,
		typingConsumer: typingConsumer,
	}
	keyFilter := filter.GetFilterMng().Register("keySender", nil)
	s.keyFilter = keyFilter
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("syncaggregate grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.SyncAggregate.Port))
	if err != nil {
		return errors.New("syncaggregate grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterSyncAggregateServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("syncaggregate grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()
	return nil
}

func (s *Server) UpdateOneTimeKey(ctx context.Context, req *pb.UpdateOneTimeKeyReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer UpdateOneTimeKey panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	err = s.keyChangeRepo.UpdateOneTimeKeyCount(req.UserID, req.DeviceID)
	if err != nil {
		log.Errorf("update onetimekey err: %s", err)
	}
	return &pb.Empty{}, nil
}

func (s *Server) UpdateDeviceKey(ctx context.Context, req *pb.UpdateDeviceKeyReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer UpdateDeviceKey panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	log.Infof("============ UpdateDeviceKey %s", req.String())
	if req.EventNID > 0 {
		if s.keyFilter.Lookup([]byte(strconv.FormatInt(req.EventNID, 10))) {
			return &pb.Empty{}, nil
		}
		s.keyFilter.Insert([]byte(strconv.FormatInt(req.EventNID, 10)))
	}
	for _, changed := range req.DeviceKeyChanges {
		keyStream := types.KeyChangeStream{
			ChangedUserID: changed.ChangedUserID,
		}
		s.keyChangeRepo.AddKeyChangeStream(&keyStream, changed.Offset, true)
	}
	return &pb.Empty{}, nil
}

func (s *Server) GetOnlinePresence(ctx context.Context, req *pb.GetOnlinePresenceReq) (result *pb.GetOnlinePresenceRsp, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer GetOnlinePresence panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	rsp := &pb.GetOnlinePresenceRsp{}
	feed := s.presenceRepo.GetHistoryByUserID(req.UserID)
	if feed == nil {
		rsp.Found = false
		rsp.Presence = "offline"
	} else {
		rsp.Found = true
		var presenceEvent gomatrixserverlib.ClientEvent
		var presenceContent types.PresenceJSON
		json.Unmarshal(feed.DataStream.Content, &presenceEvent)
		json.Unmarshal([]byte(presenceEvent.Content), &presenceContent)
		rsp.Presence = presenceContent.Presence
		rsp.StatusMsg = presenceContent.StatusMsg
		rsp.ExtStatusMsg = presenceContent.ExtStatusMsg
	}
	return rsp, nil
}

func (s *Server) SetReceiptLatest(ctx context.Context, req *pb.SetReceiptLatestReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer SetReceiptLatest panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	for _, userID := range req.Users {
		if !common.IsRelatedRequest(userID, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
			continue
		}
		log.Infof("process update receipt user:%s offset:%d", userID, req.Offset)
		s.userTimeLine.SetReceiptLatest(userID, req.Offset)
	}
	return &pb.Empty{}, nil
}

func (s *Server) AddTyping(ctx context.Context, req *pb.UpdateTypingReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer AddTyping panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	s.typingConsumer.AddRoomJoined(req.RoomID, req.RoomUsers)
	for _, user := range req.RoomUsers {
		if !common.IsRelatedRequest(user, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
			continue
		}
		s.typingConsumer.AddTyping(req.RoomID, req.UserID)
	}
	return &pb.Empty{}, nil
}

func (s *Server) RemoveTyping(ctx context.Context, req *pb.UpdateTypingReq) (result *pb.Empty, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpcServer RemoveTyping panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	s.typingConsumer.AddRoomJoined(req.RoomID, req.RoomUsers)
	for _, user := range req.RoomUsers {
		if !common.IsRelatedRequest(user, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, false) {
			continue
		}
		s.typingConsumer.RemoveTyping(req.RoomID, req.UserID)
	}
	return &pb.Empty{}, nil
}
