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
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/mediatypes"
	"github.com/finogeeks/ligase/federation/fedreq/rpc"
)

const contentUri = "mxc://%s/%s"

func NetDiskUpLoad(
	req *http.Request,
	cfg *config.Dendrite,
	device *authtypes.Device,
) util.JSONResponse {
	if req.Method != http.MethodPost {
		return util.JSONResponse{
			Code: http.StatusMethodNotAllowed,
			JSON: jsonerror.Unknown("HTTP request method must be POST."),
		}
	}

	thumbnail := "false"
	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "m.image"
		thumbnail = "true"
	}
	if strings.HasPrefix(contentType, "image") {
		contentType = "m.image" //网盘版本不固定
		thumbnail = "true"
	}

	reqUrl := fmt.Sprintf(cfg.Media.UploadUrl, contentType, thumbnail)

	res, err := httpRequest(device.UserID, req.Method, reqUrl, req)

	if err != nil {
		log.Errorw("upload file error", log.KeysAndValues{"user_id", device.UserID, "err", err})
		return httputil.LogThenError(req, err)
	}
	if res.StatusCode != http.StatusOK {
		var errInfo mediatypes.UploadError
		err = json.NewDecoder(res.Body).Decode(&errInfo)
		if err != nil {
			log.Errorw("upload file error", log.KeysAndValues{"user_id", device.UserID, "err", err})
			return httputil.LogThenError(req, err)
		}
		return util.JSONResponse{
			Code: res.StatusCode,
			JSON: jsonerror.Unknown("upload file error : " + errInfo.Error),
		}
	}

	var resp mediatypes.NetDiskResponse
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		log.Errorw("upload file error", log.KeysAndValues{"user_id", device.UserID, "err", err})
		return httputil.LogThenError(req, err)
	}

	domain, _ := common.DomainFromID(device.UserID)
	resJson := mediatypes.UploadResponse{
		ContentURI: fmt.Sprintf(contentUri, domain, resp.NetDiskID),
	}

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: resJson,
	}
}

type CombineReader struct {
	reader io.Reader
	writer io.Writer
	wErr   error
}

func (r *CombineReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("fed upload err: %v", err)
		}
	}()
	if n > 0 && r.wErr == nil {
		idx := 0
		for idx < n {
			nn, err := r.writer.Write(p[idx:n])
			idx += nn
			if err != nil {
				r.wErr = err
				break
			}
		}
	}
	return n, err
}

