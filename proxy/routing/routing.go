// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package routing

import (
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/gorilla/mux"
	"github.com/json-iterator/go"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Setup registers HTTP handlers with the given ServeMux. It also supplies the given http.Client
// to clients which need to make outbound HTTP requests.
func Setup(
	apiMux *mux.Router,
	cfg config.Dendrite,
	cacheIn service.Cache,
	rpcCli *common.RpcClient,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	tokenFilter *filter.SimpleFilter,
	feddomains *common.FedDomains,
	keyDB model.KeyDatabase,
) {
	monitor := mon.GetInstance()
	histogram := monitor.NewLabeledHistogram(
		"http_requests_duration_milliseconds",
		[]string{"method", "path", "code"},
		[]float64{50.0, 100.0, 500.0, 1000.0, 2000.0, 5000.0},
	)
	/*counter := monitor.NewLabeledCounter(
		"http_requests_total",
		[]string{"method", "path", "code"},
	)*/

	idg, _ := uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	apiMux.Handle("/_matrix/client/versions",
		common.MakeExternalAPI("versions", func(req *http.Request) util.JSONResponse {
			return util.JSONResponse{
				Code: http.StatusOK,
				JSON: struct {
					Versions []string `json:"versions"`
				}{[]string{
					"r0.0.1",
					"r0.1.0",
					"r0.2.0",
					"r0.3.0",
				}},
			}
		}),
	).Methods(http.MethodGet, http.MethodOptions)

	prefixMap := map[string]string{
		"v1":       "/_matrix/client/api/v1",
		"r0":       "/_matrix/client/r0",
		"unstable": "/_matrix/client/unstable",
		"inr0":     "/internal/_matrix/client/r0",
		"sys":      "/system/manager",
		"mediaR0":  "/_matrix/media/r0",
		"mediaV1":  "/_matrix/media/v1",
		"fedV1":    "/_matrix/federation/v1",
	}

	muxs := map[string]*mux.Router{}
	procs := map[string]*HttpProcessor{}
	for k, v := range prefixMap {
		m := apiMux.PathPrefix(v).Subrouter()
		muxs[k] = m
		proc := NewHttpProcessor(m, cfg, cacheIn, rpcCli, rsRpcCli, tokenFilter, idg, histogram, feddomains, keyDB /*, counter*/)
		procs[k] = proc
	}

	apiconsumer.ForeachAPIProcessor(func(p apiconsumer.APIProcessor) bool {
		for _, prefix := range p.GetPrefix() {
			if prefix == "inr0" {
				procs[prefix].Route(
					p.GetRoute(),
					p.GetMetricsName(),
					p.GetTopic(&cfg),
					p.GetMsgType(),
					apiconsumer.APITypeInternalAuth,
					p.GetMethod()...,
				)
			} else {
				procs[prefix].Route(
					p.GetRoute(),
					p.GetMetricsName(),
					p.GetTopic(&cfg),
					p.GetMsgType(),
					p.GetAPIType(),
					p.GetMethod()...,
				)
			}
		}
		return true
	})

	// // r0?
	// r0Processor.route("/admin/whois/{userId}", "whois", internals.MSG_GET_WHO_IS, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/login/cas/redirect", "cas_redirect", internals.MSG_GET_CAS_LOGIN_REDIRECT, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/login/cas/ticket", "cas_ticket", internals.MSG_GET_CAS_LOGIN_TICKET, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/rooms/{roomId}/report/{eventId}", "room_report", internals.MSG_POST_ROOM_REPORT, http.MethodPost, http.MethodOptions)

	// r0Processor.route("/thirdparty/protocols", "thirdparty_protocols", internals.MSG_GET_THIRDPARTY_PROTOS, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/thirdparty/protocol/{protocol}", "thirdparty_by_name", internals.MSG_GET_THIRDPARTY_PROTO_BY_NAME, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/thirdparty/location/{protocol}", "thirdparty_location_by_name", internals.MSG_GET_THIRDPARTY_LOCATION_BY_PROTO, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/thirdparty/user/{protocol}", "thirdparty_user_by_name", internals.MSG_GET_THIRDPARTY_USER_BY_PROTO, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/thirdparty/location", "thirdparty_location", internals.MSG_GET_THIRDPARTY_LOCATION, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/thirdparty/user", "thirdparty_user", internals.MSG_GET_THIRDPARTY_USER, http.MethodGet, http.MethodOptions)

	// r0Processor.route("/user/{userId}/openid/request_token", "user_openid", internals.MSG_POST_USER_OPENID, http.MethodPost, http.MethodOptions)
}
