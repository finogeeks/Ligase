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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/encryptoapi/routing"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer

	cache        service.Cache
	encryptionDB model.EncryptorAPIDatabase
	syncDB       model.SyncAPIDatabase
	idg          *uid.UidGenerator
	federation   *gomatrixserverlib.FederationClient
	serverName   []string
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	encryptionDB model.EncryptorAPIDatabase,
	syncDB model.SyncAPIDatabase,
	idg *uid.UidGenerator,
	cache service.Cache,
	rpcClient rpc.RpcClient,
	federation *gomatrixserverlib.FederationClient,
	serverName []string,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcClient = rpcClient
	c.encryptionDB = encryptionDB
	c.syncDB = syncDB
	c.idg = idg
	c.cache = cache
	c.federation = federation
	c.serverName = serverName
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("encryptoapi", c, c.Cfg.Rpc.ProxyEncryptoApiTopic, &c.Cfg.Rpc.FrontEncryptoApi)
	c.APIConsumer.Start()
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyEncryptoApiTopic
}

func init() {
	apiconsumer.SetServices("front_encrypto_api")
	apiconsumer.SetAPIProcessor(ReqPostUploadKeyByDeviceID{})
	apiconsumer.SetAPIProcessor(ReqPostUploadKey{})
	apiconsumer.SetAPIProcessor(ReqPostQueryKey{})
	apiconsumer.SetAPIProcessor(ReqPostClaimKey{})
}

type ReqPostUploadKeyByDeviceID struct{}

func (ReqPostUploadKeyByDeviceID) GetRoute() string       { return "/keys/upload/{deviceID}" }
func (ReqPostUploadKeyByDeviceID) GetMetricsName() string { return "upload keys" }
func (ReqPostUploadKeyByDeviceID) GetMsgType() int32 {
	return internals.MSG_POST_KEYS_UPLOAD_BY_DEVICE_ID
}
func (ReqPostUploadKeyByDeviceID) GetAPIType() int8 { return apiconsumer.APITypeAuth }
func (ReqPostUploadKeyByDeviceID) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostUploadKeyByDeviceID) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostUploadKeyByDeviceID) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostUploadKeyByDeviceID) NewRequest() core.Coder {
	return new(types.UploadEncrypt)
}
func (ReqPostUploadKeyByDeviceID) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*types.UploadEncrypt)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostUploadKeyByDeviceID) NewResponse(code int) core.Coder {
	return new(external.PostUploadKeysResponse)
}
func (ReqPostUploadKeyByDeviceID) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*types.UploadEncrypt)
	return routing.UploadPKeys(
		context.Background(), req, c.encryptionDB, device, c.cache,
		c.RpcClient, c.syncDB, c.idg,
	)
}

type ReqPostUploadKey struct{}

func (ReqPostUploadKey) GetRoute() string                     { return "/keys/upload" }
func (ReqPostUploadKey) GetMetricsName() string               { return "upload keys" }
func (ReqPostUploadKey) GetMsgType() int32                    { return internals.MSG_POST_KEYS_UPLOAD }
func (ReqPostUploadKey) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostUploadKey) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostUploadKey) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostUploadKey) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostUploadKey) NewRequest() core.Coder {
	return new(types.UploadEncrypt)
}
func (ReqPostUploadKey) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*types.UploadEncrypt)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostUploadKey) NewResponse(code int) core.Coder {
	return new(external.PostUploadKeysResponse)
}
func (ReqPostUploadKey) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*types.UploadEncrypt)
	return routing.UploadPKeys(
		context.Background(), req, c.encryptionDB, device, c.cache,
		c.RpcClient, c.syncDB, c.idg,
	)
}

type ReqPostQueryKey struct{}

func (ReqPostQueryKey) GetRoute() string                     { return "/keys/query" }
func (ReqPostQueryKey) GetMetricsName() string               { return "query keys" }
func (ReqPostQueryKey) GetMsgType() int32                    { return internals.MSG_POST_KEYS_QUERY }
func (ReqPostQueryKey) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostQueryKey) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostQueryKey) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostQueryKey) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostQueryKey) NewRequest() core.Coder {
	return new(external.PostQueryKeysRequest)
}
func (ReqPostQueryKey) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostQueryKeysRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostQueryKey) NewResponse(code int) core.Coder {
	return new(external.PostQueryKeysResponse)
}
func (ReqPostQueryKey) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostQueryKeysRequest)
	return routing.QueryPKeys(
		context.Background(), req, device.ID, c.cache, c.federation, c.serverName,
	)
}

type ReqPostClaimKey struct{}

func (ReqPostClaimKey) GetRoute() string                     { return "/keys/claim" }
func (ReqPostClaimKey) GetMetricsName() string               { return "claim keys" }
func (ReqPostClaimKey) GetMsgType() int32                    { return internals.MSG_POST_KEYS_CLAIM }
func (ReqPostClaimKey) GetAPIType() int8                     { return apiconsumer.APITypeAuth }
func (ReqPostClaimKey) GetMethod() []string                  { return []string{http.MethodPost, http.MethodOptions} }
func (ReqPostClaimKey) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostClaimKey) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqPostClaimKey) NewRequest() core.Coder {
	return new(external.PostClaimKeysRequest)
}
func (ReqPostClaimKey) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostClaimKeysRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostClaimKey) NewResponse(code int) core.Coder {
	return new(external.PostClaimKeysResponse)
}
func (ReqPostClaimKey) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostClaimKeysRequest)
	return routing.ClaimOneTimeKeys(
		context.Background(), req, c.cache, c.encryptionDB, c.RpcClient,
	)
}
