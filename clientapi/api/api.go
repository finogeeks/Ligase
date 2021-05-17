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
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer

	idg          *uid.UidGenerator
	apiMux       *mux.Router
	rsRpcCli     roomserverapi.RoomserverRPCAPI
	accountDB    model.AccountsDatabase
	deviceDB     model.DeviceDatabase
	federation   *fed.Federation
	keyRing      gomatrixserverlib.KeyRing
	cacheIn      service.Cache
	encryptDB    model.EncryptorAPIDatabase
	syncDB       model.SyncAPIDatabase
	presenceDB   model.PresenceDatabase
	roomDB       model.RoomServerDatabase
	tokenFilter  *filter.Filter
	localcache   *cache.LocalCacheRepo
	settings     *common.Settings
	fedDomians   *common.FedDomains
	complexCache *common.ComplexCache

	monitor             mon.Monitor
	missingEventCounter mon.LabeledCounter
}

func NewInternalMsgConsumer(
	apiMux *mux.Router, cfg config.Dendrite,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	federation *fed.Federation,
	keyRing gomatrixserverlib.KeyRing,
	cacheIn service.Cache,
	encryptDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	presenceDB model.PresenceDatabase,
	roomDB model.RoomServerDatabase,
	rpcCli *common.RpcClient,
	rpcClient rpc.RpcClient,
	tokenFilter *filter.Filter,
	settings *common.Settings,
	fedDomians *common.FedDomains,
	complexCache *common.ComplexCache,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcCli = rpcCli
	c.RpcClient = rpcClient
	c.idg, _ = uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	c.apiMux = apiMux
	c.rsRpcCli = rsRpcCli
	c.accountDB = accountDB
	c.deviceDB = deviceDB
	c.federation = federation
	c.keyRing = keyRing
	c.cacheIn = cacheIn
	c.encryptDB = encryptDB
	c.syncDB = syncDB
	c.presenceDB = presenceDB
	c.roomDB = roomDB
	c.tokenFilter = tokenFilter
	c.localcache = new(cache.LocalCacheRepo)
	c.localcache.Start(1, cfg.Cache.DurationDefault)
	c.settings = settings
	c.fedDomians = fedDomians
	c.complexCache = complexCache
	c.monitor = mon.GetInstance()
	c.missingEventCounter = c.monitor.NewLabeledCounter("dendrite_missing_event", []string{"roomID"})
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("clientapi", c, c.Cfg.Rpc.ProxyClientApiTopic, &c.Cfg.Rpc.FrontClientApiApi)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyClientApiTopic
}

func init() {
	apiconsumer.SetServices("front_clientapi_api")
	apiconsumer.SetAPIProcessor(ReqPostCreateRoom{})
	apiconsumer.SetAPIProcessor(ReqPostJoinRoomByIDOrAlias{})
	apiconsumer.SetAPIProcessor(ReqPostRoomMembership{})
	apiconsumer.SetAPIProcessor(ReqPutRoomSendWithTypeAndTxnID{})
	apiconsumer.SetAPIProcessor(ReqPutRoomStateWidthType{})
	apiconsumer.SetAPIProcessor(ReqPutRoomStateWidthTypeAndKey{})
	apiconsumer.SetAPIProcessor(ReqPutRoomRedact{})
	apiconsumer.SetAPIProcessor(ReqPutRoomRedactWithTxnID{})
	apiconsumer.SetAPIProcessor(ReqPutRoomUpdate{})
	apiconsumer.SetAPIProcessor(ReqPutRoomUpdateWithTxnID{})
	apiconsumer.SetAPIProcessor(ReqPostRegister{})
	apiconsumer.SetAPIProcessor(ReqPostRegisterLegacy{})
	apiconsumer.SetAPIProcessor(ReqGetRegitsterAvailable{})
	apiconsumer.SetAPIProcessor(ReqGetDirectoryRoomAlias{})
	apiconsumer.SetAPIProcessor(ReqPutDirectoryRoomAlias{})
	apiconsumer.SetAPIProcessor(ReqDelDirectoryRoomAlias{})
	apiconsumer.SetAPIProcessor(ReqPostLogout{})
	apiconsumer.SetAPIProcessor(ReqPostLogoutAll{})
	apiconsumer.SetAPIProcessor(ReqGetLogin{})
	apiconsumer.SetAPIProcessor(ReqPostLogin{})
	apiconsumer.SetAPIProcessor(ReqGetLoginAdmin{})
	apiconsumer.SetAPIProcessor(ReqPostLoginAdmin{})
	apiconsumer.SetAPIProcessor(ReqPostUserFilter{})
	apiconsumer.SetAPIProcessor(ReqGetUserFilterWithID{})
	apiconsumer.SetAPIProcessor(ReqGetProfile{})
	apiconsumer.SetAPIProcessor(ReqGetProfiles{})
	apiconsumer.SetAPIProcessor(ReqGetAvatarURL{})
	apiconsumer.SetAPIProcessor(ReqPutAvatarURL{})
	apiconsumer.SetAPIProcessor(ReqGetDisplayName{})
	apiconsumer.SetAPIProcessor(ReqPutDisplayName{})
	apiconsumer.SetAPIProcessor(ReqGetAssociated3PIDs{})
	apiconsumer.SetAPIProcessor(ReqPostAssociated3PIDs{})
	apiconsumer.SetAPIProcessor(ReqPostAssociated3PIDsDel{})
	apiconsumer.SetAPIProcessor(ReqPostAccount3PIDEmail{})
	apiconsumer.SetAPIProcessor(ReqGetVoipTurnServer{})
	apiconsumer.SetAPIProcessor(ReqGetThirdpartyProtos{})
	apiconsumer.SetAPIProcessor(ReqGetDevicesByUserID{})
	apiconsumer.SetAPIProcessor(ReqGetDeviceByID{})
	apiconsumer.SetAPIProcessor(ReqPutDevice{})
	apiconsumer.SetAPIProcessor(ReqDelDevice{})
	apiconsumer.SetAPIProcessor(ReqPostDelDevices{})
	apiconsumer.SetAPIProcessor(ReqPutPresenceByID{})
	apiconsumer.SetAPIProcessor(ReqGetPresenceByID{})
	apiconsumer.SetAPIProcessor(ReqGetPresenceListByID{})
	apiconsumer.SetAPIProcessor(ReqPostPresenceListByID{})
	apiconsumer.SetAPIProcessor(ReqGetRoomsTagsByID{})
	apiconsumer.SetAPIProcessor(ReqPutRoomsTagsByID{})
	apiconsumer.SetAPIProcessor(ReqDelRoomsTagsByID{})
	apiconsumer.SetAPIProcessor(ReqPutUserAccountData{})
	apiconsumer.SetAPIProcessor(ReqPutRoomAccountData{})
	apiconsumer.SetAPIProcessor(ReqGetWhoAmi{})
	apiconsumer.SetAPIProcessor(ReqGetDirectoryRoom{})
	apiconsumer.SetAPIProcessor(ReqPutDirectoryRoom{})
	apiconsumer.SetAPIProcessor(ReqGetJoinedMember{})
	apiconsumer.SetAPIProcessor(ReqGetUserNewToken{})
	apiconsumer.SetAPIProcessor(ReqGetSuperAdminToken{})
	apiconsumer.SetAPIProcessor(ReqGetSetting{})
	apiconsumer.SetAPIProcessor(ReqPutSetting{})
	apiconsumer.SetAPIProcessor(ReqGetRoomInfo{})
	apiconsumer.SetAPIProcessor(ReqInternalGetRoomInfo{})
	apiconsumer.SetAPIProcessor(ReqPostReport{})
	apiconsumer.SetAPIProcessor(ReqGetUserInfo{})
	apiconsumer.SetAPIProcessor(ReqPutUserInfo{})
	apiconsumer.SetAPIProcessor(ReqPostUserInfo{})
	apiconsumer.SetAPIProcessor(ReqDeleteUserInfo{})
	apiconsumer.SetAPIProcessor(ReqDismissRoom{})
}

type ReqPostCreateRoom struct{}

func (ReqPostCreateRoom) GetRoute() string                     { return "/createRoom" }
func (ReqPostCreateRoom) GetMetricsName() string               { return "createRoom" }
func (ReqPostCreateRoom) GetMsgType() int32                    { return internals.MSG_POST_CREATEROOM }
func (ReqPostCreateRoom) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostCreateRoom) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostCreateRoom) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostCreateRoom) GetPrefix() []string                  { return []string{"r0", "v1", "inr0"} }
func (ReqPostCreateRoom) NewRequest() core.Coder               { return new(external.PostCreateRoomRequest) }
func (ReqPostCreateRoom) NewResponse(code int) core.Coder {
	return new(external.PostCreateRoomResponse)
}
func (ReqPostCreateRoom) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostCreateRoomRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostCreateRoom) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostCreateRoomRequest)
	userID := device.UserID
	return routing.CreateRoom(
		context.Background(), req, userID, c.Cfg, c.accountDB,
		c.rsRpcCli, c.cacheIn, c.idg, c.complexCache,
	)
}

type ReqPostJoinRoomByIDOrAlias struct{}

func (ReqPostJoinRoomByIDOrAlias) GetRoute() string       { return "/join/{roomIDOrAlias}" }
func (ReqPostJoinRoomByIDOrAlias) GetMetricsName() string { return "join" }
func (ReqPostJoinRoomByIDOrAlias) GetMsgType() int32      { return internals.MSG_POST_JOIN_ALIAS }
func (ReqPostJoinRoomByIDOrAlias) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostJoinRoomByIDOrAlias) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostJoinRoomByIDOrAlias) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostJoinRoomByIDOrAlias) NewRequest() core.Coder {
	return new(external.PostRoomsJoinByAliasRequest)
}
func (ReqPostJoinRoomByIDOrAlias) NewResponse(code int) core.Coder {
	return new(external.PostRoomsJoinByAliasResponse)
}
func (ReqPostJoinRoomByIDOrAlias) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostRoomsJoinByAliasRequest)
	if vars != nil {
		msg.RoomID = vars["roomIDOrAlias"]
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostJoinRoomByIDOrAlias) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPostJoinRoomByIDOrAlias) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostRoomsJoinByAliasRequest)
	userID := device.UserID
	return routing.JoinRoomByIDOrAlias(
		context.Background(), req, userID, req.RoomID, c.Cfg, c.federation,
		c.rsRpcCli, c.keyRing, c.cacheIn, c.idg, c.complexCache,
	)
}

