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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetServices("proxy")
	apiconsumer.SetAPIProcessor(ReqPostMediaUpload{})
	apiconsumer.SetAPIProcessor(ReqGetMediaDownload{})
	apiconsumer.SetAPIProcessor(ReqGetMediaThumbnail{})
}

const ProxyMediaAPITopic = "proxyMediaApi"

type ReqPostMediaUpload struct{}

func (ReqPostMediaUpload) GetRoute() string       { return "/upload" }
func (ReqPostMediaUpload) GetMetricsName() string { return "upload" }
func (ReqPostMediaUpload) GetMsgType() int32      { return internals.MSG_POST_MEDIA_UPLOAD }
func (ReqPostMediaUpload) GetAPIType() int8       { return apiconsumer.APITypeUpload }
func (ReqPostMediaUpload) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostMediaUpload) GetTopic(cfg *config.Dendrite) string { return ProxyMediaAPITopic }
func (ReqPostMediaUpload) GetPrefix() []string                  { return []string{"mediaR0", "mediaV1"} }
func (ReqPostMediaUpload) NewRequest() core.Coder {
	return new(external.PostMediaUploadRequest)
}
func (ReqPostMediaUpload) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostMediaUploadRequest)
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostMediaUpload) NewResponse(code int) core.Coder {
	return new(external.PostMediaUploadResponse)
}
func (ReqPostMediaUpload) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return 200, nil
}

type ReqGetMediaDownload struct{}

func (ReqGetMediaDownload) GetRoute() string       { return "/download/{serverName}/{mediaId}" }
func (ReqGetMediaDownload) GetMetricsName() string { return "download" }
func (ReqGetMediaDownload) GetMsgType() int32      { return internals.MSG_GET_MEDIA_DOWNLOAD }
func (ReqGetMediaDownload) GetAPIType() int8       { return apiconsumer.APITypeDownload }
func (ReqGetMediaDownload) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetMediaDownload) GetTopic(cfg *config.Dendrite) string { return ProxyMediaAPITopic }
func (ReqGetMediaDownload) GetPrefix() []string                  { return []string{"mediaR0", "mediaV1"} }
func (ReqGetMediaDownload) NewRequest() core.Coder {
	return new(external.GetMediaDownloadRequest)
}
func (ReqGetMediaDownload) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetMediaDownloadRequest)
	if vars != nil {
		msg.ServerName = vars["servarName"]
		msg.MediaID = vars["mediaId"]
	}
	return nil
}
func (ReqGetMediaDownload) NewResponse(code int) core.Coder {
	return nil
}
func (ReqGetMediaDownload) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return 200, nil
}

type ReqGetMediaThumbnail struct{}

func (ReqGetMediaThumbnail) GetRoute() string       { return "/thumbnail/{serverName}/{mediaId}" }
func (ReqGetMediaThumbnail) GetMetricsName() string { return "thumbnail" }
func (ReqGetMediaThumbnail) GetMsgType() int32      { return internals.MSG_GET_MEDIA_THUMBNAIL }
func (ReqGetMediaThumbnail) GetAPIType() int8       { return apiconsumer.APITypeDownload }
func (ReqGetMediaThumbnail) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetMediaThumbnail) GetTopic(cfg *config.Dendrite) string { return ProxyMediaAPITopic }
func (ReqGetMediaThumbnail) GetPrefix() []string                  { return []string{"mediaR0", "mediaV1"} }
func (ReqGetMediaThumbnail) NewRequest() core.Coder {
	return new(external.GetMediaDownloadRequest)
}
func (ReqGetMediaThumbnail) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetMediaDownloadRequest)
	if vars != nil {
		msg.ServerName = vars["servarName"]
		msg.MediaID = vars["mediaId"]
	}
	return nil
}
func (ReqGetMediaThumbnail) NewResponse(code int) core.Coder {
	return nil
}
func (ReqGetMediaThumbnail) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	return 200, nil
}
