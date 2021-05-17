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

package helper

import (
	"encoding/json"

	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
)

func ToGetPusherByDeviceReq(req *pushapitypes.ReqPushUser) *pb.GetPusherByDeviceReq {
	request := &pb.GetPusherByDeviceReq{
		UserID:   req.UserID,
		DeviceID: req.DeviceID,
	}
	return request
}

func ToPusher(v *pb.Pusher) pushapitypes.Pusher {
	return pushapitypes.Pusher{
		UserName:          v.UserName,
		DeviceID:          v.DeviceID,
		PushKey:           v.PushKey,
		PushKeyTs:         v.PushKeyTs,
		Kind:              v.Kind,
		AppId:             v.AppId,
		AppDisplayName:    v.AppDisplayName,
		DeviceDisplayName: v.DeviceDisplayName,
		ProfileTag:        v.ProfileTag,
		Lang:              v.Lang,
		Append:            v.Append,
		Data:              string(v.Data),
	}
}

func ToPushers(pusher *pb.Pushers) *pushapitypes.Pushers {
	rsp := &pushapitypes.Pushers{}
	for _, v := range pusher.Pushers {
		rsp.Pushers = append(rsp.Pushers, ToPusher(v))
	}
	return rsp
}

func ToPBPusher(pusher *pushapitypes.Pusher) *pb.Pusher {
	return &pb.Pusher{
		UserName:          pusher.UserName,
		DeviceID:          pusher.DeviceID,
		PushKey:           pusher.PushKey,
		PushKeyTs:         pusher.PushKeyTs,
		Kind:              pusher.Kind,
		AppId:             pusher.AppId,
		AppDisplayName:    pusher.AppDisplayName,
		DeviceDisplayName: pusher.DeviceDisplayName,
		ProfileTag:        pusher.ProfileTag,
		Lang:              pusher.Lang,
		Append:            pusher.Append,
		Data:              []byte(pusher.Data),
	}
}

func ToPBPushers(result *pushapitypes.Pushers) *pb.Pushers {
	rsp := &pb.Pushers{}
	for _, v := range result.Pushers {
		rsp.Pushers = append(rsp.Pushers, ToPBPusher(&v))
	}
	return rsp
}

func ToGetPushRuleByUser(req *pushapitypes.ReqPushUser) *pb.GetPusherRuleByUserReq {
	request := &pb.GetPusherRuleByUserReq{
		UserID:   req.UserID,
		DeviceID: req.DeviceID,
	}
	return request
}

func ToCondition(cond *pb.PushCondition) pushapitypes.PushCondition {
	return pushapitypes.PushCondition{
		Kind:    cond.Kind,
		Key:     cond.Key,
		Pattern: cond.Pattern,
		Is:      cond.Is,
	}
}

func ToPushRule(v *pb.PushRule) pushapitypes.PushRule {
	var conditions []pushapitypes.PushCondition
	for _, vv := range v.Conditions {
		conditions = append(conditions, ToCondition(vv))
	}
	if conditions == nil {
		conditions = []pushapitypes.PushCondition{}
	}
	var actions []interface{}
	json.Unmarshal(v.Actions, &actions)
	return pushapitypes.PushRule{
		Actions:    actions,
		Default:    v.Default,
		Enabled:    v.Enabled,
		RuleId:     v.RuleId,
		Conditions: conditions,
		Pattern:    v.Pattern,
	}
}

func ToRules(result *pb.Rules) *pushapitypes.Rules {
	rsp := &pushapitypes.Rules{}
	for _, v := range result.Content {
		rsp.Content = append(rsp.Content, ToPushRule(v))
	}
	for _, v := range result.Override {
		rsp.Override = append(rsp.Override, ToPushRule(v))
	}
	for _, v := range result.Room {
		rsp.Room = append(rsp.Room, ToPushRule(v))
	}
	for _, v := range result.Sender {
		rsp.Sender = append(rsp.Sender, ToPushRule(v))
	}
	for _, v := range result.UnderRide {
		rsp.UnderRide = append(rsp.UnderRide, ToPushRule(v))
	}
	return rsp
}

func ToPBCondition(cond *pushapitypes.PushCondition) *pb.PushCondition {
	return &pb.PushCondition{
		Kind:    cond.Kind,
		Key:     cond.Key,
		Pattern: cond.Pattern,
		Is:      cond.Is,
	}
}