type ReqPostRoomMembership struct{}

func (ReqPostRoomMembership) GetRoute() string {
	return "/rooms/{roomID}/{membership:(?:join|kick|ban|unban|leave|invite|forget)}"
}
func (ReqPostRoomMembership) GetMetricsName() string { return "membership" }
func (ReqPostRoomMembership) GetMsgType() int32      { return internals.MSG_POST_ROOM_MEMBERSHIP }
func (ReqPostRoomMembership) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostRoomMembership) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostRoomMembership) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostRoomMembership) NewRequest() core.Coder {
	return new(external.PostRoomsMembershipRequest)
}
func (ReqPostRoomMembership) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostRoomsMembershipRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.Membership = vars["membership"]
	}

	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostRoomMembership) NewResponse(code int) core.Coder {
	return new(external.PostRoomsMembershipResponse)
}
func (ReqPostRoomMembership) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPostRoomMembership) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostRoomsMembershipRequest)
	userID := device.UserID
	deviceID := device.ID
	return routing.SendMembership(
		context.Background(), req, c.accountDB, userID, deviceID, req.RoomID,
		req.Membership, c.Cfg, c.rsRpcCli, c.federation, c.cacheIn, c.idg, c.complexCache,
	)
}

type ReqPutRoomSendWithTypeAndTxnID struct{}

func (ReqPutRoomSendWithTypeAndTxnID) GetRoute() string {
	return "/rooms/{roomID}/send/{eventType}/{txnID}"
}
func (ReqPutRoomSendWithTypeAndTxnID) GetMetricsName() string { return "send_message" }
func (ReqPutRoomSendWithTypeAndTxnID) GetMsgType() int32 {
	return internals.MSG_PUT_ROOM_SEND_WITH_TYPE_AND_TXNID
}
func (ReqPutRoomSendWithTypeAndTxnID) GetAPIType() int8 { return apiconsumer.APITypeAuth }
func (ReqPutRoomSendWithTypeAndTxnID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutRoomSendWithTypeAndTxnID) GetTopic(cfg *config.Dendrite) string {
	return getProxyRpcTopic(cfg)
}
func (ReqPutRoomSendWithTypeAndTxnID) NewRequest() core.Coder {
	return new(external.PutRoomStateByTypeWithTxnID)
}
func (ReqPutRoomSendWithTypeAndTxnID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomStateByTypeWithTxnID)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventType = vars["eventType"]
		msg.StateKey = vars["stateKey"]
		msg.TxnID = vars["txnID"]
		msg.IP = common.GetRemoteIP(req)
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutRoomSendWithTypeAndTxnID) NewResponse(code int) core.Coder {
	return new(external.PutRoomStateByTypeWithTxnIDResponse)
}
func (ReqPutRoomSendWithTypeAndTxnID) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomSendWithTypeAndTxnID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomStateByTypeWithTxnID)
	userID := device.UserID
	deviceID := device.ID
	return routing.PostEvent(
		context.Background(), req, userID, deviceID, req.IP, req.RoomID,
		req.EventType, &req.TxnID, nil, c.Cfg, c.rsRpcCli, c.localcache, c.idg,
	)
}

type ReqPutRoomStateWidthType struct{}

func (ReqPutRoomStateWidthType) GetRoute() string {
	return "/rooms/{roomID}/state/{eventType:[^/]+/?}"
}
func (ReqPutRoomStateWidthType) GetMetricsName() string { return "send_message" }
func (ReqPutRoomStateWidthType) GetMsgType() int32      { return internals.MSG_PUT_ROOM_STATE_WITH_TYPE }
func (ReqPutRoomStateWidthType) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutRoomStateWidthType) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutRoomStateWidthType) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomStateWidthType) NewRequest() core.Coder {
	return new(external.PutRoomStateByTypeWithTxnID)
}
func (ReqPutRoomStateWidthType) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomStateByTypeWithTxnID)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventType = vars["eventType"]
		msg.StateKey = vars["stateKey"]
		msg.TxnID = vars["txnID"]
		msg.IP = common.GetRemoteIP(req)
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutRoomStateWidthType) NewResponse(code int) core.Coder {
	return new(external.PutRoomStateByTypeWithTxnIDResponse)
}
func (ReqPutRoomStateWidthType) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomStateWidthType) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomStateByTypeWithTxnID)
	userID := device.UserID
	deviceID := device.ID
	emptyString := ""
	eventType := req.EventType
	// If there's a trailing slash, remove it
	if strings.HasSuffix(eventType, "/") {
		eventType = eventType[:len(eventType)-1]
	}
	return routing.PostEvent(
		context.Background(), req, userID, deviceID, req.IP, req.RoomID,
		eventType, nil, &emptyString, c.Cfg, c.rsRpcCli, c.localcache, c.idg,
	)
}

type ReqPutRoomStateWidthTypeAndKey struct{}

func (ReqPutRoomStateWidthTypeAndKey) GetRoute() string {
	return "/rooms/{roomID}/state/{eventType}/{stateKey}"
}
func (ReqPutRoomStateWidthTypeAndKey) GetMetricsName() string { return "send_message" }
func (ReqPutRoomStateWidthTypeAndKey) GetMsgType() int32 {
	return internals.MSG_PUT_ROOM_STATE_WITH_TYPE_AND_KEY
}
func (ReqPutRoomStateWidthTypeAndKey) GetAPIType() int8 { return apiconsumer.APITypeAuth }
func (ReqPutRoomStateWidthTypeAndKey) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutRoomStateWidthTypeAndKey) GetTopic(cfg *config.Dendrite) string {
	return getProxyRpcTopic(cfg)
}
func (ReqPutRoomStateWidthTypeAndKey) NewRequest() core.Coder {
	return new(external.PutRoomStateByTypeWithTxnID)
}
func (ReqPutRoomStateWidthTypeAndKey) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomStateByTypeWithTxnID)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventType = vars["eventType"]
		msg.StateKey = vars["stateKey"]
		msg.TxnID = vars["txnID"]
		msg.IP = common.GetRemoteIP(req)
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutRoomStateWidthTypeAndKey) NewResponse(code int) core.Coder {
	return new(external.PutRoomStateByTypeWithTxnIDResponse)
}
func (ReqPutRoomStateWidthTypeAndKey) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomStateWidthTypeAndKey) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomStateByTypeWithTxnID)
	userID := device.UserID
	deviceID := device.ID
	stateKey := req.StateKey
	return routing.PostEvent(
		context.Background(), req, userID, deviceID, req.IP, req.RoomID,
		req.EventType, nil, &stateKey, c.Cfg, c.rsRpcCli, c.localcache, c.idg,
	)
}

type ReqPutRoomRedact struct{}

func (ReqPutRoomRedact) GetRoute() string       { return "/rooms/{roomID}/redact/{eventId}" }
func (ReqPutRoomRedact) GetMetricsName() string { return "redact_event" }
func (ReqPutRoomRedact) GetMsgType() int32      { return internals.MSG_PUT_ROOM_REDACT }
func (ReqPutRoomRedact) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutRoomRedact) GetMethod() []string {
	return []string{http.MethodPut, http.MethodPost, http.MethodOptions}
}
func (ReqPutRoomRedact) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomRedact) NewRequest() core.Coder {
	return new(external.PutRedactEventRequest)
}
func (ReqPutRoomRedact) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRedactEventRequest)
	if vars != nil {
		msg.EventID = vars["eventId"]
		msg.RoomID = vars["roomID"]
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, msg); err != nil {
		return err
	}
	msg.Content = data
	return nil
}
func (ReqPutRoomRedact) NewResponse(code int) core.Coder {
	return new(external.PutRedactEventResponse)
}
func (ReqPutRoomRedact) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomRedact) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRedactEventRequest)
	userID := device.UserID
	deviceID := device.ID
	return routing.RedactEvent(
		context.Background(), req.Content, userID, deviceID, req.RoomID,
		nil, nil, c.Cfg, c.localcache, c.rsRpcCli, req.EventID, "m.room.redaction", c.idg,
	)
}

type ReqPutRoomRedactWithTxnID struct{}

func (ReqPutRoomRedactWithTxnID) GetRoute() string {
	return "/rooms/{roomID}/redact/{eventId}/{txnID}"
}
func (ReqPutRoomRedactWithTxnID) GetMetricsName() string { return "redact_event" }
func (ReqPutRoomRedactWithTxnID) GetMsgType() int32 {
	return internals.MSG_PUT_ROOM_REDACT_WITH_TXNID
}
func (ReqPutRoomRedactWithTxnID) GetAPIType() int8 { return apiconsumer.APITypeAuth }
func (ReqPutRoomRedactWithTxnID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodPost, http.MethodOptions}
}
func (ReqPutRoomRedactWithTxnID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomRedactWithTxnID) NewRequest() core.Coder {
	return new(external.PutRedactWithTxnIDEventRequest)
}
func (ReqPutRoomRedactWithTxnID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRedactWithTxnIDEventRequest)
	if vars != nil {
		msg.EventID = vars["eventId"]
		msg.RoomID = vars["roomID"]
		msg.TxnID = vars["txnID"]
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, msg); err != nil {
		return err
	}
	msg.Content = data
	return nil
}
func (ReqPutRoomRedactWithTxnID) NewResponse(code int) core.Coder {
	return new(external.PutRedactEventResponse)
}
func (ReqPutRoomRedactWithTxnID) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomRedactWithTxnID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRedactWithTxnIDEventRequest)
	userID := device.UserID
	deviceID := device.ID
	txnID := req.TxnID
	return routing.RedactEvent(
		context.Background(), req.Content, userID, deviceID, req.RoomID,
		&txnID, nil, c.Cfg, c.localcache, c.rsRpcCli, req.EventID, "m.room.redaction", c.idg,
	)
}

type ReqPutRoomUpdate struct{}

