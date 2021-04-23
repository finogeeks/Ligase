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
	"os"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetLRUInfo{})
}

type ReqGetLRUInfo struct{}

func (ReqGetLRUInfo) GetRoute() string       { return "/lru/rooms" }
func (ReqGetLRUInfo) GetMetricsName() string { return "lru_rooms" }
func (ReqGetLRUInfo) GetMsgType() int32      { return internals.MSG_GET_LRU_ROOMS }
func (ReqGetLRUInfo) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetLRUInfo) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetLRUInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetLRUInfo) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetLRUInfo) NewRequest() core.Coder {
	return new(external.GetLRURoomsRequest)
}
func (ReqGetLRUInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetLRURoomsRequest)
	msg.Timestamp = fmt.Sprintf("%d", time.Now().Second())
	return nil
}
func (ReqGetLRUInfo) NewResponse(code int) core.Coder {
	return new(external.GetLRURoomsRequest)
}
func (ReqGetLRUInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetLRURoomsRequest)
	if !common.IsRelatedRequest(req.Timestamp, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	loaded, max := c.rmHsTimeline.GetLoadedRoomNumber()

	return http.StatusOK, &external.GetLRURoomsResponse{
		Loaded: loaded,
		Max:    max,
		Server: fmt.Sprintf("%s%d", os.Getenv("SERVICE_NAME"), c.Cfg.Matrix.InstanceId),
	}
}
