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

package pushapitypes

import (
	"sync"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Pusher struct {
	UserName          string      `json:"user_name,omitempty"`
	DeviceID          string      `json:"device_id,omitempty"`
	PushKey           string      `json:"pushkey,omitempty"`
	PushKeyTs         int64       `json:"pushkey_ts,omitempty"`
	Kind              string      `json:"kind,omitempty"`
	AppId             string      `json:"app_id,omitempty"`
	AppDisplayName    string      `json:"app_display_name,omitempty"`
	DeviceDisplayName string      `json:"device_display_name,omitempty"`
	ProfileTag        string      `json:"profile_tag,omitempty"`
	Lang              string      `json:"lang,omitempty"`
	Append            bool        `json:"append,omitempty"`
	Data              interface{} `json:"data,omitempty"`
}

type PusherData interface{}

type Pushers struct {
	Pushers []Pusher `json:"pushers"`
}

func (p *Pushers) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Pushers) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type PushContent struct {
	Content interface{} `json:"content"`
}

type PushContentUsers struct {
	Data PushDataMembers `json:"data,omitempty"`
}

type PushDataMembers struct {
	Members []string `json:"members,omitempty"`
}

type PusherCacheData struct {
	UserName          string
	ProfileTag        string
	Kind              string
	AppId             string
	AppDisplayName    string
	DeviceDisplayName string
	PushKey           string
	PushKeyTs         int64
	Lang              string
	Data              string
	DeviceID          string
}

var PriorityMap = func() map[string]int {
	m := map[string]int{
		"underride": 1,
		"sender":    2,
		"room":      3,
		"content":   4,
		"override":  5,
	}
	return m
}

var RevPriorityMap = func() map[int]string {
	m := map[int]string{
		1: "underride",
		2: "sender",
		3: "room",
		4: "content",
		5: "override",
	}
	return m
}

type PushRulesData struct {
	PushRules []PushRuleData
}

type PushRuleData struct {
	UserName      string
	RuleId        string
	PriorityClass int
	Priority      int
	Conditions    []byte
	Actions       []byte
}

type PushRule struct {
	Actions    []interface{}   `json:"actions,omitempty"`
	Default    bool            `json:"default"`
	Enabled    bool            `json:"enabled"`
	RuleId     string          `json:"rule_id,omitempty"`
	Conditions []PushCondition `json:"conditions,omitempty"`
	Pattern    string          `json:"pattern,omitempty"`
}

func (p *PushRule) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PushRule) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type PushRuleWithConditions struct {
	Actions    []interface{}   `json:"actions,omitempty"`
	Default    bool            `json:"default"`
	Enabled    bool            `json:"enabled"`
	RuleId     string          `json:"rule_id,omitempty"`
	Conditions []PushCondition `json:"conditions"`
	Pattern    string          `json:"pattern,omitempty"`
}

func (p *PushRuleWithConditions) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PushRuleWithConditions) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type PushRuleCacheData struct {
	UserName      string
	RuleId        string
	PriorityClass int
	Priority      int
	Conditions    []byte
	Actions       []byte
}

type PushRuleCacheDataArray []PushRuleCacheData

func (list PushRuleCacheDataArray) Len() int {
	return len(list)
}

func (list PushRuleCacheDataArray) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list PushRuleCacheDataArray) Less(i, j int) bool {
	ci := list[i].PriorityClass
	cj := list[j].PriorityClass
	pi := list[i].Priority
	pj := list[j].Priority

	if ci > cj {
		return true
	} else if ci < cj {
		return false
	} else {
		return pi > pj
	}
}