func (ReqPutRoomUpdate) GetRoute() string       { return "/rooms/{roomID}/update/{eventId}" }
func (ReqPutRoomUpdate) GetMetricsName() string { return "update_event" }
func (ReqPutRoomUpdate) GetMsgType() int32      { return internals.MSG_PUT_ROOM_UPDATE }
func (ReqPutRoomUpdate) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutRoomUpdate) GetMethod() []string {
	return []string{http.MethodPut, http.MethodPost, http.MethodOptions}
}
func (ReqPutRoomUpdate) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomUpdate) NewRequest() core.Coder {
	return new(external.PutRedactEventRequest)
}
func (ReqPutRoomUpdate) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRedactEventRequest)
	if vars != nil {
		msg.EventID = vars["eventId"]
		msg.RoomID = vars["roomID"]
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, msg); err != nil {
		return err
	}
	msg.Content = data
	return nil
}
func (ReqPutRoomUpdate) NewResponse(code int) core.Coder {
	return new(external.PutRedactEventResponse)
}
func (ReqPutRoomUpdate) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomUpdate) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRedactEventRequest)
	userID := device.UserID
	deviceID := device.ID
	return routing.RedactEvent(
		context.Background(), req.Content, userID, deviceID, req.RoomID,
		nil, nil, c.Cfg, c.localcache, c.rsRpcCli, req.EventID, "m.room.update", c.idg,
	)
}

type ReqPutRoomUpdateWithTxnID struct{}

func (ReqPutRoomUpdateWithTxnID) GetRoute() string {
	return "/rooms/{roomID}/update/{eventId}/{txnID}"
}
func (ReqPutRoomUpdateWithTxnID) GetMetricsName() string { return "update_event" }
func (ReqPutRoomUpdateWithTxnID) GetMsgType() int32 {
	return internals.MSG_PUT_ROOM_UPDATE_WITH_TXNID
}
func (ReqPutRoomUpdateWithTxnID) GetAPIType() int8 { return apiconsumer.APITypeAuth }
func (ReqPutRoomUpdateWithTxnID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodPost, http.MethodOptions}
}
func (ReqPutRoomUpdateWithTxnID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomUpdateWithTxnID) NewRequest() core.Coder {
	return new(external.PutRedactWithTxnIDEventRequest)
}
func (ReqPutRoomUpdateWithTxnID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRedactWithTxnIDEventRequest)
	if vars != nil {
		msg.EventID = vars["eventId"]
		msg.RoomID = vars["roomID"]
		msg.TxnID = vars["txnID"]
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, msg); err != nil {
		return err
	}
	msg.Content = data
	return nil
}
func (ReqPutRoomUpdateWithTxnID) NewResponse(code int) core.Coder {
	return new(external.PutRedactEventResponse)
}
func (ReqPutRoomUpdateWithTxnID) GetPrefix() []string { return []string{"r0", "v1", "inr0"} }
func (ReqPutRoomUpdateWithTxnID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRedactWithTxnIDEventRequest)
	userID := device.UserID
	deviceID := device.ID
	txnID := req.TxnID
	return routing.RedactEvent(
		context.Background(), req.Content, userID, deviceID, req.RoomID,
		&txnID, nil, c.Cfg, c.localcache, c.rsRpcCli, req.EventID, "m.room.update", c.idg,
	)
}

type ReqPostRegister struct{}

func (ReqPostRegister) GetRoute() string                     { return "/register" }
func (ReqPostRegister) GetMetricsName() string               { return "register" }
func (ReqPostRegister) GetMsgType() int32                    { return internals.MSG_POST_REGISTER }
func (ReqPostRegister) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqPostRegister) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostRegister) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostRegister) NewRequest() core.Coder {
	return new(external.PostRegisterRequest)
}
func (ReqPostRegister) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostRegisterRequest)
	query := req.URL.Query()
	msg.Kind = query.Get("kind")
	msg.Domain = query.Get("domain")
	msg.AccessToken = query.Get("access_token")
	msg.RemoteAddr = req.RemoteAddr
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostRegister) NewResponse(code int) core.Coder {
	if code == http.StatusUnauthorized {
		return new(external.UserInteractiveResponse)
	} else {
		return new(external.RegisterResponse)
	}
}
func (ReqPostRegister) GetPrefix() []string { return []string{"r0"} }
func (ReqPostRegister) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostRegisterRequest)
	return routing.Register(
		context.Background(), req, c.accountDB, c.deviceDB, &c.Cfg, c.idg,
	)
}

type ReqPostRegisterLegacy struct{}

func (ReqPostRegisterLegacy) GetRoute() string       { return "/register" }
func (ReqPostRegisterLegacy) GetMetricsName() string { return "register" }
func (ReqPostRegisterLegacy) GetMsgType() int32      { return internals.MSG_POST_REGISTER_LEGACY }
func (ReqPostRegisterLegacy) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqPostRegisterLegacy) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostRegisterLegacy) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostRegisterLegacy) NewRequest() core.Coder {
	return new(external.LegacyRegisterRequest)
}
func (ReqPostRegisterLegacy) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.LegacyRegisterRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	// Squash username to all lowercase letters
	msg.Username = strings.ToLower(msg.Username)
	return nil
}
func (ReqPostRegisterLegacy) NewResponse(code int) core.Coder {
	return new(external.RegisterResponse)
}
func (ReqPostRegisterLegacy) GetPrefix() []string { return []string{"v1"} }
func (ReqPostRegisterLegacy) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.LegacyRegisterRequest)
	return routing.LegacyRegister(
		context.Background(), req, c.accountDB, c.deviceDB, &c.Cfg, c.idg,
	)
}

type ReqGetRegitsterAvailable struct{}

func (ReqGetRegitsterAvailable) GetRoute() string       { return "/register/available" }
func (ReqGetRegitsterAvailable) GetMetricsName() string { return "registerAvailable" }
func (ReqGetRegitsterAvailable) GetMsgType() int32      { return internals.MSG_GET_REGISTER_AVAILABLE }
func (ReqGetRegitsterAvailable) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetRegitsterAvailable) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRegitsterAvailable) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRegitsterAvailable) NewRequest() core.Coder               { return nil }
func (ReqGetRegitsterAvailable) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetRegitsterAvailable) NewResponse(code int) core.Coder {
	return new(external.GetRegisterAvailResponse)
}
func (ReqGetRegitsterAvailable) GetPrefix() []string { return []string{"r0"} }
func (ReqGetRegitsterAvailable) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.RegisterAvailable()
}

type ReqGetDirectoryRoomAlias struct{}

func (ReqGetDirectoryRoomAlias) GetRoute() string       { return "/directory/room/{roomAlias}" }
func (ReqGetDirectoryRoomAlias) GetMetricsName() string { return "directory_room" }
func (ReqGetDirectoryRoomAlias) GetMsgType() int32      { return internals.MSG_GET_DIRECTORY_ROOM_ALIAS }
func (ReqGetDirectoryRoomAlias) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetDirectoryRoomAlias) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetDirectoryRoomAlias) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetDirectoryRoomAlias) NewRequest() core.Coder {
	return new(external.GetDirectoryRoomAliasRequest)
}
func (ReqGetDirectoryRoomAlias) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetDirectoryRoomAliasRequest)
	if vars != nil {
		msg.RoomAlias = vars["roomAlias"]
	}
	return nil
}
func (ReqGetDirectoryRoomAlias) NewResponse(code int) core.Coder {
	return new(external.GetDirectoryRoomAliasResponse)
}
func (ReqGetDirectoryRoomAlias) GetPrefix() []string { return []string{"r0"} }
func (ReqGetDirectoryRoomAlias) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetDirectoryRoomAliasRequest)
	return routing.DirectoryRoom(
		context.Background(), req.RoomAlias, c.federation, &c.Cfg, c.rsRpcCli,
	)
}

type ReqPutDirectoryRoomAlias struct{}

func (ReqPutDirectoryRoomAlias) GetRoute() string       { return "/directory/room/{roomAlias}" }
func (ReqPutDirectoryRoomAlias) GetMetricsName() string { return "directory_room" }
func (ReqPutDirectoryRoomAlias) GetMsgType() int32      { return internals.MSG_PUT_DIRECTORY_ROOM_ALIAS }
func (ReqPutDirectoryRoomAlias) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutDirectoryRoomAlias) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutDirectoryRoomAlias) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutDirectoryRoomAlias) NewRequest() core.Coder {
	return new(external.PutDirectoryRoomAliasRequest)
}
func (ReqPutDirectoryRoomAlias) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutDirectoryRoomAliasRequest)
	if vars != nil {
		msg.RoomAlias = vars["roomAlias"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutDirectoryRoomAlias) NewResponse(code int) core.Coder { return nil }
func (ReqPutDirectoryRoomAlias) GetPrefix() []string             { return []string{"r0"} }
func (ReqPutDirectoryRoomAlias) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutDirectoryRoomAliasRequest)
	return routing.SetLocalAlias(
		context.Background(), req, device.UserID, req.RoomAlias, &c.Cfg, c.rsRpcCli,
	)
}

type ReqDelDirectoryRoomAlias struct{}

func (ReqDelDirectoryRoomAlias) GetRoute() string       { return "/directory/room/{roomAlias}" }
func (ReqDelDirectoryRoomAlias) GetMetricsName() string { return "directory_room" }
func (ReqDelDirectoryRoomAlias) GetMsgType() int32      { return internals.MSG_DEL_DIRECTORY_ROOM_ALIAS }
func (ReqDelDirectoryRoomAlias) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqDelDirectoryRoomAlias) GetMethod() []string {
	return []string{http.MethodDelete, http.MethodOptions}
}
func (ReqDelDirectoryRoomAlias) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqDelDirectoryRoomAlias) NewRequest() core.Coder {
	return new(external.DelDirectoryRoomAliasRequest)
}
func (ReqDelDirectoryRoomAlias) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.DelDirectoryRoomAliasRequest)
	if vars != nil {
		msg.RoomAlias = vars["roomAlias"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqDelDirectoryRoomAlias) NewResponse(code int) core.Coder { return nil }
func (ReqDelDirectoryRoomAlias) GetPrefix() []string             { return []string{"r0"} }
func (ReqDelDirectoryRoomAlias) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.DelDirectoryRoomAliasRequest)
	return routing.RemoveLocalAlias(
		context.Background(), device.UserID, req.RoomAlias, c.rsRpcCli,
	)
}

type ReqPostLogout struct{}

func (ReqPostLogout) GetRoute() string                     { return "/logout" }
func (ReqPostLogout) GetMetricsName() string               { return "logout" }
func (ReqPostLogout) GetMsgType() int32                    { return internals.MSG_POST_LOGOUT }
func (ReqPostLogout) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostLogout) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostLogout) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostLogout) NewRequest() core.Coder               { return nil }
func (ReqPostLogout) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqPostLogout) NewResponse(code int) core.Coder { return nil }
func (ReqPostLogout) GetPrefix() []string             { return []string{"r0"} }
func (ReqPostLogout) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	return routing.Logout(
		c.deviceDB, device.UserID, device.ID, c.cacheIn, c.encryptDB,
		c.syncDB, c.tokenFilter, c.RpcCli, c.RpcClient,
	)
}

