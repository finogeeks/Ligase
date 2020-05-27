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

package routing

import (
	"net/http"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/plugins/message/external"
)

// RequestEmailToken implements:
//     POST /account/3pid/email/requestToken
//     POST /register/email/requestToken
func RequestEmailToken() (int, core.Coder) {
	return http.StatusOK, &external.PostAccount3PIDEmailResponse{}
}

// CheckAndSave3PIDAssociation implements POST /account/3pid
func CheckAndSave3PIDAssociation() (int, core.Coder) {
	return http.StatusOK, nil
}

// GetAssociated3PIDs implements GET /account/3pid
func GetAssociated3PIDs() (int, core.Coder) {
	return http.StatusOK, &external.GetThreePIDsResponse{[]external.ThreePID{}}
}

// Forget3PID implements POST /account/3pid/delete
func Forget3PID() (int, core.Coder) {
	return http.StatusOK, nil
}
