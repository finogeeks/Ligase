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
	"net/http"
	"strconv"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/publicroomsapi/directory"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
)

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer

	publicRoomsDB model.PublicRoomAPIDatabase
	federation    *gomatrixserverlib.FederationClient
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	publicRoomsDB model.PublicRoomAPIDatabase,
	rpcCli *common.RpcClient,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcCli = rpcCli
	c.publicRoomsDB = publicRoomsDB
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.InitGroup("publicroomapi", c, c.Cfg.Rpc.ProxyPublicRoomApiTopic, types.PUBLICROOM_API_GROUP)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyPublicRoomApiTopic
}

func init() {
	apiconsumer.SetAPIProcessor(ReqGetPublicRooms{})
	apiconsumer.SetAPIProcessor(ReqPostPublicRooms{})
}

const ProxyPublicRoomAPITopic = "proxyPublicRoomApi"

type ReqGetPublicRooms struct{}

func (ReqGetPublicRooms) GetRoute() string       { return "/publicRooms" }
func (ReqGetPublicRooms) GetMetricsName() string { return "public_rooms" }
func (ReqGetPublicRooms) GetMsgType() int32      { return internals.MSG_GET_PUBLIC_ROOMS }
func (ReqGetPublicRooms) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetPublicRooms) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPublicRooms) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPublicRooms) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetPublicRooms) NewRequest() core.Coder {
	return new(external.GetPublicRoomsRequest)
}
func (ReqGetPublicRooms) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetPublicRoomsRequest)
	limit, err := strconv.Atoi(req.FormValue("limit"))
	// Atoi returns 0 and an error when trying to parse an empty string
	// In that case, we want to assign 0 so we ignore the error
	if err != nil && len(req.FormValue("limit")) > 0 {
		return err
	}
	msg.Limit = int64(limit)
	msg.Since = req.FormValue("since")
	return nil
}
func (ReqGetPublicRooms) NewResponse(code int) core.Coder {
	return new(external.GetPublicRoomsResponse)
}
func (ReqGetPublicRooms) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetPublicRoomsRequest)
	return directory.GetPublicRooms(
		ctx, req, c.publicRoomsDB,
	)
}

type ReqPostPublicRooms struct{}

func (ReqPostPublicRooms) GetRoute() string       { return "/publicRooms" }
func (ReqPostPublicRooms) GetMetricsName() string { return "public_rooms" }
func (ReqPostPublicRooms) GetMsgType() int32      { return internals.MSG_POST_PUBLIC_ROOMS }
func (ReqPostPublicRooms) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostPublicRooms) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostPublicRooms) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostPublicRooms) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPostPublicRooms) NewRequest() core.Coder {
	return new(external.PostPublicRoomsRequest)
}
func (ReqPostPublicRooms) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostPublicRoomsRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostPublicRooms) NewResponse(code int) core.Coder {
	return new(external.PostPublicRoomsResponse)
}
func (ReqPostPublicRooms) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostPublicRoomsRequest)
	return directory.PostPublicRooms(
		ctx, req, c.publicRoomsDB,
	)
}