type ReqPostLogoutAll struct{}

func (ReqPostLogoutAll) GetRoute() string                     { return "/logout/all" }
func (ReqPostLogoutAll) GetMetricsName() string               { return "logout_all" }
func (ReqPostLogoutAll) GetMsgType() int32                    { return internals.MSG_POST_LOGOUT_ALL }
func (ReqPostLogoutAll) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostLogoutAll) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostLogoutAll) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostLogoutAll) NewRequest() core.Coder               { return nil }
func (ReqPostLogoutAll) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqPostLogoutAll) NewResponse(code int) core.Coder { return nil }
func (ReqPostLogoutAll) GetPrefix() []string             { return []string{"r0"} }
func (ReqPostLogoutAll) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	return routing.LogoutAll(
		c.deviceDB, device.UserID, device.ID, c.cacheIn, c.encryptDB,
		c.syncDB, c.tokenFilter, c.RpcCli, c.RpcClient,
	)
}

type ReqGetLogin struct{}

func (ReqGetLogin) GetRoute() string                     { return "/login" }
func (ReqGetLogin) GetMetricsName() string               { return "login" }
func (ReqGetLogin) GetMsgType() int32                    { return internals.MSG_GET_LOGIN }
func (ReqGetLogin) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetLogin) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetLogin) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetLogin) NewRequest() core.Coder {
	return new(external.GetLoginRequest)
}
func (ReqGetLogin) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetLogin) NewResponse(code int) core.Coder { return new(external.GetLoginResponse) }
func (ReqGetLogin) GetPrefix() []string             { return []string{"r0", "v1"} }
func (ReqGetLogin) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetLoginRequest)
	return routing.LoginGet(
		context.Background(), req, c.accountDB, c.deviceDB, c.encryptDB,
		c.syncDB, c.Cfg, false, c.idg, c.tokenFilter, c.RpcCli,
	)
}

type ReqPostLogin struct{}

func (ReqPostLogin) GetRoute() string                     { return "/login" }
func (ReqPostLogin) GetMetricsName() string               { return "login" }
func (ReqPostLogin) GetMsgType() int32                    { return internals.MSG_POST_LOGIN }
func (ReqPostLogin) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqPostLogin) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostLogin) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostLogin) NewRequest() core.Coder {
	return new(external.PostLoginRequest)
}
func (ReqPostLogin) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostLoginRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	msg.IP = common.GetRemoteIP(req)
	return nil
}
func (ReqPostLogin) NewResponse(code int) core.Coder { return new(external.PostLoginResponse) }
func (ReqPostLogin) GetPrefix() []string             { return []string{"r0", "v1"} }
func (ReqPostLogin) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostLoginRequest)
	return routing.LoginPost(
		context.Background(), req, c.accountDB, c.deviceDB, c.encryptDB,
		c.syncDB, c.Cfg, false, c.idg, c.tokenFilter, c.RpcCli, c.RpcClient,
	)
}

type ReqGetLoginAdmin struct{}

func (ReqGetLoginAdmin) GetRoute() string                     { return "/adminlogin" }
func (ReqGetLoginAdmin) GetMetricsName() string               { return "login" }
func (ReqGetLoginAdmin) GetMsgType() int32                    { return internals.MSG_GET_LOGIN_ADMIN }
func (ReqGetLoginAdmin) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetLoginAdmin) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetLoginAdmin) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetLoginAdmin) NewRequest() core.Coder {
	return new(external.GetLoginAdminRequest)
}
func (ReqGetLoginAdmin) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetLoginAdmin) NewResponse(code int) core.Coder { return new(external.GetLoginResponse) }
func (ReqGetLoginAdmin) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetLoginAdmin) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetLoginRequest)
	return routing.LoginGet(
		context.Background(), req, c.accountDB, c.deviceDB, c.encryptDB,
		c.syncDB, c.Cfg, true, c.idg, c.tokenFilter, c.RpcCli,
	)
}

type ReqPostLoginAdmin struct{}

func (ReqPostLoginAdmin) GetRoute() string                     { return "/adminlogin" }
func (ReqPostLoginAdmin) GetMetricsName() string               { return "login" }
func (ReqPostLoginAdmin) GetMsgType() int32                    { return internals.MSG_POST_LOGIN_ADMIN }
func (ReqPostLoginAdmin) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqPostLoginAdmin) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostLoginAdmin) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostLoginAdmin) NewRequest() core.Coder {
	return new(external.PostLoginAdminRequest)
}
func (ReqPostLoginAdmin) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostLoginAdminRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	msg.IP = common.GetRemoteIP(req)
	return nil
}
func (ReqPostLoginAdmin) NewResponse(code int) core.Coder { return new(external.PostLoginResponse) }
func (ReqPostLoginAdmin) GetPrefix() []string             { return []string{"r0"} }
func (ReqPostLoginAdmin) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostLoginRequest)
	return routing.LoginPost(
		context.Background(), req, c.accountDB, c.deviceDB, c.encryptDB,
		c.syncDB, c.Cfg, true, c.idg, c.tokenFilter, c.RpcCli, c.RpcClient,
	)
}

type ReqPostUserFilter struct{}

func (ReqPostUserFilter) GetRoute() string                     { return "/user/{userId}/filter" }
func (ReqPostUserFilter) GetMetricsName() string               { return "put_filter" }
func (ReqPostUserFilter) GetMsgType() int32                    { return internals.MSG_POST_USER_FILTER }
func (ReqPostUserFilter) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostUserFilter) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostUserFilter) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostUserFilter) NewRequest() core.Coder {
	return new(external.PostUserFilterRequest)
}
func (ReqPostUserFilter) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostUserFilterRequest)
	if vars != nil {
		msg.UserID = vars["userId"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostUserFilter) NewResponse(code int) core.Coder {
	return new(external.PostUserFilterResponse)
}
func (ReqPostUserFilter) GetPrefix() []string { return []string{"r0"} }
func (ReqPostUserFilter) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostUserFilterRequest)
	return routing.PutFilter(
		context.Background(), req, device.UserID, c.accountDB, c.cacheIn,
	)
}

type ReqGetUserFilterWithID struct{}

func (ReqGetUserFilterWithID) GetRoute() string       { return "/user/{userId}/filter/{filterId}" }
func (ReqGetUserFilterWithID) GetMetricsName() string { return "get_filter" }
func (ReqGetUserFilterWithID) GetMsgType() int32      { return internals.MSG_GET_USER_FILTER_WITH_ID }
func (ReqGetUserFilterWithID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetUserFilterWithID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetUserFilterWithID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetUserFilterWithID) NewRequest() core.Coder {
	return new(external.GetUserFilterRequest)
}
func (ReqGetUserFilterWithID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetUserFilterRequest)
	if vars != nil {
		msg.UserID = vars["userId"]
		msg.FilterID = vars["filterId"]
	}
	return nil
}
func (ReqGetUserFilterWithID) NewResponse(code int) core.Coder {
	return new(external.GetUserFilterResponse)
}
func (ReqGetUserFilterWithID) GetPrefix() []string { return []string{"r0"} }
func (ReqGetUserFilterWithID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetUserFilterRequest)
	return routing.GetFilter(
		context.Background(), req, device.UserID, c.cacheIn,
	)
}

type ReqGetProfile struct{}

func (ReqGetProfile) GetRoute() string                     { return "/profile/{userID}" }
func (ReqGetProfile) GetMetricsName() string               { return "profile" }
func (ReqGetProfile) GetMsgType() int32                    { return internals.MSG_GET_PROFILE }
func (ReqGetProfile) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetProfile) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetProfile) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetProfile) NewRequest() core.Coder {
	return new(external.GetProfileRequest)
}
func (ReqGetProfile) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetProfileRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetProfile) NewResponse(code int) core.Coder { return new(external.GetProfileResponse) }
func (ReqGetProfile) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetProfile) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetProfileRequest)
	return routing.GetProfile(
		context.Background(), req, c.Cfg, c.federation, c.accountDB, c.cacheIn, c.fedDomians, c.complexCache,
	)
}

type ReqGetProfiles struct{}

func (ReqGetProfiles) GetRoute() string                     { return "/profiles" }
func (ReqGetProfiles) GetMetricsName() string               { return "profiles" }
func (ReqGetProfiles) GetMsgType() int32                    { return internals.MSG_POST_PROFILES }
func (ReqGetProfiles) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetProfiles) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqGetProfiles) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetProfiles) NewRequest() core.Coder {
	return new(external.GetProfilesRequest)
}
func (ReqGetProfiles) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetProfilesRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqGetProfiles) NewResponse(code int) core.Coder { return new(external.GetProfilesResponse) }
func (ReqGetProfiles) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetProfiles) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetProfilesRequest)
	return routing.GetProfiles(
		context.Background(), req, c.Cfg, c.federation, c.accountDB, c.cacheIn, c.fedDomians, c.complexCache,
	)
}

type ReqGetAvatarURL struct{}

func (ReqGetAvatarURL) GetRoute() string                     { return "/profile/{userID}/avatar_url" }
func (ReqGetAvatarURL) GetMetricsName() string               { return "profile_avatar_url" }
func (ReqGetAvatarURL) GetMsgType() int32                    { return internals.MSG_GET_PROFILE_AVATAR }
func (ReqGetAvatarURL) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetAvatarURL) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetAvatarURL) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetAvatarURL) NewRequest() core.Coder {
	return new(external.GetAvatarURLRequest)
}
func (ReqGetAvatarURL) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetAvatarURLRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetAvatarURL) NewResponse(code int) core.Coder { return new(external.GetAvatarURLResponse) }
func (ReqGetAvatarURL) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetAvatarURL) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetAvatarURLRequest)
	return routing.GetAvatarURL(
		context.Background(), req.UserID, c.Cfg, c.accountDB, c.cacheIn, c.federation, c.fedDomians, c.complexCache,
	)
}

type ReqPutAvatarURL struct{}

