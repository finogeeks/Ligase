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
	"context"
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func GetRoomTags(
	ctx context.Context,
	cache service.Cache,
	userID string,
	r *external.GetRoomsTagsByIDRequest,
) (int, core.Coder) {
	// if req.Method != http.MethodGet {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	if r.UserId != userID {
		return http.StatusForbidden, jsonerror.Forbidden("Cannot get room tags for other users")
	}

	tagIDs, _ := cache.GetRoomTagIds(r.UserId, r.RoomId)
	var tagContent interface{}
	tagMap := make(map[string]interface{})

	for _, tagID := range tagIDs {
		tag, _ := cache.GetRoomTagCacheData(tagID)
		err := json.Unmarshal([]byte(tag.Content), &tagContent)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}
		tagMap[tag.Tag] = tagContent
	}

	return http.StatusOK, &external.GetRoomsTagsByIDResponse{
		Tags: tagMap,
	}
}

func SetRoomTag(
	ctx context.Context,
	req *external.PutRoomsTagsByIDRequest,
	accountDB model.AccountsDatabase,
	userID string,
	cfg config.Dendrite,
) (int, core.Coder) {
	// if req.Method != http.MethodPut {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	if req.UserId != userID {
		return http.StatusForbidden, jsonerror.Forbidden("Cannot set room tags for other users")
	}

	// var content interface{}
	// if reqErr := httputil.UnmarshalJSONRequest(req, &content); reqErr != nil {
	// 	return *reqErr
	// }

	// contentBytes, err := json.Marshal(content)
	// if err != nil {
	// 	return httputil.LogThenError(req, err)
	// }
	contentBytes := req.Content

	log.Infof("SetRoomTag userID %s roomID %s tag %s content %s", req.UserId, req.RoomId, req.Tag, string(contentBytes))

	err := accountDB.AddRoomTag(ctx, req.UserId, req.RoomId, req.Tag, contentBytes)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	data := new(types.ActDataStreamUpdate)
	data.UserID = req.UserId
	data.RoomID = req.RoomId
	data.DataType = ""
	data.StreamType = "roomTag"

	span, ctx := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(req.UserId),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}

func DeleteRoomTag(
	ctx context.Context,
	req *external.DelRoomsTagsByIDRequest,
	accountDB model.AccountsDatabase,
	userID string,
	cfg config.Dendrite,
) (int, core.Coder) {
	// if req.Method != http.MethodDelete {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }

	if userID != req.UserId {
		return http.StatusForbidden, jsonerror.Forbidden("Cannot delete room tags for other users")
	}

	err := accountDB.DeleteRoomTag(ctx, req.UserId, req.RoomId, req.Tag)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	data := new(types.ActDataStreamUpdate)
	data.UserID = req.UserId
	data.RoomID = req.RoomId
	data.DataType = ""
	data.StreamType = "roomTag"

	span, ctx := common.StartSpanFromContext(ctx, cfg.Kafka.Producer.OutputClientData.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, cfg.Kafka.Producer.OutputClientData.Name,
		cfg.Kafka.Producer.OutputClientData.Underlying)
	common.GetTransportMultiplexer().SendWithRetry(
		cfg.Kafka.Producer.OutputClientData.Underlying,
		cfg.Kafka.Producer.OutputClientData.Name,
		&core.TransportPubMsg{
			Keys:    []byte(req.UserId),
			Obj:     data,
			Headers: common.InjectSpanToHeaderForSending(span),
		})

	return http.StatusOK, nil
}
