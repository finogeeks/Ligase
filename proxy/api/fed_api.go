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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/proxy/bridge"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	dmodel "github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type FedApiUserData struct {
	Request    *gomatrixserverlib.FederationRequest
	Idg        *uid.UidGenerator
	Cfg        *config.Dendrite
	RpcCli     roomserverapi.RoomserverRPCAPI
	CacheIn    service.Cache
	KeyDB      dmodel.KeyDatabase
	LocalCache service.LocalCache
	Origin     string
}

func genMsgSeq(idg *uid.UidGenerator) string {
	v, _ := idg.Next()
	return strconv.FormatInt(v, 16)
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyFedApiTopic
}

func init() {
	apiconsumer.SetAPIProcessor(ReqGetFedVer{})
	apiconsumer.SetAPIProcessor(ReqGetFedDirectory{})
	apiconsumer.SetAPIProcessor(ReqGetFedProfile{})
	apiconsumer.SetAPIProcessor(ReqPutFedSend{})
	apiconsumer.SetAPIProcessor(ReqPutFedInvite{})
	apiconsumer.SetAPIProcessor(ReqGetFedRoomState{})
	apiconsumer.SetAPIProcessor(ReqGetFedBackfill{})
	apiconsumer.SetAPIProcessor(ReqGetFedMissingEvents{})
	apiconsumer.SetAPIProcessor(ReqGetFedMediaInfo{})
	apiconsumer.SetAPIProcessor(ReqGetFedMediaDownload{})
	apiconsumer.SetAPIProcessor(ReqPostNotaryNotice{})
	apiconsumer.SetAPIProcessor(ReqGetFedUserInfo{})
	apiconsumer.SetAPIProcessor(ReqGetMakeJoin{})
	apiconsumer.SetAPIProcessor(ReqPutSendJoin{})
	apiconsumer.SetAPIProcessor(ReqGetMakeLeave{})
	apiconsumer.SetAPIProcessor(ReqPutSendLeave{})
	apiconsumer.SetAPIProcessor(ReqGetEventAuth{})
	apiconsumer.SetAPIProcessor(ReqPostQueryAuth{})
	apiconsumer.SetAPIProcessor(ReqGetEvent{})
	apiconsumer.SetAPIProcessor(ReqGetStateIDs{})
	apiconsumer.SetAPIProcessor(ReqGetPublicRooms{})
	apiconsumer.SetAPIProcessor(ReqPostPublicRooms{})
	apiconsumer.SetAPIProcessor(ReqPostQueryClientKeys{})
	apiconsumer.SetAPIProcessor(ReqPostClaimClientKeys{})
}

type ReqGetFedVer struct{}

func (ReqGetFedVer) GetRoute() string                     { return "/version" }
func (ReqGetFedVer) GetMetricsName() string               { return "federation_version" }
func (ReqGetFedVer) GetMsgType() int32                    { return internals.MSG_GET_FED_VER }
func (ReqGetFedVer) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetFedVer) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetFedVer) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedVer) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedVer) NewRequest() core.Coder               { return nil }
func (ReqGetFedVer) NewResponse(code int) core.Coder      { return new(model.Version) }
func (ReqGetFedVer) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetFedVer) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	log.Info("get federation version")
	return http.StatusOK, model.GetVersion()
}

type ReqGetFedDirectory struct{}

func (ReqGetFedDirectory) GetRoute() string                     { return "/query/directory" }
func (ReqGetFedDirectory) GetMetricsName() string               { return "federation_query_room_alias" }
func (ReqGetFedDirectory) GetMsgType() int32                    { return internals.MSG_GET_FED_DIRECTOR }
func (ReqGetFedDirectory) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetFedDirectory) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetFedDirectory) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedDirectory) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedDirectory) NewRequest() core.Coder               { return new(external.GetFedDirectoryRequest) }
func (ReqGetFedDirectory) NewResponse(code int) core.Coder {
	return new(external.GetFedDirectoryResponse)
}
func (ReqGetFedDirectory) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetFedDirectoryRequest)
	msg.RoomAlias = req.FormValue("room_alias")
	return nil
}
func (ReqGetFedDirectory) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetFedDirectoryRequest)
	idg := ud.(*FedApiUserData).Idg

	roomAlias := req.RoomAlias
	if roomAlias == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("Must supply room alias parameter.")
	}
	_, _ /*domain*/, err := gomatrixserverlib.SplitID('#', roomAlias)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Room alias must be in the form '#localpart:domain'")
	}

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = []byte(roomAlias)
	gobMsg.Cmd = model.CMD_FED_ROOM_DIRECTORY

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	log.Infof("roomAliasToID resp: %v", *resp)
	fedDir := &external.GetFedDirectoryResponse{}
	fedDir.Decode(resp.Body)

	return http.StatusOK, fedDir
}

type ReqGetFedProfileRequest struct {
	UserID string `json:"user_id"`
	Field  string `json:"field"`
}

func (r *ReqGetFedProfileRequest) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedProfileRequest) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedProfileResponse struct {
	AvatarURL    string `json:"avatar_url,omitempty"`
	DisplayName  string `json:"displayname,omitempty"`
	Status       string `json:"status,omitempty"`
	StatusMsg    string `json:"status_msg,omitempty"`
	ExtStatusMsg string `json:"ext_status_msg,omitempty"`
}