func (ReqPutAvatarURL) GetRoute() string                     { return "/profile/{userID}/avatar_url" }
func (ReqPutAvatarURL) GetMetricsName() string               { return "profile_avatar_url" }
func (ReqPutAvatarURL) GetMsgType() int32                    { return internals.MSG_PUT_PROFILE_AVATAR }
func (ReqPutAvatarURL) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPutAvatarURL) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutAvatarURL) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutAvatarURL) NewRequest() core.Coder {
	return new(external.PutAvatarURLRequest)
}
func (ReqPutAvatarURL) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutAvatarURLRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutAvatarURL) NewResponse(code int) core.Coder { return nil }
func (ReqPutAvatarURL) GetPrefix() []string             { return []string{"r0"} }
func (ReqPutAvatarURL) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutAvatarURLRequest)
	return routing.SetAvatarURL(
		context.Background(), req, c.accountDB, device.UserID, &c.Cfg,
		c.cacheIn, c.rsRpcCli, c.idg, c.complexCache,
	)
}

type ReqGetDisplayName struct{}

func (ReqGetDisplayName) GetRoute() string                     { return "/profile/{userID}/displayname" }
func (ReqGetDisplayName) GetMetricsName() string               { return "profile_displayname" }
func (ReqGetDisplayName) GetMsgType() int32                    { return internals.MSG_GET_PROFILE_DISPLAY_NAME }
func (ReqGetDisplayName) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetDisplayName) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetDisplayName) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetDisplayName) NewRequest() core.Coder {
	return new(external.GetDisplayNameRequest)
}
func (ReqGetDisplayName) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetDisplayNameRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetDisplayName) NewResponse(code int) core.Coder {
	return new(external.GetDisplayNameResponse)
}
func (ReqGetDisplayName) GetPrefix() []string { return []string{"r0"} }
func (ReqGetDisplayName) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetDisplayNameRequest)
	return routing.GetDisplayName(
		context.Background(), req.UserID, c.Cfg, c.accountDB, c.cacheIn, c.federation, c.fedDomians, c.complexCache,
	)
}

type ReqPutDisplayName struct{}

func (ReqPutDisplayName) GetRoute() string                     { return "/profile/{userID}/displayname" }
func (ReqPutDisplayName) GetMetricsName() string               { return "profile_displayname" }
func (ReqPutDisplayName) GetMsgType() int32                    { return internals.MSG_PUT_PROFILE_DISPLAY_NAME }
func (ReqPutDisplayName) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPutDisplayName) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutDisplayName) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutDisplayName) NewRequest() core.Coder {
	return new(external.PutDisplayNameRequest)
}
func (ReqPutDisplayName) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutDisplayNameRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutDisplayName) NewResponse(code int) core.Coder { return nil }
func (ReqPutDisplayName) GetPrefix() []string             { return []string{"r0"} }
func (ReqPutDisplayName) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutDisplayNameRequest)
	return routing.SetDisplayName(
		context.Background(), req, c.accountDB, device.UserID, &c.Cfg,
		c.cacheIn, c.rsRpcCli, c.idg, c.complexCache,
	)
}

type ReqGetAssociated3PIDs struct{}

func (ReqGetAssociated3PIDs) GetRoute() string       { return "/account/3pid" }
func (ReqGetAssociated3PIDs) GetMetricsName() string { return "account_3pid" }
func (ReqGetAssociated3PIDs) GetMsgType() int32      { return internals.MSG_GET_ACCOUNT_3PID }
func (ReqGetAssociated3PIDs) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetAssociated3PIDs) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetAssociated3PIDs) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetAssociated3PIDs) NewRequest() core.Coder               { return nil }
func (ReqGetAssociated3PIDs) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetAssociated3PIDs) NewResponse(code int) core.Coder {
	return new(external.GetThreePIDsResponse)
}
func (ReqGetAssociated3PIDs) GetPrefix() []string { return []string{"r0"} }
func (ReqGetAssociated3PIDs) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.GetAssociated3PIDs()
}

type ReqPostAssociated3PIDs struct{}

func (ReqPostAssociated3PIDs) GetRoute() string       { return "/account/3pid" }
func (ReqPostAssociated3PIDs) GetMetricsName() string { return "account_3pid" }
func (ReqPostAssociated3PIDs) GetMsgType() int32      { return internals.MSG_POST_ACCOUNT_3PID }
func (ReqPostAssociated3PIDs) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostAssociated3PIDs) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostAssociated3PIDs) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostAssociated3PIDs) NewRequest() core.Coder {
	return new(external.PostAccount3PIDRequest)
}
func (ReqPostAssociated3PIDs) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostAccount3PIDRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostAssociated3PIDs) NewResponse(code int) core.Coder { return nil }
func (ReqPostAssociated3PIDs) GetPrefix() []string             { return []string{"r0"} }
func (ReqPostAssociated3PIDs) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.CheckAndSave3PIDAssociation()
}

type ReqPostAssociated3PIDsDel struct{}

func (ReqPostAssociated3PIDsDel) GetRoute() string       { return "/account/3pid/delete" }
func (ReqPostAssociated3PIDsDel) GetMetricsName() string { return "account_3pid" }
func (ReqPostAssociated3PIDsDel) GetMsgType() int32      { return internals.MSG_POST_ACCOUNT_3PID_DEL }
func (ReqPostAssociated3PIDsDel) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostAssociated3PIDsDel) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostAssociated3PIDsDel) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostAssociated3PIDsDel) NewRequest() core.Coder {
	return new(external.PostAccount3PIDDelRequest)
}
func (ReqPostAssociated3PIDsDel) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostAccount3PIDDelRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostAssociated3PIDsDel) NewResponse(code int) core.Coder { return nil }
func (ReqPostAssociated3PIDsDel) GetPrefix() []string             { return []string{"unstable"} }
func (ReqPostAssociated3PIDsDel) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.Forget3PID()
}

type ReqPostAccount3PIDEmail struct{}

func (ReqPostAccount3PIDEmail) GetRoute() string {
	return "/{path:(?:account/3pid|register)}/email/requestToken"
}
func (ReqPostAccount3PIDEmail) GetMetricsName() string { return "account_3pid_email_request_token" }
func (ReqPostAccount3PIDEmail) GetMsgType() int32      { return internals.MSG_POST_ACCOUNT_3PID_EMAIL }
func (ReqPostAccount3PIDEmail) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqPostAccount3PIDEmail) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostAccount3PIDEmail) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostAccount3PIDEmail) NewRequest() core.Coder {
	return new(external.PostAccount3PIDEmailRequest)
}
func (ReqPostAccount3PIDEmail) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostAccount3PIDEmailRequest)
	if vars != nil {
		msg.Path = vars["path"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostAccount3PIDEmail) NewResponse(code int) core.Coder {
	return new(external.PostAccount3PIDEmailResponse)
}
func (ReqPostAccount3PIDEmail) GetPrefix() []string { return []string{"r0"} }
func (ReqPostAccount3PIDEmail) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.RequestEmailToken()
}

type ReqGetVoipTurnServer struct{}

func (ReqGetVoipTurnServer) GetRoute() string       { return "/voip/turnServer" }
func (ReqGetVoipTurnServer) GetMetricsName() string { return "turn_server" }
func (ReqGetVoipTurnServer) GetMsgType() int32      { return internals.MSG_GET_TURN_SERVER }
func (ReqGetVoipTurnServer) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetVoipTurnServer) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetVoipTurnServer) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetVoipTurnServer) NewRequest() core.Coder               { return nil }
func (ReqGetVoipTurnServer) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetVoipTurnServer) NewResponse(code int) core.Coder {
	return new(external.PostVoipTurnServerResponse)
}
func (ReqGetVoipTurnServer) GetPrefix() []string { return []string{"r0"} }
func (ReqGetVoipTurnServer) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	return routing.RequestTurnServer(context.Background(), device.UserID, c.Cfg)
}

type ReqGetThirdpartyProtos struct{}

func (ReqGetThirdpartyProtos) GetRoute() string       { return "/thirdparty/protocols" }
func (ReqGetThirdpartyProtos) GetMetricsName() string { return "thirdparty_protocols" }
func (ReqGetThirdpartyProtos) GetMsgType() int32      { return internals.MSG_GET_THIRDPARTY_PROTOS }
func (ReqGetThirdpartyProtos) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetThirdpartyProtos) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetThirdpartyProtos) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetThirdpartyProtos) NewRequest() core.Coder               { return nil }
func (ReqGetThirdpartyProtos) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetThirdpartyProtos) NewResponse(code int) core.Coder { return nil }
func (ReqGetThirdpartyProtos) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetThirdpartyProtos) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	// TODO: Return the third party protcols
	return http.StatusOK, nil
}

type ReqGetDevicesByUserID struct{}

func (ReqGetDevicesByUserID) GetRoute() string       { return "/devices" }
func (ReqGetDevicesByUserID) GetMetricsName() string { return "get_devices" }
func (ReqGetDevicesByUserID) GetMsgType() int32      { return internals.MSG_GET_DEVICES }
func (ReqGetDevicesByUserID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetDevicesByUserID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetDevicesByUserID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetDevicesByUserID) NewRequest() core.Coder               { return nil }
func (ReqGetDevicesByUserID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetDevicesByUserID) NewResponse(code int) core.Coder { return new(external.DeviceList) }
func (ReqGetDevicesByUserID) GetPrefix() []string             { return []string{"r0", "unstable"} }
func (ReqGetDevicesByUserID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	return routing.GetDevicesByUserID(c.cacheIn, device.UserID)
}

type ReqGetDeviceByID struct{}

func (ReqGetDeviceByID) GetRoute() string                     { return "/devices/{deviceID}" }
func (ReqGetDeviceByID) GetMetricsName() string               { return "get_device" }
func (ReqGetDeviceByID) GetMsgType() int32                    { return internals.MSG_GET_DEVICES_BY_ID }
func (ReqGetDeviceByID) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetDeviceByID) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetDeviceByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetDeviceByID) NewRequest() core.Coder {
	return new(external.GetDeviceRequest)
}
func (ReqGetDeviceByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetDeviceRequest)
	if vars != nil {
		msg.DeviceID = vars["deviceID"]
	}
	return nil
}
func (ReqGetDeviceByID) NewResponse(code int) core.Coder { return new(external.Device) }
func (ReqGetDeviceByID) GetPrefix() []string             { return []string{"r0", "unstable"} }
func (ReqGetDeviceByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetDeviceRequest)
	return routing.GetDeviceByID(c.cacheIn, device.UserID, req.DeviceID)
}

type ReqPutDevice struct{}

