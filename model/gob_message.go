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
	"context"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

// transaction request for federation
type TxnReq struct {
	// Content	gomatrixserverlib.RawJSON
	Origin gomatrixserverlib.ServerName
	// gomatrixserverlib.Transaction
	Context context.Context
}

type Head struct {
	// Err error  // gob didn't support enc/dec errors.errorString{}
	ErrStr string
	// 消息类型，包括请求，响应，推送三种类型
	MsgType MsgType
	// 消息序号
	MsgSeq string
	// 消息所属逻辑节点号
	NodeId int64
	// api类型
	ApiType ApiType
	// api接口功能号
	Cmd Command
}

// 内部Gob格式消息体
type GobMessage struct {
	Head
	Key []byte
	//Body Payload
	Body []byte
}