func NetDiskDownLoad(
	w http.ResponseWriter,
	req *http.Request,
	service string,
	mediaID mediatypes.MediaID,
	fileType string,
	cfg *config.Dendrite,
	rpcCli *common.RpcClient,
	idg *uid.UidGenerator,
	useFed bool,
) int {
	if req.Method != http.MethodGet {
		responseError(w, util.JSONResponse{
			Code: http.StatusMethodNotAllowed,
			JSON: jsonerror.Unknown("request method must be GET"),
		})
		return http.StatusMethodNotAllowed
	}

	if !useFed && !common.CheckValidDomain(service, cfg.Matrix.ServerName) {
		responseError(w, util.JSONResponse{
			Code: http.StatusNotFound,
			JSON: jsonerror.Unknown("Unknown serverName"),
		})
		return http.StatusNotFound
	}

	mediaID = getMediaId(mediaID)
	if mediaID == "" {
		responseError(w, util.JSONResponse{
			Code: http.StatusNotFound,
			JSON: jsonerror.Unknown("mediaId Required"),
		})
		return http.StatusNotFound
	}

	var method, width string
	var reqUrl string
	switch fileType {
	case "download":
		reqUrl = fmt.Sprintf(cfg.Media.DownloadUrl, mediaID)
	case "thumbnail":
		req.ParseForm()
		method = req.Form.Get("method")
		width = req.Form.Get("width")
		widthInt, _ := strconv.Atoi(width)
		if method == "scale" {
			if widthInt <= 100 {
				reqUrl = fmt.Sprintf(cfg.Media.ThumbnailUrl, mediaID, "small")
			} else if widthInt <= 300 {
				reqUrl = fmt.Sprintf(cfg.Media.ThumbnailUrl, mediaID, "middle")
			} else {
				reqUrl = fmt.Sprintf(cfg.Media.ThumbnailUrl, mediaID, "large")
			}
		} else {
			reqUrl = fmt.Sprintf(cfg.Media.ThumbnailUrl, mediaID, "large")
		}
	}

	res, err := httpRequest("", req.Method, reqUrl, req)

	fromRemoteDomain := false
	if err != nil {
		log.Warnf("NetDiskDownLoad http response, err: %v, res: %v", err, res)
		if !useFed || common.CheckValidDomain(service, cfg.Matrix.ServerName) {
			log.Errorw("download file error", log.KeysAndValues{"mediaId", mediaID, "err", err})
			responseError(w, util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.Unknown(err.Error()),
			})
			return http.StatusInternalServerError
		}
		fromRemoteDomain = true
	}
	if err == nil && res.StatusCode != http.StatusOK {
		log.Warnf("NetDiskDownLoad http response, err: %v, res: %v", err, res)
		if !useFed || common.CheckValidDomain(service, cfg.Matrix.ServerName) {
			var errInfo mediatypes.UploadError
			err = json.NewDecoder(res.Body).Decode(&errInfo)
			if err != nil {
				log.Errorw("download file error", log.KeysAndValues{"mediaId", mediaID, "err", err})
				responseError(w, util.JSONResponse{
					Code: res.StatusCode,
					JSON: jsonerror.Unknown(err.Error()),
				})
				return res.StatusCode
			}

			responseError(w, util.JSONResponse{
				Code: res.StatusCode,
				JSON: jsonerror.Unknown("fail download file from net disk : " + errInfo.Error),
			})
			return res.StatusCode
		}
		fromRemoteDomain = true
	}
	if fromRemoteDomain {
		log.Info("MediaId: ", mediaID, " download from remote")
		id, _ := idg.Next()
		body, statusCode, header, err := rpc.Download(cfg, rpcCli, strconv.FormatInt(id, 36), service, string(mediaID), width, method, fileType)
		if err != nil {
			responseError(w, util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.Unknown(err.Error()),
			})
			return http.StatusInternalServerError
		}

		for key, value := range header {
			for _, v := range value {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(statusCode)

		if statusCode/100 != 2 {
			log.Errorf("fed download, from remote statusCode: %d", statusCode)
			return statusCode
		}

		cr := &CombineReader{reader: body, writer: w}

		thumbnail := "false"
		contentType := header.Get("Content-Type")
		if contentType == "" {
			contentType = "m.image"
			thumbnail = "true"
		}
		if strings.HasPrefix(contentType, "image") {
			contentType = "m.image" //网盘版本不固定
			thumbnail = "true"
		}

		reqUrl := fmt.Sprintf(cfg.Media.UploadUrl, contentType, thumbnail)

		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: time.Second * 15,
			}).DialContext,
			DisableKeepAlives: true, // fd will leak if set it false(default value)
		}
		client := &http.Client{Transport: transport}

		newReq, err := http.NewRequest("POST", reqUrl, cr)
		if err != nil {
			log.Errorf("fed download, upload to local NewRequest error: %v", err)
			return http.StatusInternalServerError
		}

		headStr, _ := json.Marshal(header)
		log.Infof("fed download, header for media upload request: %s, %s", string(headStr), header.Get("Content-Length"))

		newReq.Header = header
		newReq.ContentLength, _ = strconv.ParseInt(req.Header.Get("Content-Length"), 10, 0)

		headStr, _ = json.Marshal(newReq.Header)
		log.Infof("fed download, header for net disk request: %s", string(headStr))

		res, err := client.Do(newReq)

		if err != nil {
			log.Errorf("fed download, upload file error: %v", err)
			return http.StatusInternalServerError
		}
		if res.StatusCode != http.StatusOK {
			var errInfo mediatypes.UploadError
			err = json.NewDecoder(res.Body).Decode(&errInfo)
			if err != nil {
				log.Errorf("fed download, upload file error: %v", err)
				return res.StatusCode
			}
		}

		var resp mediatypes.NetDiskResponse
		err = json.NewDecoder(res.Body).Decode(&resp)
		if err != nil {
			log.Errorf("fed download, upload file error: %v", err)
			return http.StatusInternalServerError
		}

		// domain, _ := common.DomainFromID(device.UserID)
		// resJson := mediatypes.UploadResponse{
		// 	ContentURI: fmt.Sprintf(contentUri, domain, resp.NetDiskID),
		// }

		//respDownload(w, header, statusCode, body)
		log.Info("MediaId: ", mediaID, " download in fed success")
	} else {
		log.Info("MediaId: ", mediaID, " start download response")
		respDownload(w, res.Header, res.StatusCode, res.Body)
		defer func() {
			if res != nil {
				if res.Body != nil {
					res.Body.Close()
				}
			}
		}()
		log.Info("MediaId: ", mediaID, " download success")
	}

	return http.StatusOK
}

func respDownload(w http.ResponseWriter, header http.Header, statusCode int, body io.Reader) {
	for key, value := range header {
		for _, v := range value {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(statusCode)
	n, err := io.Copy(w, body)
	if err != nil {
		log.Errorf("download io.Copy error: %#v, copyN %d", err, n)
	}
}

func getMediaId(mediaID mediatypes.MediaID) mediatypes.MediaID {
	s := strings.Split(string(mediaID), "/")
	return mediatypes.MediaID(s[len(s)-1])
}

func responseError(
	w http.ResponseWriter,
	res util.JSONResponse,
) {
	resBytes, err := json.Marshal(res.JSON)
	if err != nil {
		log.Errorw("Failed to marshal JSONResponse", log.KeysAndValues{"error", err})
		// this should never fail to be marshalled so drop err to the floor
		res = util.MessageResponse(http.StatusInternalServerError, "Internal Server Error")
		resBytes, _ = json.Marshal(res.JSON)
	}

	w.WriteHeader(res.Code)
	log.Infow(fmt.Sprintf("Responding (%d bytes)", len(resBytes)), log.KeysAndValues{"code", res.Code})

	// we don't really care that much if we fail to write the error response
	w.Write(resBytes) // nolint: errcheck
}

func httpRequest(userID, method, reqUrl string, req *http.Request) (*http.Response, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Second * 15,
		}).DialContext,
		DisableKeepAlives: true, // fd will leak if set it false(default value)
	}
	client := &http.Client{Transport: transport}

	newReq, err := http.NewRequest(method, reqUrl, req.Body)
	if err != nil {
		return nil, err
	}

	headStr, _ := json.Marshal(req.Header)
	log.Infof("header for media upload request: %s, %s", string(headStr), req.Header.Get("Content-Length"))

	newReq.Header = req.Header
	if userID != "" {
		newReq.Header.Add("X-Consumer-Custom-ID", userID)
	}
	newReq.ContentLength, _ = strconv.ParseInt(req.Header.Get("Content-Length"), 10, 0)

	headStr, _ = json.Marshal(newReq.Header)
	log.Infof("header for net disk request: %s", string(headStr))

	return client.Do(newReq)
}
