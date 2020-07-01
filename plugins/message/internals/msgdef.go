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

package internals

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/skunkworks/log"
	jsoniter "github.com/json-iterator/go"
	capn "zombiezen.com/go/capnproto2"
)

var jjson = jsoniter.ConfigCompatibleWithStandardLibrary

//最低byte表示调用方式，最高2byte为分类，中间byte为业务类型
//0x00 --get
//0x01 --put
//0x02 --post
//0x03 --del
const (
	MSG_GET_VERSIONS int32 = 0x00000000

	MSG_RESP_MESSAGE int32 = 0x00000100
	MSG_RESP_ERROR   int32 = 0x00000101

	MSG_GET_LOGIN        int32 = 0x00010000
	MSG_GET_LOGIN_ADMIN  int32 = 0x00010001
	MSG_POST_LOGIN       int32 = 0x00010102
	MSG_POST_LOGIN_ADMIN int32 = 0x00010103
	MSG_POST_LOGOUT      int32 = 0x00010202
	MSG_POST_LOGOUT_ALL  int32 = 0x00010302

	MSG_POST_REGISTER           int32 = 0x00020002
	MSG_POST_REGISTER_LEGACY    int32 = 0x00020003
	MSG_POST_REGISTER_EMAIL     int32 = 0x00020102
	MSG_POST_REGISTER_MSISDN    int32 = 0x00020202
	MSG_POST_ACCOUT_PASS        int32 = 0x00020302
	MSG_POST_ACCOUT_PASS_EMAIL  int32 = 0x00020402
	MSG_POST_ACCOUT_PASS_MSISDN int32 = 0x00020502
	MSG_POST_ACCOUNT_DEACTIVATE int32 = 0x00020602
	MSG_GET_REGISTER_AVAILABLE  int32 = 0x00020700

	MSG_GET_ACCOUNT_3PID         int32 = 0x00030000
	MSG_POST_ACCOUNT_3PID        int32 = 0x00030102
	MSG_POST_ACCOUNT_3PID_DEL    int32 = 0x00030202
	MSG_POST_ACCOUNT_3PID_EMAIL  int32 = 0x00030302
	MSG_POST_ACCOUNT_3PID_MSISDN int32 = 0x00030402

	MSG_GET_ACCOUNT_WHOAMI int32 = 0x00040000

	MSG_POST_USER_FILTER        int32 = 0x00050002
	MSG_GET_USER_FILTER_WITH_ID int32 = 0x00050100

	MSG_GET_SYNC           int32 = 0x00060000
	MSG_GET_EVENTS         int32 = 0x00060100
	MSG_GET_INITIAL_SYNC   int32 = 0x00060200
	MSG_GET_EVENTS_WITH_ID int32 = 0x00060300

	MSG_GET_ROOM_EVENT_WITH_ID           int32 = 0x00070000
	MSG_GET_ROOM_EVENT_WITH_TYPE_AND_KEY int32 = 0x00070100
	MSG_GET_ROOM_EVENT_WITH_TYPE         int32 = 0x00070200
	MSG_GET_ROOM_STATE                   int32 = 0x00070300
	MSG_GET_ROOM_MEMBERS                 int32 = 0x00070400
	MSG_GET_ROOM_JOIN_MEMBERS            int32 = 0x00070500
	MSG_GET_ROOM_MESSAGES                int32 = 0x00070600
	MSG_GET_ROOM_INITIAL_SYNC            int32 = 0x00070700
	MSG_POST_ROOM_INFO                   int32 = 0x00070602
	MSG_INTERNAL_POST_ROOM_INFO          int32 = 0x00070802

	MSG_PUT_ROOM_STATE_WITH_TYPE_AND_KEY  int32 = 0x00080001
	MSG_PUT_ROOM_STATE_WITH_TYPE          int32 = 0x00080101
	MSG_PUT_ROOM_SEND_WITH_TYPE_AND_TXNID int32 = 0x00080201

	MSG_PUT_ROOM_REDACT            int32 = 0x00090001
	MSG_PUT_ROOM_REDACT_WITH_TXNID int32 = 0x00090002
	MSG_PUT_ROOM_UPDATE            int32 = 0x00090003
	MSG_PUT_ROOM_UPDATE_WITH_TXNID int32 = 0x00090004

	MSG_POST_CREATEROOM int32 = 0x000a0002

	MSG_PUT_DIRECTORY_ROOM_ALIAS int32 = 0x000b0001
	MSG_GET_DIRECTORY_ROOM_ALIAS int32 = 0x000b0100
	MSG_DEL_DIRECTORY_ROOM_ALIAS int32 = 0x000b0203

	MSG_GET_JOIN_ROOMS int32 = 0x000c0000

	MSG_POST_ROOM_INVITE     int32 = 0x000d0002
	MSG_POST_ROOM_MEMBERSHIP int32 = 0x000d0102
	MSG_POST_JOIN_ALIAS      int32 = 0x000d0202

	MSG_POST_ROOM_LEAVE   int32 = 0x000e0002
	MSG_POST_ROOM_FORGET  int32 = 0x000e0102
	MSG_POST_ROOM_KICK    int32 = 0x000e0202
	MSG_POST_ROOM_DISMISS int32 = 0x000e0302

	MSG_POST_ROOM_BAN   int32 = 0x000f0202
	MSG_POST_ROOM_UNBAN int32 = 0x000f0302

	MSG_GET_DIRECTORY_LIST_ROOM int32 = 0x00100000
	MSG_PUT_DIRECTORY_LIST_ROOM int32 = 0x00100101
	MSG_GET_PUBLIC_ROOMS        int32 = 0x00100200
	MSG_POST_PUBLIC_ROOMS       int32 = 0x00100302

	MSG_POST_USER_DIRECTORY_SEARCH int32 = 0x00110002

	MSG_PUT_PROFILE_DISPLAY_NAME int32 = 0x00120001
	MSG_GET_PROFILE_DISPLAY_NAME int32 = 0x00120100
	MSG_PUT_PROFILE_AVATAR       int32 = 0x00120201
	MSG_GET_PROFILE_AVATAR       int32 = 0x00120300
	MSG_GET_PROFILE              int32 = 0x00120400
	MSG_POST_PROFILES            int32 = 0x00120500
	MSG_GET_USER_INFO            int32 = 0x00120600
	MSG_PUT_USER_INFO            int32 = 0x00120601
	MSG_POST_USER_INFO           int32 = 0x00120602
	MSG_DELETE_USER_INFO         int32 = 0x00120603

	MSG_GET_TURN_SERVER int32 = 0x00130000

	MSG_PUT_TYPING int32 = 0x00140001

	MSG_POST_RECEIPT int32 = 0x00150002

	MSG_POST_READMARK int32 = 0x00160002
	MSG_GET_UNREAD    int32 = 0x00160102

	MSG_PUT_USR_PRESENCE       int32 = 0x00170001
	MSG_GET_USR_PRESENCE       int32 = 0x00170100
	MSG_POST_USR_LIST_PRESENCE int32 = 0x00170201
	MSG_GET_USR_LIST_PRESENCE  int32 = 0x00170300

	MSG_POST_MEDIA_UPLOAD          int32 = 0x00180002
	MSG_GET_MEDIA_DOWNLOAD         int32 = 0x00180100
	MSG_GET_MEDIA_DOWNLOAD_BY_NAME int32 = 0x00180200
	MSG_GET_MEDIA_THUMBNAIL        int32 = 0x00180300
	MSG_GET_MEDIA_PREVIEW_URL      int32 = 0x00180400
	MSG_GET_MEDIA_CONFIG           int32 = 0x00180500

	MSG_PUT_SENT_TO_DEVICE int32 = 0x00190001

	MSG_POST_REPORT_ROOM         int32 = 0x00190010
	MSG_POST_REPORT_DEVICE_STATE int32 = 0x00190100

	MSG_GET_DEVICES       int32 = 0x001a0000
	MSG_GET_DEVICES_BY_ID int32 = 0x001a0100
	MSG_PUT_DEVICES_BY_ID int32 = 0x001a0201
	MSG_DEL_DEVICES_BY_ID int32 = 0x001a0303
	MSG_POST_DEL_DEVICES  int32 = 0x001a0402

	MSG_POST_KEYS_UPLOAD_BY_DEVICE_ID int32 = 0x001b0002
	MSG_POST_KEYS_UPLOAD              int32 = 0x001b0052
	MSG_POST_KEYS_QUERY               int32 = 0x001b0102
	MSG_POST_KEYS_CLAIM               int32 = 0x001b0202
	MSG_GET_KEYS_CHANGES              int32 = 0x001b0300

	MSG_GET_VISIBILITY_RANGE int32 = 0x001b1000

	MSG_GET_PUSHERS  int32 = 0x001c0000
	MSG_POST_PUSHERS int32 = 0x001c0102

	MSG_GET_NOTIFICATIONS int32 = 0x001d0000

	MSG_GET_PUSHRULES         int32 = 0x001e0000
	MSG_GET_PUSHRULES_GLOBAL  int32 = 0x001e0050
	MSG_GET_PUSHRULES_BY_ID   int32 = 0x001e0100
	MSG_DEL_PUSHRULES_BY_ID   int32 = 0x001e0203
	MSG_PUT_PUSHRULES_BY_ID   int32 = 0x001e0301
	MSG_GET_PUSHRULES_ENABLED int32 = 0x001e0400
	MSG_PUT_PUSHRULES_ENABLED int32 = 0x001e0501
	MSG_GET_PUSHRULES_ACTIONS int32 = 0x001e0600
	MSG_PUT_PUSHRULES_ACTIONS int32 = 0x001e0701
	MSG_POST_USERS_PUSH_KEY   int32 = 0x001e0801

	MSG_POST_SEARCH int32 = 0x001f0002

	MSG_GET_ROOM_TAG_BY_ID int32 = 0x00200000
	MSG_PUT_ROOM_TAG_BY_ID int32 = 0x00200101
	MSG_DEL_ROOM_TAG_BY_ID int32 = 0x00200203

	MSG_PUT_USER_ACCOUNT_DATA int32 = 0x00210001
	MSG_PUT_ROOM_ACCOUNT_DATA int32 = 0x00210101
	MSG_GET_USER_NEW_TOKEN    int32 = 0x00210201
	MSG_GET_SUPER_ADMIN_TOKEN int32 = 0x00210301

	MSG_GET_WHO_IS int32 = 0x00220001

	MSG_GET_ROOM_EVENT_CONTEXT int32 = 0x00230001

	MSG_GET_CAS_LOGIN_REDIRECT int32 = 0x00240001
	MSG_GET_CAS_LOGIN_TICKET   int32 = 0x00240101

	MSG_POST_ROOM_REPORT int32 = 0x00250002

	MSG_GET_THIRDPARTY_PROTOS            int32 = 0x00260001
	MSG_GET_THIRDPARTY_PROTO_BY_NAME     int32 = 0x00260101
	MSG_GET_THIRDPARTY_LOCATION_BY_PROTO int32 = 0x00260201
	MSG_GET_THIRDPARTY_USER_BY_PROTO     int32 = 0x00260301
	MSG_GET_THIRDPARTY_LOCATION          int32 = 0x00260401
	MSG_GET_THIRDPARTY_USER              int32 = 0x00260501

	MSG_POST_USER_OPENID int32 = 0x00270002

	MSG_POST_SYSTEM_MANAGER int32 = 0x00280001

	MSG_GET_FED_VER               int32 = 0x00290001
	MSG_GET_FED_DIRECTOR          int32 = 0x00290101
	MSG_GET_FED_PROFILE           int32 = 0x00290201
	MSG_PUT_FED_SEND              int32 = 0x00290301
	MSG_PUT_FED_INVITE            int32 = 0x00290401
	MSG_GET_FED_ROOM_STATE        int32 = 0x00290501
	MSG_GET_FED_BACKFILL          int32 = 0x00290601
	MSG_GET_FED_MISSING_EVENTS    int32 = 0x00290611
	MSG_GET_FED_MEDIA_DOWNLOAD    int32 = 0x00290701
	MSG_GET_FED_MEDIA_INFO        int32 = 0x00290801
	MSG_GET_FED_USER_INFO         int32 = 0x00290901
	MSG_GET_FED_MAKE_JOIN         int32 = 0x00291901
	MSG_PUT_FED_SEND_JOIN         int32 = 0x00292001
	MSG_GET_FED_MAKE_LEAVE        int32 = 0x00293901
	MSG_PUT_FED_SEND_LEAVE        int32 = 0x00294001
	MSG_GET_FED_EVENT_AUTH        int32 = 0x00294101
	MSG_POST_FED_QUERY_AUTH       int32 = 0x00294201
	MSG_GET_FED_EVENT             int32 = 0x00294301
	MSG_GET_FED_STATE_IDS         int32 = 0x00294401
	MSG_GET_FED_PUBLIC_ROOMS      int32 = 0x00294501
	MSG_POST_FED_PUBLIC_ROOMS     int32 = 0x00294601
	MSG_GET_FED_USER_DEVICES      int32 = 0x00294701
	MSG_GET_FED_CLIENT_KEYS       int32 = 0x00294801
	MSG_GET_FED_CLIENT_KEYS_CLAIM int32 = 0x00294901

	MSG_PUT_FED_EXCHANGE_THIRD_PARTY_INVITE int32 = 0x00294901

	MSG_GET_SETTING         int32 = 0x00300001
	MSG_PUT_SETTING         int32 = 0x00300101
	MSG_POST_SERVERNAME     int32 = 0x00300002
	MSG_GET_SERVERNAME      int32 = 0x00300102
	MSG_POST_REPORT_MISSING int32 = 0x00300202

	MSG_POST_NOTARY_NOTICE int32 = 0x00400001

	MSG_GET_RCS_FRIENDSHIPS int32 = 0x00500000
	MSG_GET_RCS_ROOMID      int32 = 0x00500100
)

