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

package grpc

import (
	"context"
	"strconv"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"google.golang.org/grpc"
)

type CallLog struct {
	name string
}

func newCallLog(name string, req interface{}) *CallLog {
	// data, _ := json.Marshal(req)
	// log.Infof("--------- call rpc %s %s", name, string(data))
	return &CallLog{name: name}
}

func (c *CallLog) end(result interface{}) {
	// data, _ := json.Marshal(result)
	// log.Infof("--------- end call rpc %s %s", c.name, string(data))
}

type IConnGetter interface {
	GetConn(mgr *GrpcConnectManager, cfg *config.RpcConf, instance uint32) (*grpc.ClientConn, error)
}

type ConnGetter struct{}

func (c *ConnGetter) GetConn(mgr *GrpcConnectManager, cfg *config.RpcConf, instance uint32) (*grpc.ClientConn, error) {
	return mgr.GetConn(cfg.ServerName, cfg.Addresses[instance])
}

type ConnGetterWithConsul struct{}

func (c *ConnGetterWithConsul) GetConn(mgr *GrpcConnectManager, cfg *config.RpcConf, instance uint32) (*grpc.ClientConn, error) {
	return mgr.GetConnWithConsul(cfg.ServerName, "tag="+cfg.ConsulTagPrefix+strconv.Itoa(int(instance)))
}

func init() {
	rpc.Register("grpc", NewClient)
	rpc.Register("grpc_with_consul", NewClientWithConsul)
}

type Client struct {
	cfg        *config.Dendrite
	connMgr    *GrpcConnectManager
	connGetter IConnGetter
}

func NewClient(cfg *config.Dendrite) rpc.RpcClient {
	return &Client{
		cfg:        cfg,
		connMgr:    NewGrpcConnectManager(cfg.Rpc.ConsulURL),
		connGetter: &ConnGetter{},
	}
}

func NewClientWithConsul(cfg *config.Dendrite) rpc.RpcClient {
	return &Client{
		cfg:        cfg,
		connMgr:    NewGrpcConnectManager(cfg.Rpc.ConsulURL),
		connGetter: &ConnGetterWithConsul{},
	}
}

func (r *Client) getAddrByInstance(addrs []string, instance uint32) string {
	if len(addrs) == 1 {
		return addrs[0]
	}
	return addrs[instance]
}

func (r *Client) SyncLoad(ctx context.Context, req *syncapitypes.SyncServerRequest) (*syncapitypes.SyncServerResponse, error) {
	//cl := newCallLog("SyncLoad", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.SyncInstance)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.SyncLoad(ctx, helper.ToSyncProcessReq(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToSyncServerResponse(rsp)
	//cl.end(result)
	return result, nil
}

func (r *Client) SyncProcess(ctx context.Context, req *syncapitypes.SyncServerRequest) (*syncapitypes.SyncServerResponse, error) {
	//cl := newCallLog("SyncProcess", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.SyncInstance)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.SyncProcess(ctx, helper.ToSyncProcessReq(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToSyncServerResponse(rsp)
	//cl.end(result)
	return result, nil
}

