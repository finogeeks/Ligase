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

package routing

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/filter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/mediatypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/proxy/api"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/grpc"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

type MsgProcessor interface {
	ProcessInput(coder core.Coder) util.JSONResponse
}

type HttpProcessor struct {
	router      *mux.Router
	cfg         config.Dendrite
	cacheIn     service.Cache
	rpcClient   rpc.RpcClient
	tokenFilter *filter.SimpleFilter
	idg         *uid.UidGenerator
	keys        gomatrixserverlib.KeyRing
	rsRpcCli    roomserverapi.RoomserverRPCAPI
	fed         *gomatrixserverlib.FederationClient

	histogram  mon.LabeledHistogram
	feddomains *common.FedDomains
	keyDB      model.KeyDatabase
	localCache service.LocalCache
	// counter   mon.LabeledCounter
}

func NewHttpProcessor(
	r *mux.Router, cfg config.Dendrite,
	cacheIn service.Cache, rpcClient rpc.RpcClient,
	rsRpcCli roomserverapi.RoomserverRPCAPI, tokenFilter *filter.SimpleFilter,
	idg *uid.UidGenerator,
	histogram mon.LabeledHistogram,
	feddomains *common.FedDomains,
	keyDB model.KeyDatabase,
	// counter mon.LabeledCounter,
) *HttpProcessor {
	localCache := new(cache.LocalCacheRepo)
	localCache.Start(1, cfg.Cache.DurationDefault)
	return &HttpProcessor{
		router:      r,
		cfg:         cfg,
		cacheIn:     cacheIn,
		tokenFilter: tokenFilter,
		idg:         idg,
		rpcClient:   rpcClient,
		rsRpcCli:    rsRpcCli,
		histogram:   histogram,
		feddomains:  feddomains,
		keyDB:       keyDB,
		localCache:  localCache,
		// counter:     counter,
	}
}

func (w *HttpProcessor) Start() {
}

func (w *HttpProcessor) Route(path, metricsName, topic string, msgType int32, apiType int8, methods ...string) {
	processor := apiconsumer.GetAPIProcessor(msgType)
	if processor == nil {
		log.Errorf("route failed, cant find processor for url: %s", path)
		return
	}
	if topic == "" {
		log.Errorf("route failed, topic not spec in %s", path)
		return
	}

	newRequest := func(req *http.Request) (core.Coder, error) {
		r := processor.NewRequest()
		if r != nil {
			var vars = mux.Vars(req)
			err := processor.FillRequest(r, req, vars)
			if err != nil {
				return nil, err
			}
		}
		return r, nil
	}

	handler := func(req *http.Request, device *authtypes.Device) util.JSONResponse {
		r, err := newRequest(req)
		if err != nil {
			return util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.Unknown(err.Error()),
			}
		}
		return w.ProcessInput(req, r, processor, device, topic)
	}
	if apiType == apiconsumer.APITypeAuth {
		w.router.Handle(path,
			common.MakeAuthAPI(metricsName, w.cacheIn, w.cfg, w.tokenFilter, w.histogram, handler),
		).Methods(methods...)
	} else if apiType == apiconsumer.APITypeExternal {
		w.router.Handle(path,
			common.MakeExternalAPI(metricsName, func(req *http.Request) util.JSONResponse {
				start := time.Now()
				response := handler(req, nil)

				duration := float64(time.Since(start)) / float64(time.Millisecond)
				code := strconv.Itoa(response.Code)
				if req.Method != "OPTION" {
					w.histogram.WithLabelValues(req.Method, metricsName, code).Observe(duration)
					// w.counter.WithLabelValues(req.Method, metricsName, code).Inc()
				}

				return response
			}),
		).Methods(methods...)
	} else if apiType == apiconsumer.APITypeInternal {
		w.router.Handle(path,
			common.MakeInternalAPI(metricsName, func(req *http.Request) util.JSONResponse {
				return handler(req, nil)
			}),
		).Methods(methods...)
	} else if apiType == apiconsumer.APITypeInternalAuth {
		w.router.Handle(path,
			common.MakeInternalAuthAPI(metricsName, w.cacheIn, handler),
		).Methods(methods...)
	} else if apiType == apiconsumer.APITypeDownload {
		w.router.Handle(path,
			http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
				start := time.Now()

				req = util.RequestWithLogging(req)

				// Set common headers returned regardless of the outcome of the request
				util.SetCORSHeaders(wr)
				// Content-Type will be overridden in case of returning file data, else we respond with JSON-formatted errors
				//wr.Header().Set("Content-Type", "application/json")

				vars := mux.Vars(req)
				fileType := vars["fileType"]
				if fileType == "" {
					fileType = metricsName
				}

				httpCode := NetDiskDownLoad(
					wr,
					req,
					vars["serverName"],
					mediatypes.MediaID(vars["mediaId"]),
					fileType,
					&w.cfg,
					w.rpcClient,
					w.idg,
					true,
				)

				duration := float64(time.Since(start)) / float64(time.Millisecond)
				code := strconv.Itoa(httpCode)
				if req.Method != "OPTION" {
					w.histogram.WithLabelValues(req.Method, metricsName, code).Observe(duration)
					// w.counter.WithLabelValues(req.Method, metricsName, code).Inc()
				}
			}),
		).Methods(methods...)
	} else if apiType == apiconsumer.APITypeUpload {
		w.router.Handle(path,
			common.MakeAuthAPI(metricsName, w.cacheIn, w.cfg, w.tokenFilter, w.histogram, func(req *http.Request, device *authtypes.Device) util.JSONResponse {
				return NetDiskUpLoad(req, &w.cfg, device)
			}),
		).Methods(methods...)
	} else if apiType == apiconsumer.APITypeFed {
		serverName := gomatrixserverlib.ServerName(domain.FirstDomain)
		w.router.Handle(path,
			common.MakeFedAPI(metricsName, serverName, w.keys, w.histogram, func(httpReq *http.Request, request *gomatrixserverlib.FederationRequest) util.JSONResponse {
				//handle whitelist
				if request == nil {
					log.Errorf("remote server is nil")
					return util.JSONResponse{
						Code: http.StatusInternalServerError,
						JSON: jsonerror.NotTrusted("empty origin"),
					}
				}
				origin := string(request.Origin())

				if !w.FilterRequest(request) {
					log.Errorf("remote server:%s is not in local server:%s whitelist", origin, serverName)
					return util.JSONResponse{
						Code: http.StatusInternalServerError,
						JSON: jsonerror.NotTrusted(origin),
					}
				}
				log.Infof("remote server:%s is in local server:%s whitelist", origin, serverName)
				r, err := newRequest(httpReq)
				if err != nil {
					return util.JSONResponse{
						Code: http.StatusInternalServerError,
						JSON: jsonerror.Unknown(err.Error()),
					}
				}
				ud := api.FedApiUserData{
					Request:    request,
					Idg:        w.idg,
					Cfg:        &w.cfg,
					RpcCli:     w.rsRpcCli,
					CacheIn:    w.cacheIn,
					KeyDB:      w.keyDB,
					LocalCache: w.localCache,
					Origin:     origin,
				}
				code, resp := processor.Process(&ud, r, nil)
				return util.JSONResponse{
					Code: code,
					JSON: resp,
				}
			}),
		).Methods(methods...)
	}
}

