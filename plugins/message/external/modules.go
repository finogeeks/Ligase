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

package external

//GET /_matrix/client/r0/voip/turnServer
type GetTurnServerResponse struct {
	UserName string   `json:"username"`
	Password string   `json:"password"`
	Uris     []string `json:"uris"`
	TTL      int      `json:"ttl"`
}

//PUT /_matrix/client/r0/rooms/{roomId}/typing/{userId}
type PutRoomUserTypingRequest struct {
	UserID  string `json:"userId"`
	RoomID  string `json:"roomId"`
	Typing  bool   `json:"typing"`
	TimeOut int    `json:"timeout"`
}

// POST /_matrix/client/r0/rooms/{roomId}/receipt/{receiptType}/{eventId}
type PostRoomReceiptRequest struct {
	RoomID      string `json:"roomId"`
	ReceiptType string `json:"receiptType"`
	EventID     string `json:"eventId"`
}

//POST /_matrix/client/r0/rooms/{roomId}/read_markers
type PostRoomReadMarkersRequest struct {
	RoomID      string `json:"roomId"`
	ReceiptType string `json:"receiptType"`
	FullyRead   string `json:"m.fully_read"`
	Read        string `json:"m.read"`
}

//PUT /_matrix/client/r0/presence/{userId}/status
type PutUserPresenceRequest struct {
	UserID   string `json:"userId"`
	Presence string `json:"presence"`
	Status   string `json:"status_msg"`
}

//GET /_matrix/client/r0/presence/{userId}/status
type GetUserPresenceRequest struct {
	UserID string `json:"userId"`
}

type GetUserPresenceResponse struct {
	Presence        string  `json:"presence"`
	LastActiveAgo   int     `json:"last_active_ago"`
	Status          *string `json:"status_msg"`
	CurrentlyActive bool    `json:"currently_active"`
}

//POST /_matrix/client/r0/presence/list/{userId}
type PostUserPresenceListRequest struct {
	UserID string   `json:"userId"`
	Invite []string `json:"invite"`
	Drop   []string `json:"drop"`
}

// GET /_matrix/client/r0/presence/list/{userId}
type GetUserPresenceListRequest struct {
	UserID string `json:"userId"`
}

type GetUserPresenceListResponse []PresenceEvent

type PresenceEvent struct {
	Content map[string]interface{} `json:"content"`
	Status  string                 `json:"type"`
}

// POST /_matrix/media/r0/upload
type PostMediaUploadRequest struct {
	FileName    string `json:"filename"`
	ContentType string `json:"content-type"`
}

type PostMediaUploadResponse struct {
	ContentURI string `json:"content_uri"`
}

//GET /_matrix/media/r0/download/{serverName}/{mediaId}
type GetMediaDownloadRequest struct {
	ServerName  string `json:"serverName"`
	MediaID     string `json:"mediaId"`
	AllowRemote string `json:"allow_remote"`
}

//GET /_matrix/media/r0/download/{serverName}/{mediaId}/{fileName}
type GetMediaDownloadByFileNameRequest struct {
	ServerName  string `json:"serverName"`
	MediaID     string `json:"mediaId"`
	FileName    string `json:"fileName"`
	AllowRemote string `json:"allow_remote"`
}

//GET /_matrix/media/r0/thumbnail/{serverName}/{mediaId}
type GetMediaThumbnailRequest struct {
	ServerName  string `json:"serverName"`
	MediaID     string `json:"mediaId"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Method      int    `json:"method"`
	AllowRemote string `json:"allow_remote"`
}

//GET /_matrix/media/r0/preview_url
type GetMediaPreviewURLRequest struct {
	URL string `json:"url"`
	Ts  int    `json:"ts"`
}

type GetMediaPreviewURLResponse struct {
	Size  int64  `json:"matrix:image:size"`
	Image string `json:"og:image"`
}

// GET /_matrix/media/r0/config
//todo

//PUT /_matrix/client/r0/sendToDevice/{eventType}/{txnId}
type PutSendToDeviceRequest struct {
	EventType string `json:"eventType"`
	TxnId     string `json:"txnId"`
	//Messages  map[string]map[string]interface{} `json:"messages"`
	Content []byte `json:"content"`
}

// GET /_matrix/client/r0/devices
type GetDevicesResponse struct {
	Devices []Device `json:"devices"`
}

// GET /_matrix/client/r0/devices/{deviceId}
type GetDevicesByIDRequest struct {
	DeviceId string `json:"deviceId"`
}

type GetDevicesByIDResponse struct {
	DeviceID    string `json:"device_id"`
	DisplayName string `json:"display_name"`
	LastSeenIP  string `json:"last_seen_ip"`
	LastSeenTS  int    `json:"last_seen_ts`
}