func (ReqPutDevice) GetRoute() string                     { return "/devices/{deviceID}" }
func (ReqPutDevice) GetMetricsName() string               { return "device_data" }
func (ReqPutDevice) GetMsgType() int32                    { return internals.MSG_PUT_DEVICES_BY_ID }
func (ReqPutDevice) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPutDevice) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutDevice) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutDevice) NewRequest() core.Coder {
	return new(external.PutDeviceRequest)
}
func (ReqPutDevice) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutDeviceRequest)
	if vars != nil {
		msg.DeviceID = vars["deviceID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutDevice) NewResponse(code int) core.Coder { return nil }
func (ReqPutDevice) GetPrefix() []string             { return []string{"r0", "unstable"} }
func (ReqPutDevice) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutDeviceRequest)
	return routing.UpdateDeviceByID(context.Background(), req, c.deviceDB, device.UserID, device.ID, req.DeviceID, c.cacheIn)
}

type ReqDelDevice struct{}

func (ReqDelDevice) GetRoute() string                     { return "/devices/{deviceID}" }
func (ReqDelDevice) GetMetricsName() string               { return "del_device" }
func (ReqDelDevice) GetMsgType() int32                    { return internals.MSG_DEL_DEVICES_BY_ID }
func (ReqDelDevice) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqDelDevice) GetMethod() []string                  { return []string{http.MethodDelete, http.MethodOptions} }
func (ReqDelDevice) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqDelDevice) NewRequest() core.Coder {
	return new(external.DelDeviceRequest)
}
func (ReqDelDevice) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.DelDeviceRequest)
	if vars != nil {
		msg.DeviceID = vars["deviceID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqDelDevice) NewResponse(code int) core.Coder {
	//return new(external.DelDeviceAuthResponse)
	if code == http.StatusUnauthorized {
		return new(external.DeviceList)
	} else {
		return nil
	}
}
func (ReqDelDevice) GetPrefix() []string { return []string{"r0", "unstable"} }
func (ReqDelDevice) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.DelDeviceRequest)
	return routing.DeleteDeviceByID(req, req.DeviceID, c.Cfg, c.cacheIn,
		c.encryptDB, c.tokenFilter, c.syncDB, c.deviceDB, c.RpcCli, c.RpcClient,
	)
}

type ReqPostDelDevices struct{}

func (ReqPostDelDevices) GetRoute() string                     { return "/delete_devices" }
func (ReqPostDelDevices) GetMetricsName() string               { return "delete_devices" }
func (ReqPostDelDevices) GetMsgType() int32                    { return internals.MSG_POST_DEL_DEVICES }
func (ReqPostDelDevices) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostDelDevices) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostDelDevices) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostDelDevices) NewRequest() core.Coder {
	return new(external.PostDelDevicesRequest)
}
func (ReqPostDelDevices) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostDelDevicesRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostDelDevices) NewResponse(code int) core.Coder { return nil }
func (ReqPostDelDevices) GetPrefix() []string             { return []string{"r0", "unstable"} }
func (ReqPostDelDevices) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostDelDevicesRequest)
	return routing.DeleteDevices(req, device, c.cacheIn,
		c.encryptDB, c.tokenFilter, c.syncDB, c.deviceDB, c.RpcCli, c.RpcClient)
}

type ReqPutPresenceByID struct{}

func (ReqPutPresenceByID) GetRoute() string                     { return "/presence/{userID}/status" }
func (ReqPutPresenceByID) GetMetricsName() string               { return "presence_data" }
func (ReqPutPresenceByID) GetMsgType() int32                    { return internals.MSG_PUT_USR_PRESENCE }
func (ReqPutPresenceByID) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPutPresenceByID) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutPresenceByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutPresenceByID) NewRequest() core.Coder {
	return new(external.PutPresenceRequest)
}
func (ReqPutPresenceByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutPresenceRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutPresenceByID) NewResponse(code int) core.Coder { return nil }
func (ReqPutPresenceByID) GetPrefix() []string             { return []string{"r0", "inr0"} }
func (ReqPutPresenceByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutPresenceRequest)
	return routing.UpdatePresenceByID(context.Background(), req,
		c.presenceDB, &c.Cfg, c.cacheIn, device.UserID, device.ID, c.complexCache,
	)
}

type ReqGetPresenceByID struct{}

func (ReqGetPresenceByID) GetRoute() string                     { return "/presence/{userID}/status" }
func (ReqGetPresenceByID) GetMetricsName() string               { return "get_presence" }
func (ReqGetPresenceByID) GetMsgType() int32                    { return internals.MSG_GET_USR_PRESENCE }
func (ReqGetPresenceByID) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetPresenceByID) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetPresenceByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPresenceByID) NewRequest() core.Coder {
	return new(external.GetPresenceRequest)
}
func (ReqGetPresenceByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetPresenceRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetPresenceByID) NewResponse(code int) core.Coder {
	return new(external.GetPresenceResponse)
}
func (ReqGetPresenceByID) GetPrefix() []string { return []string{"r0", "inr0"} }
func (ReqGetPresenceByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetPresenceRequest)
	return routing.GetPresenceByID(c.RpcClient, c.cacheIn, c.federation, &c.Cfg, req.UserID)
}

type ReqGetPresenceListByID struct{}

func (ReqGetPresenceListByID) GetRoute() string       { return "/presence/list/{userID}" }
func (ReqGetPresenceListByID) GetMetricsName() string { return "get_presence_list" }
func (ReqGetPresenceListByID) GetMsgType() int32      { return internals.MSG_GET_USR_LIST_PRESENCE }
func (ReqGetPresenceListByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetPresenceListByID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetPresenceListByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetPresenceListByID) NewRequest() core.Coder {
	return new(external.GetPresenceListRequest)
}
func (ReqGetPresenceListByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetPresenceListRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetPresenceListByID) NewResponse(code int) core.Coder {
	return new(external.GetPresenceListResponse)
}
func (ReqGetPresenceListByID) GetPrefix() []string { return []string{"r0"} }
func (ReqGetPresenceListByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.GetPresenceListByID()
}

type ReqPostPresenceListByID struct{}

func (ReqPostPresenceListByID) GetRoute() string       { return "/presence/list/{userID}" }
func (ReqPostPresenceListByID) GetMetricsName() string { return "get_presence_list" }
func (ReqPostPresenceListByID) GetMsgType() int32      { return internals.MSG_POST_USR_LIST_PRESENCE }
func (ReqPostPresenceListByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostPresenceListByID) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostPresenceListByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostPresenceListByID) NewRequest() core.Coder {
	return new(external.PostPresenceListRequest)
}
func (ReqPostPresenceListByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostPresenceListRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostPresenceListByID) NewResponse(code int) core.Coder { return new(external.PresenceJSON) }
func (ReqPostPresenceListByID) GetPrefix() []string             { return []string{"r0"} }
func (ReqPostPresenceListByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.UpdatePresenceListByID()
}

type ReqGetRoomsTagsByID struct{}

func (ReqGetRoomsTagsByID) GetRoute() string       { return "/user/{userId}/rooms/{roomId}/tags" }
func (ReqGetRoomsTagsByID) GetMetricsName() string { return "get_room_tags" }
func (ReqGetRoomsTagsByID) GetMsgType() int32      { return internals.MSG_GET_ROOM_TAG_BY_ID }
func (ReqGetRoomsTagsByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomsTagsByID) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomsTagsByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomsTagsByID) NewRequest() core.Coder {
	return new(external.GetRoomsTagsByIDRequest)
}
func (ReqGetRoomsTagsByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomsTagsByIDRequest)
	if vars != nil {
		msg.UserId = vars["userId"]
		msg.RoomId = vars["roomId"]
	}
	return nil
}
func (ReqGetRoomsTagsByID) NewResponse(code int) core.Coder {
	return new(external.GetRoomsTagsByIDResponse)
}
func (ReqGetRoomsTagsByID) GetPrefix() []string { return []string{"r0"} }
func (ReqGetRoomsTagsByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomsTagsByIDRequest)
	return routing.GetRoomTags(context.Background(), c.cacheIn, device.UserID, req)
}

type ReqPutRoomsTagsByID struct{}

func (ReqPutRoomsTagsByID) GetRoute() string       { return "/user/{userId}/rooms/{roomId}/tags/{tag}" }
func (ReqPutRoomsTagsByID) GetMetricsName() string { return "put_room_tag" }
func (ReqPutRoomsTagsByID) GetMsgType() int32      { return internals.MSG_PUT_ROOM_TAG_BY_ID }
func (ReqPutRoomsTagsByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutRoomsTagsByID) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutRoomsTagsByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomsTagsByID) NewRequest() core.Coder {
	return new(external.PutRoomsTagsByIDRequest)
}
func (ReqPutRoomsTagsByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomsTagsByIDRequest)
	if vars != nil {
		msg.UserId = vars["userId"]
		msg.RoomId = vars["roomId"]
		msg.Tag = vars["tag"]
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutRoomsTagsByID) NewResponse(code int) core.Coder { return nil }
func (ReqPutRoomsTagsByID) GetPrefix() []string             { return []string{"r0"} }
func (ReqPutRoomsTagsByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomsTagsByIDRequest)
	return routing.SetRoomTag(context.Background(), req, c.accountDB, device.UserID, c.Cfg)
}

type ReqDelRoomsTagsByID struct{}

func (ReqDelRoomsTagsByID) GetRoute() string       { return "/user/{userId}/rooms/{roomId}/tags/{tag}" }
func (ReqDelRoomsTagsByID) GetMetricsName() string { return "delete_room_tag" }
func (ReqDelRoomsTagsByID) GetMsgType() int32      { return internals.MSG_DEL_ROOM_TAG_BY_ID }
func (ReqDelRoomsTagsByID) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqDelRoomsTagsByID) GetMethod() []string {
	return []string{http.MethodDelete, http.MethodOptions}
}
func (ReqDelRoomsTagsByID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqDelRoomsTagsByID) NewRequest() core.Coder {
	return new(external.DelRoomsTagsByIDRequest)
}
func (ReqDelRoomsTagsByID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.DelRoomsTagsByIDRequest)
	if vars != nil {
		msg.UserId = vars["userId"]
		msg.RoomId = vars["roomId"]
		msg.Tag = vars["tag"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqDelRoomsTagsByID) NewResponse(code int) core.Coder { return nil }
func (ReqDelRoomsTagsByID) GetPrefix() []string             { return []string{"r0"} }
func (ReqDelRoomsTagsByID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.DelRoomsTagsByIDRequest)
	return routing.DeleteRoomTag(context.Background(), req, c.accountDB, device.UserID, c.Cfg)
}