func (r *ReqGetFedProfileResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedProfileResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedProfile struct{}

func (ReqGetFedProfile) GetRoute() string                     { return "/query/profile" }
func (ReqGetFedProfile) GetMetricsName() string               { return "federation_query_profile" }
func (ReqGetFedProfile) GetMsgType() int32                    { return internals.MSG_GET_FED_PROFILE }
func (ReqGetFedProfile) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetFedProfile) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetFedProfile) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedProfile) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedProfile) NewRequest() core.Coder               { return new(ReqGetFedProfileRequest) }
func (ReqGetFedProfile) NewResponse(code int) core.Coder      { return new(ReqGetFedProfileResponse) }
func (ReqGetFedProfile) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*ReqGetFedProfileRequest)
	msg.UserID = req.FormValue("user_id")
	msg.Field = req.FormValue("field")
	return nil
}
func (ReqGetFedProfile) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*ReqGetFedProfileRequest)
	cfg := ud.(*FedApiUserData).Cfg
	idg := ud.(*FedApiUserData).Idg

	userID := req.UserID
	field := req.Field

	if userID == "" {
		return http.StatusBadRequest, jsonerror.MissingArgument("The request body did not contain required argument 'user_id'.")
	}

	_, domain, err := gomatrixserverlib.SplitID('@', userID)
	if err != nil {
		return http.StatusBadRequest, jsonerror.InvalidArgumentValue(err.Error())
	}

	if common.CheckValidDomain(string(domain), cfg.Matrix.ServerName) {
		log.Infof("source dest: %s", domain)
		// return jsonerror.InternalServerError()
	}

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = []byte(userID)

	if field == "" {
		/*if request == nil || request.Origin() == config.GetConfig().Homeserver.ServerName {
			gobMsg.Cmd = model.CMD_HS_PROFILE
		} else {*/
		gobMsg.Cmd = model.CMD_FED_PROFILE
		//}
	} else if field == "avatar_url" {
		gobMsg.Cmd = model.CMD_FED_AVATARURL
	} else if field == "displayname" {
		gobMsg.Cmd = model.CMD_FED_DISPLAYNAME
	}

	log.Infof("gobMsg.Cmd: %d", gobMsg.Cmd)

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	log.Infof("GetProfile resp: %v", resp)

	profile := &ReqGetFedProfileResponse{}
	profile.Decode(resp.Body)

	return http.StatusOK, profile
}

type ReqPutFedSendRequest struct {
	TxnID string `json:"txnID"`
}

func (r *ReqPutFedSendRequest) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqPutFedSendRequest) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqPutFedSendResponse gomatrixserverlib.RespSend

func (r *ReqPutFedSendResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqPutFedSendResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqPutFedSend struct{}

func (ReqPutFedSend) GetRoute() string                     { return "/send/{txnID}/" }
func (ReqPutFedSend) GetMetricsName() string               { return "federation_send" }
func (ReqPutFedSend) GetMsgType() int32                    { return internals.MSG_PUT_FED_SEND }
func (ReqPutFedSend) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPutFedSend) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutFedSend) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutFedSend) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPutFedSend) NewRequest() core.Coder               { return new(ReqPutFedSendRequest) }
func (ReqPutFedSend) NewResponse(code int) core.Coder      { return new(ReqPutFedSendResponse) }
func (ReqPutFedSend) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*ReqPutFedSendRequest)
	msg.TxnID = vars["txnID"]
	return nil
}
func (ReqPutFedSend) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*ReqPutFedSendRequest)
	request := ud.(*FedApiUserData).Request
	localCache := ud.(*FedApiUserData).LocalCache

	// txnID := req.TxnID
	/*	t := model.TxnReq{
			// Context: httpReq.Context(),
		}
	*/ // t.Content = request.Content()
	// t.Origin = request.Origin()

	trans := gomatrixserverlib.Transaction{}
	err := json.Unmarshal(request.Content(), &trans)
	if err != nil {
		log.Errorf("api send msg error: %v", err)
		return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
	}

	keys := []byte{}
	if len(trans.PDUs) > 0 {
		keys = []byte(trans.PDUs[0].RoomID())
	} else if len(trans.EDUs) > 0 {
		for _, edu := range trans.EDUs {
			switch edu.Type {
			case "profile":
				keys = []byte("0")
				break
			case "receipt":
				var content types.ReceiptContent
				if err := json.Unmarshal(edu.Content, &content); err != nil {
					log.Errorf("send edu error: %v", err)
					keys = []byte("0")
				} else {
					keys = []byte(content.RoomID)
				}
			case "typing":
				var content types.TypingContent
				if err := json.Unmarshal(edu.Content, &content); err != nil {
					log.Errorf("send edu error: %v", err)
					keys = []byte("0")
				} else {
					keys = []byte(content.RoomID)
				}
			}
			break
		}
	}

	var key string
	if req.TxnID != "" {
		key = "fedsend:" + string(req.TxnID)
		_, ok := localCache.Get(0, key)
		if ok {
			return http.StatusOK, nil
		}
	}

	bytes, _ := json.Marshal(request)
	log.Infof("federation_send recv send request, org: %s, fed req: %v", request.Origin(), string(bytes))

	gobMsg := model.GobMessage{}
	gobMsg.Body = request.Content()
	gobMsg.Key = keys
	gobMsg.MsgSeq = string(req.TxnID)
	gobMsg.Cmd = model.CMD_FED_SEND

	//only for debug
	if adapter.GetDebugLevel() == adapter.DEBUG_LEVEL_DEBUG {
		delay := adapter.Random(0, 10)
		log.Debugf("fed recv transationID:%s sleep %ds", req.TxnID, delay)
		time.Sleep(time.Duration(delay) * time.Second)
	}
	// resp, err := bridge.SendAndRecv(gobMsg, 30000)
	// if err != nil {
	// 	log.Errorf("bridge.SendAndRecv err %v", err)
	// 	return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	// } else if resp.Head.ErrStr != "" {
	// 	log.Errorf("bridge.SendAndRecv resp.Head.ErrStr %s", resp.Head.ErrStr)
	// 	return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	// }

	// bridge.Send(gobMsg)

	prod := ud.(*FedApiUserData).Cfg.Kafka.Producer.FedBridgeOut

	err = common.GetTransportMultiplexer().SendAndRecvWithRetry(
		prod.Underlying,
		prod.Name,
		&core.TransportPubMsg{
			//Format: core.FORMAT_GOB,
			Keys: keys,
			Obj:  gobMsg,
		})
	if err != nil {
		log.Errorf("api send msg write kafka error: %v", err)
		return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
	}

	if key != "" {
		localCache.Put(0, key, struct{}{})
	}

	return http.StatusOK, nil
}

