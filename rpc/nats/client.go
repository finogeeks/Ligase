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

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	rpc.Register("nats", NewClient)
}

type OldRpcCli interface {
	Request(topic string, req []byte, timeout int) ([]byte, error)
}

type Client struct {
	cli *common.RpcClient
	cfg *config.Dendrite
}

func NewClient(cfg *config.Dendrite) rpc.RpcClient {
	cli := common.NewRpcClient(cfg.Nats.Uri)
	cli.Start(false)
	return &Client{cli: cli, cfg: cfg}
}

func (r *Client) SyncLoad(ctx context.Context, syncReq *syncapitypes.SyncServerRequest) (*syncapitypes.SyncServerResponse, error) {
	bytes, err := json.Marshal(*syncReq)
	if err != nil {
		return nil, err // TODO:
	}
	log.Infof("SyncMng.callSyncLoad load traceid:%s slot:%d user %s device %s instance:%d", syncReq.TraceID, syncReq.Slot, syncReq.UserID, syncReq.DeviceID, syncReq.SyncInstance)
	data, err := r.cli.Request(types.SyncServerTopicDef, bytes, int(r.cfg.Sync.RpcTimeout))
	if err != nil {
		return nil, err // TODO:
	}
	var result syncapitypes.SyncServerResponse
	err = json.Unmarshal(data, &result)
	return &result, err
}

func (r *Client) SyncProcess(ctx context.Context, syncReq *syncapitypes.SyncServerRequest) (*syncapitypes.SyncServerResponse, error) {
	bytes, err := json.Marshal(*syncReq)
	if err != nil {
		log.Errorf("SyncMng.buildSyncData marshal callSyncLoad content error,traceid:%s slot:%d device %s user %s error %v", syncReq.TraceID, syncReq.Slot, syncReq.DeviceID, syncReq.UserID, err)
		return nil, err
	}
	bs := time.Now().UnixNano() / 1000000
	//log.Infof("SyncMng.buildSyncData sync traceid:%s slot:%d user %s device %s request %s", req.traceId,req.slot, req.device.UserID, req.device.ID, string(bytes))
	data, err := r.cli.Request(types.SyncServerTopicDef, bytes, int(r.cfg.Sync.RpcTimeout))
	spend := time.Now().UnixNano()/1000000 - bs
	if err != nil {
		log.Errorf("SyncMng.buildSyncData call rpc for syncServer sync traceid:%s slot:%d spend:%d ms user %s device %s error %v", syncReq.TraceID, syncReq.Slot, spend, syncReq.UserID, syncReq.DeviceID, err)
		return nil, err
	}
	var result types.CompressContent
	err = json.Unmarshal(data, &result)
	if err != nil {
		log.Errorf("SyncMng.buildSyncData response traceid:%s slot:%d spend:%d ms user:%s, device:%s, Unmarshal error %v", syncReq.TraceID, syncReq.Slot, spend, syncReq.UserID, syncReq.DeviceID, err)
		return nil, err
	}
	if result.Compressed {
		result.Content = common.DoUnCompress(result.Content)
	}
	var response syncapitypes.SyncServerResponse
	err = json.Unmarshal(result.Content, &response)
	if err != nil {
		log.Errorf("SyncMng.buildSyncData SyncServerResponse response traceid:%s slot:%d spend:%d ms user:%s, device:%s Unmarshal error %v", syncReq.TraceID, syncReq.Slot, spend, syncReq.UserID, syncReq.DeviceID, err)
		return nil, err
	}
	return &response, nil
}

func (r *Client) GetPusherByDevice(ctx context.Context, req *pushapitypes.ReqPushUser) (*pushapitypes.Pushers, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	datas, err := json.Marshal(&pushapitypes.PushDataRequest{
		Payload: payload,
		ReqType: types.GET_PUSHER_BY_DEVICE,
		Slot:    common.CalcStringHashCode(req.UserID) % r.cfg.MultiInstance.SyncServerTotal,
	})
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	data, err := r.cli.Request(types.PushDataTopicDef, datas, 15000)
	if err != nil {
		return nil, err
	}
	var resp pushapitypes.RpcResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	pushers := pushapitypes.Pushers{}
	err = json.Unmarshal(resp.Payload, &pushers)
	return &pushers, nil
}

