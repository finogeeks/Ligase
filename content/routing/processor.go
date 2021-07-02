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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/content/download"
	"github.com/finogeeks/ligase/content/repos"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/mediatypes"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/gorilla/mux"
)

const contentUri = "mxc://%s/%s"
const HeaderCustomID = "X-Consumer-Custom-ID"
const FakeUserID = "@fakeUser:fakeDomain.com"

const (
	ACTION_ADD_FAVORITE    = "add"
	ACTION_DELETE_FAVORITE = "delete"
)

var jsonContentType = []string{"application/json; charset=utf-8"}

type Processor struct {
	cfg       *config.Dendrite
	histogram mon.LabeledHistogram
	repo      *repos.DownloadStateRepo
	consumer  *download.DownloadConsumer
	idg       *uid.UidGenerator
	httpCli   *http.Client
	mediaURI  []string
}

func NewProcessor(
	cfg *config.Dendrite,
	histogram mon.LabeledHistogram,
	repo *repos.DownloadStateRepo,
	consumer *download.DownloadConsumer,
	idg *uid.UidGenerator,
	mediaURI []string,
) *Processor {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Second * 15,
		}).DialContext,
		DisableKeepAlives: true, // fd will leak if set it false(default value)
	}
	httpCli := &http.Client{Transport: transport}
	return &Processor{
		cfg:       cfg,
		histogram: histogram,
		repo:      repo,
		consumer:  consumer,
		idg:       idg,
		httpCli:   httpCli,
		mediaURI:  mediaURI,
	}
}

//add lose client query param , and client query param override server
//if bath client and server has, should use client, such as type
func (p *Processor) buildUrl(req *http.Request, reqUrl string) string {
	m, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return reqUrl
	}
	u, err := url.Parse(reqUrl)
	if err != nil {
		return reqUrl
	}
	q := u.Query()
	for k, v := range m {
		q.Set(k, v[0])
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (p *Processor) WriteHeader(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = jsonContentType
	}
}

// /upload
func (p *Processor) Upload(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	vs := req.URL.Query()
	isEmote := vs.Get("isemote") == "true"
	thumbnail := "false"
	contentType := req.Header.Get("Content-Type")
	mediaType := contentType
	if contentType == "" || strings.HasPrefix(contentType, "image") {
		mediaType = "m.image" //网盘版本不固定
		thumbnail = "true"
	}

	reqUrl := fmt.Sprintf(p.cfg.Media.UploadUrl, url.QueryEscape(mediaType), url.QueryEscape(thumbnail))
	reqUrl = p.buildUrl(req, reqUrl)
	res, err := p.httpRequest(device.UserID, req.Method, reqUrl, req)
	if err != nil {
		log.Errorw("upload file error 1", log.KeysAndValues{"user_id", device.UserID, "err", err, "url", reqUrl})
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return
	}
	if isEmote {
		p.uploadEmoteResp(rw, res, device.UserID)
		return
	}
	if res.StatusCode != http.StatusOK {
		var errInfo mediatypes.UploadError
		data, _ := ioutil.ReadAll(res.Body)
		//err = json.NewDecoder(res.Body).Decode(&errInfo)
		err = json.Unmarshal(data, &errInfo)
		if err != nil {
			log.Errorw("upload file error 2", log.KeysAndValues{"user_id", device.UserID, "err", err})
			log.Errorf("%v", data)
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte("Internal Server Error. " + err.Error()))
			return
		}
		rw.WriteHeader(res.StatusCode)
		rw.Write([]byte("Unknown Error. " + errInfo.Error))
		return
	}
	var resp mediatypes.NetDiskResponse
	data_, _ := ioutil.ReadAll(res.Body)
	//err = json.NewDecoder(res.Body).Decode(&resp)
	err = json.Unmarshal(data_, &resp)
	if err != nil {
		log.Errorw("upload file error 3", log.KeysAndValues{"user_id", device.UserID, "err", err})
		log.Errorf("%v", data_)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return
	}

	domain, _ := common.DomainFromID(device.UserID)
	resJson := mediatypes.UploadResponse{
		ContentURI: fmt.Sprintf(contentUri, domain, resp.NetDiskID),
	}

	data, _ := json.Marshal(resJson)
	//set header must before write header code, if not, set header cannot take effect
	p.WriteHeader(rw)
	rw.WriteHeader(http.StatusOK)
	rw.Write(data)
	log.Infof("userID:%s upload media succ", device.UserID)
}

