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

package rpc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type FederationRpcClient struct {
	cfg       *config.Dendrite
	rpcClient *common.RpcClient
	aliase    roomserverapi.RoomserverAliasAPI
	qry       roomserverapi.RoomserverQueryAPI
	input     roomserverapi.RoomserverInputAPI
}

func NewFederationRpcClient(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	aliase roomserverapi.RoomserverAliasAPI,
	qry roomserverapi.RoomserverQueryAPI,
	input roomserverapi.RoomserverInputAPI,
) *FederationRpcClient {
	fed := &FederationRpcClient{
		cfg:       cfg,
		rpcClient: rpcClient,
		aliase:    aliase,
		qry:       qry,
		input:     input,
	}

	return fed
}

func (fed *FederationRpcClient) Start() {
}

func (fed *FederationRpcClient) GetAliasRoomID(
	ctx context.Context,
	req *roomserverapi.GetAliasRoomIDRequest,
	response *roomserverapi.GetAliasRoomIDResponse,
) error {
	content := roomserverapi.RoomserverAliasRequest{
		GetAliasRoomIDRequest: req,
	}
	bytes, err := json.Marshal(content)

	log.Infof("-------GetRoomAlias send data:%v", string(bytes))
	data, err := fed.rpcClient.Request(fed.cfg.Rpc.AliasTopic, bytes, 30000)

	log.Infof("-------GetRoomAlias resp data:%v, err:%v", string(data), err)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	return err
}

func (fed *FederationRpcClient) InputRoomEvents(
	ctx context.Context,
	rawEvent *roomserverapi.RawEvent,
) (int, error) {
	log.Infof("-------FederationRpcClient InputRoomEvents start")
	if fed.input != nil {
		return fed.input.InputRoomEvents(ctx, rawEvent)
	}

	bytes, err := json.Marshal(rawEvent)
	if err != nil {
		return 0, err
	}

	log.Infof("-------FederationRpcClient InputRoomEvents request topic:%s val:%s", fed.cfg.Rpc.RoomInputTopic, string(bytes))

	err = common.GetTransportMultiplexer().SendAndRecvWithRetry(
		fed.cfg.Kafka.Producer.InputRoomEvent.Underlying,
		fed.cfg.Kafka.Producer.InputRoomEvent.Name,
		&core.TransportPubMsg{
			Keys:  []byte(rawEvent.RoomID),
			Topic: fed.cfg.Kafka.Producer.InputRoomEvent.Topic,
			Obj:   rawEvent,
			Inst:  fed.cfg.Kafka.Producer.InputRoomEvent.Inst,
		})

	if err != nil {
		return 0, err
	}
	return len(rawEvent.BulkEvents.Events), nil
}

func (fed *FederationRpcClient) AllocRoomAlias(
	ctx context.Context,
	request *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	if fed.aliase != nil {
		return fed.aliase.AllocRoomAlias(ctx, request, response)
	}
	return nil
}

func (fed *FederationRpcClient) SetRoomAlias(
	ctx context.Context,
	request *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	if fed.aliase != nil {
		return fed.aliase.SetRoomAlias(ctx, request, response)
	}
	return nil
}

func (fed *FederationRpcClient) RemoveRoomAlias(
	ctx context.Context,
	request *roomserverapi.RemoveRoomAliasRequest,
	response *roomserverapi.RemoveRoomAliasResponse,
) error {
	if fed.aliase != nil {
		return fed.aliase.RemoveRoomAlias(ctx, request, response)
	}
	return nil
}

func (fed *FederationRpcClient) QueryEventsByID( //fed&pub
	ctx context.Context,
	request *roomserverapi.QueryEventsByIDRequest,
	response *roomserverapi.QueryEventsByIDResponse,
) error {
	if fed.qry != nil {
		return fed.qry.QueryEventsByID(ctx, request, response)
	}

	content := roomserverapi.RoomserverRpcRequest{
		QueryEventsByID: request,
	}
	bytes, err := json.Marshal(content)
	data, err := fed.rpcClient.Request(fed.cfg.Rpc.RsQryTopic, bytes, 30000)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	return err
}

func (fed *FederationRpcClient) QueryRoomEventByID( //cli
	ctx context.Context,
	request *roomserverapi.QueryRoomEventByIDRequest,
	response *roomserverapi.QueryRoomEventByIDResponse,
) error {
	if fed.qry != nil {
		return fed.qry.QueryRoomEventByID(ctx, request, response)
	}
	return nil
}

func (fed *FederationRpcClient) QueryJoinRooms( //cli & mig
	ctx context.Context,
	request *roomserverapi.QueryJoinRoomsRequest,
	response *roomserverapi.QueryJoinRoomsResponse,
) error {
	if fed.qry != nil {
		return fed.qry.QueryJoinRooms(ctx, request, response)
	}
	return nil
}

func (fed *FederationRpcClient) QueryRoomState( //cli & mig
	ctx context.Context,
	request *roomserverapi.QueryRoomStateRequest,
	response *roomserverapi.QueryRoomStateResponse,
) error {
	if fed.qry != nil {
		return fed.qry.QueryRoomState(ctx, request, response)
	}

	content := roomserverapi.RoomserverRpcRequest{
		QueryRoomState: request,
	}
	bytes, err := json.Marshal(content)

	data, err := fed.rpcClient.Request(fed.cfg.Rpc.RsQryTopic, bytes, 30000)

	if err == nil {
		json.Unmarshal(data, response)
		if response.RoomExists == false {
			return errors.New("room not exits")
		}
		return nil
	}

	return err
}

func (fed *FederationRpcClient) QueryBackFillEvents( //fed
	ctx context.Context,
	req *roomserverapi.QueryBackFillEventsRequest,
	response *roomserverapi.QueryBackFillEventsResponse,
) error {
	log.Infof("-------FederationRpcClient QueryBackFillEvents start, %v", fed.qry)
	if fed.qry != nil {
		return fed.qry.QueryBackFillEvents(ctx, req, response)
	}

	content := roomserverapi.RoomserverRpcRequest{
		QueryBackFillEvents: req,
	}
	bytes, err := json.Marshal(content)

	data, err := fed.rpcClient.Request(fed.cfg.Rpc.RsQryTopic, bytes, 30000)

	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	return err
}

func (fed *FederationRpcClient) QueryEventAuth( //fed
	ctx context.Context,
	req *roomserverapi.QueryEventAuthRequest,
	response *roomserverapi.QueryEventAuthResponse,
) error {
	log.Infof("-------FederationRpcClient QueryBackFillEvents start, %v", fed.qry)
	if fed.qry != nil {
		return fed.qry.QueryEventAuth(ctx, req, response)
	}

	content := roomserverapi.RoomserverRpcRequest{
		QueryEventAuth: req,
	}
	bytes, err := json.Marshal(content)

	data, err := fed.rpcClient.Request(fed.cfg.Rpc.RsQryTopic, bytes, 30000)

	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	return err
}

func (fed *FederationRpcClient) ProcessReceipt(edu *gomatrixserverlib.EDU) {
	fed.rpcClient.Pub(types.ReceiptTopicDef, edu.Content)
}

func (fed *FederationRpcClient) ProcessTyping(edu *gomatrixserverlib.EDU) {
	fed.rpcClient.Pub(types.TypingTopicDef, edu.Content)
}

func (fed *FederationRpcClient) ProcessProfile(edu *gomatrixserverlib.EDU) {
	fed.rpcClient.Pub(types.ProfileUpdateTopicDef, edu.Content)
}
