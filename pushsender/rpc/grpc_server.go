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
	"net/http"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Payload struct {
	ctx context.Context
	req *pushapitypes.PushPubContents
}

type Server struct {
	cfg        *config.Dendrite
	pushFilter *filter.Filter
	pushDB     model.PushAPIDatabase
	pushCount  *sync.Map
	idg        *uid.UidGenerator
	lock       *sync.Mutex
	httpClient *common.HttpClient
	grpcServer *grpc.Server

	chanSize uint32
	msgChan  []chan Payload
}

func NewServer(
	cfg *config.Dendrite,
	pushDB model.PushAPIDatabase,
) *Server {
	s := &Server{
		cfg:        cfg,
		pushDB:     pushDB,
		lock:       new(sync.Mutex),
		httpClient: common.NewHttpClient(),
		chanSize:   16,
	}
	s.pushCount = new(sync.Map)
	idg, _ := uid.NewDefaultIdGenerator(0)
	s.idg = idg
	pushFilter := filter.GetFilterMng().Register("pushSender", nil)
	s.pushFilter = pushFilter
	return s
}

func (s *Server) Start() error {
	if s.grpcServer != nil {
		return errors.New("push grpc server already start")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Rpc.Push.Port))
	if err != nil {
		return errors.New("push grpc server start err: " + err.Error())
	}
	s.grpcServer = grpc.NewServer()
	pb.RegisterPushServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Errorf("push grpc server Serve err: " + err.Error())
			panic(err)
		}
	}()

	s.msgChan = make([]chan Payload, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan Payload, 512)
		go s.startWorker(s.msgChan[i])
	}
	return nil
}

func (s *Server) PushData(ctx context.Context, req *pb.PushDataReq) (*pb.Empty, error) {
	traceId, _ := s.idg.Next()
	log.Infof("traceid:%d PushDataConsumer cb roomID:%s eventID:%s", traceId, req.Input.RoomID, req.Input.EventID)

	idx := common.CalcStringHashCode(req.Input.RoomID) % s.chanSize
	var contents []*pushapitypes.PushPubContent
	json.Unmarshal(req.Contents, &contents)
	s.msgChan[idx] <- Payload{ctx, &pushapitypes.PushPubContents{
		Input:             helper.ToClientEvent2(req.Input),
		SenderDisplayName: req.SenderDisplayName,
		RoomName:          req.RoomName,
		RoomAlias:         req.RoomAlias,
		Contents:          contents,
		CreateContent:     req.CreateContent,
		Slot:              idx,
		TraceId:           fmt.Sprintf("%d", traceId),
	}}

	return &pb.Empty{}, nil
}

func (s *Server) startWorker(msgChan chan Payload) {
	for payload := range msgChan {
		s.doPushData(payload.req)
	}
}

func (s *Server) doPushData(data *pushapitypes.PushPubContents) {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("traceid:%s doPushData slot:%d roomID:%s eventID:%s len(contents):%d spend:%d", data.TraceId, data.Slot, data.Input.RoomID, data.Input.EventID, len(data.Contents), spend)
	}(bs)
	if s.pushFilter.Lookup([]byte(data.Input.EventID)) {
		log.Infof("traceid:%s doPushData slot:%d lookup roomID:%s eventID:%s has exsit", data.TraceId, data.Slot, data.Input.RoomID, data.Input.EventID)
		return
	}
	s.pushFilter.Insert([]byte(data.Input.EventID))
	for _, content := range data.Contents {
		s.pushData(data, content)
	}
}

func (s *Server) SetPushFailTimes(pusher pushapitypes.PusherWitchInterfaceData, pusherKey string, success bool, traceId string) int {
	log.Infof("traceid:%s SetPushFailTimes userId:%s deviceId:%s pusherKey:%s success:%t", traceId, pusher.UserName, pusher.DeviceID, pusherKey, success)
	s.lock.Lock()
	defer s.lock.Unlock()
	if success {
		s.pushCount.Store(pusherKey, 0)
		return 0
	}
	if val, ok := s.pushCount.Load(pusherKey); ok {
		count := val.(int)
		count++
		s.pushCount.Store(pusherKey, count)
		return count
	} else {
		s.pushCount.Store(pusherKey, 1)
		return 1
	}
}

func (s *Server) pushData(
	data *pushapitypes.PushPubContents,
	pushContent *pushapitypes.PushPubContent,
) {
	if pushContent == nil || pushContent.Pushers == nil {
		return
	}
	for _, pusher := range pushContent.Pushers.Pushers {
		var url string
		pushChannel := "ios"
		var pusherData interface{}
		if v, ok := pusher.Data.(map[string]interface{}); ok {
			var data map[string]interface{}
			data = v
			if v, ok := data["url"]; ok {
				url = v.(string)
			}
			delete(data, "url")
			if v, ok := data["push_channel"]; ok {
				pushChannel = v.(string)
			}
			pusherData = data
		} else {
			continue
		}
		notify := s.createNotify(data, pushContent, pusher, pusherData)
		if notify == nil {
			continue
		}
		request, err := json.Marshal(notify)
		if err != nil {
			log.Errorw("traceid:%s process marshal error", log.KeysAndValues{"traceid", data.TraceId, "err", err})
			continue
		}
		if pushChannel == "ios" || pushChannel == "" {
			if s.cfg.PushService.PushServerUrl != "" {
				url = s.cfg.PushService.PushServerUrl
			}
		} else {
			if s.cfg.PushService.AndroidPushServerUrl != "" {
				url = s.cfg.PushService.AndroidPushServerUrl
			}
		}
		go s.doPush(url, request, pusher, data.TraceId)
	}
}

