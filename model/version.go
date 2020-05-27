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

package model

import (
	"net/http"

	"github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Version struct {
	Server Server `json:"server"`
}

type Server struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

func (v *Version) Encode() ([]byte, error) {
	return json.Marshal(v)
}

func (v *Version) Decode(input []byte) error {
	return json.Unmarshal(input, v)
}

// Version returns the server version
func GetVersionResp() util.JSONResponse {
	return util.JSONResponse{Code: http.StatusOK, JSON: GetVersion()}
}

func GetVersion() *Version {
	return &Version{Server{"dev", "Dendrite"}}
}