func (r *Client) GetPusherByDevice(ctx context.Context, req *pushapitypes.ReqPushUser) (*pushapitypes.Pushers, error) {
	cl := newCallLog("GetPusherByDevice", req)
	instance := common.CalcStringHashCode(req.UserID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPusherByDevice(ctx, helper.ToGetPusherByDeviceReq(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToPushers(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) GetPushRuleByUser(ctx context.Context, req *pushapitypes.ReqPushUser) (*pushapitypes.Rules, error) {
	cl := newCallLog("GetPushRuleByUser", req)
	instance := common.CalcStringHashCode(req.UserID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPushRuleByUser(ctx, helper.ToGetPushRuleByUser(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToRules(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) GetPushDataBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (*pushapitypes.RespPushUsersData, error) {
	cl := newCallLog("GetPushDataBatch", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.Slot)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPushDataBatch(ctx, helper.ToGetPushDataBatch(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToRespPushUsersData(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) GetPusherBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (*pushapitypes.RespUsersPusher, error) {
	cl := newCallLog("GetPusherBatch", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.Slot)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPusherBatch(ctx, helper.ToGetPusherBatchReq(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToRespUsersPusher(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) OnReceipt(ctx context.Context, req *types.ReceiptContent) error {
	cl := newCallLog("OnReceipt", req)
	instance := common.CalcStringHashCode(req.RoomID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return err
	}
	c := pb.NewSyncServerClient(conn)
	_, err = c.OnReceipt(ctx, helper.ToOnReceiptReq(req))
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) OnTyping(ctx context.Context, req *types.TypingContent) error {
	cl := newCallLog("OnTyping", req)
	instance := common.CalcStringHashCode(req.RoomID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return err
	}
	c := pb.NewSyncServerClient(conn)
	_, err = c.OnTyping(ctx, helper.ToOnTypingReq(req))
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) OnUnRead(ctx context.Context, req *syncapitypes.SyncUnreadRequest) (*syncapitypes.SyncUnreadResponse, error) {
	cl := newCallLog("OnUnRead", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.SyncInstance)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.OnUnread(ctx, helper.ToOnUnreadReq(req))
	if err != nil {
		return nil, err
	}
	result := helper.ToSyncUnreadResponse(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) UpdateOneTimeKey(ctx context.Context, req *types.KeyUpdateContent) error {
	cl := newCallLog("UpdateOneTimeKey", req)
	instance := common.CalcStringHashCode(req.OneTimeKeyChangeUserId) % r.cfg.MultiInstance.Total
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, instance)
	if err != nil {
		return err
	}
	c := pb.NewSyncAggregateClient(conn)
	_, err = c.UpdateOneTimeKey(ctx, helper.ToUpdateOneTimeKeyReq(req))
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) UpdateDeviceKey(ctx context.Context, req *types.KeyUpdateContent) error {
	cl := newCallLog("UpdateDeviceKey", req)
	var errs []error
	for i := uint32(0); i < r.cfg.MultiInstance.Total; i++ {
		conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, i)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		c := pb.NewSyncAggregateClient(conn)
		_, err = c.UpdateDeviceKey(ctx, helper.ToUpdateDeviceKeyReq(req))
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) GetOnlinePresence(ctx context.Context, userID string) (*types.OnlinePresence, error) {
	cl := newCallLog("GetOnlinePresence", userID)
	instance := common.CalcStringHashCode(userID) % r.cfg.MultiInstance.Total
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, instance)
	if err != nil {
		return nil, err
	}
	c := pb.NewSyncAggregateClient(conn)
	rsp, err := c.GetOnlinePresence(ctx, &pb.GetOnlinePresenceReq{UserID: userID})
	if err != nil {
		return nil, err
	}
	result := helper.ToOnlinePresence(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) SetReceiptLatest(ctx context.Context, req *syncapitypes.ReceiptUpdate) error {
	cl := newCallLog("SetReceiptLatest", req)
	var errs []error
	for i := uint32(0); i < r.cfg.MultiInstance.Total; i++ {
		updateReceiptOffset := &syncapitypes.ReceiptUpdate{
			Users:  []string{},
			Offset: req.Offset,
			RoomID: req.RoomID,
		}
		for _, user := range req.Users {
			if common.IsRelatedRequest(user, i, r.cfg.MultiInstance.Total, false) {
				updateReceiptOffset.Users = append(updateReceiptOffset.Users, user)
			}
		}
		if len(updateReceiptOffset.Users) > 0 {
			conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, i)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			c := pb.NewSyncAggregateClient(conn)
			_, err = c.SetReceiptLatest(ctx, helper.ToSetReceiptLatestReq(req))
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) AddTyping(ctx context.Context, req *syncapitypes.TypingUpdate) error {
	cl := newCallLog("AddTyping", req)
	var errs []error
	for i := uint32(0); i < r.cfg.MultiInstance.Total; i++ {
		updateTyping := &syncapitypes.TypingUpdate{
			Type:      req.Type,
			RoomID:    req.RoomID,
			UserID:    req.UserID,
			DeviceID:  req.DeviceID,
			RoomUsers: []string{},
		}
		for _, user := range req.RoomUsers {
			if common.IsRelatedRequest(user, i, r.cfg.MultiInstance.Total, false) {
				updateTyping.RoomUsers = append(updateTyping.RoomUsers, user)
			}
		}
		if len(updateTyping.RoomUsers) > 0 {
			conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, i)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			c := pb.NewSyncAggregateClient(conn)
			_, err = c.AddTyping(ctx, helper.ToUpdateTypingReq(updateTyping))
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) RemoveTyping(ctx context.Context, req *syncapitypes.TypingUpdate) error {
	cl := newCallLog("RemoveTyping", req)
	var errs []error
	for i := uint32(0); i < r.cfg.MultiInstance.Total; i++ {
		updateTyping := &syncapitypes.TypingUpdate{
			Type:      req.Type,
			RoomID:    req.RoomID,
			UserID:    req.UserID,
			DeviceID:  req.DeviceID,
			RoomUsers: []string{},
		}
		for _, user := range req.RoomUsers {
			if common.IsRelatedRequest(user, i, r.cfg.MultiInstance.Total, false) {
				updateTyping.RoomUsers = append(updateTyping.RoomUsers, user)
			}
		}
		if len(updateTyping.RoomUsers) > 0 {
			conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, i)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			c := pb.NewSyncAggregateClient(conn)
			_, err = c.RemoveTyping(ctx, helper.ToUpdateTypingReq(req))
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) UpdateProfile(ctx context.Context, req *types.ProfileContent) error {
	cl := newCallLog("UpdateProfile", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Front, 0)
	if err != nil {
		return err
	}
	c := pb.NewClientapiClient(conn)
	_, err = c.UpdateProfile(ctx, &pb.UpdateProfileReq{
		UserID:       req.UserID,
		DisplayName:  req.DisplayName,
		AvatarUrl:    req.AvatarUrl,
		Presence:     req.Presence,
		StatusMsg:    req.StatusMsg,
		ExtStatusMsg: req.ExtStatusMsg,
		UserName:     req.UserName,
		JobNumber:    req.JobNumber,
		Mobile:       req.Mobile,
		Landline:     req.Landline,
		Email:        req.Email,
		State:        int32(req.State),
	})
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) AddFilterToken(ctx context.Context, req *types.FilterTokenContent) error {
	cl := newCallLog("AddFilterToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Proxy, 0)
	if err != nil {
		return err
	}
	c := pb.NewProxyClient(conn)
	_, err = c.AddFilterToken(ctx, &pb.AddFilterTokenReq{
		UserID:   req.UserID,
		DeviceID: req.DeviceID,
	})
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) DelFilterToken(ctx context.Context, req *types.FilterTokenContent) error {
	cl := newCallLog("DelFilterToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Proxy, 0)
	if err != nil {
		return err
	}
	c := pb.NewProxyClient(conn)
	_, err = c.DelFilterToken(ctx, &pb.DelFilterTokenReq{
		UserID:   req.UserID,
		DeviceID: req.DeviceID,
	})
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) VerifyToken(ctx context.Context, req *types.VerifyTokenRequest) (*types.VerifyTokenResponse, error) {
	cl := newCallLog("VerifyToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Proxy, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewProxyClient(conn)
	rsp, err := c.VerifyToken(ctx, &pb.VerifyTokenReq{
		Token:      req.Token,
		RequestURI: req.RequestURI,
	})
	if err != nil {
		return nil, err
	}
	result := &types.VerifyTokenResponse{
		Error:  rsp.Error,
		Device: *helper.ToDevice(rsp.Device),
	}
	cl.end(result)
	return result, nil
}

func (r *Client) HandleEventByRcs(ctx context.Context, req *gomatrixserverlib.Event) (*types.RCSOutputEventContent, error) {
	cl := newCallLog("HandleEventByRcs", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Rcs, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRcsServerClient(conn)
	rsp, err := c.HandleEventByRcs(ctx, helper.ToPBEvent(req))
	if err != nil {
		return nil, err
	}
	ret := &types.RCSOutputEventContent{Succeed: rsp.Succeed}
	for _, v := range rsp.Events {
		ret.Events = append(ret.Events, *helper.ToEvent(v))
	}
	cl.end(ret)
	return ret, nil
}

func (r *Client) UpdateToken(ctx context.Context, req *types.LoginInfoContent) error {
	cl := newCallLog("UpdateToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.TokenWriter, 0)
	if err != nil {
		return err
	}
	c := pb.NewTokenWriterClient(conn)
	_, err = c.UpdateToken(ctx, &pb.UpdateTokenReq{
		UserID:      req.UserID,
		DeviceID:    req.DeviceID,
		Token:       req.Token,
		DisplayName: req.DisplayName,
		Identifier:  req.Identifier,
	})
	if err != nil {
		return err
	}
	cl.end(nil)
	return nil
}

func (r *Client) QueryPublicRoomState(ctx context.Context, req *publicroomsapi.QueryPublicRoomsRequest) (*publicroomsapi.QueryPublicRoomsResponse, error) {
	cl := newCallLog("QueryPublicRoomState", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.PublicRoom, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewPublicRoomClient(conn)
	resp, err := c.QueryPublicRoomState(ctx, &pb.QueryPublicRoomStateReq{
		Limit:  req.Limit,
		Since:  req.Since,
		Filter: req.Filter,
	})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &publicroomsapi.QueryPublicRoomsResponse{
		NextBatch: resp.NextBatch,
		PrevBatch: resp.PrevBatch,
		Estimate:  resp.Estimate,
	}
	for _, v := range resp.Chunk {
		result.Chunk = append(result.Chunk, *helper.ToPublicRoom(v))
	}
	return result, nil
}

func (r *Client) QueryEventsByID(ctx context.Context, req *roomserverapi.QueryEventsByIDRequest) (*roomserverapi.QueryEventsByIDResponse, error) {
	cl := newCallLog("QueryEventsByID", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryEventsByID(ctx, &pb.QueryEventsByIDReq{EventIDs: req.EventIDs})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.QueryEventsByIDResponse{
		EventIDs: resp.EventIDs,
	}
	for _, v := range resp.Events {
		result.Events = append(result.Events, helper.ToEvent(v))
	}
	return result, nil
}

func (r *Client) QueryRoomEventByID(ctx context.Context, req *roomserverapi.QueryRoomEventByIDRequest) (*roomserverapi.QueryRoomEventByIDResponse, error) {
	cl := newCallLog("QueryRoomEventByID", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryRoomEventByID(ctx, &pb.QueryRoomEventByIDReq{EventID: req.EventID, RoomID: req.RoomID})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.QueryRoomEventByIDResponse{
		EventID: resp.EventID,
		RoomID:  resp.RoomID,
		Event:   helper.ToEvent(resp.Event),
	}
	return result, nil
}

func (r *Client) QueryJoinRooms(ctx context.Context, req *roomserverapi.QueryJoinRoomsRequest) (*roomserverapi.QueryJoinRoomsResponse, error) {
	cl := newCallLog("QueryJoinRooms", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryJoinRooms(ctx, &pb.QueryJoinRoomsReq{UserID: req.UserID})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.QueryJoinRoomsResponse{
		UserID: resp.UserID,
		Rooms:  resp.Rooms,
	}
	return result, nil
}

func (r *Client) QueryRoomState(ctx context.Context, req *roomserverapi.QueryRoomStateRequest) (*roomserverapi.QueryRoomStateResponse, error) {
	cl := newCallLog("QueryRoomState", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryRoomState(ctx, &pb.QueryRoomStateReq{RoomID: req.RoomID})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.QueryRoomStateResponse{
		RoomID:            resp.RoomID,
		RoomExists:        resp.RoomExists,
		Creator:           helper.ToEvent(resp.Creator),
		JoinRule:          helper.ToEvent(resp.JoinRule),
		HistoryVisibility: helper.ToEvent(resp.HistoryVisibility),
		Visibility:        helper.ToEvent(resp.Visibility),
		Name:              helper.ToEvent(resp.Name),
		Topic:             helper.ToEvent(resp.Topic),
		Desc:              helper.ToEvent(resp.Desc),
		CanonicalAlias:    helper.ToEvent(resp.CanonicalAlias),
		Power:             helper.ToEvent(resp.Power),
		Alias:             helper.ToEvent(resp.Alias),
		Avatar:            helper.ToEvent(resp.Avatar),
		GuestAccess:       helper.ToEvent(resp.GuestAccess),
	}
	for k, v := range resp.Join {
		result.Join[k] = helper.ToEvent(v)
	}
	for k, v := range resp.Leave {
		result.Leave[k] = helper.ToEvent(v)
	}
	for k, v := range resp.Invite {
		result.Invite[k] = helper.ToEvent(v)
	}
	for k, v := range resp.ThirdInvite {
		result.ThirdInvite[k] = helper.ToEvent(v)
	}
	return result, nil
}

func (r *Client) QueryBackFillEvents(ctx context.Context, req *roomserverapi.QueryBackFillEventsRequest) (*roomserverapi.QueryBackFillEventsResponse, error) {
	cl := newCallLog("QueryBackFillEvents", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryBackFillEvents(ctx, &pb.QueryBackFillEventsReq{
		EventID: req.EventID,
		Limit:   int64(req.Limit),
		RoomID:  req.RoomID,
		Dir:     req.Dir,
		Domain:  req.Domain,
		Origin:  req.Origin,
	})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.QueryBackFillEventsResponse{
		Error:          resp.Error,
		Origin:         resp.Origin,
		OriginServerTs: gomatrixserverlib.Timestamp(resp.OriginServerTs),
	}
	for _, v := range resp.Pdus {
		result.PDUs = append(result.PDUs, *helper.ToEvent(v))
	}
	return result, nil
}

func (r *Client) QueryEventAuth(ctx context.Context, req *roomserverapi.QueryEventAuthRequest) (*roomserverapi.QueryEventAuthResponse, error) {
	cl := newCallLog("QueryEventAuth", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryEventAuth(ctx, &pb.QueryEventAuthReq{EventID: req.EventID})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.QueryEventAuthResponse{}
	for _, v := range resp.AuthEvents {
		result.AuthEvents = append(result.AuthEvents, helper.ToEvent(v))
	}
	return result, nil
}

func (r *Client) SetRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (*roomserverapi.SetRoomAliasResponse, error) {
	cl := newCallLog("SetRoomAlias", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.SetRoomAlias(ctx, &pb.SetRoomAliasReq{
		UserID: req.UserID,
		Alias:  req.Alias,
		RoomID: req.RoomID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.SetRoomAliasResponse{AliasExists: resp.AliasExists}
	return result, nil
}

func (r *Client) GetAliasRoomID(ctx context.Context, req *roomserverapi.GetAliasRoomIDRequest) (*roomserverapi.GetAliasRoomIDResponse, error) {
	cl := newCallLog("GetAliasRoomID", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.GetAliasRoomID(ctx, &pb.GetAliasRoomIDReq{
		Alias: req.Alias,
	})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.GetAliasRoomIDResponse{RoomID: resp.RoomID}
	return result, nil
}

func (r *Client) RemoveRoomAlias(ctx context.Context, req *roomserverapi.RemoveRoomAliasRequest) (*roomserverapi.RemoveRoomAliasResponse, error) {
	newCallLog("RemoveRoomAlias", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	_, err = c.RemoveRoomAlias(ctx, &pb.RemoveRoomAliasReq{
		UserID: req.UserID,
		Alias:  req.Alias,
	})
	if err != nil {
		return nil, err
	}
	result := &roomserverapi.RemoveRoomAliasResponse{}
	return result, nil
}

func (r *Client) AllocRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (*roomserverapi.SetRoomAliasResponse, error) {
	cl := newCallLog("AllocRoomAlias", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.AllocRoomAlias(ctx, &pb.AllocRoomAliasReq{
		UserID: req.UserID,
		Alias:  req.Alias,
		RoomID: req.RoomID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result := &roomserverapi.SetRoomAliasResponse{AliasExists: resp.AliasExists}
	return result, nil
}
