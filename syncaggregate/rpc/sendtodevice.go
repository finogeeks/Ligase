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
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/nats-io/go-nats"
	"net/http"
)

type StdRpcConsumer struct {
	rpcClient        *common.RpcClient
	stdEventTimeline *repos.STDEventStreamRepo
	cache            service.Cache
	syncDB           model.SyncAPIDatabase
	chanSize         uint32
	msgChan          []chan *types.StdContent
	cfg              *config.Dendrite
}

func NewStdRpcConsumer(
	rpcClient *common.RpcClient,
	stdEventTimeline *repos.STDEventStreamRepo,
	cache service.Cache,
	syncDB model.SyncAPIDatabase,
	cfg *config.Dendrite,
) *StdRpcConsumer {
	s := &StdRpcConsumer{
		rpcClient:        rpcClient,
		stdEventTimeline: stdEventTimeline,
		cache:            cache,
		syncDB:           syncDB,
		chanSize:         8,
		cfg:              cfg,
	}

	return s
}

func (s *StdRpcConsumer) GetCB() nats.MsgHandler {
	return s.cb
}

func (s *StdRpcConsumer) GetTopic() string {
	return types.StdTopicDef
}

func (s *StdRpcConsumer) Clean() {
}

func (s *StdRpcConsumer) cb(msg *nats.Msg) {
	var result types.StdContent
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc send to device cb error %v", err)
		return
	}
	result.Reply = msg.Reply
	idx := common.CalcStringHashCode(result.Sender) % s.chanSize
	s.msgChan[idx] <- &result
}

func (s *StdRpcConsumer) startWorker(msgChan chan *types.StdContent) {
	for data := range msgChan {
		s.processStd(&data.StdRequest, data.Reply, data.Sender, data.EventType)
	}
}

func (s *StdRpcConsumer) Start() error {
	s.msgChan = make([]chan *types.StdContent, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *types.StdContent, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)

	return nil
}

func (s *StdRpcConsumer) processStd(stdRq *types.StdRequest, reply, sender, eventType string) {
	for uid, deviceMap := range stdRq.Sender {
		if common.IsRelatedRequest(uid, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
			// uid is local domain
			for deviceID, cont := range deviceMap {
				//log.Errorf("start process sendToDevice for user %s, device %s", uid, deviceID)
				dataStream := types.StdEvent{}
				dataStream.Type = eventType
				dataStream.Sender = sender
				dataStream.Content = cont

				// wildcard all devices
				if deviceID == "*" {
					deviceCollection := s.cache.GetDevicesByUserID(uid)
					for _, val := range *deviceCollection {
						if !common.IsActualDevice(val.DeviceType) {
							log.Infof("sendToDevice to all devices for user %s, but device %s is not actual", uid, val.ID)
							continue
						}

						s.stdEventTimeline.AddSTDEventStream(&dataStream, uid, val.ID)
					}
				} else {
					/*dev := cache.GetDeviceByDeviceID(deviceID, uid)
					if dev != nil && dev.UserID != "" {
						if !common.IsActualDevice(dev.DeviceType) {
							log.Errorf("sendToDevice to device for user %s, but device %s is not actual", uid, deviceID)
							continue
						}
					} else {
						//device not exists
						log.Errorf("sendToDevice device for user %s, but device %s is not exists", uid, deviceID)
						continue
					}*/

					//log.Errorf("add sendToDevice event pos %d sender %s type %s uid %s device %s", pos, device.UserID, eventType, uid, deviceID)
					s.stdEventTimeline.AddSTDEventStream(&dataStream, uid, deviceID)
				}
			}
		}
	}

	resp := util.JSONResponse{
		Code: http.StatusOK,
		JSON: struct{}{},
	}
	s.rpcClient.PubObj(reply, resp)
}