//PUT /_matrix/client/r0/devices/{deviceId}
type PutDevicesByIDRequest struct {
	DeviceId    string `json:"deviceId"`
	DisplayName string `json:"display_name"`
}

//DELETE /_matrix/client/r0/devices/{deviceId}
type DelDevicesByIDRequest struct {
	DeviceId string   `json:"deviceId"`
	Auth     AuthData `json:"auth"`
}

// POST /_matrix/client/r0/delete_devices
type PostDevicesDelRequest struct {
	Devices []string `json:"devices"`
	Auth    AuthData `json:"auth"`
}

//POST /_matrix/client/r0/keys/upload
type PostUploadKeysRequest struct {
	DeviceKeys  DeviceKeys             `json:"device_keys"`
	OneTimeKeys map[string]interface{} `json:"one_time_keys"`
}

type DeviceKeys struct {
	UserID     string                       `json:"user_id"`
	DeviceID   string                       `json:"device_id"`
	Algorithms []string                     `json:"algorithms"`
	Keys       map[string]string            `json:"keys"`
	Signatures map[string]map[string]string `json:"signatures"`
	Unsigned   UnsignedDeviceInfo           `json:"unsigned,omitempty"`
}

type UnsignedDeviceInfo struct {
	DeviceDisplayName string `json:"device_display_name"`
}

type PostUploadKeysResponse struct {
	OneTimeKeyCounts map[string]int `json:"one_time_key_counts"`
}

//POST /_matrix/client/r0/keys/query
type PostQueryKeysRequest struct {
	TimeOut    int64                  `json:"timeout"`
	DeviceKeys map[string]interface{} `json:"device_keys"`
	Token      string                 `json:"token"`
}

type PostQueryKeysResponse struct {
	Failures   map[string]interface{}           `json:"failures"`
	DeviceKeys map[string]map[string]DeviceKeys `json:"device_keys"`
}

//POST /_matrix/client/r0/keys/claim
type PostClaimKeysRequest struct {
	TimeOut     int                          `json:"timeout"`
	OneTimeKeys map[string]map[string]string `json:"one_time_keys"`
}

type PostClaimKeysResponse struct {
	Failures   map[string]interface{}                       `json:"failures"`
	DeviceKeys map[string]map[string]map[string]interface{} `json:"one_time_keys"`
}

//GET /_matrix/client/r0/keys/changes
type GetKeysChangesRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type GetKeysChangesResponse struct {
	Changed []string `json:"changed"`
	Left    []string `json:"left"`
}

//GET /_matrix/client/r0/pushers
type GetPushersResponse struct {
	Pusher []Pusher `json:"pushers"`
}

type Pusher struct {
	Pushkey           string     `json:"pushkey"`
	Kind              string     `json:"kind"`
	AppID             string     `json:"app_id"`
	AppDisplayName    string     `json:"app_display_name"`
	DeviceDisplayName string     `json:"device_display_name"`
	ProfileTag        string     `json:"profile_tag"`
	Lang              string     `json:"lang"`
	Data              PusherData `json:"data"`
}

type PusherData struct {
	URL         string `json:"url"`
	Format      string `json:"format"`
	PushType    string `json:"push_type"`
	PushChannel string `json:"push_channel"`
}