type ReqPutFedInvite struct{}

func (ReqPutFedInvite) GetRoute() string                     { return "/invite/{roomID}/{eventID}" }
func (ReqPutFedInvite) GetMetricsName() string               { return "federation_invite" }
func (ReqPutFedInvite) GetMsgType() int32                    { return internals.MSG_PUT_FED_INVITE }
func (ReqPutFedInvite) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPutFedInvite) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutFedInvite) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutFedInvite) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPutFedInvite) NewRequest() core.Coder               { return new(external.PutFedInviteRequest) }
func (ReqPutFedInvite) NewResponse(code int) core.Coder      { return new(gomatrixserverlib.RespInvite) }
func (ReqPutFedInvite) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutFedInviteRequest)
	msg.RoomID = vars["roomID"]
	msg.EventID = vars["eventID"]
	return nil
}
func (ReqPutFedInvite) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.PutFedInviteRequest)
	request := ud.(*FedApiUserData).Request
	idg := ud.(*FedApiUserData).Idg

	roomID := req.RoomID
	eventID := req.EventID

	// Decode the event JSON from the request.
	if err := json.Unmarshal(request.Content(), &req.Event); err != nil {
		return http.StatusBadRequest, jsonerror.NotJSON("The request body could not be decoded into valid JSON. " + err.Error())
	}

	if roomID != req.Event.RoomID() {
		return http.StatusBadRequest, jsonerror.BadJSON("The room ID in the request path must match the room ID in the invite event JSON")
	}

	if eventID != req.Event.EventID() {
		return http.StatusBadRequest, jsonerror.BadJSON("The event ID in the request path must match the event ID in the invite event JSON")
	}

	log.Infof("event.org: %s, requ.org: %s", req.Event.Origin(), request.Origin())

	gobMsg := model.GobMessage{}
	gobMsg.Body = request.Content()
	gobMsg.Key = []byte(roomID)
	gobMsg.Cmd = model.CMD_FED_INVITE
	gobMsg.MsgSeq = genMsgSeq(idg)

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res gomatrixserverlib.RespInvite
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqGetFedRoomStateRequest struct {
	RoomID  string `json:"roomID"`
	EventID string `json:"eventID"`
}

func (r *ReqGetFedRoomStateRequest) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedRoomStateRequest) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedRoomStateResponse gomatrixserverlib.RespState

func (r *ReqGetFedRoomStateResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedRoomStateResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedRoomState struct{}

func (ReqGetFedRoomState) GetRoute() string                     { return "/state/{roomID}/" }
func (ReqGetFedRoomState) GetMetricsName() string               { return "federation_state_qry" }
func (ReqGetFedRoomState) GetMsgType() int32                    { return internals.MSG_GET_FED_ROOM_STATE }
func (ReqGetFedRoomState) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetFedRoomState) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetFedRoomState) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedRoomState) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedRoomState) NewRequest() core.Coder               { return new(ReqGetFedRoomStateRequest) }
func (ReqGetFedRoomState) NewResponse(code int) core.Coder      { return new(ReqGetFedRoomStateResponse) }
func (ReqGetFedRoomState) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*ReqGetFedRoomStateRequest)
	msg.RoomID = vars["roomID"]
	msg.EventID = req.FormValue("event_id")
	return nil
}
func (ReqGetFedRoomState) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*ReqGetFedRoomStateRequest)
	idg := ud.(*FedApiUserData).Idg

	roomID := req.RoomID

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = []byte(roomID)
	gobMsg.Cmd = model.CMD_FED_ROOM_STATE

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res roomserverapi.QueryRoomStateResponse
	json.Unmarshal(resp.Body, &res)

	var state gomatrixserverlib.RespState
	state.StateEvents = res.GetAllState()
	log.Infof("RoomStates resp: %v", res)

	return http.StatusOK, (*ReqGetFedRoomStateResponse)(&state)
}

type ReqGetFedBackfillRequest struct {
	RoomID string `json:"roomID"`
	V      string `json:"v"`
	Limit  string `json:"limit"`
	Dir    string `json:"dir"`
	Domain string `json:"domain"`
}

func (r *ReqGetFedBackfillRequest) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedBackfillRequest) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

//type ReqGetFedBackfillResponse gomatrixserverlib.Transaction
type ReqGetFedBackfillResponse roomserverapi.QueryBackFillEventsResponse