func (w *HttpProcessor) genInput(coder core.Coder, processor apiconsumer.APIProcessor, device *authtypes.Device) (*internals.InputMsg, []uint32, error) {
	input := new(internals.InputMsg)
	if device != nil {
		input.Device = (*internals.Device)(device)
	}
	input.MsgType = processor.GetMsgType()
	if coder != nil {
		payload, err := coder.Encode()
		if err != nil {
			return nil, nil, err
		}
		input.Payload = payload
	}
	instance := []uint32{}
	if calctor, ok := processor.(apiconsumer.CalcInstance); ok {
		instance = calctor.CalcInstance(coder, device, &w.cfg)
	}
	return input, instance, nil
}

func (w *HttpProcessor) ProcessInput(req *http.Request, coder core.Coder, processor apiconsumer.APIProcessor, device *authtypes.Device, topic string) util.JSONResponse {
	input, instance, err := w.genInput(coder, processor, device)
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: jsonerror.Unknown(err.Error()),
		}
	}

	ctx := context.Background() // dont use req.Context()
	outputMsg, err := w.send(ctx, topic, input, processor.GetMsgType(), instance)
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: jsonerror.Unknown(err.Error()),
		}
	}

	return w.handleRecv(processor, outputMsg)
}

func (w *HttpProcessor) handleRecv(processor apiconsumer.APIProcessor, outputMsg *internals.OutputMsg) util.JSONResponse {
	var respBody core.Coder
	if outputMsg.MsgType == internals.MSG_RESP_MESSAGE {
		respBody = &internals.RespMessage{}
	} else if outputMsg.MsgType == internals.MSG_RESP_ERROR {
		respBody = &internals.MatrixError{}
	} else {
		respBody = processor.NewResponse(outputMsg.Code)
	}

	var resp util.JSONResponse
	resp.Code = outputMsg.Code
	if resp.Headers != nil {
		err := json.Unmarshal(outputMsg.Headers, &resp.Headers)
		if err != nil {
			return util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.Unknown(err.Error()),
			}
		}
	}

	if respBody != nil {
		err := respBody.Decode(outputMsg.Body)
		if err != nil {
			log.Errorf("Response Coder Decode error: %s", err)
			return util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.Unknown(err.Error()),
			}
		}
		resp.JSON = respBody
	} else {
		resp.JSON = struct{}{}
	}

	return resp
}

func (w *HttpProcessor) send(ctx context.Context, topic string, input *internals.InputMsg, msgType int32, instance []uint32) (*internals.OutputMsg, error) {
	bytes, err := input.Encode()
	if err != nil {
		return nil, err
	}

	var resp []byte

	service := apiconsumer.GetAPIService(msgType)
	rpcConf, ok := w.cfg.GetRpcConfig(service)
	if !ok {
		return nil, fmt.Errorf("rpc config %s not found %d", service, msgType)
	}

	if len(instance) == 0 {
		instance = []uint32{0}
	}
	var resps [][]byte
	var errs []error
	for i := 0; i < len(instance); i++ {
		conn, err := w.rpcClient.(*grpc.Client).GetConn(rpcConf, instance[i])
		if err != nil {
			return nil, err
		}
		c := pb.NewApiServerClient(conn)
		rsp, err := c.ApiRequest(ctx, &pb.ApiRequestReq{Topic: topic, Data: bytes})
		if err != nil {
			errs = append(errs, err)
		} else {
			resps = append(resps, rsp.Data)
		}
	}
	if len(errs) > 0 {
		log.Errorf("rpc request %s err %v", service, errs)
		return nil, errs[0]
	}
	resp = resps[0]
	outputMsg := new(internals.OutputMsg)
	err = outputMsg.Decode(resp)
	if err != nil {
		log.Errorf("OutputMsg decode error %s", err.Error())
		return nil, err
	}
	return outputMsg, nil
}

func (w *HttpProcessor) OnMessage(topic string, partition int32, data []byte) {

}

func (w *HttpProcessor) FilterRequest(req *gomatrixserverlib.FederationRequest) bool {
	_, ok := w.feddomains.GetDomainInfo(string(req.Origin()))
	if !ok {
		return false
	}
	return true
}