// POST /_matrix/client/r0/pushers/set
type PostSetPushersRequest struct {
	Pushkey           string     `json:"pushkey"`
	Kind              string     `json:"kind"`
	AppID             string     `json:"app_id"`
	AppDisplayName    string     `json:"app_display_name"`
	DeviceDisplayName string     `json:"device_display_name"`
	ProfileTag        string     `json:"profile_tag"`
	Lang              string     `json:"lang"`
	Data              PusherData `json:"data"`
	Append            bool       `json:"append"`
}

//GET /_matrix/client/r0/notifications
type GetNotificationsRequest struct {
	From  string `json:"from"`
	Limit int    `json:"limit"`
	Only  string `json:"only"`
}

type GetNotificationsResponse struct {
	NextToken     string         `json:"next_token"`
	Notifications []Notification `json:"notifications"`
}

type Notification struct {
	Actions    []interface{} `json:"actions"`
	Event      RoomEvent     `json:"event"`
	ProfileTag string        `json:"profile_tag"`
	Read       bool          `json:"read"`
	RoomID     string        `json:"room_id"`
	Ts         int           `json:"ts"`
}

//GET /_matrix/client/r0/pushrules/
type GetPushrulesResponse struct {
	Global Ruleset `json:"global"`
}

type Ruleset struct {
	Content   []PushRule `json:"content"`
	Override  []PushRule `json:"override"`
	Room      []PushRule `json:"room"`
	Sender    []PushRule `json:"sender"`
	Underride []PushRule `json:"underride"`
}

type PushRule struct {
	Actions    []interface{}   `json:"actions"`
	Default    bool            `json:"default"`
	Enabled    bool            `json:"enabled"`
	RuleID     string          `json:"rule_id"`
	Conditions []PushCondition `json:"conditions"`
	Pattern    string          `json:"pattern"`
}