func (r *ReqGetFedBackfillResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedBackfillResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedBackfill struct{}

func (ReqGetFedBackfill) GetRoute() string                     { return "/backfill/{roomID}/" }
func (ReqGetFedBackfill) GetMetricsName() string               { return "backfill" }
func (ReqGetFedBackfill) GetMsgType() int32                    { return internals.MSG_GET_FED_BACKFILL }
func (ReqGetFedBackfill) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetFedBackfill) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetFedBackfill) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedBackfill) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedBackfill) NewRequest() core.Coder               { return new(ReqGetFedBackfillRequest) }
func (ReqGetFedBackfill) NewResponse(code int) core.Coder      { return new(ReqGetFedBackfillResponse) }
func (ReqGetFedBackfill) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*ReqGetFedBackfillRequest)
	msg.RoomID = vars["roomID"]
	msg.V = req.FormValue("v")
	msg.Limit = req.FormValue("limit")
	msg.Dir = req.FormValue("dir")
	msg.Domain = req.FormValue("domain")
	return nil
}
func (ReqGetFedBackfill) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*ReqGetFedBackfillRequest)
	idg := ud.(*FedApiUserData).Idg

	roomID := req.RoomID
	eventID := req.V
	limit := req.Limit
	dir := req.Dir

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Cmd = model.CMD_FED_BACKFILL

	rid, err := url.PathUnescape(roomID)
	if err != nil {
		rid = roomID
	}
	var backfill external.GetFedBackFillRequest
	backfill.RoomID = rid
	backfill.Limit, _ = strconv.Atoi(limit)
	backfill.BackFillIds = eventID
	backfill.Dir = dir
	backfill.Domain = req.Domain
	backfill.Origin = ud.(*FedApiUserData).Origin
	body, _ := backfill.Encode()
	gobMsg.Body = body

	log.Infof("BackFill recv request: rid:%s eventID:%v, limit:%v", rid, eventID, limit)

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		log.Errorf("BackFill recv request: rid:%s eventID:%v, limit:%v, err:%v", rid, eventID, limit, err)
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		log.Errorf("BackFill recv request: rid:%s eventID:%v, limit:%v, err:%s", rid, eventID, limit, resp.Head.ErrStr)
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	log.Infof("BackFill recv request: rid:%s eventID:%v, limit:%v, resp:%s", rid, eventID, limit, resp.Body)
	evs := roomserverapi.QueryBackFillEventsResponse{}
	err = json.Unmarshal(resp.Body, &evs)
	if err != nil {
		log.Errorf("BackFill unmarshal error %s", err.Error())
	}
	// t := gomatrixserverlib.Transaction{}
	// t.Origin = gomatrixserverlib.ServerName(evs.Origin)
	// if evs.OriginServerTs != "" {
	// 	ts, _ := strconv.ParseUint(evs.OriginServerTs, 0, 64)
	// 	t.OriginServerTS = gomatrixserverlib.Timestamp(ts)
	// }
	// t.PDUs = evs.PDUs
	// log.Infof("BackFill resp: %v", t)

	return http.StatusOK, (*ReqGetFedBackfillResponse)(&evs)
}

type ReqGetFedMissingEvents struct{}

func (ReqGetFedMissingEvents) GetRoute() string       { return "/get_missing_events/{roomId}" }
func (ReqGetFedMissingEvents) GetMetricsName() string { return "get_missing_events" }
func (ReqGetFedMissingEvents) GetMsgType() int32      { return internals.MSG_GET_FED_MISSING_EVENTS }
func (ReqGetFedMissingEvents) GetAPIType() int8       { return apiconsumer.APITypeFed }
func (ReqGetFedMissingEvents) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetFedMissingEvents) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedMissingEvents) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedMissingEvents) NewRequest() core.Coder               { return new(external.GetMissingEventsRequest) }
func (ReqGetFedMissingEvents) NewResponse(code int) core.Coder {
	return new(external.GetMissingEventsResponse)
}
func (ReqGetFedMissingEvents) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetMissingEventsRequest)
	msg.RoomID = vars["roomId"]
	return nil
}
func (ReqGetFedMissingEvents) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	// req := msg.(*external.GetMissingEventsRequest)
	idg := ud.(*FedApiUserData).Idg

	// roomID := req.RoomID

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Cmd = model.CMD_FED_GET_MISSING_EVENTS

	return 200, nil

	// rid, err := url.PathUnescape(roomID)
	// if err != nil {
	// 	rid = roomID
	// }
	// var backfill external.GetFedBackFillRequest
	// backfill.RoomID = rid
	// backfill.Limit, _ = strconv.Atoi(limit)
	// backfill.BackFillIds = eventID
	// backfill.Dir = "b"
	// backfill.Domain = req.Domain
	// body, _ := backfill.Encode()
	// gobMsg.Body = body

	// log.Infof("BackFill recv request: rid:%s eventID:%v, limit:%v", rid, eventID, limit)

	// resp, err := bridge.SendAndRecv(gobMsg, 30000)
	// if err != nil {
	// 	log.Errorf("BackFill recv request: rid:%s eventID:%v, limit:%v, err:%v", rid, eventID, limit, err)
	// 	return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	// } else if resp.Head.ErrStr != "" {
	// 	log.Errorf("BackFill recv request: rid:%s eventID:%v, limit:%v, err:%s", rid, eventID, limit, resp.Head.ErrStr)
	// 	return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	// }

	// log.Infof("BackFill recv request: rid:%s eventID:%v, limit:%v, resp:%s", rid, eventID, limit, resp.Body)
	// evs := roomserverapi.QueryBackFillEventsResponse{}
	// err = json.Unmarshal(resp.Body, &evs)
	// if err != nil {
	// 	log.Errorf("BackFill unmarshal error %s", err.Error())
	// }
	// // t := gomatrixserverlib.Transaction{}
	// // t.Origin = gomatrixserverlib.ServerName(evs.Origin)
	// // if evs.OriginServerTs != "" {
	// // 	ts, _ := strconv.ParseUint(evs.OriginServerTs, 0, 64)
	// // 	t.OriginServerTS = gomatrixserverlib.Timestamp(ts)
	// // }
	// // t.PDUs = evs.PDUs
	// // log.Infof("BackFill resp: %v", t)

	// return http.StatusOK, (*ReqGetFedBackfillResponse)(&evs)
}

type ReqGetFedMediaInfoRequest struct {
	NetdiskID string
	UserID    string
}

func (r *ReqGetFedMediaInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedMediaInfoRequest) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedMediaInfoResponse gomatrixserverlib.RespMediaInfo

func (r *ReqGetFedMediaInfoResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedMediaInfoResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

var netdiskHttpCli = http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Second * 15,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     time.Second * 90,
	},
}

type ReqGetFedMediaInfo struct{}

func (ReqGetFedMediaInfo) GetRoute() string {
	return "/media/info/{mediaID}/{userID}"
}
func (ReqGetFedMediaInfo) GetMetricsName() string { return "media_info" }
func (ReqGetFedMediaInfo) GetMsgType() int32      { return internals.MSG_GET_FED_MEDIA_INFO }
func (ReqGetFedMediaInfo) GetAPIType() int8       { return apiconsumer.APITypeFed }
func (ReqGetFedMediaInfo) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetFedMediaInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedMediaInfo) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedMediaInfo) NewRequest() core.Coder               { return new(ReqGetFedMediaInfoRequest) }
func (ReqGetFedMediaInfo) NewResponse(code int) core.Coder      { return new(ReqGetFedMediaInfoResponse) }
func (ReqGetFedMediaInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*ReqGetFedMediaInfoRequest)
	msg.NetdiskID = vars["mediaID"]
	msg.UserID = vars["userID"]
	return nil
}
func (ReqGetFedMediaInfo) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*ReqGetFedMediaInfoRequest)
	cfg := ud.(*FedApiUserData).Cfg

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf(cfg.Media.MediaInfoUrl, req.NetdiskID), nil)
	if err != nil {
		log.Errorf("GetFedMediaInfo request error %v", err)
		return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error. " + err.Error())
	}
	request.Header.Set("X-Consumer-Custom-ID", req.UserID)

	response, err := netdiskHttpCli.Do(request)
	if err != nil {
		log.Errorf("GetFedMediaInfo response error %v", err)
		return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error. " + err.Error())
	}
	if response == nil || response.Body == nil {
		log.Errorf("GetFedMediaInfo response body nil")
		return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error")
	}
	defer response.Body.Close()
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorf("GetFedMediaInfo body ready error %v", err)
		return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error. " + err.Error())
	}
	resp := ReqGetFedMediaInfoResponse{}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Errorf("GetFedMediaInfo body unmarshal error %v", err)
		return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error. " + err.Error())
	}

	return http.StatusOK, &resp
}

type ReqGetFedMediaDownload struct{}

func (ReqGetFedMediaDownload) GetRoute() string {
	return "/media/download/{serverName}/{mediaId}/{fileType}"
}
func (ReqGetFedMediaDownload) GetMetricsName() string { return "media_download" }
func (ReqGetFedMediaDownload) GetMsgType() int32      { return internals.MSG_GET_FED_MEDIA_DOWNLOAD }
func (ReqGetFedMediaDownload) GetAPIType() int8       { return apiconsumer.APITypeDownload }
func (ReqGetFedMediaDownload) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetFedMediaDownload) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedMediaDownload) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedMediaDownload) NewRequest() core.Coder               { return nil }
func (ReqGetFedMediaDownload) NewResponse(code int) core.Coder      { return nil }
func (ReqGetFedMediaDownload) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetFedMediaDownload) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return 200, nil
}

type ReqPostNotaryNotice struct{}

func (ReqPostNotaryNotice) GetRoute() string                     { return "/notice" }
func (ReqPostNotaryNotice) GetMetricsName() string               { return "notary_notice" }
func (ReqPostNotaryNotice) GetMsgType() int32                    { return internals.MSG_POST_NOTARY_NOTICE }
func (ReqPostNotaryNotice) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPostNotaryNotice) GetMethod() []string                  { return []string{http.MethodPost} }
func (ReqPostNotaryNotice) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostNotaryNotice) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPostNotaryNotice) NewRequest() core.Coder               { return new(external.ReqPostNotaryNoticeRequest) }
func (ReqPostNotaryNotice) NewResponse(code int) core.Coder {
	return new(external.ReqPostNotaryNoticeResponse)
}
func (ReqPostNotaryNotice) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqPostNotaryNotice) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	resp := external.ReqPostNotaryNoticeResponse{Ack: 1}

	req := msg.(*external.ReqPostNotaryNoticeRequest)
	idg := ud.(*FedApiUserData).Idg

	rawReq := ud.(*FedApiUserData).Request.Content()
	if err := json.Unmarshal(rawReq, req); err != nil {
		return http.StatusBadRequest, &resp
	}

	log.Infof("_-----------------  notary notice, recv rawReq: %s", string(rawReq))

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	body, _ := req.Encode()
	gobMsg.Body = body
	gobMsg.Cmd = model.CMD_FED_NOTARY_NOTICE

	bridegResp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if bridegResp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(bridegResp.Head.ErrStr)
	}

	return http.StatusOK, &resp
}

type ReqGetFedUserInfoRequest struct {
	UserID string `json:"user_id"`
}

func (r *ReqGetFedUserInfoRequest) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedUserInfoRequest) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedUserInfoResponse struct {
	UserName  string `json:"user_name,omitempty"`
	JobNumber string `json:"job_number,omitempty"`
	Mobile    string `json:"mobile,omitempty"`
	Landline  string `json:"landline,omitempty"`
	Email     string `json:"email,omitempty"`
	State     int    `json:"state,omitempty"`
}

func (r *ReqGetFedUserInfoResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *ReqGetFedUserInfoResponse) Decode(input []byte) error {
	return json.Unmarshal(input, r)
}

type ReqGetFedUserInfo struct{}