// /download/{serverName}/{mediaId}
func (p *Processor) Download(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	start := time.Now()
	httpCode := http.StatusOK
	defer func() {
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		code := strconv.Itoa(httpCode)
		if req.Method != "OPTION" {
			p.histogram.WithLabelValues(req.Method, "download", code).Observe(duration)
		}
	}()
	req = util.RequestWithLogging(req)
	util.SetCORSHeaders(rw)

	vars := mux.Vars(req)

	dstDomain := vars["serverName"]
	mediaID := vars["mediaId"]

	httpCode = p.doDownload(rw, req, dstDomain, mediatypes.MediaID(mediaID), "download", true, false)
}

// /thumbnail/{serverName}/{mediaId}
func (p *Processor) Thumbnail(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	start := time.Now()
	httpCode := http.StatusOK
	defer func() {
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		code := strconv.Itoa(httpCode)
		if req.Method != "OPTION" {
			p.histogram.WithLabelValues(req.Method, "thumbnail", code).Observe(duration)
		}
	}()
	req = util.RequestWithLogging(req)
	util.SetCORSHeaders(rw)

	vars := mux.Vars(req)

	dstDomain := vars["serverName"]
	mediaID := vars["mediaId"]

	httpCode = p.doDownload(rw, req, dstDomain, mediatypes.MediaID(mediaID), "thumbnail", true, false)
}

func (p *Processor) FedDownload(rw http.ResponseWriter, req *http.Request) {
	start := time.Now()
	httpCode := http.StatusOK
	defer func() {
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		code := strconv.Itoa(httpCode)
		if req.Method != "OPTION" {
			p.histogram.WithLabelValues(req.Method, "fed download", code).Observe(duration)
		}
	}()
	req = util.RequestWithLogging(req)
	util.SetCORSHeaders(rw)

	vars := mux.Vars(req)

	dstDomain := vars["serverName"]
	mediaID := vars["mediaId"]
	fileType := vars["fileType"]

	httpCode = p.doDownload(rw, req, dstDomain, mediatypes.MediaID(mediaID), fileType, false, true)
}

func (p *Processor) FedThumbnail(rw http.ResponseWriter, req *http.Request) {
	start := time.Now()
	httpCode := http.StatusOK
	defer func() {
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		code := strconv.Itoa(httpCode)
		if req.Method != "OPTION" {
			p.histogram.WithLabelValues(req.Method, "fed thumbnail", code).Observe(duration)
		}
	}()
	req = util.RequestWithLogging(req)
	util.SetCORSHeaders(rw)

	vars := mux.Vars(req)

	dstDomain := vars["serverName"]
	mediaID := vars["mediaId"]
	fileType := vars["fileType"]

	httpCode = p.doDownload(rw, req, dstDomain, mediatypes.MediaID(mediaID), fileType, false, true)
}