type ReqPutUserAccountData struct{}

func (ReqPutUserAccountData) GetRoute() string       { return "/user/{userID}/account_data/{type}" }
func (ReqPutUserAccountData) GetMetricsName() string { return "user_account_data" }
func (ReqPutUserAccountData) GetMsgType() int32      { return internals.MSG_PUT_USER_ACCOUNT_DATA }
func (ReqPutUserAccountData) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutUserAccountData) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutUserAccountData) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutUserAccountData) NewRequest() core.Coder {
	return new(external.PutUserAccountDataRequest)
}
func (ReqPutUserAccountData) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutUserAccountDataRequest)
	if vars != nil {
		msg.UserId = vars["userID"]
		msg.Type = vars["type"]
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	msg.Content = data
	return nil
}
func (ReqPutUserAccountData) NewResponse(code int) core.Coder { return nil }
func (ReqPutUserAccountData) GetPrefix() []string             { return []string{"r0", "inr0"} }
func (ReqPutUserAccountData) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutUserAccountDataRequest)
	if req.UserId != device.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("userID does not match the current user")
	}
	return routing.SaveAccountData(context.Background(), c.accountDB, device.ID, c.Cfg, req.UserId, "", req.Type, req.Content, c.cacheIn)
}

type ReqPutRoomAccountData struct{}

func (ReqPutRoomAccountData) GetRoute() string {
	return "/user/{userID}/rooms/{roomID}/account_data/{type}"
}
func (ReqPutRoomAccountData) GetMetricsName() string { return "user_account_data" }
func (ReqPutRoomAccountData) GetMsgType() int32      { return internals.MSG_PUT_ROOM_ACCOUNT_DATA }
func (ReqPutRoomAccountData) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutRoomAccountData) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutRoomAccountData) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutRoomAccountData) NewRequest() core.Coder {
	return new(external.PutRoomAccountDataRequest)
}
func (ReqPutRoomAccountData) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomAccountDataRequest)
	if vars != nil {
		msg.UserId = vars["userID"]
		msg.RoomID = vars["roomID"]
		msg.Type = vars["type"]
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	msg.Content = data
	return nil
}
func (ReqPutRoomAccountData) NewResponse(code int) core.Coder { return nil }
func (ReqPutRoomAccountData) GetPrefix() []string             { return []string{"r0", "inr0"} }
func (ReqPutRoomAccountData) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomAccountDataRequest)
	return routing.SaveAccountData(
		context.Background(), c.accountDB, device.ID, c.Cfg, req.UserId,
		req.RoomID, req.Type, req.Content, c.cacheIn,
	)
}

type ReqGetWhoAmi struct{}

func (ReqGetWhoAmi) GetRoute() string                     { return "/account/whoami" }
func (ReqGetWhoAmi) GetMetricsName() string               { return "who_am_i" }
func (ReqGetWhoAmi) GetMsgType() int32                    { return internals.MSG_GET_ACCOUNT_WHOAMI }
func (ReqGetWhoAmi) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetWhoAmi) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetWhoAmi) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetWhoAmi) NewRequest() core.Coder               { return nil }
func (ReqGetWhoAmi) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	return nil
}
func (ReqGetWhoAmi) NewResponse(code int) core.Coder { return new(external.GetAccountWhoAmI) }
func (ReqGetWhoAmi) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetWhoAmi) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return routing.GetWhoAmi(device.UserID)
}

type ReqGetDirectoryRoom struct{}

func (ReqGetDirectoryRoom) GetRoute() string       { return "/directory/list/room/{roomID}" }
func (ReqGetDirectoryRoom) GetMetricsName() string { return "directory_list" }
func (ReqGetDirectoryRoom) GetMsgType() int32      { return internals.MSG_GET_DIRECTORY_LIST_ROOM }
func (ReqGetDirectoryRoom) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetDirectoryRoom) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetDirectoryRoom) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetDirectoryRoom) NewRequest() core.Coder {
	return new(external.GetDirectoryRoomRequest)
}
func (ReqGetDirectoryRoom) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetDirectoryRoomRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqGetDirectoryRoom) NewResponse(code int) core.Coder {
	return new(external.GetDirectoryRoomResponse)
}
func (ReqGetDirectoryRoom) GetPrefix() []string { return []string{"r0"} }
func (ReqGetDirectoryRoom) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetDirectoryRoomRequest)
	return routing.GetVisibility(context.Background(), req.RoomID, c.rsRpcCli)
}

type ReqPutDirectoryRoom struct{}

func (ReqPutDirectoryRoom) GetRoute() string       { return "/directory/list/room/{roomID}" }
func (ReqPutDirectoryRoom) GetMetricsName() string { return "directory_list" }
func (ReqPutDirectoryRoom) GetMsgType() int32      { return internals.MSG_PUT_DIRECTORY_LIST_ROOM }
func (ReqPutDirectoryRoom) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutDirectoryRoom) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutDirectoryRoom) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutDirectoryRoom) NewRequest() core.Coder {
	return new(external.PutRoomStateByTypeWithTxnID)
}
func (ReqPutDirectoryRoom) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutRoomStateByTypeWithTxnID)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventType = vars["eventType"]
		msg.StateKey = vars["stateKey"]
		msg.TxnID = vars["txnID"]
		msg.IP = common.GetRemoteIP(req)
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutDirectoryRoom) NewResponse(code int) core.Coder {
	return new(external.PutRoomStateByTypeWithTxnIDResponse)
}
func (ReqPutDirectoryRoom) GetPrefix() []string { return []string{"r0"} }
func (ReqPutDirectoryRoom) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutRoomStateByTypeWithTxnID)
	userID := device.UserID
	deviceID := device.ID
	stateKey := req.StateKey
	return routing.PostEvent(
		context.Background(), req, userID, deviceID, req.IP, req.RoomID,
		req.EventType, nil, &stateKey, c.Cfg, c.rsRpcCli, c.localcache, c.idg,
	)
}

type ReqGetJoinedMember struct{}

func (ReqGetJoinedMember) GetRoute() string                     { return "/rooms/{roomID}/joined_members" }
func (ReqGetJoinedMember) GetMetricsName() string               { return "rooms_members" }
func (ReqGetJoinedMember) GetMsgType() int32                    { return internals.MSG_GET_ROOM_JOIN_MEMBERS }
func (ReqGetJoinedMember) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetJoinedMember) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetJoinedMember) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetJoinedMember) NewRequest() core.Coder {
	return new(external.GetJoinedMemberRequest)
}
func (ReqGetJoinedMember) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetJoinedMemberRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqGetJoinedMember) NewResponse(code int) core.Coder {
	return new(external.GetJoinedMemberResponse)
}
func (ReqGetJoinedMember) GetPrefix() []string { return []string{"r0"} }
func (ReqGetJoinedMember) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetJoinedMemberRequest)
	return routing.OnRoomJoinedRequest(context.Background(), device.UserID, req.RoomID, c.rsRpcCli, c.cacheIn, c.complexCache)
}

type ReqGetRoomInfo struct{}

func (ReqGetRoomInfo) GetRoute() string                     { return "/rooms" }
func (ReqGetRoomInfo) GetMetricsName() string               { return "room_info" }
func (ReqGetRoomInfo) GetMsgType() int32                    { return internals.MSG_POST_ROOM_INFO }
func (ReqGetRoomInfo) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetRoomInfo) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqGetRoomInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomInfo) NewRequest() core.Coder {
	return new(external.GetRoomInfoRequest)
}
func (ReqGetRoomInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomInfoRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqGetRoomInfo) NewResponse(code int) core.Coder {
	return new(external.GetRoomInfoResponse)
}
func (ReqGetRoomInfo) GetPrefix() []string { return []string{"r0"} }
func (ReqGetRoomInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomInfoRequest)
	return routing.OnRoomInfoRequest(context.Background(), device.UserID, req.RoomIDs, c.rsRpcCli, c.cacheIn)
}

type ReqInternalGetRoomInfo struct{}

func (ReqInternalGetRoomInfo) GetRoute() string       { return "/rooms" }
func (ReqInternalGetRoomInfo) GetMetricsName() string { return "room_info" }
func (ReqInternalGetRoomInfo) GetMsgType() int32      { return internals.MSG_INTERNAL_POST_ROOM_INFO }
func (ReqInternalGetRoomInfo) GetAPIType() int8       { return apiconsumer.APITypeInternalAuth }
func (ReqInternalGetRoomInfo) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqInternalGetRoomInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqInternalGetRoomInfo) NewRequest() core.Coder {
	return new(external.GetRoomInfoRequest)
}
func (ReqInternalGetRoomInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomInfoRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqInternalGetRoomInfo) NewResponse(code int) core.Coder {
	return new(external.GetRoomInfoResponse)
}
func (ReqInternalGetRoomInfo) GetPrefix() []string { return []string{"inr0"} }
func (ReqInternalGetRoomInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomInfoRequest)
	return routing.OnRoomInfoRequest(context.Background(), device.UserID, req.RoomIDs, c.rsRpcCli, c.cacheIn)
}

type ReqGetUserNewToken struct{}

func (ReqGetUserNewToken) GetRoute() string                     { return "/user/new_token" }
func (ReqGetUserNewToken) GetMetricsName() string               { return "new_token" }
func (ReqGetUserNewToken) GetMsgType() int32                    { return internals.MSG_GET_USER_NEW_TOKEN }
func (ReqGetUserNewToken) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqGetUserNewToken) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetUserNewToken) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetUserNewToken) NewRequest() core.Coder {
	return new(external.PostLoginRequest)
}
func (ReqGetUserNewToken) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostLoginRequest)
	msg.IP = common.GetRemoteIP(req)
	return nil
}
func (ReqGetUserNewToken) NewResponse(code int) core.Coder {
	return new(external.PostLoginResponse)
}
func (ReqGetUserNewToken) GetPrefix() []string { return []string{"r0"} }
func (ReqGetUserNewToken) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostLoginRequest)
	return routing.GenNewToken(context.Background(), c.deviceDB, device, c.tokenFilter, c.idg, c.Cfg, c.encryptDB, c.syncDB, c.RpcCli, c.RpcClient, req.IP)
}

type ReqGetSuperAdminToken struct{}

