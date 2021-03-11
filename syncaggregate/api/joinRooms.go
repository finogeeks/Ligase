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
	"fmt"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetJoinedRooms{})
}

type ReqGetJoinedRooms struct{}

func (ReqGetJoinedRooms) GetRoute() string       { return "/joined_rooms" }
func (ReqGetJoinedRooms) GetMetricsName() string { return "joined_rooms" }
func (ReqGetJoinedRooms) GetMsgType() int32      { return internals.MSG_GET_JOIN_ROOMS }
func (ReqGetJoinedRooms) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetJoinedRooms) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetJoinedRooms) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetJoinedRooms) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetJoinedRooms) NewRequest() core.Coder {
	return nil
}
func (ReqGetJoinedRooms) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetJoinedRooms) NewResponse(code int) core.Coder {
	return new(syncapitypes.JoinedRoomsResp)
}
func (ReqGetJoinedRooms) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	userID := device.UserID

	resp := new(syncapitypes.JoinedRoomsResp)
	var err error
	resp.JoinedRooms, err = c.userTimeLine.GetJoinRoomsArr(userID)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.NotFound(fmt.Sprintf("Could not find user joined rooms %s", userID))
	}

	for _, roomId := range resp.JoinedRooms {
		log.Infof("OnIncomingJoinedRoomMessagesRequest user:%s load join room :%s", userID, roomId)
	}

	return http.StatusOK, resp
}
