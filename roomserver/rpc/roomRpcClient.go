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
	"errors"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type RoomserverRpcClient struct {
	cfg           *config.Dendrite
	rpcClient     *common.RpcClient
	rpcCli        rpc.RpcClient
	aliase        roomserverapi.RoomserverAliasAPI
	qry           roomserverapi.RoomserverQueryAPI
	input         roomserverapi.RoomserverInputAPI
	idg           *uid.UidGenerator
	inputUseKafka bool
}

func NewRoomserverRpcClient(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	rpcCli rpc.RpcClient,
	aliase roomserverapi.RoomserverAliasAPI,
	qry roomserverapi.RoomserverQueryAPI,
	input roomserverapi.RoomserverInputAPI,
) roomserverapi.RoomserverRPCAPI {
	idg, _ := uid.NewDefaultIdGenerator(cfg.Matrix.InstanceId)
	useKfka := false
	if cfg.Kafka.Producer.InputRoomEvent.Underlying == "kafka" {
		useKfka = true
	}
	s := &RoomserverRpcClient{
		cfg:           cfg,
		rpcClient:     rpcClient,
		rpcCli:        rpcCli,
		aliase:        aliase,
		qry:           qry,
		input:         input,
		idg:           idg,
		inputUseKafka: useKfka,
	}

	return s
}

