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

//emote
type (
	EmoteItem struct {
		NetdiskID string `json:"netdiskID"`
		FileType  string `json:"fileType"`
		GroupID   string `json:"groupID"`
	}
	UploadEmoteResp struct {
		Emotes  	[]EmoteItem `json:"emotes"`
		NetdiskID 	string 		`json:"netdiskID"` //only use for old
		FileType    string 		`json:"fileType"`
		Total       int 		`json:"total"`
		GroupID		string		`json:"groupID"`
		Finish		bool 		`json:"finish"`
		WaitId		string 		`json:"waitId"`
	}
	listItem struct {
		NetdiskID  string   `json:"netdiskID"`
		ResourceID string   `json:"resourceID"`
		Timestamp  int64    `json:"timestamp"`
		Operator   string   `json:"operator"`
		From       string   `json:"from"`
		Tag        []string `json:"tag"`

		Type         string           `json:"type"`
		Content      string           `json:"content"`
		Owner        string           `json:"owner"`
		Public       bool             `json:"public"`
		Traceable    bool             `json:"traceable"`
		Delete       bool             `json:"delete"`
	}
	EmoteDetail struct {
		listItem
		FileType string `json:"fileType"`
		GroupID  string `json:"groupID"`
	}
	ListEmoteItem struct {
		Emotes 	[]EmoteDetail 	`json:"emotes"`
		Total   int 			`json:"total"`
	}

	MediaContentInfo struct {
		Body         string        `json:"body"`
		Info         MediaBaseInfo `json:"info"`
		Url          string        `json:"url"`
		MsgType      string        `json:"msgtype"`
		IsEmote      bool          `json:"isemote"`
		SrcNetdiskID string        `json:"srcnetdiskid"`
	}

	MediaBaseInfo struct {
		Size     int64  `json:"size"`
		MimeType string `json:"mimetype"`
		W        int    `json:"w"`
		H        int    `json:"h"`
	}
)