type PushCondition struct {
	Kind    string `json:"kind,omitempty"`
	Key     string `json:"key,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	Is      string `json:"is,omitempty"`
}

//GET /_matrix/client/r0/pushrules/
type GetPushrulesGlobalResponse Ruleset

//GET /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}
type GetPushrulesByIDRequest struct {
	Scope  string `json:"scope"`
	Kind   string `json:"kind"`
	RuleID string `json:"ruleId"`
}

type GetPushrulesByIDResponse struct {
	Actions    []interface{}   `json:"actions,omitempty"`
	Default    bool            `json:"default"`
	Enabled    bool            `json:"enabled"`
	RuleID     string          `json:"rule_id,omitempty"`
	Conditions []PushCondition `json:"conditions,omitempty"` // TODO: 有时需要，有时不能要
	Pattern    string          `json:"pattern,omitempty"`
}

//DELETE /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}
type DelPushrulesByIDRequest struct {
	Scope  string `json:"scope"`
	Kind   string `json:"kind"`
	RuleID string `json:"ruleId"`
}

// PUT /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}
type PutPushrulesByIDRequest struct {
	Actions    []interface{}   `json:"actions"`
	Conditions []PushCondition `json:"conditions"`
	Pattern    string          `json:"pattern"`
	Scope      string          `json:"scope"`
	Kind       string          `json:"kind"`
	RuleID     string          `json:"ruleId"`
	Before     string          `json:"before"`
	After      string          `json:"after"`
}

//GET /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/enabled
type GetPushrulesEnabledByIDRequest struct {
	Scope  string `json:"scope"`
	Kind   string `json:"kind"`
	RuleID string `json:"ruleId"`
}

type GetPushrulesEnabledByIDResponse struct {
	Enabled bool `json:"enabled"`
}

//  PUT /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/enabled
type PutPushrulesEnabledByIDRequest struct {
	Scope   string `json:"scope"`
	Kind    string `json:"kind"`
	RuleID  string `json:"ruleId"`
	Enabled bool   `json:"enabled"`
}

// GET /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/actions
type GetPushrulesActionsByIDRequest struct {
	Scope  string `json:"scope"`
	Kind   string `json:"kind"`
	RuleID string `json:"ruleId"`
}

type GetPushrulesActionsByIDResponse struct {
	Actions []interface{} `json:"actions"`
}

//PUT /_matrix/client/r0/pushrules/{scope}/{kind}/{ruleId}/actions
type PutPushrulesActionsByIDRequest struct {
	Scope   string   `json:"scope"`
	Kind    string   `json:"kind"`
	RuleID  string   `json:"ruleId"`
	Actions []string `json:"actions"`
}

//POST /_matrix/client/r0/users/pushkey
type PostUsersPushKeyRequest struct {
	Users []string `json:"users"`
}
type PostUsersPushKeyResponse struct {
	PushersRes []PusherRes `json:"pushers"`
}

type PusherRes struct {
	UserName  string `json:"user_name,omitempty"`
	AppId     string `json:"app_id,omitempty"`
	Kind      string `json:"kind,omitempty"`
	PushKey   string `json:"pushkey,omitempty"`
	PushKeyTs int64  `json:"ts,omitempty"`
}

//GET /_matrix/client/r0/user/{userId}/rooms/{roomId}/tags
type GetRoomsTagsByIDRequest struct {
	UserId string `json:"userId"`
	RoomId string `json:"roomId"`
}

type GetRoomsTagsByIDResponse struct {
	Tags map[string]interface{} `json:"tags"`
}

type Tag struct {
	Order float32 `json:"order"`
}

//PUT /_matrix/client/r0/user/{userId}/rooms/{roomId}/tags/{tag}
type PutRoomsTagsByIDRequest struct {
	UserId string `json:"userId"`
	RoomId string `json:"roomId"`
	Tag    string `json:"tag"`
	//Order   string `json:"order"`
	Content []byte `json:"contentBytes"`
}

//DELETE /_matrix/client/r0/user/{userId}/rooms/{roomId}/tags/{tag}
type DelRoomsTagsByIDRequest struct {
	UserId string `json:"userId"`
	RoomId string `json:"roomId"`
	Tag    string `json:"tag"`
}

//PUT /_matrix/client/r0/user/{userId}/account_data/{type}
type PutUserAccountDataRequest struct {
	UserId  string `json:"userId"`
	Type    string `json:"type"`
	Content []byte `json:"content"`
}

//PUT /_matrix/client/r0/user/{userId}/rooms/{roomId}/account_data/{type}
type PutRoomAccountDataRequest struct {
	UserId  string `json:"userId"`
	RoomID  string `json:"roomId"`
	Type    string `json:"type"`
	Content []byte `json:"content"`
}

//GET /_matrix/client/r0/rooms/{roomId}/context/{eventId}
type GetRoomEventContextRequest struct {
	RoomID  string `json:"roomId"`
	EventID string `json:"eventId"`
	Limit   int64  `json:"limit"`
}

type GetRoomEventContextResponse struct {
	Start        string       `json:"start"`
	End          string       `json:"end"`
	EventsBefore []RoomEvent  `json:"events_before"`
	Event        RoomEvent    `json:"event"`
	EventsAfter  []RoomEvent  `json:"events_after"`
	State        []StateEvent `json:"state"`
}

//GET /_matrix/client/r0/rooms/{roomId}/search/{eventId}
type GetRoomEventSearchRequest struct {
	RoomID  string `json:"roomId"`
	EventID string `json:"eventId"`
	Size    int64  `json:"size"`
}

//GET /_matrix/client/r0/admin/whois/{userId}
type GetWhoIsRequest struct {
	UserID string `json:"userId"`
}

type GetWhoIsResponse struct {
	UserID  string                `json:"user_id"`
	Devices map[string]DeviceInfo `json:"devices"`
}

type DeviceInfo struct {
	Sessions []SessionInfo `json:"sessions"`
}

type SessionInfo struct {
	Connections []ConnectionInfo `json:"connections`
}

type ConnectionInfo struct {
	Ip        string `json:"ip"`
	LastSeen  int    `json:"last_seen"`
	UserAgent string `json:"user_agent"`
}

//GET /_matrix/client/r0/login/cas/redirect
type GetCasLoginRedirectRequest struct {
	RedirectURL string `json:"redirectUrl"`
}

