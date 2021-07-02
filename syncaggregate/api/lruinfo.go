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
	apiconsumer.SetAPIProcessor(ReqGetLRUSendToDevice{})
	apiconsumer.SetAPIProcessor(ReqPutLRUSendToDevice{})
}

type ReqGetLRUSendToDevice struct{}

func (ReqGetLRUSendToDevice) GetRoute() string       { return "/lru/syncaggregate/sendtodevice" }
func (ReqGetLRUSendToDevice) GetMetricsName() string { return "get_lru_syncaggregate_sendtodevice" }
func (ReqGetLRUSendToDevice) GetMsgType() int32 {
	return internals.MSG_GET_LRU_SYNC_AGGREGATE_SENDTODEVICE
}
func (ReqGetLRUSendToDevice) GetAPIType() int8 { return apiconsumer.APITypeAuth }
func (ReqGetLRUSendToDevice) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetLRUSendToDevice) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetLRUSendToDevice) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetLRUSendToDevice) NewRequest() core.Coder {
	return new(external.GetLRUSendToDeviceRequest)
}
func (ReqGetLRUSendToDevice) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetLRUSendToDeviceRequest)
	msg.Timestamp = fmt.Sprintf("%d", time.Now().Unix())
	return nil
}
func (ReqGetLRUSendToDevice) NewResponse(code int) core.Coder {
	return new(external.GetLRUSendToDeviceResponse)
}
func (ReqGetLRUSendToDevice) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetLRUSendToDeviceRequest)
	return []uint32{common.CalcStringHashCode(req.Timestamp) % cfg.MultiInstance.Total}
}
func (ReqGetLRUSendToDevice) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetLRUSendToDeviceRequest)
	if !common.IsRelatedRequest(req.Timestamp, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	loaded, max := c.stdEventTimeline.GetLoadedNumber()

	return http.StatusOK, &external.GetLRUSendToDeviceResponse{
		Loaded: loaded,
		Max:    max,
		Server: fmt.Sprintf("%s%d", os.Getenv("SERVICE_NAME"), c.Cfg.MultiInstance.Instance),
	}
}

type ReqPutLRUSendToDevice struct{}

func (ReqPutLRUSendToDevice) GetRoute() string       { return "/lru/syncaggregate/sendtodevice" }
func (ReqPutLRUSendToDevice) GetMetricsName() string { return "put_lru_syncaggregate_sendtodevice" }
func (ReqPutLRUSendToDevice) GetMsgType() int32 {
	return internals.MSG_PUT_LRU_SYNC_AGGREGATE_SENDTODEVICE
}
func (ReqPutLRUSendToDevice) GetAPIType() int8 { return apiconsumer.APITypeInternal }
func (ReqPutLRUSendToDevice) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutLRUSendToDevice) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutLRUSendToDevice) GetPrefix() []string                  { return []string{"inr0"} }
func (ReqPutLRUSendToDevice) NewRequest() core.Coder {
	return new(external.PutLRUSendToDeviceRequest)
}
func (ReqPutLRUSendToDevice) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutLRUSendToDeviceRequest)
	if err := common.UnmarshalJSON(req, msg); err != nil {
		return err
	}
	return nil
}
func (ReqPutLRUSendToDevice) NewResponse(code int) core.Coder {
	return new(external.PutLRUSendToDeviceResponse)
}
func (ReqPutLRUSendToDevice) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.PutLRUSendToDeviceRequest)
	return []uint32{common.CalcStringHashCode(req.UserID) % cfg.MultiInstance.Total}
}
func (ReqPutLRUSendToDevice) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutLRUSendToDeviceRequest)
	if !common.IsRelatedRequest(req.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	c.stdEventTimeline.LoadHistory(req.UserID, req.DeviceID, false)

	return http.StatusOK, &external.PutLRUSendToDeviceResponse{
		UserID:   req.UserID,
		DeviceID: req.DeviceID,
	}
}
