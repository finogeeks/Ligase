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
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetServices("front_rcs_api")
	apiconsumer.SetAPIProcessor(ReqGetFriendships{})
	apiconsumer.SetAPIProcessor(ReqGetRoomID{})
}

type ReqGetFriendships struct{}

func (ReqGetFriendships) GetRoute() string       { return "/rcs/friendships" }
func (ReqGetFriendships) GetMetricsName() string { return "rcs_friendships" }
func (ReqGetFriendships) GetMsgType() int32      { return internals.MSG_GET_RCS_FRIENDSHIPS }
func (ReqGetFriendships) GetAPIType() int8       { return apiconsumer.APITypeInternal }
func (ReqGetFriendships) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetFriendships) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFriendships) GetPrefix() []string                  { return []string{"inr0"} }
func (ReqGetFriendships) NewRequest() core.Coder {
	return new(external.GetFriendshipsRequest)
}
func (ReqGetFriendships) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetFriendshipsRequest)
	query := req.URL.Query()
	msg.Type = query.Get("type")
	return nil
}
func (ReqGetFriendships) NewResponse(code int) core.Coder {
	return new(external.GetFriendshipsResponse)
}
func (ReqGetFriendships) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetFriendshipsRequest)
	userID := device.UserID
	resp, err := c.getFriendships(req, userID)
	if err != nil {
		log.Errorf("Failed to get friendships: %v\n", err)
		return http.StatusInternalServerError, resp
	}
	return http.StatusOK, resp
}

type ReqGetRoomID struct{}

func (ReqGetRoomID) GetRoute() string       { return "/rcs/friendship" }
func (ReqGetRoomID) GetMetricsName() string { return "rcs_friendship" }
func (ReqGetRoomID) GetMsgType() int32      { return internals.MSG_GET_RCS_ROOMID }
func (ReqGetRoomID) GetAPIType() int8       { return apiconsumer.APITypeInternal }
func (ReqGetRoomID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomID) GetPrefix() []string                  { return []string{"inr0"} }
func (ReqGetRoomID) NewRequest() core.Coder {
	return new(external.GetFriendshipRequest)
}
func (ReqGetRoomID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetFriendshipRequest)
	query := req.URL.Query()
	msg.FcID = query.Get("fcid")
	msg.ToFcID = query.Get("toFcid")
	return nil
}
func (ReqGetRoomID) NewResponse(code int) core.Coder {
	return new(external.GetFriendshipResponse)
}
func (ReqGetRoomID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetFriendshipRequest)
	resp, err := c.getFriendship(req)
	if err != nil {
		log.Errorf("Failed to get friendship: %v\n", err)
		return http.StatusInternalServerError, resp
	}
	return http.StatusOK, resp
}