func (c *RoomserverRpcClient) AllocRoomAlias(
	ctx context.Context,
	req *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	if c.aliase != nil {
		return c.aliase.AllocRoomAlias(ctx, req, response)
	}

	resp, err := c.rpcCli.AllocRoomAlias(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp
	return nil
}

func (c *RoomserverRpcClient) SetRoomAlias( //cli
	ctx context.Context,
	req *roomserverapi.SetRoomAliasRequest,
	response *roomserverapi.SetRoomAliasResponse,
) error {
	if c.aliase != nil {
		return c.aliase.SetRoomAlias(ctx, req, response)
	}

	resp, err := c.rpcCli.SetRoomAlias(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp
	return nil
}

func (c *RoomserverRpcClient) GetAliasRoomID( //fed & cli
	ctx context.Context,
	req *roomserverapi.GetAliasRoomIDRequest,
	response *roomserverapi.GetAliasRoomIDResponse,
) error {
	if c.aliase != nil {
		log.Infof("-------RoomserverRpcClient GetAliasRoomID direct call")
		return c.aliase.GetAliasRoomID(ctx, req, response)
	}

	log.Infof("-------RoomserverRpcClient GetAliasRoomID call rpc")
	resp, err := c.rpcCli.GetAliasRoomID(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp
	return nil
}

func (c *RoomserverRpcClient) RemoveRoomAlias(
	ctx context.Context,
	req *roomserverapi.RemoveRoomAliasRequest,
	response *roomserverapi.RemoveRoomAliasResponse,
) error {
	if c.aliase != nil {
		return c.aliase.RemoveRoomAlias(ctx, req, response)
	}
	resp, err := c.rpcCli.RemoveRoomAlias(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp
	return nil
}

func (c *RoomserverRpcClient) QueryEventsByID(
	ctx context.Context,
	req *roomserverapi.QueryEventsByIDRequest,
	response *roomserverapi.QueryEventsByIDResponse,
) error {
	if c.qry != nil {
		return c.qry.QueryEventsByID(ctx, req, response)
	}

	resp, err := c.rpcCli.QueryEventsByID(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp

	return nil
}

func (c *RoomserverRpcClient) QueryRoomEventByID( //cli
	ctx context.Context,
	req *roomserverapi.QueryRoomEventByIDRequest,
	response *roomserverapi.QueryRoomEventByIDResponse,
) error {
	if c.qry != nil {
		return c.qry.QueryRoomEventByID(ctx, req, response)
	}

	resp, err := c.rpcCli.QueryRoomEventByID(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp

	return nil
}

func (c *RoomserverRpcClient) QueryJoinRooms( //cli & mig
	ctx context.Context,
	req *roomserverapi.QueryJoinRoomsRequest,
	response *roomserverapi.QueryJoinRoomsResponse,
) error {
	if c.qry != nil {
		return c.qry.QueryJoinRooms(ctx, req, response)
	}

	resp, err := c.rpcCli.QueryJoinRooms(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp

	return nil
}

func (c *RoomserverRpcClient) QueryRoomState( //cli & mig
	ctx context.Context,
	req *roomserverapi.QueryRoomStateRequest,
	response *roomserverapi.QueryRoomStateResponse,
) error {
	if c.qry != nil {
		return c.qry.QueryRoomState(ctx, req, response)
	}

	resp, err := c.rpcCli.QueryRoomState(ctx, req)
	if err != nil {
		return err
	}
	*response = *resp
	if response.RoomExists == false {
		return errors.New("room not exits")
	}
	return nil
}

//use kafka in a async way, use nats in a sync way
func (c *RoomserverRpcClient) InputRoomEvents(
	ctx context.Context,
	rawEvent *roomserverapi.RawEvent,
) (int, error) {
	log.Debugf("-------RoomserverRpcClient InputRoomEvents start")
	if c.input != nil {
		if rawEvent.TxnID != nil {
			log.Debugf("-------RoomserverRpcClient input not nil, direct call txnId:%s", rawEvent.TxnID)
		} else {
			log.Debugf("-------RoomserverRpcClient input not nil, direct call")
		}
		return c.input.InputRoomEvents(context.TODO(), rawEvent)
	}

	if c.inputUseKafka == true {
		log.Debugf("-------RoomserverRpcClient send to node %s", c.cfg.Kafka.Producer.InputRoomEvent.Name)
		// TODO: 返回0先，如果成功则也没问题，如果失败则认为所有事件失败
		return 0, common.GetTransportMultiplexer().SendAndRecvWithRetry(
			c.cfg.Kafka.Producer.InputRoomEvent.Underlying,
			c.cfg.Kafka.Producer.InputRoomEvent.Name,
			&core.TransportPubMsg{
				Keys: []byte(rawEvent.RoomID),
				Obj:  rawEvent,
				Inst: c.cfg.Kafka.Producer.InputRoomEvent.Inst,
			})
	}

	log.Debugf("-------RoomserverRpcClient InputRoomEvents request topic:%s val:%+v", c.cfg.Rpc.RoomInputTopic, rawEvent)
	resp, err := c.rpcCli.InputRoomEvents(ctx, rawEvent)
	if err != nil {
		return 0, err
	}

	log.Debugf("-------RoomserverRpcClient InputRoomEvents resp:%v", resp)

	if resp.ErrCode < 0 {
		return resp.N, errors.New(resp.ErrMsg)
	}

	return resp.N, nil
}

func (c *RoomserverRpcClient) QueryBackFillEvents( //fed
	ctx context.Context,
	req *roomserverapi.QueryBackFillEventsRequest,
	response *roomserverapi.QueryBackFillEventsResponse,
) error {
	log.Infof("-------RoomserverRpcClient QueryBackFillEvents start, %v", c.qry)
	if c.qry != nil {
		return c.qry.QueryBackFillEvents(ctx, req, response)
	}

	content := roomserverapi.RoomserverRpcRequest{
		QueryBackFillEvents: req,
	}
	bytes, err := json.Marshal(content)

	log.Infof("-------QueryBackFillEvents send data:%v", string(bytes))
	resp, err := c.rpcCli.QueryBackFillEvents(ctx, req)

	if err != nil {
		return err
	}
	*response = *resp

	return err
}

func (c *RoomserverRpcClient) QueryEventAuth(
	ctx context.Context,
	req *roomserverapi.QueryEventAuthRequest,
	response *roomserverapi.QueryEventAuthResponse,
) error {
	log.Infof("-------RoomserverRpcClient QueryEventAuth start, %v", c.qry)
	if c.qry != nil {
		return c.qry.QueryEventAuth(ctx, req, response)
	}

	content := roomserverapi.RoomserverRpcRequest{
		QueryEventAuth: req,
	}
	bytes, err := json.Marshal(content)

	log.Infof("-------QueryEventAuth send data:%v", string(bytes))
	resp, err := c.rpcCli.QueryEventAuth(ctx, req)

	if err != nil {
		return err
	}
	*response = *resp

	return nil
}

func (c *RoomserverRpcClient) ProcessReceipt(edu *gomatrixserverlib.EDU) {

}

func (c *RoomserverRpcClient) ProcessTyping(edu *gomatrixserverlib.EDU) {

}

func (c *RoomserverRpcClient) ProcessProfile(edu *gomatrixserverlib.EDU) {

}