func (ReqGetFedUserInfo) GetRoute() string                     { return "/query/user_info" }
func (ReqGetFedUserInfo) GetMetricsName() string               { return "federation_query_user_info" }
func (ReqGetFedUserInfo) GetMsgType() int32                    { return internals.MSG_GET_FED_USER_INFO }
func (ReqGetFedUserInfo) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetFedUserInfo) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetFedUserInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetFedUserInfo) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetFedUserInfo) NewRequest() core.Coder               { return new(ReqGetFedUserInfoRequest) }
func (ReqGetFedUserInfo) NewResponse(code int) core.Coder      { return new(ReqGetFedUserInfoResponse) }
func (ReqGetFedUserInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*ReqGetFedUserInfoRequest)
	msg.UserID = req.FormValue("user_id")
	return nil
}
func (ReqGetFedUserInfo) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*ReqGetFedUserInfoRequest)
	cfg := ud.(*FedApiUserData).Cfg
	idg := ud.(*FedApiUserData).Idg

	userID := req.UserID
	if userID == "" {
		return http.StatusBadRequest, jsonerror.MissingArgument("The request body did not contain required argument 'user_id'.")
	}

	_, domain, err := gomatrixserverlib.SplitID('@', userID)
	if err != nil {
		return http.StatusBadRequest, jsonerror.InvalidArgumentValue(err.Error())
	}
	if common.CheckValidDomain(string(domain), cfg.Matrix.ServerName) {
		log.Infof("source dest: %s", domain)
		// return jsonerror.InternalServerError()
	}

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = []byte(userID)
	gobMsg.Cmd = model.CMD_FED_USER_INFO

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}
	log.Infof("GetUserInfo resp: %v", resp)

	userInfo := &ReqGetFedUserInfoResponse{}
	userInfo.Decode(resp.Body)

	return http.StatusOK, userInfo
}

type ReqGetMakeJoin struct{}

func (ReqGetMakeJoin) GetRoute() string                     { return "/make_join/{roomID}/{userID}" }
func (ReqGetMakeJoin) GetMetricsName() string               { return "make_join" }
func (ReqGetMakeJoin) GetMsgType() int32                    { return internals.MSG_GET_FED_MAKE_JOIN }
func (ReqGetMakeJoin) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetMakeJoin) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetMakeJoin) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetMakeJoin) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetMakeJoin) NewRequest() core.Coder               { return new(external.GetMakeJoinRequest) }
func (ReqGetMakeJoin) NewResponse(code int) core.Coder      { return new(gomatrixserverlib.RespMakeJoin) }
func (ReqGetMakeJoin) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetMakeJoinRequest)
	msg.RoomID = vars["roomID"]
	msg.UserID = vars["userID"]
	msg.Ver = req.Form["ver"]
	return nil
}
func (ReqGetMakeJoin) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetMakeJoinRequest)
	idg := ud.(*FedApiUserData).Idg

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Key = []byte(req.RoomID)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_MAKEJOIN

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res gomatrixserverlib.RespMakeJoin
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqPutSendJoin struct{}

func (ReqPutSendJoin) GetRoute() string                     { return "/send_join/{roomID}/{eventID}" }
func (ReqPutSendJoin) GetMetricsName() string               { return "send_join" }
func (ReqPutSendJoin) GetMsgType() int32                    { return internals.MSG_PUT_FED_SEND_JOIN }
func (ReqPutSendJoin) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPutSendJoin) GetMethod() []string                  { return []string{http.MethodPut} }
func (ReqPutSendJoin) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutSendJoin) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPutSendJoin) NewRequest() core.Coder               { return new(external.PutSendJoinRequest) }
func (ReqPutSendJoin) NewResponse(code int) core.Coder      { return new(gomatrixserverlib.RespSendJoin) }
func (ReqPutSendJoin) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutSendJoinRequest)
	msg.RoomID = vars["roomID"]
	msg.EventID = vars["eventID"]
	return nil
}
func (ReqPutSendJoin) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.PutSendJoinRequest)
	idg := ud.(*FedApiUserData).Idg

	json.Unmarshal(ud.(*FedApiUserData).Request.Content(), &req.Event)
	roomID := req.RoomID
	if roomID == "" {
		roomID = req.Event.RoomID()
	}
	if req.Event.EventID() == "" {
	}
	if req.Event.RoomID() == "" {
	}

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Key = []byte(roomID)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_SENDJOIN

	log.Debugf("send join req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res gomatrixserverlib.RespSendJoin
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqGetMakeLeave struct{}

func (ReqGetMakeLeave) GetRoute() string                     { return "/make_leave/{roomID}/{userID}" }
func (ReqGetMakeLeave) GetMetricsName() string               { return "make_leave" }
func (ReqGetMakeLeave) GetMsgType() int32                    { return internals.MSG_GET_FED_MAKE_LEAVE }
func (ReqGetMakeLeave) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetMakeLeave) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetMakeLeave) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetMakeLeave) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetMakeLeave) NewRequest() core.Coder               { return new(external.GetMakeLeaveRequest) }
func (ReqGetMakeLeave) NewResponse(code int) core.Coder      { return new(gomatrixserverlib.RespMakeLeave) }
func (ReqGetMakeLeave) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetMakeLeaveRequest)
	msg.RoomID = vars["roomID"]
	msg.UserID = vars["userID"]
	return nil
}
func (ReqGetMakeLeave) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetMakeLeaveRequest)
	idg := ud.(*FedApiUserData).Idg

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Key = []byte(req.RoomID)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_MAKELEAVE

	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res gomatrixserverlib.RespMakeLeave
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqPutSendLeave struct{}

