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

package directory

import (
	"context"
	"net/http"
	"strconv"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type publicRoomReq struct {
	Since  string `json:"since,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
	Filter filter `json:"filter,omitempty"`
}

type filter struct {
	SearchTerms string `json:"generic_search_term,omitempty"`
}

// GetPublicRooms implements GET /publicRooms
func GetPublicRooms(
	ctx context.Context,
	request *external.GetPublicRoomsRequest,
	publicRoomDatabase model.PublicRoomAPIDatabase,
) (int, core.Coder) {
	var limit int64
	var offset int64
	//var request publicRoomReq
	var response external.GetPublicRoomsResponse

	// if fillErr := fillPublicRoomsReq(req, &request); fillErr != nil {
	// 	return *fillErr
	// }

	limit = request.Limit
	offset, err := strconv.ParseInt(request.Since, 10, 64)
	// ParseInt returns 0 and an error when trying to parse an empty string
	// In that case, we want to assign 0 so we ignore the error
	if err != nil && len(request.Since) > 0 {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if response.Estimate, err = publicRoomDatabase.CountPublicRooms(ctx); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if offset > 0 {
		response.PrevBatch = strconv.Itoa(int(offset) - 1)
	}
	nextIndex := int(offset) + int(limit)
	if response.Estimate > int64(nextIndex) {
		response.NextBatch = strconv.Itoa(nextIndex)
	}

	if chunk, err := publicRoomDatabase.GetPublicRooms(
		ctx, offset, limit, request.Filter.SearchTerms,
	); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	} else {
		for _, v := range chunk {
			response.Chunk = append(response.Chunk, external.PublicRoomsChunk(v))
		}
	}

	return http.StatusOK, &response
}

// PostPublicRooms implements POST /publicRooms
func PostPublicRooms(
	ctx context.Context,
	request *external.PostPublicRoomsRequest,
	publicRoomDatabase model.PublicRoomAPIDatabase,
) (int, core.Coder) {
	var limit int64
	var offset int64
	//var request publicRoomReq
	var response external.GetPublicRoomsResponse

	// if fillErr := fillPublicRoomsReq(req, &request); fillErr != nil {
	// 	return *fillErr
	// }

	limit = request.Limit
	offset, err := strconv.ParseInt(request.Since, 10, 64)
	// ParseInt returns 0 and an error when trying to parse an empty string
	// In that case, we want to assign 0 so we ignore the error
	if err != nil && len(request.Since) > 0 {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if response.Estimate, err = publicRoomDatabase.CountPublicRooms(ctx); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if offset > 0 {
		response.PrevBatch = strconv.Itoa(int(offset) - 1)
	}
	nextIndex := int(offset) + int(limit)
	if response.Estimate > int64(nextIndex) {
		response.NextBatch = strconv.Itoa(nextIndex)
	}

	if chunk, err := publicRoomDatabase.GetPublicRooms(
		ctx, offset, limit, request.Filter.SearchTerms,
	); err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	} else {
		for _, v := range chunk {
			response.Chunk = append(response.Chunk, external.PublicRoomsChunk(v))
		}
	}

	return http.StatusOK, &response
}

// fillPublicRoomsReq fills the Limit, Since and Filter attributes of a GET or POST request
// on /publicRooms by parsing the incoming HTTP request
func fillPublicRoomsReq(httpReq *http.Request, request *publicRoomReq) *util.JSONResponse {
	if httpReq.Method == http.MethodGet {
		limit, err := strconv.Atoi(httpReq.FormValue("limit"))
		// Atoi returns 0 and an error when trying to parse an empty string
		// In that case, we want to assign 0 so we ignore the error
		if err != nil && len(httpReq.FormValue("limit")) > 0 {
			reqErr := httputil.LogThenError(httpReq, err)
			return &reqErr
		}
		request.Limit = int64(limit)
		request.Since = httpReq.FormValue("since")
		return nil
	} else if httpReq.Method == http.MethodPost {
		return httputil.UnmarshalJSONRequest(httpReq, request)
	}

	return &util.JSONResponse{
		Code: http.StatusMethodNotAllowed,
		JSON: jsonerror.NotFound("Bad method"),
	}
}
