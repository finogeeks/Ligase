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

package external

// GET /_matrix/client/r0/user_info/{userID}
type GetUserInfoRequest struct {
	UserID string `json:"userID"`
}

type GetUserInfoResponse struct {
	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state,omitempty"`
}

// PUT /_matrix/client/r0/user_info/{userID}
type PutUserInfoRequest struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state,omitempty"`
}

// POST /_matrix/client/r0/user_info/{userID}
type PostUserInfoRequest struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state,omitempty"`
}

// GET /_matrix/client/r0/user_info_list
type GetUserInfoListRequest struct {
	UserIDs []string `json:"user_ids"`
}

type GetUserInfoListResponse struct {
	UserInfoList []UserInfoItem `json:"user_info_list"`
}

type UserInfoItem struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state,omitempty"`
}

// DELETE /_matrix/client/r0/user_info/{userID}
type DeleteUserInfoRequest struct {
	UserID string `json:"userID"`
}
