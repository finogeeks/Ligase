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
	"context"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
	"net/http"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqPostReportDeviceState{})
}

type ReqPostReportDeviceState struct{}

func (ReqPostReportDeviceState) GetRoute() string       { return "/report/device/state" }
func (ReqPostReportDeviceState) GetMetricsName() string { return "report device state" }
func (ReqPostReportDeviceState) GetMsgType() int32      { return internals.MSG_POST_REPORT_DEVICE_STATE }
func (ReqPostReportDeviceState) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostReportDeviceState) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostReportDeviceState) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostReportDeviceState) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostReportDeviceState) NewRequest() core.Coder {
	return new(external.PostReportDeviceState)
}
func (ReqPostReportDeviceState) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostReportDeviceState)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostReportDeviceState) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPostReportDeviceState) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.PostReportDeviceState)
	c.sm.GetOnlineRepo().UpdateState(device.UserID, device.ID, req.State)
	log.Infof("report device state:%d,user:%s,device:%s", req.State, device.UserID, device.ID)
	return http.StatusOK, nil
}