func (ReqGetSuperAdminToken) GetRoute() string       { return "/user/super_token" }
func (ReqGetSuperAdminToken) GetMetricsName() string { return "super_token" }
func (ReqGetSuperAdminToken) GetMsgType() int32      { return internals.MSG_GET_SUPER_ADMIN_TOKEN }
func (ReqGetSuperAdminToken) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetSuperAdminToken) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetSuperAdminToken) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetSuperAdminToken) NewRequest() core.Coder {
	return new(external.PostLoginRequest)
}
func (ReqGetSuperAdminToken) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostLoginRequest)
	msg.IP = common.GetRemoteIP(req)
	return nil
}
func (ReqGetSuperAdminToken) NewResponse(code int) core.Coder {
	return new(external.PostLoginResponse)
}
func (ReqGetSuperAdminToken) GetPrefix() []string { return []string{"r0"} }
func (ReqGetSuperAdminToken) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostLoginRequest)
	return routing.GetSuperAdminToken(context.Background(), c.deviceDB, device, c.tokenFilter, c.idg, c.Cfg, c.encryptDB, c.syncDB, c.RpcCli, c.RpcClient, req.IP)
}

type ReqGetSetting struct{}

func (ReqGetSetting) GetRoute() string                     { return "/setting/{settingKey}" }
func (ReqGetSetting) GetMetricsName() string               { return "get_setting" }
func (ReqGetSetting) GetMsgType() int32                    { return internals.MSG_GET_SETTING }
func (ReqGetSetting) GetAPIType() int8                     { return apiconsumer.APITypeInternalAuth }
func (ReqGetSetting) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetSetting) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetSetting) NewRequest() core.Coder               { return new(external.ReqGetSettingRequest) }
func (ReqGetSetting) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.ReqGetSettingRequest)
	if vars != nil {
		msg.SettingKey = vars["settingKey"]
	}
	return nil
}
func (ReqGetSetting) NewResponse(code int) core.Coder { return new(internals.RawBytes) }
func (ReqGetSetting) GetPrefix() []string             { return []string{"inr0"} }
func (ReqGetSetting) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.ReqGetSettingRequest)
	str, err := c.cacheIn.GetSettingRaw(req.SettingKey)
	if err != nil {
		return http.StatusNotFound, jsonerror.NotFound("Unknown setting " + req.SettingKey)
	}
	ret := new(internals.RawBytes)
	*ret = internals.RawBytes(str)
	log.Infof("get setting %s %s", req.SettingKey, *ret)
	return http.StatusOK, ret
}

type ReqPutSetting struct{}

func (ReqPutSetting) GetRoute() string                     { return "/setting/{settingKey}" }
func (ReqPutSetting) GetMetricsName() string               { return "put_setting" }
func (ReqPutSetting) GetMsgType() int32                    { return internals.MSG_PUT_SETTING }
func (ReqPutSetting) GetAPIType() int8                     { return apiconsumer.APITypeInternalAuth }
func (ReqPutSetting) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutSetting) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutSetting) NewRequest() core.Coder               { return new(external.ReqPutSettingRequest) }
func (ReqPutSetting) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.ReqPutSettingRequest)
	if vars != nil {
		msg.SettingKey = vars["settingKey"]
	}
	content, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	msg.Content = string(content)
	return nil
}
func (ReqPutSetting) NewResponse(code int) core.Coder { return nil }
func (ReqPutSetting) GetPrefix() []string             { return []string{"inr0"} }
func (ReqPutSetting) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.ReqPutSettingRequest)

	v, _ := c.cacheIn.GetSettingRaw(req.SettingKey)
	err := c.cacheIn.SetSetting(req.SettingKey, req.Content)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("Internal Server Error")
	}

	log.Infof("put setting %s: %s -> %s", req.SettingKey, v, req.Content)
	c.roomDB.SettingsInsertRaw(context.TODO(), req.SettingKey, req.Content)
	c.settings.UpdateSetting(req.SettingKey, req.Content)

	common.GetTransportMultiplexer().SendWithRetry(
		c.Cfg.Kafka.Producer.SettingUpdate.Underlying,
		c.Cfg.Kafka.Producer.SettingUpdate.Name,
		&core.TransportPubMsg{
			Keys: []byte{},
			Obj:  req,
		})

	return http.StatusOK, nil
}

type ReqPostReport struct{}

func (ReqPostReport) GetRoute() string                     { return "/report/missing_event/{roomID}" }
func (ReqPostReport) GetMetricsName() string               { return "report_missing_event" }
func (ReqPostReport) GetMsgType() int32                    { return internals.MSG_POST_REPORT_MISSING }
func (ReqPostReport) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostReport) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostReport) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostReport) NewRequest() core.Coder               { return new(external.PostReportMissingEventsRequest) }
func (ReqPostReport) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostReportMissingEventsRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	err := common.UnmarshalJSON(req, &msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostReport) NewResponse(code int) core.Coder { return nil }
func (ReqPostReport) GetPrefix() []string             { return []string{"r0", "v1", "inr0"} }
func (ReqPostReport) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostReportMissingEventsRequest)

	log.Errorf("Client missing event, roomID: %s, depth: %v", req.RoomID, req.Depth)
	c.missingEventCounter.WithLabelValues(req.RoomID).Inc()
	return http.StatusOK, nil
}

type ReqGetUserInfo struct{}

func (ReqGetUserInfo) GetRoute() string                     { return "/user_info/{userID}" }
func (ReqGetUserInfo) GetMetricsName() string               { return "get_user_info" }
func (ReqGetUserInfo) GetMsgType() int32                    { return internals.MSG_GET_USER_INFO }
func (ReqGetUserInfo) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqGetUserInfo) GetMethod() []string                  { return []string{http.MethodGet, http.MethodOptions} }
func (ReqGetUserInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetUserInfo) NewRequest() core.Coder {
	return new(external.GetUserInfoRequest)
}
func (ReqGetUserInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetUserInfoRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqGetUserInfo) NewResponse(code int) core.Coder { return new(external.GetUserInfoResponse) }
func (ReqGetUserInfo) GetPrefix() []string             { return []string{"r0"} }
func (ReqGetUserInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetUserInfoRequest)
	return routing.GetUserInfo(
		context.Background(), req, c.Cfg, c.federation, c.accountDB, c.cacheIn,
	)
}

type ReqPostUserInfo struct{}

func (ReqPostUserInfo) GetRoute() string                     { return "/user_info/{userID}" }
func (ReqPostUserInfo) GetMetricsName() string               { return "post_user_info" }
func (ReqPostUserInfo) GetMsgType() int32                    { return internals.MSG_POST_USER_INFO }
func (ReqPostUserInfo) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostUserInfo) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostUserInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostUserInfo) NewRequest() core.Coder {
	return new(external.PostUserInfoRequest)
}
func (ReqPostUserInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostUserInfoRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostUserInfo) NewResponse(code int) core.Coder { return nil }
func (ReqPostUserInfo) GetPrefix() []string             { return []string{"r0"} }
func (ReqPostUserInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostUserInfoRequest)
	return routing.AddUserInfo(
		context.Background(), req, c.accountDB, device.UserID, &c.Cfg,
		c.cacheIn, c.rsRpcCli, c.idg, c.complexCache,
	)
}

type ReqPutUserInfo struct{}

func (ReqPutUserInfo) GetRoute() string                     { return "/user_info/{userID}" }
func (ReqPutUserInfo) GetMetricsName() string               { return "put_user_info" }
func (ReqPutUserInfo) GetMsgType() int32                    { return internals.MSG_PUT_USER_INFO }
func (ReqPutUserInfo) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPutUserInfo) GetMethod() []string                  { return []string{http.MethodPut, http.MethodOptions} }
func (ReqPutUserInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutUserInfo) NewRequest() core.Coder {
	return new(external.PutUserInfoRequest)
}
func (ReqPutUserInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutUserInfoRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutUserInfo) NewResponse(code int) core.Coder { return nil }
func (ReqPutUserInfo) GetPrefix() []string             { return []string{"r0"} }
func (ReqPutUserInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutUserInfoRequest)
	return routing.SetUserInfo(
		context.Background(), req, c.accountDB, device.UserID, &c.Cfg,
		c.cacheIn, c.rsRpcCli, c.idg, c.complexCache,
	)
}

type ReqDeleteUserInfo struct{}

func (ReqDeleteUserInfo) GetRoute() string                     { return "/user_info/{userID}" }
func (ReqDeleteUserInfo) GetMetricsName() string               { return "delete_user_info" }
func (ReqDeleteUserInfo) GetMsgType() int32                    { return internals.MSG_DELETE_USER_INFO }
func (ReqDeleteUserInfo) GetAPIType() int8                     { return apiconsumer.APITypeExternal }
func (ReqDeleteUserInfo) GetMethod() []string                  { return []string{http.MethodDelete, http.MethodOptions} }
func (ReqDeleteUserInfo) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqDeleteUserInfo) NewRequest() core.Coder {
	return new(external.DeleteUserInfoRequest)
}
func (ReqDeleteUserInfo) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.DeleteUserInfoRequest)
	if vars != nil {
		msg.UserID = vars["userID"]
	}
	return nil
}
func (ReqDeleteUserInfo) NewResponse(code int) core.Coder { return nil }
func (ReqDeleteUserInfo) GetPrefix() []string             { return []string{"r0"} }
func (ReqDeleteUserInfo) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.DeleteUserInfoRequest)
	return routing.DeleteUserInfo(
		context.Background(), req, c.Cfg, c.federation, c.accountDB, c.cacheIn,
	)
}

type ReqDismissRoom struct{}

func (ReqDismissRoom) GetRoute() string                     { return "/{roomID}/dismiss" }
func (ReqDismissRoom) GetMetricsName() string               { return "dismiss_room" }
func (ReqDismissRoom) GetMsgType() int32                    { return internals.MSG_POST_ROOM_DISMISS }
func (ReqDismissRoom) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqDismissRoom) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqDismissRoom) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqDismissRoom) NewRequest() core.Coder {
	return new(external.DismissRoomRequest)
}
func (ReqDismissRoom) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.DismissRoomRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	return nil
}
func (ReqDismissRoom) NewResponse(code int) core.Coder { return nil }
func (ReqDismissRoom) GetPrefix() []string             { return []string{"r0"} }
func (ReqDismissRoom) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.DismissRoomRequest)
	userID := device.UserID
	deviceID := device.ID
	return routing.DismissRoom(
		context.Background(), req, c.accountDB, userID, deviceID, req.RoomID,
		c.Cfg, c.rsRpcCli, c.federation, c.cacheIn, c.idg, c.complexCache,
	)
}