func (p *Processor) doDownload(
	w http.ResponseWriter,
	req *http.Request,
	domain string,
	mediaID mediatypes.MediaID,
	fileType string,
	useFed bool,
	isFromFed bool,
) int {
	if req.Method != http.MethodGet {
		p.responseError(w, util.JSONResponse{
			Code: http.StatusMethodNotAllowed,
			JSON: jsonerror.Unknown("request method must be GET"),
		})
		return http.StatusMethodNotAllowed
	}

	netdiskID := getNetDiskID(mediaID)
	if netdiskID == "" {
		p.responseError(w, util.JSONResponse{
			Code: http.StatusNotFound,
			JSON: jsonerror.Unknown("mediaId Required"),
		})
		return http.StatusNotFound
	}

	var scaleType string
	var method, width string
	var reqUrl string
	switch fileType {
	case "download":
		reqUrl = fmt.Sprintf(p.cfg.Media.DownloadUrl, netdiskID)
	case "thumbnail":
		req.ParseForm()
		log.Infof("download thumbnail form, type: %s, method: %s, width: %s", req.Form.Get("type"), req.Form.Get("method"), req.Form.Get("width"))
		scaleType = req.Form.Get("type")
		if scaleType == "" {
			method = req.Form.Get("method")
			width = req.Form.Get("width")
			widthInt, _ := strconv.Atoi(width)
			if method == "scale" {
				if widthInt <= 100 {
					scaleType = "small"
				} else if widthInt <= 300 {
					scaleType = "middle"
				} else {
					scaleType = "large"
				}
			} else {
				scaleType = "large"
			}
		}
		reqUrl = fmt.Sprintf(p.cfg.Media.ThumbnailUrl, netdiskID, scaleType)
	}

	if p.repo.TryResponseFromLocal(domain, netdiskID, scaleType, w) {
		return http.StatusOK
	}

	var res *http.Response
	var err error
	if isFromFed {
		res, err = p.httpRequest(FakeUserID, req.Method, reqUrl, req)
	} else {
		res, err = p.httpRequest("", req.Method, reqUrl, req)
	}

	fromRemoteDomain := false
	if err != nil {
		log.Warnf("NetDiskDownLoad http response, err: %v", err)
		if !useFed || common.CheckValidDomain(domain, p.cfg.Matrix.ServerName) {
			log.Errorw("download file error", log.KeysAndValues{"mediaId", netdiskID, "err", err})
			p.responseError(w, util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: jsonerror.Unknown(err.Error()),
			})
			return http.StatusInternalServerError
		}
		fromRemoteDomain = true
	}
	if err == nil && res.StatusCode != http.StatusOK {
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorf("NetDiskDownLoad read response err: %v", err)
		}
		log.Warnf("NetDiskDownLoad http response, statusCode: %d resp: %s", res.StatusCode, data)

		if !useFed || common.CheckValidDomain(domain, p.cfg.Matrix.ServerName) {
			var errInfo mediatypes.UploadError
			err = json.Unmarshal(data, &errInfo)
			if err != nil {
				log.Errorw("download file error", log.KeysAndValues{"mediaId", netdiskID, "err", err})
				p.responseError(w, util.JSONResponse{
					Code: res.StatusCode,
					JSON: jsonerror.Unknown(err.Error()),
				})
				return res.StatusCode
			}

			p.responseError(w, util.JSONResponse{
				Code: res.StatusCode,
				JSON: jsonerror.Unknown("fail download file from netdisk : " + errInfo.Error),
			})
			return res.StatusCode
		}
		fromRemoteDomain = true
	}

	if fromRemoteDomain {
		p.consumer.AddReq(domain, netdiskID, scaleType)
		p.repo.WaitStartDownload(req.Context(), domain, netdiskID, scaleType)
		return p.doDownload(w, req, domain, mediaID, fileType, false, isFromFed)
	} else {
		log.Info("MediaId: ", netdiskID, " start download response")
		p.respDownload(w, res.Header, res.StatusCode, res.Body)
		defer func() {
			if res != nil {
				if res.Body != nil {
					res.Body.Close()
				}
			}
		}()
		log.Info("MediaId: ", netdiskID, " download success")
	}

	return http.StatusOK
}