func (ReqPutSendLeave) GetRoute() string                     { return "/send_leave/{roomID}/{eventID}" }
func (ReqPutSendLeave) GetMetricsName() string               { return "send_leave" }
func (ReqPutSendLeave) GetMsgType() int32                    { return internals.MSG_PUT_FED_SEND_LEAVE }
func (ReqPutSendLeave) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPutSendLeave) GetMethod() []string                  { return []string{http.MethodPut} }
func (ReqPutSendLeave) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutSendLeave) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPutSendLeave) NewRequest() core.Coder               { return new(external.PutSendLeaveRequest) }
func (ReqPutSendLeave) NewResponse(code int) core.Coder      { return new(gomatrixserverlib.RespSendLeave) }
func (ReqPutSendLeave) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutSendLeaveRequest)
	msg.RoomID = vars["roomID"]
	msg.EventID = vars["eventID"]
	return nil
}
func (ReqPutSendLeave) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.PutSendLeaveRequest)
	idg := ud.(*FedApiUserData).Idg

	json.Unmarshal(ud.(*FedApiUserData).Request.Content(), &req.Event)

	roomID := req.RoomID
	if roomID == "" {
		roomID = req.Event.RoomID()
	}

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Key = []byte(roomID)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_SENDLEAVE

	log.Debugf("send leave req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res gomatrixserverlib.RespSendLeave
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqGetEventAuth struct{}

func (ReqGetEventAuth) GetRoute() string                     { return "/event_auth/{roomId}/{eventId}" }
func (ReqGetEventAuth) GetMetricsName() string               { return "event_auth" }
func (ReqGetEventAuth) GetMsgType() int32                    { return internals.MSG_GET_FED_EVENT_AUTH }
func (ReqGetEventAuth) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetEventAuth) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetEventAuth) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetEventAuth) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetEventAuth) NewRequest() core.Coder               { return new(external.GetEventAuthRequest) }
func (ReqGetEventAuth) NewResponse(code int) core.Coder      { return new(external.GetEventAuthResponse) }
func (ReqGetEventAuth) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetEventAuthRequest)
	msg.RoomID = vars["roomId"]
	msg.EventID = vars["eventId"]
	return nil
}
func (ReqGetEventAuth) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetEventAuthRequest)
	idg := ud.(*FedApiUserData).Idg

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_EVENT_AUTH

	log.Debugf("get event auth req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.GetEventAuthResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqPostQueryAuth struct{}

func (ReqPostQueryAuth) GetRoute() string                     { return "/query_auth/{roomId}/{eventId}" }
func (ReqPostQueryAuth) GetMetricsName() string               { return "query_auth" }
func (ReqPostQueryAuth) GetMsgType() int32                    { return internals.MSG_POST_FED_QUERY_AUTH }
func (ReqPostQueryAuth) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPostQueryAuth) GetMethod() []string                  { return []string{http.MethodPost} }
func (ReqPostQueryAuth) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostQueryAuth) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPostQueryAuth) NewRequest() core.Coder               { return new(external.PostQueryAuthRequest) }
func (ReqPostQueryAuth) NewResponse(code int) core.Coder      { return new(external.PostQueryAuthResponse) }
func (ReqPostQueryAuth) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostQueryAuthRequest)
	msg.RoomID = vars["roomId"]
	msg.EventID = vars["eventId"]
	return nil
}
func (ReqPostQueryAuth) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.PostQueryAuthRequest)
	idg := ud.(*FedApiUserData).Idg

	json.Unmarshal(ud.(*FedApiUserData).Request.Content(), &req)

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_QUERY_AUTH

	log.Debugf("query auth req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.PostQueryAuthResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqGetEvent struct{}

func (ReqGetEvent) GetRoute() string                     { return "/event/{eventId}" }
func (ReqGetEvent) GetMetricsName() string               { return "get_event" }
func (ReqGetEvent) GetMsgType() int32                    { return internals.MSG_GET_FED_EVENT }
func (ReqGetEvent) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetEvent) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetEvent) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetEvent) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetEvent) NewRequest() core.Coder               { return new(external.GetEventRequest) }
func (ReqGetEvent) NewResponse(code int) core.Coder      { return new(external.GetEventResponse) }
func (ReqGetEvent) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetEventRequest)
	msg.EventID = vars["eventId"]
	return nil
}
func (ReqGetEvent) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetEventRequest)
	idg := ud.(*FedApiUserData).Idg

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_EVENT

	log.Debugf("get event req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.GetEventResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqGetStateIDs struct{}

func (ReqGetStateIDs) GetRoute() string                     { return "/state_ids/{roomId}" }
func (ReqGetStateIDs) GetMetricsName() string               { return "state_ids" }
func (ReqGetStateIDs) GetMsgType() int32                    { return internals.MSG_GET_FED_STATE_IDS }
func (ReqGetStateIDs) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetStateIDs) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetStateIDs) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetStateIDs) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetStateIDs) NewRequest() core.Coder               { return new(external.GetStateIDsRequest) }
func (ReqGetStateIDs) NewResponse(code int) core.Coder      { return new(external.GetStateIDsResponse) }
func (ReqGetStateIDs) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetStateIDsRequest)
	msg.RoomID = vars["roomId"]
	msg.EventID = req.FormValue("event_id")
	return nil
}
func (ReqGetStateIDs) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetStateIDsRequest)
	idg := ud.(*FedApiUserData).Idg

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_STATE_IDS

	log.Debugf("get state ids req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.GetStateIDsResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqGetPublicRooms struct{}

func (ReqGetPublicRooms) GetRoute() string                     { return "/publicRooms" }
func (ReqGetPublicRooms) GetMetricsName() string               { return "get_public_rooms" }
func (ReqGetPublicRooms) GetMsgType() int32                    { return internals.MSG_GET_FED_PUBLIC_ROOMS }
func (ReqGetPublicRooms) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqGetPublicRooms) GetMethod() []string                  { return []string{http.MethodGet} }
func (ReqGetPublicRooms) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPublicRooms) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqGetPublicRooms) NewRequest() core.Coder               { return new(external.GetFedPublicRoomsRequest) }
func (ReqGetPublicRooms) NewResponse(code int) core.Coder {
	return new(external.GetFedPublicRoomsResponse)
}
func (ReqGetPublicRooms) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetFedPublicRoomsRequest)
	limit := req.FormValue("limit")
	msg.Limit, _ = strconv.ParseInt(limit, 10, 64)
	msg.Since = req.FormValue("since")
	includeAllNetworks := req.FormValue("include_all_networks")
	msg.IncludeAllNetworks, _ = strconv.ParseBool(includeAllNetworks)
	msg.ThirdPartyInstanceID = req.FormValue("third_party_instance_id")
	return nil
}
func (ReqGetPublicRooms) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.GetFedPublicRoomsRequest)
	idg := ud.(*FedApiUserData).Idg

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body, _ = json.Marshal(req)
	gobMsg.Cmd = model.CMD_FED_GET_PUBLIC_ROOMS

	log.Debugf("get publicRooms req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.GetFedPublicRoomsResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqPostPublicRooms struct{}

func (ReqPostPublicRooms) GetRoute() string                     { return "/publicRooms" }
func (ReqPostPublicRooms) GetMetricsName() string               { return "post_public_rooms" }
func (ReqPostPublicRooms) GetMsgType() int32                    { return internals.MSG_POST_FED_PUBLIC_ROOMS }
func (ReqPostPublicRooms) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPostPublicRooms) GetMethod() []string                  { return []string{http.MethodPost} }
func (ReqPostPublicRooms) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostPublicRooms) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPostPublicRooms) NewRequest() core.Coder               { return new(external.PostFedPublicRoomsRequest) }
func (ReqPostPublicRooms) NewResponse(code int) core.Coder {
	return new(external.PostFedPublicRoomsResponse)
}
func (ReqPostPublicRooms) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqPostPublicRooms) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	// req := msg.(*external.PostFedPublicRoomsRequest)
	idg := ud.(*FedApiUserData).Idg

	// json.Unmarshal(ud.(*FedApiUserData).Request.Content(), &req)

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = ud.(*FedApiUserData).Request.Content()
	gobMsg.Cmd = model.CMD_FED_POST_PUBLIC_ROOMS

	log.Debugf("post publicRooms req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.PostFedPublicRoomsResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqPostQueryClientKeys struct{}

func (ReqPostQueryClientKeys) GetRoute() string                     { return "/user/keys/query" }
func (ReqPostQueryClientKeys) GetMetricsName() string               { return "query_user_keys" }
func (ReqPostQueryClientKeys) GetMsgType() int32                    { return internals.MSG_GET_FED_CLIENT_KEYS }
func (ReqPostQueryClientKeys) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPostQueryClientKeys) GetMethod() []string                  { return []string{http.MethodPost} }
func (ReqPostQueryClientKeys) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostQueryClientKeys) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPostQueryClientKeys) NewRequest() core.Coder {
	return new(external.PostQueryClientKeysRequest)
}
func (ReqPostQueryClientKeys) NewResponse(code int) core.Coder {
	return new(external.PostQueryClientKeysResponse)
}
func (ReqPostQueryClientKeys) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqPostQueryClientKeys) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	// req := msg.(*external.PostFedPublicRoomsRequest)
	idg := ud.(*FedApiUserData).Idg

	// json.Unmarshal(ud.(*FedApiUserData).Request.Content(), &req)

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = ud.(*FedApiUserData).Request.Content()
	gobMsg.Cmd = model.CMD_FED_CLIENT_KEYS

	log.Debugf("post query client keys req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.PostQueryClientKeysResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}

type ReqPostClaimClientKeys struct{}

func (ReqPostClaimClientKeys) GetRoute() string                     { return "/user/keys/claim" }
func (ReqPostClaimClientKeys) GetMetricsName() string               { return "cliam_user_keys" }
func (ReqPostClaimClientKeys) GetMsgType() int32                    { return internals.MSG_GET_FED_CLIENT_KEYS_CLAIM }
func (ReqPostClaimClientKeys) GetAPIType() int8                     { return apiconsumer.APITypeFed }
func (ReqPostClaimClientKeys) GetMethod() []string                  { return []string{http.MethodPost} }
func (ReqPostClaimClientKeys) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostClaimClientKeys) GetPrefix() []string                  { return []string{"fedV1"} }
func (ReqPostClaimClientKeys) NewRequest() core.Coder {
	return new(external.PostClaimClientKeysRequest)
}
func (ReqPostClaimClientKeys) NewResponse(code int) core.Coder {
	return new(external.PostClaimClientKeysResponse)
}
func (ReqPostClaimClientKeys) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqPostClaimClientKeys) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	// req := msg.(*external.PostFedPublicRoomsRequest)
	idg := ud.(*FedApiUserData).Idg

	// json.Unmarshal(ud.(*FedApiUserData).Request.Content(), &req)

	gobMsg := model.GobMessage{}
	gobMsg.MsgSeq = genMsgSeq(idg)
	gobMsg.Body = ud.(*FedApiUserData).Request.Content()
	gobMsg.Cmd = model.CMD_FED_CLIENT_KEYS_CLAIM

	log.Debugf("post claim client keys req: %s", gobMsg.Body)
	resp, err := bridge.SendAndRecv(gobMsg, 30000)
	if err != nil {
		return http.StatusRequestTimeout, jsonerror.Unknown(err.Error())
	} else if resp.Head.ErrStr != "" {
		return http.StatusInternalServerError, jsonerror.Unknown(resp.Head.ErrStr)
	}

	var res external.PostClaimClientKeysResponse
	json.Unmarshal(resp.Body, &res)

	return http.StatusOK, &res
}
