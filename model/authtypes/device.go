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

// Device represents a client's device (mobile, web, etc)
type Device struct {
	ID           string `json:"id,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	DeviceType   string `json:"device_type,omitempty"`
	IsHuman      bool   `json:"is_human,omitempty"`
	Identifier   string `json:"identifier,omitempty"`
	CreateTs     int64  `json:"create_ts,omitempty"`
	LastActiveTs int64  `json:"last_active_ts,omitempty"`
}
