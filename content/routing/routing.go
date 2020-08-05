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
	"encoding/json"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/content/download"
	"github.com/finogeeks/ligase/content/repos"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/gorilla/mux"
)

func Setup(
	apiMux *mux.Router,
	cfg *config.Dendrite,
	cacheIn service.Cache,
	feddomains *common.FedDomains,
	repo *repos.DownloadStateRepo,
	rpcCli *common.RpcClient,
	consumer *download.DownloadConsumer,
	idg *uid.UidGenerator,
) {
	monitor := mon.GetInstance()
	histogram := monitor.NewLabeledHistogram(
		"http_requests_duration_milliseconds",
		[]string{"method", "path", "code"},
		[]float64{50.0, 100.0, 500.0, 1000.0, 2000.0, 5000.0},
	)

	prefixR0 := "/_matrix/media/r0"
	prefixV1 := "/_matrix/media/v1"

	muxR0 := apiMux.PathPrefix(prefixR0).Subrouter()
	muxV1 := apiMux.PathPrefix(prefixV1).Subrouter()

	processor := NewProcessor(cfg, histogram, repo, rpcCli, consumer, idg, []string{prefixR0, prefixV1})

	makeMediaAPI(muxR0, true, "/upload", processor.Upload, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/upload", processor.Upload, rpcCli, http.MethodPost, http.MethodOptions)

	makeMediaAPI(muxR0, false, "/download/{serverName}/{mediaId}", processor.Download, rpcCli, http.MethodGet, http.MethodOptions)
	makeMediaAPI(muxV1, false, "/download/{serverName}/{mediaId}", processor.Download, rpcCli, http.MethodGet, http.MethodOptions)

	makeMediaAPI(muxR0, false, "/thumbnail/{serverName}/{mediaId}", processor.Thumbnail, rpcCli, http.MethodGet, http.MethodOptions)
	makeMediaAPI(muxV1, false, "/thumbnail/{serverName}/{mediaId}", processor.Thumbnail, rpcCli, http.MethodGet, http.MethodOptions)

	makeMediaAPI(muxR0, true, "/favorite", processor.Favorite, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/favorite", processor.Favorite, rpcCli, http.MethodPost, http.MethodOptions)

	makeMediaAPI(muxR0, true, "/unfavorite/{netdiskID}", processor.Unfavorite, rpcCli, http.MethodDelete, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/unfavorite/{netdiskID}", processor.Unfavorite, rpcCli, http.MethodDelete, http.MethodOptions)

	makeMediaAPI(muxR0, true, "/forward/room/{roomID}", processor.SingleForward, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/forward/room/{roomID}", processor.SingleForward, rpcCli, http.MethodPost, http.MethodOptions)

	makeMediaAPI(muxR0, true, "/multi-forward", processor.MultiForward, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/multi-forward", processor.MultiForward, rpcCli, http.MethodPost, http.MethodOptions)

	makeMediaAPI(muxR0, true, "/multi-forward-res", processor.MultiResForward, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/multi-forward-res", processor.MultiResForward, rpcCli, http.MethodPost, http.MethodOptions)

	//emote
	//wait eif emote upload finish
	makeMediaAPI(muxR0, true, "/wait/emote", processor.WaitEmote, rpcCli, http.MethodGet, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/wait/emote", processor.WaitEmote, rpcCli, http.MethodGet, http.MethodOptions)
	//check emote is exsit
	makeMediaAPI(muxR0, true, "/check/emote/{serverName}/{mediaId}", processor.CheckEmote, rpcCli, http.MethodGet, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/check/emote/{serverName}/{mediaId}", processor.CheckEmote, rpcCli, http.MethodGet, http.MethodOptions)
	//favorite emote
	makeMediaAPI(muxR0, true, "/favorite/emote/{serverName}/{mediaId}", processor.FavoriteEmote, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/favorite/emote/{serverName}/{mediaId}", processor.FavoriteEmote, rpcCli, http.MethodPost, http.MethodOptions)
	//get emote list
	makeMediaAPI(muxR0, true, "/list/emote", processor.ListEmote, rpcCli, http.MethodGet, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/list/emote", processor.ListEmote, rpcCli, http.MethodGet, http.MethodOptions)
	//favorite file to emote
	makeMediaAPI(muxR0, true, "/favorite/fileemote/{serverName}/{mediaId}", processor.FavoriteFileEmote, rpcCli, http.MethodPost, http.MethodOptions)
	makeMediaAPI(muxV1, true, "/favorite/fileemote/{serverName}/{mediaId}", processor.FavoriteFileEmote, rpcCli, http.MethodPost, http.MethodOptions)

	fedV1 := apiMux.PathPrefix("/_matrix/federation/v1/media").Subrouter()
	makeFedAPI(fedV1, "/download/{serverName}/{mediaId}/{fileType}", processor.FedDownload, http.MethodGet, http.MethodOptions)
	makeFedAPI(fedV1, "/thumbnail/{serverName}/{mediaId}/{fileType}", processor.FedThumbnail, http.MethodGet, http.MethodOptions)
}

func verifyToken(rw http.ResponseWriter, req *http.Request, rpcCli *common.RpcClient) (*authtypes.Device, bool) {
	token, err := common.ExtractAccessToken(req)
	if err != nil {
		log.Errorf("Content token error %s %v", req.RequestURI, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return nil, false
	}

	rpcReq := types.VerifyTokenRequest{
		Token:      token,
		RequestURI: req.RequestURI,
	}
	reqData, _ := json.Marshal(&rpcReq)
	data, err := rpcCli.Request(types.VerifyTokenTopicDef, reqData, 30000)
	if err != nil {
		log.Errorf("Content token verify error %s %v", req.RequestURI, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return nil, false
	}

	content := types.VerifyTokenResponse{}
	err = json.Unmarshal(data, &content)
	if err != nil {
		log.Errorf("Content verify token response unmarshal error %s %v", req.RequestURI, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return nil, false
	}
	if content.Error != "" {
		log.Errorf("Content verify token response error %s %v", req.RequestURI, err)
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte(content.Error))
		return nil, false
	}

	device := content.Device

	return &device, true
}

func makeMediaAPI(r *mux.Router, atuh bool, url string, handler func(http.ResponseWriter, *http.Request, *authtypes.Device), rpcCli *common.RpcClient, method ...string) {
	r.HandleFunc(url, func(rw http.ResponseWriter, req *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("Media API: %s panic %v", req.RequestURI, e)
			}
		}()
		if atuh {
			device, ok := verifyToken(rw, req, rpcCli)
			if !ok {
				return
			}
			handler(rw, req, device)
		} else {
			handler(rw, req, nil)
		}
	}).Methods(method...)
}

func makeFedAPI(r *mux.Router, url string, handler func(http.ResponseWriter, *http.Request), method ...string) {
	r.HandleFunc(url, func(rw http.ResponseWriter, req *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("Fed API: %s panic %v", req.RequestURI, e)
			}
		}()
		handler(rw, req)
	}).Methods(method...)
}
