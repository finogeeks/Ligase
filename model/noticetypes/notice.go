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

package noticetypes

type GetCertsResponse struct {
	RootCA     string `json:"root_ca,omitempty"`
	ServerCert string `json:"cert_pem,omitempty"`
	ServerKey  string `json:"key_pem,omitempty"`
	CRL        string `json:"CRL,omitempty"`
	// CrlSnapshot map[string][]string `json:"crl_snapshot,omitempty"`
}
