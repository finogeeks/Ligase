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

package api

import (
	"net/http"
	"sync"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetUserUnread{})
}

type ReqGetUserUnread struct{}

func (ReqGetUserUnread) GetRoute() string       { return "/unread" }
func (ReqGetUserUnread) GetMetricsName() string { return "unread" }
func (ReqGetUserUnread) GetMsgType() int32      { return internals.MSG_GET_UNREAD }
func (ReqGetUserUnread) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetUserUnread) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetUserUnread) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetUserUnread) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetUserUnread) NewRequest() core.Coder {
	return new(external.GetUserUnread)
}
func (ReqGetUserUnread) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetUserUnread)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetUserUnread) NewResponse(code int) core.Coder {
	return new(GetUserUnreadResponse)
}
func (ReqGetUserUnread) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	//req := msg.(*external.GetUserUnread)

	userID := device.UserID
	joinMap, err := c.userTimeLine.GetJoinRooms(userID)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
	}

	count := int64(0)
	countMap := new(sync.Map)
	requestMap := make(map[uint32]*syncapitypes.SyncUnreadRequest)
	if joinMap != nil {
		joinMap.Range(func(key, value interface{}) bool {
			instance := common.GetSyncInstance(key.(string), c.Cfg.MultiInstance.SyncServerTotal)
			var request *syncapitypes.SyncUnreadRequest
			if data, ok := requestMap[instance]; ok {
				request = data
			} else {
				request = &syncapitypes.SyncUnreadRequest{}
				requestMap[instance] = request
			}
			request.JoinRooms = append(request.JoinRooms, key.(string))
			request.UserID = userID
			return true
		})
	}

	var wg sync.WaitGroup
	for instance, syncReq := range requestMap {
		wg.Add(1)
		go func(
			instance uint32,
			syncReq *syncapitypes.SyncUnreadRequest,
			countMap *sync.Map,
		) {
			syncReq.SyncInstance = instance
			bytes, err := json.Marshal(*syncReq)
			if err == nil {
				data, err := c.RpcCli.Request(types.SyncUnreadTopicDef, bytes, 30000)
				if err == nil {
					var result syncapitypes.SyncUnreadResponse
					err = json.Unmarshal(data, &result)
					if err != nil {
						log.Errorf("sync unread response Unmarshal error %v", err)
					} else {
						countMap.Store(syncReq.SyncInstance, result.Count)
					}
				} else {
					log.Errorf("call rpc for syncServer unread user %s error %v", syncReq.UserID, err)
				}
			} else {
				log.Errorf("marshal call sync unread content error, user %s error %v", syncReq.UserID, err)
			}
			wg.Done()
		}(instance, syncReq, countMap)
	}
	wg.Wait()

	countMap.Range(func(key, value interface{}) bool {
		count = count + value.(int64)
		return true
	})

	return http.StatusOK, &GetUserUnreadResponse{count}
}

type GetUserUnreadResponse struct {
	Count int64 `json:"count"`
}

func (r *GetUserUnreadResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *GetUserUnreadResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}
