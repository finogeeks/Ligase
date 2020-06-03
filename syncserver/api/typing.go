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
	"github.com/finogeeks/ligase/common/jsonerror"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqPutTyping{})
}

type ReqPutTyping struct{}

func (ReqPutTyping) GetRoute() string       { return "/rooms/{roomID}/typing/{userID}" }
func (ReqPutTyping) GetMetricsName() string { return "rooms_typing" }
func (ReqPutTyping) GetMsgType() int32      { return internals.MSG_PUT_TYPING }
func (ReqPutTyping) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutTyping) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutTyping) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutTyping) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPutTyping) NewRequest() core.Coder {
	return new(external.PutRoomUserTypingRequest)
}
func (ReqPutTyping) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomUserTypingRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutTyping) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPutTyping) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomUserTypingRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	data := types.TypingContent{
		UserID: req.UserID,
		RoomID: req.RoomID,
	}
	if req.Typing {
		data.Type = "add"
	} else {
		data.Type = "remove"
	}

	state := c.rsCurState.GetRoomState(data.RoomID)
	if state != nil {
		update := syncapitypes.TypingUpdate{
			Type:   data.Type,
			UserID: data.UserID,
			RoomID: data.RoomID,
		}
		domainMap := make(map[string]bool)
		state.GetJoinMap().Range(func(key, value interface{}) bool {
			update.RoomUsers = append(update.RoomUsers, key.(string))
			domain, _ := common.DomainFromID(key.(string))
			if common.CheckValidDomain(domain, c.Cfg.Matrix.ServerName) == false {
				domainMap[domain] = true
			}
			return true
		})

		bytes, err := json.Marshal(update)
		if err == nil {
			c.RpcCli.Pub(types.TypingUpdateTopicDef, bytes)
		} else {
			log.Errorf("TypingRpcConsumer pub typing update error %v", err)
		}

		senderDomain, _ := common.DomainFromID(data.UserID)
		if common.CheckValidDomain(senderDomain, c.Cfg.Matrix.ServerName) {
			content, _ := json.Marshal(data)
			for domain := range domainMap {
				edu := gomatrixserverlib.EDU{
					Type:        "typing",
					Origin:      senderDomain,
					Destination: domain,
					Content:     content,
				}
				bytes, err := json.Marshal(edu)
				if err == nil {
					c.RpcCli.Pub(types.EduTopicDef, bytes)
				} else {
					log.Errorf("TypingRpcConsumer pub typing edu error %v", err)
				}
			}
		}
	}
	return http.StatusOK, nil
}
