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

package mediatypes

// MediaID is a string representing the unique identifier for a file (could be a hash but does not have to be)
type MediaID string

type NetDiskResponse struct {
	NetDiskID string `json:"netdiskID,omitempty"`
	ResouceID string `json:"resourceID,omitempty"`
}

type UploadResponse struct {
	ContentURI string `json:"content_uri,omitempty"`
}

type UploadError struct {
	Error string `json:"error,omitempty"`
}