func ToPBPushRule(v *pushapitypes.PushRule) *pb.PushRule {
	var conditions []*pb.PushCondition
	for _, vv := range v.Conditions {
		conditions = append(conditions, ToPBCondition(&vv))
	}
	actions, _ := json.Marshal(v.Actions)
	return &pb.PushRule{
		Actions:    actions,
		Default:    v.Default,
		Enabled:    v.Enabled,
		RuleId:     v.RuleId,
		Conditions: conditions,
		Pattern:    v.Pattern,
	}
}

func ToPBRules(result *pushapitypes.Rules) *pb.Rules {
	rsp := &pb.Rules{}
	for _, v := range result.Content {
		rsp.Content = append(rsp.Content, ToPBPushRule(&v))
	}
	for _, v := range result.Override {
		rsp.Override = append(rsp.Override, ToPBPushRule(&v))
	}
	for _, v := range result.Room {
		rsp.Room = append(rsp.Room, ToPBPushRule(&v))
	}
	for _, v := range result.Sender {
		rsp.Sender = append(rsp.Sender, ToPBPushRule(&v))
	}
	for _, v := range result.UnderRide {
		rsp.UnderRide = append(rsp.UnderRide, ToPBPushRule(&v))
	}
	return rsp
}

func ToGetPushDataBatch(req *pushapitypes.ReqPushUsers) *pb.GetPushDataBatchReq {
	request := &pb.GetPushDataBatchReq{
		Users: req.Users,
		Slot:  req.Slot,
	}
	return request
}

func ToGetPusherBatchReq(req *pushapitypes.ReqPushUsers) *pb.GetPusherBatchReq {
	request := &pb.GetPusherBatchReq{
		Users: req.Users,
		Slot:  req.Slot,
	}
	return request
}

func ToRespPushData(data *pb.RespPushData) pushapitypes.RespPushData {
	pushers := pushapitypes.Pushers{}
	rules := pushapitypes.Rules{}

	if data.Pushers != nil {
		pushers = *ToPushers(data.Pushers)
	}
	if data.Rules != nil {
		rules = *ToRules(data.Rules)
	}

	return pushapitypes.RespPushData{
		Pushers: pushers,
		Rules:   rules,
	}
}

func ToPBRespPushData(data *pushapitypes.RespPushData) *pb.RespPushData {
	pushers := ToPBPushers(&data.Pushers)
	rules := ToPBRules(&data.Rules)

	return &pb.RespPushData{
		Pushers: pushers,
		Rules:   rules,
	}
}

func ToRespPushUsersData(result *pb.GetPushDataBatchRsp) *pushapitypes.RespPushUsersData {
	rsp := &pushapitypes.RespPushUsersData{}
	if result.Data != nil {
		rsp.Data = make(map[string]pushapitypes.RespPushData, len(result.Data))
		for k, v := range result.Data {
			rsp.Data[k] = ToRespPushData(v)
		}
	}
	return rsp
}

func ToGetPushDataBatchRsp(result *pushapitypes.RespPushUsersData) *pb.GetPushDataBatchRsp {
	rsp := &pb.GetPushDataBatchRsp{}
	if result.Data != nil {
		rsp.Data = make(map[string]*pb.RespPushData, len(result.Data))
		for k, v := range result.Data {
			rsp.Data[k] = ToPBRespPushData(&v)
		}
	}
	return rsp
}

func ToRespUsersPusher(result *pb.GetPusherBatchRsp) *pushapitypes.RespUsersPusher {
	rsp := &pushapitypes.RespUsersPusher{}
	if result.Data != nil {
		rsp.Data = make(map[string][]pushapitypes.Pusher, len(result.Data))
		for k, v := range result.Data {
			for _, vv := range v.Pushers {
				rsp.Data[k] = append(rsp.Data[k], ToPusher(vv))
			}
		}
	}
	return rsp
}

func ToGetPusherBatchRsp(result *pushapitypes.RespUsersPusher) *pb.GetPusherBatchRsp {
	rsp := &pb.GetPusherBatchRsp{}
	if result.Data != nil {
		rsp.Data = make(map[string]*pb.Pushers, len(result.Data))
		for k, v := range result.Data {
			for _, vv := range v {
				m, ok := rsp.Data[k]
				if !ok {
					m = new(pb.Pushers)
					rsp.Data[k] = m
				}
				m.Pushers = append(m.Pushers, ToPBPusher(&vv))
			}
		}
	}
	return rsp
}
