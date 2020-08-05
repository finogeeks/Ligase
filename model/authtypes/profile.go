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

package authtypes

// Profile represents the profile for a Matrix account on this home server.
type Profile struct {
	UserID      string
	DisplayName string
	AvatarURL   string
}

// UserInfo represents the user information for a Matrix account on this home server.
type UserInfo struct {
	UserID    string
	UserName  string
	JobNumber string
	Mobile    string
	Landline  string
	Email     string
	State 	  int
}
