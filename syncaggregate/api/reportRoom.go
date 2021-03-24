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
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqPostReportRoom{})
}

type ReqPostReportRoom struct{}

func (ReqPostReportRoom) GetRoute() string       { return "/report/user/room" }
func (ReqPostReportRoom) GetMetricsName() string { return "report user cur device roomId" }
func (ReqPostReportRoom) GetMsgType() int32      { return internals.MSG_POST_REPORT_ROOM }
func (ReqPostReportRoom) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostReportRoom) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostReportRoom) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostReportRoom) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostReportRoom) NewRequest() core.Coder {
	return new(external.PostReportRoomRequest)
}
func (ReqPostReportRoom) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostReportRoomRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostReportRoom) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPostReportRoom) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.PostReportRoomRequest)
	if req.RoomID != "" {
		isJoin, _ := c.userTimeLine.CheckIsJoinRoom(device.UserID, req.RoomID)
		if !isJoin {
			return http.StatusForbidden, jsonerror.BadJSON(fmt.Sprintf("user:%s is not in room:%s", device.UserID, req.RoomID))
		}
	} else {
		//compatibility leave room and previous version
		req.RoomID = "none"
	}
	c.userTimeLine.SetUserCurRoom(device.UserID, device.ID, req.RoomID)
	log.Infof("user:%s,device:%s,report roomID:%s", device.UserID, device.ID, req.RoomID)
	return http.StatusOK, nil
}