func (r *Client) GetPushRuleByUser(ctx context.Context, req *pushapitypes.ReqPushUser) (*pushapitypes.Rules, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	datas, err := json.Marshal(&pushapitypes.PushDataRequest{
		Payload: payload,
		ReqType: types.GET_PUSHRULE_BY_USER,
		Slot:    common.CalcStringHashCode(req.UserID) % r.cfg.MultiInstance.SyncServerTotal,
	})
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	data, err := r.cli.Request(types.PushDataTopicDef, datas, 15000)
	if err != nil {
		return nil, err
	}
	var resp pushapitypes.RpcResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	rules := pushapitypes.Rules{}
	err = json.Unmarshal(resp.Payload, &rules)
	return &rules, nil
}

func (r *Client) GetPushDataBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (*pushapitypes.RespPushUsersData, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	datas, err := json.Marshal(&pushapitypes.PushDataRequest{
		Payload: payload,
		ReqType: types.GET_PUSHDATA_BATCH,
		Slot:    req.Slot,
	})
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	data, err := r.cli.Request(types.PushDataTopicDef, datas, 15000)
	if err != nil {
		return nil, err
	}
	var resp pushapitypes.RpcResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	pushData := pushapitypes.RespPushUsersData{}
	err = json.Unmarshal(resp.Payload, &pushData)
	return &pushData, nil
}

func (r *Client) GetPusherBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (*pushapitypes.RespUsersPusher, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	datas, err := json.Marshal(&pushapitypes.PushDataRequest{
		Payload: payload,
		ReqType: types.GET_PUSHER_BATCH,
		Slot:    req.Slot,
	})
	if err != nil {
		return nil, errors.New("rpc local error: " + err.Error())
	}
	data, err := r.cli.Request(types.PushDataTopicDef, datas, 15000)
	if err != nil {
		return nil, err
	}
	var resp pushapitypes.RpcResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	pushers := pushapitypes.RespUsersPusher{}
	err = json.Unmarshal(resp.Payload, &pushers)
	return &pushers, nil
}

func (r *Client) OnReceipt(ctx context.Context, req *types.ReceiptContent) error {
	r.cli.PubObj(types.ReceiptTopicDef, req)
	return nil
}

func (r *Client) OnTyping(ctx context.Context, req *types.TypingContent) error {
	r.cli.PubObj(types.TypingTopicDef, req)
	return nil
}

func (r *Client) OnUnRead(ctx context.Context, req *syncapitypes.SyncUnreadRequest) (*syncapitypes.SyncUnreadResponse, error) {
	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data, err := r.cli.Request(types.SyncUnreadTopicDef, bytes, 30000)
	if err != nil {
		return nil, err
	}
	var result syncapitypes.SyncUnreadResponse
	err = json.Unmarshal(data, &result)
	return &result, err
}

func (r *Client) UpdateOneTimeKey(ctx context.Context, req *types.KeyUpdateContent) error {
	req.Type = types.ONETIMEKEYUPDATE
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r.cli.Pub(types.KeyUpdateTopicDef, bytes)
	return nil
}

func (r *Client) UpdateDeviceKey(ctx context.Context, req *types.KeyUpdateContent) error {
	req.Type = types.DEVICEKEYUPDATE
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r.cli.Pub(types.KeyUpdateTopicDef, bytes)
	return nil
}

