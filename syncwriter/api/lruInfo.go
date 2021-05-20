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
	apiconsumer.SetAPIProcessor(ReqGetLRURooms{})
	apiconsumer.SetAPIProcessor(ReqPutLRURoom{})
}

type ReqGetLRURooms struct{}

func (ReqGetLRURooms) GetRoute() string       { return "/lru/syncwriter/rooms" }
func (ReqGetLRURooms) GetMetricsName() string { return "get_lru_syncwirter_rooms" }
func (ReqGetLRURooms) GetMsgType() int32      { return internals.MSG_GET_LRU_ROOMS }
func (ReqGetLRURooms) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetLRURooms) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetLRURooms) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetLRURooms) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetLRURooms) NewRequest() core.Coder {
	return new(external.GetLRURoomsRequest)
}
func (ReqGetLRURooms) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetLRURoomsRequest)
	msg.Timestamp = fmt.Sprintf("%d", time.Now().Unix())
	return nil
}
func (ReqGetLRURooms) NewResponse(code int) core.Coder {
	return new(external.GetLRURoomsResponse)
}
func (ReqGetLRURooms) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetLRURoomsRequest)
	if !common.IsRelatedRequest(req.Timestamp, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	loaded, max := c.rmHsTimeline.GetLoadedRoomNumber()

	return http.StatusOK, &external.GetLRURoomsResponse{
		Loaded: loaded,
		Max:    max,
		Server: fmt.Sprintf("%s%d", os.Getenv("SERVICE_NAME"), c.Cfg.MultiInstance.Instance),
	}
}

type ReqPutLRURoom struct{}

func (ReqPutLRURoom) GetRoute() string       { return "/lru/syncwriter/room" }
func (ReqPutLRURoom) GetMetricsName() string { return "put_lru_syncwriter_room" }
func (ReqPutLRURoom) GetMsgType() int32      { return internals.MSG_PUT_LRU_ROOM }
func (ReqPutLRURoom) GetAPIType() int8       { return apiconsumer.APITypeInternal }
func (ReqPutLRURoom) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutLRURoom) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutLRURoom) GetPrefix() []string                  { return []string{"inr0"} }
func (ReqPutLRURoom) NewRequest() core.Coder {
	return new(external.PutLRURoomRequest)
}
func (ReqPutLRURoom) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutLRURoomRequest)
	if err := common.UnmarshalJSON(req, msg); err != nil {
		return err
	}
	return nil
}
func (ReqPutLRURoom) NewResponse(code int) core.Coder {
	return new(external.PutLRURoomResponse)
}
func (ReqPutLRURoom) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutLRURoomRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	c.rmHsTimeline.LoadHistory(req.RoomID, false)
	c.rmHsTimeline.LoadDomainMaxStream(req.RoomID)
	c.rsTimeline.LoadStreamStates(req.RoomID, false)

	return http.StatusOK, &external.PutLRURoomResponse{
		RoomID: req.RoomID,
	}
}