type PushCondition struct {
	Kind    string `json:"kind,omitempty"`
	Key     string `json:"key,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	Is      string `json:"is,omitempty"`
}

type EnabledType struct {
	Enabled bool `json:"enabled"`
}

func (p *EnabledType) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *EnabledType) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type RuleSet struct {
	Content   []interface{} `json:"content"`
	Override  []interface{} `json:"override"`
	Room      []interface{} `json:"room"`
	Sender    []interface{} `json:"sender"`
	UnderRide []interface{} `json:"underride"`
}

func (p *RuleSet) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *RuleSet) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type Rules struct {
	Content   []PushRule `json:"content"`
	Override  []PushRule `json:"override"`
	Room      []PushRule `json:"room"`
	Sender    []PushRule `json:"sender"`
	UnderRide []PushRule `json:"underride"`
}

type GlobalRule struct {
	Device map[string]interface{} `json:"device"`
	Global RuleSet                `json:"global"`
}

func (p *GlobalRule) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *GlobalRule) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type Tweak struct {
	SetTweak string      `json:"set_tweak,omitempty"`
	Value    interface{} `json:"value,omitempty"`
}

type Actions struct {
	Actions []interface{} `json:"actions,omitempty"`
}

type PushActions struct {
	Actions []interface{} `json:"actions,omitempty"`
}

func (p *PushActions) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PushActions) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}

type TweakAction struct {
	Notify    string
	Sound     string
	HighLight bool
}

type Notify struct {
	Notify Notification `json:"notification,omitempty"`
}

type Notification struct {
	EventId           string      `json:"event_id,omitempty"`
	RoomId            string      `json:"room_id,omitempty"`
	Type              string      `json:"type,omitempty"`
	Sender            string      `json:"sender,omitempty"`
	SenderDisplayName string      `json:"sender_display_name,omitempty"`
	RoomName          string      `json:"room_name,omitempty"`
	RoomAlias         string      `json:"room_alias,omitempty"`
	UserIsTarget      bool        `json:"user_is_target,omitempty"`
	Priority          string      `json:"prio,omitempty"`
	Content           interface{} `json:"content,omitempty"`
	Counts            Counts      `json:"counts,omitempty"`
	Devices           []Device    `json:"devices,omitempty"`
	CreateEvent       interface{} `json:"create_event,omitempty"`
}

type Counts struct {
	UnRead      int64 `json:"unread,omitempty"`
	MissedCalls int64 `json:"missed_calls,omitempty"`
}

type Device struct {
	DeviceID  string      `json:"device_id,omitempty"`
	UserName  string      `json:"user_name,omitempty"`
	AppId     string      `json:"app_id,omitempty"`
	PushKey   string      `json:"pushkey,omitempty"`
	PushKeyTs int64       `json:"pushkey_ts,omitempty"`
	Data      interface{} `json:"data"`
	Tweak     Tweaks      `json:"tweaks,omitempty"`
}

type Tweaks struct {
	Sound     string `json:"sound,omitempty"`
	HighLight bool   `json:"highlight,omitempty"`
}

type PushAck struct {
	Rejected []string `json:"rejected,omitempty"`
}

type PushPubContents struct {
	Input             *gomatrixserverlib.ClientEvent `json:"input,omitempty"`
	SenderDisplayName string                         `json:"senderDisplayName,omitempty"`
	RoomName          string                         `json:"roomName,omitempty"`
	RoomAlias         string                         `json:"roomAlias,omitempty"`
	Contents          []*PushPubContent              `json:"contents,omitempty"`
	CreateContent     *interface{}                   `json:"create_content"`
}

type PushPubContent struct {
	Pushers     *Pushers     `json:"pushers,omitempty"`
	UserID      string       `json:"userID,omitempty"`
	Action      *TweakAction `json:"action,omitempty"`
	NotifyCount int64        `json:"notify_count"`
}

var PushTopicDef = "pushdata-topic"

type ReceiptTs struct {
	Ts int64 `json:"ts"`
}

type ReceiptUser struct {
	Users map[string]ReceiptTs `json:"m.read"`
}

type ReceiptContent struct {
	Map map[string]ReceiptUser
}

type RoomReceipt struct {
	RoomID   string
	EvID     string
	EvOffSet int64
	Content  *sync.Map //key:user, value: ts
}

type PusherUsers struct {
	Users []string `json:"users,omitempty"`
}

type PusherRes struct {
	UserName  string `json:"user_name,omitempty"`
	AppId     string `json:"app_id,omitempty"`
	Kind      string `json:"kind,omitempty"`
	PushKey   string `json:"pushkey,omitempty"`
	PushKeyTs int64  `json:"ts,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
}

type PushersRes struct {
	PushersRes []PusherRes `json:"pushers"`
}

func (p *PushersRes) Encode() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PushersRes) Decode(input []byte) error {
	return json.Unmarshal(input, p)
}