const (
	NONE_FORMAT = iota
	CAPN_FORMAT
	JSON_FORMAT
)

const (
	HTTP_RESP_DISCARD int = -100
)

type RespMessage struct {
	Message string `json:"message"`
}

func (m *RespMessage) Encode() ([]byte, error) {
	return jjson.Marshal(m)
}

func (m *RespMessage) Decode(input []byte) error {
	return jjson.Unmarshal(input, m)
}

func (m *RespMessage) New(code int) core.Coder {
	return new(RespMessage)
}

type OutputMsg struct {
	Reply   int64
	MsgType int32
	Code    int
	Headers []byte
	Body    []byte
}

const MAX_PARTITION = 16

func (m *OutputMsg) Encode() ([]byte, error) {
	// size := 24
	// data := make([]byte, 0, size+8+len(m.Headers)+len(m.Body))
	// w := bytes.NewBuffer(data)
	// binary.Write(w, binary.BigEndian, m.Reply)
	// binary.Write(w, binary.BigEndian, m.MsgType)
	// binary.Write(w, binary.BigEndian, uint32(m.Code))
	// binary.Write(w, binary.BigEndian, uint32(0))
	// binary.Write(w, binary.BigEndian, uint32(len(m.Body)))
	// data = w.Bytes()
	// data = append(data, m.Body...)
	// return data, nil
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	inputMsgCapn, err := NewRootOutputMsgCapn(seg)
	if err != nil {
		return nil, err
	}

	inputMsgCapn.SetReply(m.Reply)
	inputMsgCapn.SetMsgType(m.MsgType)
	inputMsgCapn.SetCode(int64(m.Code))
	inputMsgCapn.SetHeaders(m.Headers)
	inputMsgCapn.SetBody(m.Body)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (m *OutputMsg) Decode(input []byte) error {
	// r := bytes.NewReader(input)
	// binary.Read(r, binary.BigEndian, &m.Reply)
	// binary.Read(r, binary.BigEndian, &m.MsgType)
	// v := uint32(0)
	// binary.Read(r, binary.BigEndian, &v)
	// m.Code = int(v)
	// binary.Read(r, binary.BigEndian, &v)
	// if v > 0 {
	// 	headers := make([]byte, v)
	// 	r.Read(headers)
	// 	m.Headers = headers
	// }
	// binary.Read(r, binary.BigEndian, &v)
	// if v > 0 {
	// 	body := make([]byte, v)
	// 	r.Read(body)
	// 	m.Body = body
	// }
	// return nil
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	inputMsgCapn, err := ReadRootOutputMsgCapn(msg)
	if err != nil {
		return nil
	}

	m.Reply = inputMsgCapn.Reply()
	m.MsgType = inputMsgCapn.MsgType()
	m.Code = int(inputMsgCapn.Code())
	m.Headers, err = inputMsgCapn.Headers()
	if err != nil {
		return err
	}
	m.Body, err = inputMsgCapn.Body()
	if err != nil {
		return err
	}
	return nil
}

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

type InputMsg struct {
	Reply   int64           `json:"reply"`
	MsgType int32           `json:"msg_type"`
	Device  *Device         `json:"device"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (m *InputMsg) Encode() ([]byte, error) {
	msg, seg, err := capn.NewMessage(capn.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	inputMsgCapn, err := NewRootInputMsgCapn(seg)
	if err != nil {
		return nil, err
	}

	inputMsgCapn.SetReply(m.Reply)
	inputMsgCapn.SetMsgType(m.MsgType)
	if m.Device != nil {
		device, err := inputMsgCapn.NewDevice()
		if err != nil {
			return nil, err
		}
		device.SetId(m.Device.ID)
		device.SetUserID(m.Device.UserID)
		device.SetDisplayName(m.Device.DisplayName)
		device.SetDeviceType(m.Device.DeviceType)
		device.SetIsHuman(m.Device.IsHuman)
		device.SetIdentifier(m.Device.Identifier)
		device.SetCreateTs(m.Device.CreateTs)
		device.SetLastActiveTs(m.Device.LastActiveTs)
	}
	inputMsgCapn.SetPayload(m.Payload)
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (m *InputMsg) Decode(input []byte) error {
	msg, err := capn.Unmarshal(input)
	if err != nil {
		return err
	}

	inputMsgCapn, err := ReadRootInputMsgCapn(msg)
	if err != nil {
		return nil
	}

	m.Reply = inputMsgCapn.Reply()
	m.MsgType = inputMsgCapn.MsgType()
	if inputMsgCapn.HasDevice() {
		device, err := inputMsgCapn.Device()
		if err != nil {
			return err
		}
		m.Device = new(Device)
		m.Device.ID, err = device.Id()
		if err != nil {
			return err
		}
		m.Device.UserID, err = device.UserID()
		if err != nil {
			return err
		}
		m.Device.DisplayName, err = device.DisplayName()
		if err != nil {
			return err
		}
		m.Device.DeviceType, err = device.DeviceType()
		if err != nil {
			return err
		}
		m.Device.IsHuman = device.IsHuman()
		m.Device.CreateTs = device.CreateTs()
		m.Device.LastActiveTs = device.LastActiveTs()
	}

	m.Payload, err = inputMsgCapn.Payload()
	if err != nil {
		return err
	}
	return nil
}

func (m *InputMsg) GetTopic(key string) string {
	hash := fnv.New32a()
	hash.Write([]byte(key))
	part := hash.Sum32() % MAX_PARTITION

	switch m.MsgType & 0x7fff0000 {
	case 0x00010000: //login&out useid key
		fallthrough
	case 0x00020000: //register useid key
		return fmt.Sprintf("auth.%d", part)
	case 0x00050000: //filter userid key
		fallthrough
	case 0x000c0000: //join_rooms userid key
		fallthrough
	case 0x00120000: //profile userid key
		fallthrough
	case 0x00170000: //presence userid key
		fallthrough
	case 0x00200000: //tag userid key
		fallthrough
	case 0x00210000: //accdata userid key
		fallthrough
	case 0x00220000: //who is userid key
		return fmt.Sprintf("user.%d", part)
	case 0x00060000:
		return fmt.Sprintf("sync.%d", part) //sync..
	case 0x00100000: //publicroom room key
		fallthrough
	case 0x00070000:
		return fmt.Sprintf("room_qry.%d", part) //qry room key
	case 0x00080000: //send room key
		fallthrough
	case 0x00090000: //redact room key
		fallthrough
	case 0x000a0000: //create room key
		fallthrough
	case 0x000d0000, 0x000e0000, 0x000f0000: //invite&join&leave...
		return fmt.Sprintf("room_put.%d", part)
	case 0x000b0000: //create room key
		return fmt.Sprintf("room_alias.%d", part)
	case 0x00140000, 0x00150000, 0x00160000: //typing/receipt/read room key
		return fmt.Sprintf("room_edu.%d", part)
	case 0x00180000: //media media key
		return fmt.Sprintf("media.%d", part)
	case 0x00190000: //std device key
		fallthrough
	case 0x001a0000: //std device key
		return fmt.Sprintf("device.%d", part)

	case 0x001b0000: //keys device key
		return fmt.Sprintf("keys.%d", part)
	case 0x001c0000, 0x001d0000, 0x001e0000: //notif userid key
		return fmt.Sprintf("keys.%d", part)

	default:
		log.Errorf("unknown topic msgtype:%8x", m.MsgType)
		return "unknown"
	}
}

func (m *InputMsg) GetCategory() int32 {
	return (m.MsgType & 0x7fff0000) >> 16
}

type IRoutableMsg interface {
}

// MatrixError represents the "standard error response" in Matrix.
// http://matrix.org/docs/spec/client_server/r0.2.0.html#api-standards
type MatrixError struct {
	ErrCode string `json:"errcode"`
	Err     string `json:"error"`
}

func (e *MatrixError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrCode, e.Err)
}

func (e *MatrixError) Encode() ([]byte, error) {
	return jjson.Marshal(e)
}

func (e *MatrixError) Decode(input []byte) error {
	return jjson.Unmarshal(input, e)
}

// JSONMap 用在api-gw和后端服务器之间作为传送中介，尽量少用
// 如果用在json的序列化和反序列化之间作为中介值，有可能会出现int64丢失精度问题
type JSONMap map[string]interface{}

func (c JSONMap) Encode() ([]byte, error) {
	return jjson.Marshal(c)
}

func (c JSONMap) Decode(input []byte) error {
	if len(input) == 0 {
		return nil
	}
	return jjson.Unmarshal(input, &c)
}

// JSONArr 用在api-gw和后端服务器之间作为传送中介，尽量少用
// 如果用在json的序列化和反序列化之间作为中介值，有可能会出现int64丢失精度问题
type JSONArr []interface{}

func (c *JSONArr) Encode() ([]byte, error) {
	return jjson.Marshal(*c)
}

func (c *JSONArr) Decode(input []byte) error {
	if len(input) == 0 {
		return nil
	}
	return jjson.Unmarshal(input, c)
}

type RawBytes []byte

func (r *RawBytes) Encode() ([]byte, error) {
	if r == nil {
		return nil, errors.New("RawBytes has not new")
	}
	return *r, nil
}

func (r *RawBytes) Decode(data []byte) error {
	if r == nil {
		return errors.New("RawBytes has not new")
	}
	*r = data
	return nil
}