func (p *Processor) httpRequest(userID, method, reqUrl string, req *http.Request) (*http.Response, error) {
	newReq, err := http.NewRequest(method, reqUrl, req.Body)
	if err != nil {
		return nil, err
	}

	newReq.Header = req.Header
	if userID != "" && newReq.Header.Get(HeaderCustomID) == "" {
		newReq.Header.Add(HeaderCustomID, userID)
	}
	newReq.ContentLength, _ = strconv.ParseInt(req.Header.Get("Content-Length"), 10, 0)

	headStr, _ := json.Marshal(newReq.Header)
	log.Infof("url for netdisk request, userID: %s method: %s url: %s header:%s", userID, method, reqUrl, string(headStr))

	return p.httpCli.Do(newReq)
}

func (p *Processor) respDownload(w http.ResponseWriter, header http.Header, statusCode int, body io.Reader) {
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

func (p *Processor) responseError(
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

func getNetDiskID(mediaID mediatypes.MediaID) string {
	s := strings.Split(string(mediaID), "/")
	return s[len(s)-1]
}

type CombineReader struct {
	reader io.Reader
	writer io.Writer
	req    http.Request
	wErr   error
}

func (r *CombineReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	defer func() {
		if e := recover(); e != nil {
			return
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
	return
}

func mapGetString(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func response(w http.ResponseWriter, status int, msg string) {

}

func (p *Processor) checkRequest(req *http.Request) {

}

func (p *Processor) forwardProxy(rw http.ResponseWriter, req *http.Request, url string, data []byte, op string) []byte {
	newReq, err := http.NewRequest(req.Method, url, bytes.NewReader(data))
	if err != nil {
		log.Errorf("%s: new request error %v", op, err)
		return nil
	}

	// newReq.Header = req.Header.Clone()
	for k, v := range req.Header {
		for _, vv := range v {
			newReq.Header.Add(k, vv)
		}
	}
	newReq.ContentLength = int64(len(data))

	resp, err := p.httpCli.Do(newReq)
	if err != nil {
		log.Errorf("%s response error url:%s %v", op, url, err)
		return nil
	}
	if resp == nil {
		log.Errorf("%s response nil url:%s", op, url)
		return nil
	}

	for k, v := range resp.Header {
		for _, vv := range v {
			rw.Header().Add(k, vv)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	var respData []byte
	if resp.Body != nil {
		respData, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("%s response body read err: %v", op, err)
			return nil
		}
		defer resp.Body.Close()
		rw.Write(respData)
	}

	return respData
}

type FavoriteOutput struct {
	Action     string `json:"action"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	SenderID   string `json:"senderID"`
	SenderName string `json:"senderName"`
	RoomID     string `json:"roomID"`
	Timestamp  int64  `json:"timestamp"`
	OwnerID    string `json:"ownerID"`
	OwnerName  string `json:"ownerName"`
	NetdiskID  string `json:"netdiskID"`
}

type ForwardRequest struct {
	Type         string `json:"type"`
	Content      string `json:"content"`
	SenderID     string `json:"senderID"`
	From         string `json:"from"`
	SrcNetdiskID string `json:"srcNetdiskID"`
	SrcRoom      string `json:"srcRoom"`   // 留给大数据用
	SrcPeople    string `json:"srcPeople"` // 留给大数据用
	Public       bool   `json:"public"`    // 只有forward用到
	Traceable    bool   `json:"traceable"`
	OwnerID      string `json:"ownerID"`
	OwnerName    string `json:"ownerName"`
}

type FavoriteResponse struct {
	NetdiskID string `json:"netdiskID"`
}

func (p *Processor) Favorite(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	if req.Body == nil {
		log.Errorf("Favorite %s req.Body is nil", req.Header.Get(HeaderCustomID))
		return
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("Favorite %s req body error %v", req.Header.Get(HeaderCustomID), err)
		return
	}
	defer req.Body.Close()
	var forwardReq ForwardRequest
	err = json.Unmarshal(data, &forwardReq)
	if err != nil {
		log.Errorf("Favorite %s unmarshal %v error %v", req.Header.Get(HeaderCustomID), data, err)
		return
	}

	var content map[string]interface{}
	err = json.Unmarshal([]byte(forwardReq.Content), &content)
	if err != nil {
		log.Errorf("Favorite %s unmarshal content %v error %v", forwardReq.Content, err)
		return
	}

	if common.IsMediaEv(content) {
		if url, ok := mapGetString(content, "o_url"); ok {
			domain, netdiskID := common.SplitMxc(url)
			if domain != "" {
				p.repo.Wait(req.Context(), domain, "", netdiskID)
			}
		}
	}

	reqURI := req.RequestURI
	for _, v := range p.mediaURI {
		if strings.HasPrefix(reqURI, v) {
			reqURI = strings.TrimPrefix(reqURI, v)
			break
		}
	}

	reqUrl := p.cfg.Media.NetdiskUrl + reqURI
	log.Debugf("forwardproxy url %s", reqUrl)
	resp := p.forwardProxy(rw, req, reqUrl, data, "Favorite")
	if resp != nil {
		var respInfo FavoriteResponse
		if err := json.Unmarshal(resp, &respInfo); err != nil {
			log.Errorf("Failed to unmarshal FavoriteResponse: %v\n", err)
			return
		}

		output := FavoriteOutput{
			Action:     ACTION_ADD_FAVORITE,
			Type:       forwardReq.Type,
			Content:    forwardReq.Content,
			SenderID:   forwardReq.SenderID,
			SenderName: forwardReq.From,
			RoomID:     forwardReq.SrcRoom,
			Timestamp:  time.Now().UnixNano() / 1000000,
			OwnerID:    forwardReq.OwnerID,
			OwnerName:  forwardReq.OwnerName,
			NetdiskID:  respInfo.NetdiskID,
		}

		common.GetTransportMultiplexer().SendAndRecvWithRetry(
			p.cfg.Kafka.Producer.FavoriteInfo.Underlying,
			p.cfg.Kafka.Producer.FavoriteInfo.Name,
			&core.TransportPubMsg{
				Keys: []byte(output.RoomID),
				Obj:  output,
			})
	}
}

func (p *Processor) Unfavorite(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	reqURI := req.RequestURI
	for _, v := range p.mediaURI {
		if strings.HasPrefix(reqURI, v) {
			reqURI = strings.TrimPrefix(reqURI, v)
			break
		}
	}

	reqUrl := p.cfg.Media.NetdiskUrl + reqURI
	resp := p.forwardProxy(rw, req, reqUrl, nil, "Unfavorite")
	if resp != nil {
		vars := mux.Vars(req)
		netdiskID := vars["netdiskID"]
		output := FavoriteOutput{
			Action:    ACTION_DELETE_FAVORITE,
			NetdiskID: netdiskID,
		}

		common.GetTransportMultiplexer().SendAndRecvWithRetry(
			p.cfg.Kafka.Producer.FavoriteInfo.Underlying,
			p.cfg.Kafka.Producer.FavoriteInfo.Name,
			&core.TransportPubMsg{
				Keys: []byte(output.RoomID),
				Obj:  output,
			})
	}
}

func (p *Processor) SingleForward(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	if req.Body == nil {
		log.Errorf("Favorite %s req.Body is nil", req.Header.Get(HeaderCustomID))
		return
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("Favorite %s req body error %v", req.Header.Get(HeaderCustomID), err)
		return
	}
	defer req.Body.Close()
	var forwardReq ForwardRequest
	err = json.Unmarshal(data, &forwardReq)
	if err != nil {
		log.Errorf("Favorite %s unmarshal %v error %v", req.Header.Get(HeaderCustomID), data, err)
		return
	}

	var content map[string]interface{}
	err = json.Unmarshal([]byte(forwardReq.Content), &content)
	if err != nil {
		log.Errorf("Favorite %s unmarshal content %v error %v", forwardReq.Content, err)
		return
	}

	if common.IsMediaEv(content) {
		if url, ok := mapGetString(content, "o_url"); ok {
			domain, netdiskID := common.SplitMxc(url)
			if domain != "" {
				p.repo.Wait(req.Context(), domain, "", netdiskID)
			}
		}
	}

	reqURI := req.RequestURI
	for _, v := range p.mediaURI {
		if strings.HasPrefix(reqURI, v) {
			reqURI = strings.TrimPrefix(reqURI, v)
			break
		}
	}

	reqUrl := p.cfg.Media.NetdiskUrl + reqURI
	log.Debugf("forwardproxy url %s", reqUrl)
	p.forwardProxy(rw, req, reqUrl, data, "SingleForward")
}

type MultiForwardRequest struct {
	Type         string      `json:"type"`
	Content      string      `json:"content"`
	From         string      `json:"from"`
	TargetRooms  []string    `json:"target_rooms"`
	TargetUsers  []string    `json:"target_users"`
	SrcNetdiskID string      `json:"srcNetdiskID"`
	SrcRoom      string      `json:"srcRoom"`   // 留给大数据用
	SrcPeople    string      `json:"srcPeople"` // 留给大数据用
	Public       bool        `json:"public"`
	Traceable    bool        `json:"traceable"`
	SecurityWall interface{} `json:"securityWall"`
}

func (p *Processor) MultiForward(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	if req.Body == nil {
		log.Errorf("Favorite %s req.Body is nil", req.Header.Get(HeaderCustomID))
		return
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("Favorite %s req body error %v", req.Header.Get(HeaderCustomID), err)
		return
	}
	defer req.Body.Close()
	var forwardReq MultiForwardRequest
	err = json.Unmarshal(data, &forwardReq)
	if err != nil {
		log.Errorf("Favorite %s unmarshal %v error %v", req.Header.Get(HeaderCustomID), data, err)
		return
	}

	var content map[string]interface{}
	err = json.Unmarshal([]byte(forwardReq.Content), &content)
	if err != nil {
		log.Errorf("Favorite %s unmarshal content %v error %v", forwardReq.Content, err)
		return
	}

	if common.IsMediaEv(content) {
		if url, ok := mapGetString(content, "o_url"); ok {
			domain, netdiskID := common.SplitMxc(url)
			if domain != "" {
				p.repo.Wait(req.Context(), domain, "", netdiskID)
			}
		}
	}

	reqURI := req.RequestURI
	for _, v := range p.mediaURI {
		if strings.HasPrefix(reqURI, v) {
			reqURI = strings.TrimPrefix(reqURI, v)
			break
		}
	}

	reqUrl := p.cfg.Media.NetdiskUrl + reqURI
	log.Debugf("forwardproxy url %s", reqUrl)
	p.forwardProxy(rw, req, reqUrl, data, "MultiForward")
}

type MultiForwardPublicRequest struct {
	Fcid          string   `json:"fcid"`
	SrcNetdiskIDs []string `json:"srcNetdiskIDs"`
}

func (p *Processor) MultiResForward(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	if req.Body == nil {
		log.Errorf("Favorite %s req.Body is nil", req.Header.Get(HeaderCustomID))
		return
	}
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("Favorite %s req body error %v", req.Header.Get(HeaderCustomID), err)
		return
	}
	defer req.Body.Close()
	var forwardReq MultiForwardPublicRequest
	err = json.Unmarshal(data, &forwardReq)
	if err != nil {
		log.Errorf("Favorite %s unmarshal %v error %v", req.Header.Get(HeaderCustomID), data, err)
		return
	}

	for _, v := range forwardReq.SrcNetdiskIDs {
		domain, netdiskID := common.SplitMxc(v)
		if domain != "" {
			p.repo.Wait(req.Context(), domain, "", netdiskID)
		}
	}

	reqURI := req.RequestURI
	for _, v := range p.mediaURI {
		if strings.HasPrefix(reqURI, v) {
			reqURI = strings.TrimPrefix(reqURI, v)
			break
		}
	}

	reqUrl := p.cfg.Media.NetdiskUrl + reqURI
	log.Debugf("forwardproxy url %s", reqUrl)
	p.forwardProxy(rw, req, reqUrl, data, "MultiResForward")
}