func (r *Client) GetOnlinePresence(ctx context.Context, userID string) (*types.OnlinePresence, error) {
	req := types.OnlinePresence{UserID: userID}
	data, _ := json.Marshal(&req)
	respData, err := r.cli.Request(types.PresenceTopicDef, data, 30000)
	if err != nil {
		return nil, err
	}
	var resp types.OnlinePresence
	err = json.Unmarshal(respData, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (r *Client) SetReceiptLatest(ctx context.Context, req *syncapitypes.ReceiptUpdate) error {
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r.cli.Pub(types.ReceiptUpdateTopicDef, bytes)
	return nil
}

func (r *Client) AddTyping(ctx context.Context, req *syncapitypes.TypingUpdate) error {
	req.Type = "add"
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r.cli.Pub(types.TypingUpdateTopicDef, bytes)
	return nil
}

func (r *Client) RemoveTyping(ctx context.Context, req *syncapitypes.TypingUpdate) error {
	req.Type = "remove"
	bytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r.cli.Pub(types.TypingUpdateTopicDef, bytes)
	return nil
}

func (r *Client) UpdateProfile(ctx context.Context, req *types.ProfileContent) error {
	content, _ := json.Marshal(req)
	r.cli.Pub(types.ProfileUpdateTopicDef, content)
	return nil
}

func (r *Client) AddFilterToken(ctx context.Context, req *types.FilterTokenContent) error {
	req.FilterType = types.FILTERTOKENADD
	content, _ := json.Marshal(req)
	r.cli.Pub(types.FilterTokenTopicDef, content)
	return nil
}

func (r *Client) DelFilterToken(ctx context.Context, req *types.FilterTokenContent) error {
	req.FilterType = types.FILTERTOKENDEL
	content, _ := json.Marshal(req)
	r.cli.Pub(types.FilterTokenTopicDef, content)
	return nil
}

func (r *Client) VerifyToken(ctx context.Context, req *types.VerifyTokenRequest) (*types.VerifyTokenResponse, error) {
	reqData, _ := json.Marshal(&req)
	data, err := r.cli.Request(types.VerifyTokenTopicDef, reqData, 30000)
	if err != nil {
		return nil, err
	}

	content := types.VerifyTokenResponse{}
	err = json.Unmarshal(data, &content)
	if err != nil {
		return nil, err
	}

	return &content, nil
}

func (r *Client) HandleEventByRcs(ctx context.Context, req *gomatrixserverlib.Event) (*types.RCSOutputEventContent, error) {
	inCont := types.RCSInputEventContent{
		Event: *req,
	}
	bytes, err := json.Marshal(inCont)
	if err != nil {
		return nil, err
	}
	data, err := r.cli.Request(types.RCSEventTopicDef, bytes, 35000)
	if err != nil {
		return nil, err
	}
	var cont types.RCSOutputEventContent
	err = json.Unmarshal(data, &cont)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *Client) UpdateToken(ctx context.Context, req *types.LoginInfoContent) error {
	content, _ := json.Marshal(req)
	r.cli.Pub(types.LoginTopicDef, content)
	return nil
}

func (r *Client) QueryPublicRoomState(ctx context.Context, req *publicroomsapi.QueryPublicRoomsRequest) (*publicroomsapi.QueryPublicRoomsResponse, error) {
	bytes, _ := json.Marshal(publicroomsapi.PublicRoomsRpcRequest{
		QueryPublicRooms: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.PrQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response publicroomsapi.QueryPublicRoomsResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) QueryEventsByID(ctx context.Context, req *roomserverapi.QueryEventsByIDRequest) (*roomserverapi.QueryEventsByIDResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverRpcRequest{
		QueryEventsByID: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.QueryEventsByIDResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) QueryRoomEventByID(ctx context.Context, req *roomserverapi.QueryRoomEventByIDRequest) (*roomserverapi.QueryRoomEventByIDResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverRpcRequest{
		QueryRoomEventByID: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.QueryRoomEventByIDResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) QueryJoinRooms(ctx context.Context, req *roomserverapi.QueryJoinRoomsRequest) (*roomserverapi.QueryJoinRoomsResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverRpcRequest{
		QueryJoinRooms: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.QueryJoinRoomsResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) QueryRoomState(ctx context.Context, req *roomserverapi.QueryRoomStateRequest) (*roomserverapi.QueryRoomStateResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverRpcRequest{
		QueryRoomState: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.QueryRoomStateResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) QueryBackFillEvents(ctx context.Context, req *roomserverapi.QueryBackFillEventsRequest) (*roomserverapi.QueryBackFillEventsResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverRpcRequest{
		QueryBackFillEvents: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.QueryBackFillEventsResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) QueryEventAuth(ctx context.Context, req *roomserverapi.QueryEventAuthRequest) (*roomserverapi.QueryEventAuthResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverRpcRequest{
		QueryEventAuth: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.QueryEventAuthResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) SetRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (*roomserverapi.SetRoomAliasResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverAliasRequest{
		SetRoomAliasRequest: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.AliasTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.SetRoomAliasResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) GetAliasRoomID(ctx context.Context, req *roomserverapi.GetAliasRoomIDRequest) (*roomserverapi.GetAliasRoomIDResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverAliasRequest{
		GetAliasRoomIDRequest: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.AliasTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.GetAliasRoomIDResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) RemoveRoomAlias(ctx context.Context, req *roomserverapi.RemoveRoomAliasRequest) (*roomserverapi.RemoveRoomAliasResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverAliasRequest{
		RemoveRoomAliasRequest: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.AliasTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.RemoveRoomAliasResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *Client) AllocRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (*roomserverapi.SetRoomAliasResponse, error) {
	bytes, _ := json.Marshal(roomserverapi.RoomserverAliasRequest{
		AllocRoomAliasRequest: req,
	})

	data, err := r.cli.Request(r.cfg.Rpc.AliasTopic, bytes, 30000)
	if err != nil {
		return nil, err
	}

	var response roomserverapi.SetRoomAliasResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
