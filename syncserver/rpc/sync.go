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
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/syncserver/consumers"
	"github.com/nats-io/nats.go"
)

type SyncServerRpcConsumer struct {
	roomHistory *repos.RoomHistoryTimeLineRepo
	rpcClient   *common.RpcClient
	chanSize    uint32
	msgChan     []chan *syncapitypes.SyncServerRequest
	syncServer  *consumers.SyncServer
	cfg         *config.Dendrite
}

func NewSyncServerRpcConsumer(
	rpcClient *common.RpcClient,
	syncServer *consumers.SyncServer,
	cfg *config.Dendrite,
) *SyncServerRpcConsumer {
	s := &SyncServerRpcConsumer{
		rpcClient:  rpcClient,
		syncServer: syncServer,
		chanSize:   16,
		cfg:        cfg,
	}

	return s
}

func (s *SyncServerRpcConsumer) SetRoomHistory(roomHistory *repos.RoomHistoryTimeLineRepo) *SyncServerRpcConsumer {
	s.roomHistory = roomHistory
	return s
}

func (s *SyncServerRpcConsumer) GetCB() nats.MsgHandler {
	return s.cb
}

func (s *SyncServerRpcConsumer) GetTopic() string {
	return types.SyncServerTopicDef
}

func (s *SyncServerRpcConsumer) Clean() {
}

func (s *SyncServerRpcConsumer) cb(msg *nats.Msg) {
	var result syncapitypes.SyncServerRequest
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc sync cb error %v", err)
		return
	}
	if common.IsRelatedSyncRequest(result.SyncInstance, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, s.cfg.MultiInstance.MultiWrite) {
		log.Infof("traceid:%s is related sync req instance:%d,server instance:%d,server total:%d userid:%s deviceid:%s", result.TraceID, result.SyncInstance, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, result.UserID, result.DeviceID)
		result.Reply = msg.Reply
		idx := common.CalcStringHashCode(result.UserID) % s.chanSize
		result.RSlot = idx
		s.msgChan[idx] <- &result
	} else {
		log.Infof("traceid:%s not related sync req instance:%d,server instance:%d,server total:%d userid:%s deviceid:%s", result.TraceID, result.SyncInstance, s.cfg.MultiInstance.Instance, s.cfg.MultiInstance.Total, result.UserID, result.DeviceID)
	}
}

func (s *SyncServerRpcConsumer) startWorker(msgChan chan *syncapitypes.SyncServerRequest) {
	for data := range msgChan {
		go s.syncServer.OnSyncRequest(data)
	}
}

func (s *SyncServerRpcConsumer) Start() error {
	s.msgChan = make([]chan *syncapitypes.SyncServerRequest, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan *syncapitypes.SyncServerRequest, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.Reply(s.GetTopic(), s.cb)

	return nil
}
