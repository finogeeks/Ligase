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

package handlers

import (
	"context"
	"github.com/finogeeks/ligase/plugins/message/internals"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type AuthHandler struct {
}

func (auth *AuthHandler) ProcessInputMsg(ctx context.Context, msg *internals.InputMsg) {
	log.Infof("AuthHandler recv msg:%08x", msg.MsgType)
	switch msg.MsgType {
	case internals.MSG_GET_LOGIN:
	case internals.MSG_POST_LOGIN:
	case internals.MSG_POST_LOGOUT:
	case internals.MSG_POST_LOGOUT_ALL:

	case internals.MSG_POST_REGISTER:
	case internals.MSG_POST_REGISTER_EMAIL:
	case internals.MSG_POST_REGISTER_MSISDN:
	case internals.MSG_POST_ACCOUT_PASS:
	case internals.MSG_POST_ACCOUT_PASS_EMAIL:
	case internals.MSG_POST_ACCOUT_PASS_MSISDN:
	case internals.MSG_POST_ACCOUNT_DEACTIVATE:
	case internals.MSG_GET_REGISTER_AVAILABLE:
	}

}
