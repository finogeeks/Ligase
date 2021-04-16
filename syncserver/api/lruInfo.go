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

	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetLRUInfo{})
}

type LRURooms struct {
	Loaded int `json:"loaded"`
	Max    int `json:"max"`
}

func (r *LRURooms) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *LRURooms) Decode(input []byte) error {
	return json.Unmarshal(input, r)
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
	return nil
}
func (ReqGetLRUInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetLRUInfo) NewResponse(code int) core.Coder {
	return new(LRURooms)
}
func (ReqGetLRUInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)

	loaded, max := c.rmHsTimeline.GetLoadedRoomNumber()

	return http.StatusOK, &LRURooms{
		Loaded: loaded,
		Max:    max,
	}
}
