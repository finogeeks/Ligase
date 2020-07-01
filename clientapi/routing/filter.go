// Copyright 2017 Jan Christian Grünhage
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
	"context"
	"net/http"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
)

// GetFilter implements GET /_matrix/client/r0/user/{userId}/filter/{filterId}
func GetFilter(
	ctx context.Context,
	req *external.GetUserFilterRequest,
	userID string,
	cache service.Cache,
) (int, core.Coder) {
	// if req.Method != http.MethodGet {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }
	if userID != req.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("Cannot get filters for other users")
	}

	res, ok := cache.GetAccountFilterById(userID, req.FilterID)
	if !ok {
		return http.StatusNotFound, jsonerror.NotFound("No such filter")
	}

	filter := &external.GetUserFilterResponse{}
	err := json.Unmarshal([]byte(res), &filter)
	if err != nil {
		httputil.LogThenErrorCtx(ctx, err)
	}

	return http.StatusOK, filter
}

//PutFilter implements POST /_matrix/client/r0/user/{userId}/filter
func PutFilter(
	ctx context.Context,
	filter *external.PostUserFilterRequest,
	userID string,
	accountDB model.AccountsDatabase,
	cache service.Cache,
) (int, core.Coder) {
	// if req.Method != http.MethodPost {
	// 	return util.JSONResponse{
	// 		Code: http.StatusMethodNotAllowed,
	// 		JSON: jsonerror.NotFound("Bad method"),
	// 	}
	// }
	if userID != filter.UserID {
		return http.StatusForbidden, jsonerror.Forbidden("Cannot create filters for other users")
	}

	// var filter external.PostUserFilterRequest

	// if reqErr := httputil.UnmarshalJSONRequest(req, &filter); reqErr != nil {
	// 	return *reqErr
	// }

	filterArray, err := json.Marshal(filter)
	if err != nil {
		return http.StatusBadRequest, jsonerror.BadJSON("Filter is malformed")
	}

	filterString := string(filterArray)
	filterHash := common.GetStringHash(filterString)
	/*if filterID, ok := cache.GetAccountFilterIDByContent(userID, filterHash); ok {
		return util.JSONResponse{
			Code: http.StatusOK,
			JSON: external.PostUserFilterResponse{FilterID: filterID},
		}
	}*/

	err = accountDB.PutFilter(ctx, userID, filterHash, filterString)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	return http.StatusOK, &external.PostUserFilterResponse{FilterID: filterHash}
}