func (s *Server) createNotify(data *pushapitypes.PushPubContents, pushContent *pushapitypes.PushPubContent, pusher pushapitypes.PusherWitchInterfaceData, pusherData interface{}) *pushapitypes.Notify {
	userIsTarget := false
	if (data.Input.StateKey != nil) && (pushContent.UserID == *data.Input.StateKey) {
		userIsTarget = true
	}
	var content pushapitypes.PushContent
	err := json.Unmarshal(data.Input.Content, &content.Content)
	if err != nil {
		log.Errorf("traceid:%s PushData Unmarshal err: %d  Event: %s", err, string(data.Input.EventID))
		return nil
	}
	roomName := data.RoomName
	if roomName == "" {
		roomName = data.SenderDisplayName
	}
	notify := &pushapitypes.Notify{
		Notify: pushapitypes.Notification{
			EventId:           data.Input.EventID,
			RoomId:            data.Input.RoomID,
			Type:              data.Input.Type,
			Sender:            data.Input.Sender,
			SenderDisplayName: data.SenderDisplayName,
			RoomName:          roomName,
			RoomAlias:         data.RoomAlias,
			UserIsTarget:      userIsTarget,
			Priority:          "high",
			Content:           content.Content,
			Counts: pushapitypes.Counts{
				UnRead: pushContent.NotifyCount,
			},
			Devices: []pushapitypes.Device{
				{
					DeviceID:  pusher.DeviceID,
					UserName:  pusher.UserName,
					AppId:     pusher.AppId,
					PushKey:   pusher.PushKey,
					PushKeyTs: pusher.PushKeyTs,
					Data:      pusherData,
					Tweak: pushapitypes.Tweaks{
						Sound:     pushContent.Action.Sound,
						HighLight: pushContent.Action.HighLight,
					},
				},
			},
			CreateEvent: data.CreateContent,
		},
	}
	return notify
}

func (s *Server) doPush(url string, request []byte, pusher pushapitypes.PusherWitchInterfaceData, traceId string) {
	code, body, err := s.HttpRequest(url, request, traceId)
	if err != nil {
		log.Errorw("http request error", log.KeysAndValues{"traceid", traceId, "userId", pusher.UserName, "deviceId", pusher.DeviceID, "content", string(request), "error", err})
		return
	}
	pusherKey := fmt.Sprintf("%s:%s", pusher.AppId, pusher.PushKey)
	if code != http.StatusOK {
		log.Errorw("http request error", log.KeysAndValues{"traceid", traceId, "status_code", code, "response", string(body), "userId", pusher.UserName, "deviceId", pusher.DeviceID, "appId", pusher.AppId, "pushkey", pusher.PushKey, "content", string(request)})
		failCount := s.SetPushFailTimes(pusher, pusherKey, false, traceId)
		if failCount > s.cfg.PushService.RemoveFailTimes {
			log.Warnf("traceid:%s for failed too many del userId:%s, deviceId:%s, appId:%s, pushKey:%s, display:%s", traceId, pusher.UserName, pusher.DeviceID, pusher.AppId, pusher.PushKey, pusher.DeviceDisplayName)
			if err := s.pushDB.DeletePushersByKey(context.TODO(), pusher.AppId, pusher.PushKey); err != nil {
				log.Errorw("delete pusher error", log.KeysAndValues{"traceid", traceId, "err", err, "userId", pusher.UserName, "deviceId", pusher.DeviceID, "AppId", pusher.AppId, "PushKey", pusher.PushKey})
			}
		}
	} else {
		//用以追踪IOS重复推送问题
		log.Infof("traceid:%s push content success userId:%s, deviceId:%s appid:%s, pushkey:%s , content:%s", traceId, pusher.UserName, pusher.DeviceID, pusher.AppId, pusher.PushKey, string(request))
		s.SetPushFailTimes(pusher, pusherKey, true, traceId)
		var ack pushapitypes.PushAck
		err = json.Unmarshal(body, &ack)
		if len(ack.Rejected) > 0 {
			for _, v := range ack.Rejected {
				log.Warnf("traceid:%s for reject del userId:%s deviceId:%s pushKey:%s", traceId, pusher.UserName, pusher.DeviceID, v)
				if err := s.pushDB.DeletePushersByKeyOnly(context.TODO(), v); err != nil {
					log.Errorw("traceid:%s delete pusher userId:%s deviceId:%s pushKey:%s error", log.KeysAndValues{"traceid", traceId, "userId", pusher.UserName, "deviceId", pusher.DeviceID, "PushKey", pusher.PushKey, "err", err})
				}
			}
		}
	}
}

func (s *Server) HttpRequest(
	url string,
	content []byte,
	traceId string,
) (int, []byte, error) {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("traceid:%s post req to %s spend:%d", traceId, url, spend)
	}(bs)
	resp, err := s.httpClient.Post(url, content)
	return resp.StatusCode(), resp.Body(), err
}