//GET /_matrix/client/r0/login/cas/ticket
type GetCasLoginTickerRequest struct {
	RedirectURL string `json:"redirectUrl"`
	Ticket      string `json:"ticket"`
}

//POST /_matrix/client/r0/rooms/{roomId}/report/{eventId}
type PostRoomReportRequest struct {
	RoomID  string `json:"roomId"`
	EventID string `json:"eventId"`
	Score   int    `json:"score"`
	Reason  string `json:"reason"`
}

//GET /_matrix/client/r0/thirdparty/protocols
type GetThirdPartyProtocalsResponse struct {
	UserFields     []string              `json:"user_fields"`
	LocationFields []string              `json:"location_fields"`
	Icon           string                `json:"icon"`
	FieldsTypes    map[string]FieldsType `json:"fields_types"`
	Instances      []ProtocolInstance    `json:"instances"`
}

type FieldsType struct {
	Regexp      string `json:"regexp"`
	PlaceHolder string `json:"placeholder`
}

type ProtocolInstance struct {
	Desc      string      `json:"desc"`
	Icon      string      `json:"icon"`
	Fields    interface{} `json:"fields"`
	NetworkID string      `json:"network_id"`
}

//GET /_matrix/client/r0/thirdparty/protocol/{protocol}
type GetThirdPartyProtocalByNameRequest struct {
	Protocol string `json:"protocol"`
}

type GetThirdPartyProtocalByNameResponse struct {
	UserFields     []string              `json:"user_fields"`
	LocationFields []string              `json:"location_fields"`
	Icon           string                `json:"icon"`
	FieldsTypes    map[string]FieldsType `json:"fields_types"`
	Instances      []ProtocolInstance    `json:"instances"`
}

//GET /_matrix/client/r0/thirdparty/location/{protocol}
type GetThirdPartyLocationByProtocolRequest struct {
	Protocol     string `json:"protocol"`
	SearchFields string `json:"searchFields"`
}

type GetThirdPartyLocationByProtocolResponse struct {
	Body []Location `json:"<body"`
}

type Location struct {
	Alias    string      `json:"alias"`
	Protocol string      `json:"protocol"`
	Fields   interface{} `json:"fields"`
}

//GET /_matrix/client/r0/thirdparty/user/{protocol}
type GetThirdPartyUserByProtocolRequest struct {
	Protocol string `json:"protocol"`
	Fields   string `json:"fields..."`
}

type GetThirdPartyUserByProtocolResponse struct {
	Body []ThirdPartyUser `json:"<body>"`
}

type ThirdPartyUser struct {
	UserID   string      `json:"userid"`
	Protocol string      `json:"protocol"`
	Fields   interface{} `json:"fields"`
}

//GET /_matrix/client/r0/thirdparty/location
type GetThirdPartyLocationRequest struct {
	Alias string `json:"alias"`
}

type GetThirdPartyLocationResponse struct {
	Body []Location `json:"<body>"`
}

//GET /_matrix/client/r0/thirdparty/user
type GetThirdPartyUserRequest struct {
	UserID string `json:"userid"`
}

type GetThirdPartyUserResponse struct {
	Body []ThirdPartyUser `json:"<body>"`
}

//POST /_matrix/client/r0/user/{userId}/openid/request_token
type PostUserOpenIDRequest struct {
	UserID string `json:"userId"`
}

type PostUserOpenIDResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	MatrixServerName string `json:"matrix_server_name"`
	ExpiresIn        int    `json:"expires_in"`
}

//POST /system/manager//{type}
type PostSystemManagerRequest struct {
	Type string `json:"type"`
}

//GET /unread/{userID}
type GetUserUnread struct {
	UserID string `json:"userID"`
}

type PostReportRoomRequest struct {
	RoomID string `json:"room_id"`
}

type PostReportDeviceState struct {
	State int `json:"state"`
}

// POST /notary/
type ReqPostNotaryNoticeRequest struct {
	Action       string `json:"action"`
	TargetDomain string `json:"target_domain"`
}
type ReqPostNotaryNoticeResponse struct {
	Ack int `json:"ack"`
}
