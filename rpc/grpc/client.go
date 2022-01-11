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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/finogeeks/ligase/common/localExporter"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/rpc/grpc/helper"
	"github.com/finogeeks/ligase/rpc/grpc/pb"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
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

func toErr(e interface{}) error {
	if v, ok := e.(error); ok {
		return v
	} else {
		return fmt.Errorf("%#v", e)
	}
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

func (r *Client) GetConn(rpcConf *config.RpcConf, instance uint32) (*grpc.ClientConn, error) {
	return r.connGetter.GetConn(r.connMgr, rpcConf, instance)
}

func (r *Client) getAddrByInstance(addrs []string, instance uint32) string {
	if len(addrs) == 1 {
		return addrs[0]
	}
	return addrs[instance]
}

func (r *Client) SyncLoad(ctx context.Context, req *syncapitypes.SyncServerRequest) (result *syncapitypes.SyncServerResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SyncLoad panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	//cl := newCallLog("SyncLoad", req)

	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.SyncInstance)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.SyncLoad(ctx, helper.ToSyncProcessReq(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "SyncLoad", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "SyncLoad", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToSyncServerResponse(rsp)
	//cl.end(result)
	return result, nil
}

func (r *Client) SyncProcess(ctx context.Context, req *syncapitypes.SyncServerRequest) (result *syncapitypes.SyncServerResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SyncProcess panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	//cl := newCallLog("SyncProcess", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.SyncInstance)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.SyncProcess(ctx, helper.ToSyncProcessReq(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "SyncProcess", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "SyncProcess", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToSyncServerResponse(rsp)
	//cl.end(result)
	return result, nil
}

func (r *Client) GetPusherByDevice(ctx context.Context, req *pushapitypes.ReqPushUser) (result *pushapitypes.Pushers, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetPusherByDevice panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetPusherByDevice", req)
	instance := common.CalcStringHashCode(req.UserID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPusherByDevice(ctx, helper.ToGetPusherByDeviceReq(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "GetPusherByDevice", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "GetPusherByDevice", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToPushers(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) GetPushRuleByUser(ctx context.Context, req *pushapitypes.ReqPushUser) (result *pushapitypes.Rules, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetPushRuleByUser panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetPushRuleByUser", req)
	instance := common.CalcStringHashCode(req.UserID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPushRuleByUser(ctx, helper.ToGetPushRuleByUser(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "GetPushRuleByUser", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "GetPushRuleByUser", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToRules(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) GetPushDataBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (result *pushapitypes.RespPushUsersData, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetPushDataBatch panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetPushDataBatch", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.Slot)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPushDataBatch(ctx, helper.ToGetPushDataBatch(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_SERVER, localExporter.CHAT_SYNC_SERVER, "GetPushDataBatch", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_SERVER, localExporter.CHAT_SYNC_SERVER, "GetPushDataBatch", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToRespPushUsersData(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) GetPusherBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (result *pushapitypes.RespUsersPusher, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetPusherBatch panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetPusherBatch", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.Slot)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.GetPusherBatch(ctx, helper.ToGetPusherBatchReq(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_SERVER, localExporter.CHAT_SYNC_SERVER, "GetPusherBatch", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_SERVER, localExporter.CHAT_SYNC_SERVER, "GetPusherBatch", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToRespUsersPusher(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) OnReceipt(ctx context.Context, req *types.ReceiptContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client OnReceipt panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("OnReceipt", req)
	instance := common.CalcStringHashCode(req.RoomID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return err
	}
	c := pb.NewSyncServerClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client OnReceipt panic recovered err %#v", e)
			}
		}()
		_, err = c.OnReceipt(ctx, helper.ToOnReceiptReq(req))
		if err != nil {
			log.Error("OnReceipt err %s", err)
			return
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) OnTyping(ctx context.Context, req *types.TypingContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client OnTyping panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("OnTyping", req)
	instance := common.CalcStringHashCode(req.RoomID) % r.cfg.MultiInstance.SyncServerTotal
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, instance)
	if err != nil {
		return err
	}
	c := pb.NewSyncServerClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client OnTyping panic recovered err %#v", e)
			}
		}()
		_, err = c.OnTyping(ctx, helper.ToOnTypingReq(req))
		if err != nil {
			log.Error("OnReceipt err %s", err)
			return
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) OnUnRead(ctx context.Context, req *syncapitypes.SyncUnreadRequest) (result *syncapitypes.SyncUnreadResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client OnUnRead panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("OnUnRead", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncServer, req.SyncInstance)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncServerClient(conn)
	rsp, err := c.OnUnread(ctx, helper.ToOnUnreadReq(req))
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "OnUnRead", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_SYNC_AGG, localExporter.CHAT_SYNC_SERVER, "OnUnRead", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToSyncUnreadResponse(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) UpdateOneTimeKey(ctx context.Context, req *types.KeyUpdateContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client UpdateOneTimeKey panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("UpdateOneTimeKey", req)
	instance := common.CalcStringHashCode(req.OneTimeKeyChangeUserId) % r.cfg.MultiInstance.Total
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, instance)
	if err != nil {
		return err
	}
	c := pb.NewSyncAggregateClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client UpdateOneTimeKey panic recovered err %#v", e)
			}
		}()
		_, err = c.UpdateOneTimeKey(ctx, helper.ToUpdateOneTimeKeyReq(req))
		if err != nil {
			log.Error("UpdateOneTimeKey err %s", err)
			return
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) UpdateDeviceKey(ctx context.Context, req *types.KeyUpdateContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client UpdateDeviceKey panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("UpdateDeviceKey", req)
	var errs []error
	for i := uint32(0); i < r.cfg.MultiInstance.Total; i++ {
		conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, i)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		c := pb.NewSyncAggregateClient(conn)
		go func(c pb.SyncAggregateClient, req *types.KeyUpdateContent) {
			defer func() {
				if e := recover(); e != nil {
					log.Errorf("grpc client UpdateDeviceKey panic recovered err %#v", e)
				}
			}()
			_, err = c.UpdateDeviceKey(ctx, helper.ToUpdateDeviceKeyReq(req))
			if err != nil {
				log.Error("UpdateDeviceKey err %s", err)
				//errs = append(errs, err)
				//continue
			}
		}(c, req)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) GetOnlinePresence(ctx context.Context, userID string) (result *types.OnlinePresence, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetOnlinePresence panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetOnlinePresence", userID)
	instance := common.CalcStringHashCode(userID) % r.cfg.MultiInstance.Total
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.SyncAggregate, instance)
	if err != nil {
		return nil, err
	}
	start := time.Now().UnixNano() / 1e6
	c := pb.NewSyncAggregateClient(conn)
	rsp, err := c.GetOnlinePresence(ctx, &pb.GetOnlinePresenceReq{UserID: userID})
	spend := time.Now().UnixNano()/1e6 - start
	if err != nil {
		localExporter.ExportGrpcRequestDuration(localExporter.CHAT_FRONT, localExporter.CHAT_SYNC_AGG, "GetOnlinePresence", strconv.Itoa(http.StatusInternalServerError), float64(spend))
		return nil, err
	}
	localExporter.ExportGrpcRequestDuration(localExporter.CHAT_FRONT, localExporter.CHAT_SYNC_AGG, "GetOnlinePresence", strconv.Itoa(http.StatusOK), float64(spend))
	result = helper.ToOnlinePresence(rsp)
	cl.end(result)
	return result, nil
}

func (r *Client) SetReceiptLatest(ctx context.Context, req *syncapitypes.ReceiptUpdate) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SetReceiptLatest panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
			go func(c pb.SyncAggregateClient, req *syncapitypes.ReceiptUpdate) {
				defer func() {
					if e := recover(); e != nil {
						log.Errorf("grpc client SetReceiptLatest panic recovered err %#v", e)
					}
				}()
				_, err = c.SetReceiptLatest(ctx, helper.ToSetReceiptLatestReq(req))
				if err != nil {
					log.Errorf("SetReceiptLatest err %s", err)
				}
			}(c, req)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) AddTyping(ctx context.Context, req *syncapitypes.TypingUpdate) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client AddTyping panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
			go func(c pb.SyncAggregateClient, updateTyping *syncapitypes.TypingUpdate) {
				defer func() {
					if e := recover(); e != nil {
						log.Errorf("grpc client AddTyping panic recovered err %#v", e)
					}
				}()
				_, err = c.AddTyping(ctx, helper.ToUpdateTypingReq(updateTyping))
				if err != nil {
					log.Errorf("AddTyping err %s", err)
				}
			}(c, updateTyping)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) RemoveTyping(ctx context.Context, req *syncapitypes.TypingUpdate) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client RemoveTyping panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
			go func(c pb.SyncAggregateClient, req *syncapitypes.TypingUpdate) {
				defer func() {
					if e := recover(); e != nil {
						log.Errorf("grpc client RemoveTyping panic recovered err %#v", e)
					}
				}()
				_, err = c.RemoveTyping(ctx, helper.ToUpdateTypingReq(req))
				if err != nil {
					log.Errorf("RemoveTyping err %s", err)
				}
			}(c, req)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	cl.end(nil)
	return nil
}

func (r *Client) UpdateProfile(ctx context.Context, req *types.ProfileContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client UpdateProfile panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("UpdateProfile", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Front, 0)
	if err != nil {
		return err
	}
	c := pb.NewClientapiClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client UpdateProfile panic recovered err %#v", e)
			}
		}()
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
			log.Errorf("UpdateProfile err %s", err)
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) AddFilterToken(ctx context.Context, req *types.FilterTokenContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client AddFilterToken panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("AddFilterToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Proxy, 0)
	if err != nil {
		return err
	}
	c := pb.NewProxyClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client AddFilterToken panic recovered err %#v", e)
			}
		}()
		_, err = c.AddFilterToken(ctx, &pb.AddFilterTokenReq{
			UserID:     req.UserID,
			DeviceID:   req.DeviceID,
			ClientType: req.ClientType,
			DeviceName: req.DeviceName,
		})
		if err != nil {
			log.Errorf("AddFilterToken err %s", err)
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) DelFilterToken(ctx context.Context, req *types.FilterTokenContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client DelFilterToken panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("DelFilterToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Proxy, 0)
	if err != nil {
		return err
	}
	c := pb.NewProxyClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client DelFilterToken panic recovered err %#v", e)
			}
		}()
		_, err = c.DelFilterToken(ctx, &pb.DelFilterTokenReq{
			UserID:   req.UserID,
			DeviceID: req.DeviceID,
		})
		if err != nil {
			log.Errorf("DelFilterToken err %s", err)
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) VerifyToken(ctx context.Context, req *types.VerifyTokenRequest) (result *types.VerifyTokenResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client VerifyToken panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &types.VerifyTokenResponse{
		Error: rsp.Error,
	}
	if rsp.Device != nil {
		result.Device = *helper.ToDevice(rsp.Device)
	}
	cl.end(result)
	return result, nil
}

func (r *Client) HandleEventByRcs(ctx context.Context, req *gomatrixserverlib.Event) (result *types.RCSOutputEventContent, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client HandleEventByRcs panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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

func (r *Client) UpdateToken(ctx context.Context, req *types.LoginInfoContent) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client UpdateToken panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("UpdateToken", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.TokenWriter, 0)
	if err != nil {
		return err
	}
	c := pb.NewTokenWriterClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client UpdateToken panic recovered err %#v", e)
			}
		}()
		_, err = c.UpdateToken(ctx, &pb.UpdateTokenReq{
			UserID:      req.UserID,
			DeviceID:    req.DeviceID,
			Token:       req.Token,
			DisplayName: req.DisplayName,
			Identifier:  req.Identifier,
		})
		if err != nil {
			log.Errorf("UpdateToken err %s", err)
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) QueryPublicRoomState(ctx context.Context, req *publicroomsapi.QueryPublicRoomsRequest) (result *publicroomsapi.QueryPublicRoomsResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryPublicRoomState panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &publicroomsapi.QueryPublicRoomsResponse{
		NextBatch: resp.NextBatch,
		PrevBatch: resp.PrevBatch,
		Estimate:  resp.Estimate,
	}
	for _, v := range resp.Chunk {
		result.Chunk = append(result.Chunk, *helper.ToPublicRoom(v))
	}
	return result, nil
}

func (r *Client) QueryEventsByID(ctx context.Context, req *roomserverapi.QueryEventsByIDRequest) (result *roomserverapi.QueryEventsByIDResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryEventsByID panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.QueryEventsByIDResponse{
		EventIDs: resp.EventIDs,
	}
	for _, v := range resp.Events {
		result.Events = append(result.Events, helper.ToEvent(v))
	}
	return result, nil
}

func (r *Client) QueryRoomEventByID(ctx context.Context, req *roomserverapi.QueryRoomEventByIDRequest) (result *roomserverapi.QueryRoomEventByIDResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryRoomEventByID panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("QueryRoomEventByID", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		log.Errorf("QueryRoomEventByID get conn err:%s", err.Error())
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	resp, err := c.QueryRoomEventByID(ctx, &pb.QueryRoomEventByIDReq{EventID: req.EventID, RoomID: req.RoomID})
	if err != nil {
		log.Errorf("QueryRoomEventByID query err:%s", err.Error())
		return nil, err
	}
	cl.end(resp)
	result = &roomserverapi.QueryRoomEventByIDResponse{
		EventID: resp.EventID,
		RoomID:  resp.RoomID,
		Event:   helper.ToEvent(resp.Event),
	}
	return result, nil
}

func (r *Client) QueryJoinRooms(ctx context.Context, req *roomserverapi.QueryJoinRoomsRequest) (result *roomserverapi.QueryJoinRoomsResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryJoinRooms panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.QueryJoinRoomsResponse{
		UserID: resp.UserID,
		Rooms:  resp.Rooms,
	}
	return result, nil
}

func (r *Client) QueryRoomState(ctx context.Context, req *roomserverapi.QueryRoomStateRequest) (result *roomserverapi.QueryRoomStateResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryRoomState panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.QueryRoomStateResponse{
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
		Join:              make(map[string]*gomatrixserverlib.Event, len(resp.Join)),
		Leave:             make(map[string]*gomatrixserverlib.Event, len(resp.Leave)),
		Invite:            make(map[string]*gomatrixserverlib.Event, len(resp.Invite)),
		ThirdInvite:       make(map[string]*gomatrixserverlib.Event, len(resp.ThirdInvite)),
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

func (r *Client) QueryBackFillEvents(ctx context.Context, req *roomserverapi.QueryBackFillEventsRequest) (result *roomserverapi.QueryBackFillEventsResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryBackFillEvents panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.QueryBackFillEventsResponse{
		Error:          resp.Error,
		Origin:         resp.Origin,
		OriginServerTs: gomatrixserverlib.Timestamp(resp.OriginServerTs),
	}
	for _, v := range resp.Pdus {
		result.PDUs = append(result.PDUs, *helper.ToEvent(v))
	}
	return result, nil
}

func (r *Client) QueryEventAuth(ctx context.Context, req *roomserverapi.QueryEventAuthRequest) (result *roomserverapi.QueryEventAuthResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client QueryEventAuth panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.QueryEventAuthResponse{}
	for _, v := range resp.AuthEvents {
		result.AuthEvents = append(result.AuthEvents, helper.ToEvent(v))
	}
	return result, nil
}

func (r *Client) SetRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (result *roomserverapi.SetRoomAliasResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SetRoomAlias panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.SetRoomAliasResponse{AliasExists: resp.AliasExists}
	return result, nil
}

func (r *Client) GetAliasRoomID(ctx context.Context, req *roomserverapi.GetAliasRoomIDRequest) (result *roomserverapi.GetAliasRoomIDResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetAliasRoomID panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.GetAliasRoomIDResponse{RoomID: resp.RoomID}
	return result, nil
}

func (r *Client) RemoveRoomAlias(ctx context.Context, req *roomserverapi.RemoveRoomAliasRequest) (result *roomserverapi.RemoveRoomAliasResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client RemoveRoomAlias panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.RemoveRoomAliasResponse{}
	return result, nil
}

func (r *Client) AllocRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (result *roomserverapi.SetRoomAliasResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client AllocRoomAlias panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
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
	result = &roomserverapi.SetRoomAliasResponse{AliasExists: resp.AliasExists}
	return result, nil
}

func (r *Client) InputRoomEvents(ctx context.Context, req *roomserverapi.RawEvent) (result *roomserverapi.InputRoomEventsResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client InputRoomEvents panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("InputRoomEvents", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.RoomServer, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewRoomServerClient(conn)
	request := &pb.InputRoomEventsReq{
		RoomID:     req.RoomID,
		Kind:       int32(req.Kind),
		Trust:      req.Trust,
		BulkEvents: &pb.BulkEvent{SvrName: req.BulkEvents.SvrName},
		Query:      req.Query,
	}
	if req.TxnID != nil {
		request.TxnID = &pb.TransactionID{
			DeviceID:      req.TxnID.DeviceID,
			TransactionID: req.TxnID.TransactionID,
			Ip:            req.TxnID.IP,
		}
	}
	for _, v := range req.BulkEvents.Events {
		request.BulkEvents.Events = append(request.BulkEvents.Events, helper.ToPBEvent(&v))
	}
	resp, err := c.InputRoomEvents(ctx, request)
	if err != nil {
		return nil, err
	}
	cl.end(resp)
	result = &roomserverapi.InputRoomEventsResponse{ErrCode: int(resp.ErrCode), ErrMsg: resp.ErrMsg, N: int(resp.N)}
	return result, nil
}

func (r *Client) SendEduToRemote(ctx context.Context, req *gomatrixserverlib.EDU) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SendEduToRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("SendEduToRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return err
	}
	c := pb.NewFederationClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client SendEduToRemote panic recovered err %#v", e)
			}
		}()
		_, err = c.SendEDU(ctx, &pb.SendEDUReq{
			Type:        req.Type,
			Origin:      req.Origin,
			Destination: req.Destination,
			Content:     req.Content,
		})
		if err != nil {
			log.Errorf("SendEduToRemote err %v", err)
		}
	}()
	cl.end(nil)
	return nil
}

func (r *Client) GetAliasRoomIDFromRemote(ctx context.Context, req *external.GetDirectoryRoomAliasRequest, targetDomain string) (result *external.GetDirectoryRoomAliasResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetAliasRoomIDFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetAliasRoomIDFromRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.GetAliasRoomID(ctx, &pb.GetFedAliasRoomIDReq{
		TargetDomain: targetDomain,
		RoomAlias:    req.RoomAlias,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &external.GetDirectoryRoomAliasResponse{
		RoomID:  rsp.RoomID,
		Servers: rsp.Servers,
	}, nil
}

func (r *Client) GetProfileFromRemote(ctx context.Context, req *external.GetProfileRequest, targetDomain string) (result *external.GetProfileResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetProfileFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetProfileFromRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.GetProfile(ctx, &pb.GetProfileReq{
		TargetDomain: targetDomain,
		UserID:       req.UserID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &external.GetProfileResponse{
		AvatarURL:    rsp.AvatarURL,
		DisplayName:  rsp.DisplayName,
		Status:       rsp.Status,
		StatusMsg:    rsp.StatusMsg,
		ExtStatusMsg: rsp.ExtStatusMsg,
	}, nil
}

func (r *Client) GetAvatarFromRemote(ctx context.Context, req *external.GetProfileRequest, targetDomain string) (result *external.GetAvatarURLResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetAvatarFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetAvatarFromRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.GetAvatar(ctx, &pb.GetAvatarReq{
		TargetDomain: targetDomain,
		UserID:       req.UserID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &external.GetAvatarURLResponse{
		AvatarURL: rsp.AvatarURL,
	}, nil
}

func (r *Client) GetDisplayNameFromRemote(ctx context.Context, req *external.GetProfileRequest, targetDomain string) (result *external.GetDisplayNameResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetDisplayNameFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetDisplayNameFromRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.GetDisplayName(ctx, &pb.GetDisplayNameReq{
		TargetDomain: targetDomain,
		UserID:       req.UserID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &external.GetDisplayNameResponse{
		DisplayName: rsp.DisplayName,
	}, nil
}

func (r *Client) GetRoomStateFromRemote(ctx context.Context, req *external.GetFedRoomStateRequest, targetDomain string) (result *gomatrixserverlib.RespState, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetRoomStateFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetDisplayNameFromRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.GetRoomState(ctx, &pb.GetRoomStateReq{
		TargetDomain: targetDomain,
		RoomID:       req.RoomID,
		EventID:      req.EventID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	states := &gomatrixserverlib.RespState{}
	for _, v := range rsp.StateEvents {
		states.StateEvents = append(states.StateEvents, *helper.ToEvent(v))
	}
	for _, v := range rsp.AuthEvents {
		states.AuthEvents = append(states.AuthEvents, *helper.ToEvent(v))
	}
	return states, nil
}

type DownloadReader struct {
	ch  chan []byte
	buf bytes.Buffer
}

func (r *DownloadReader) Read(p []byte) (n int, err error) {
	idx := 0
	for r.buf.Len() > 0 {
		nn, ee := r.buf.Read(p[idx:])
		idx += nn
		if idx >= len(p) {
			return idx, ee
		}
	}
	data := <-r.ch
	if len(data) == 0 {
		return idx, io.EOF
	}
	nn := copy(p[idx:], data)
	idx += nn
	if nn < len(data) {
		if len(data)-nn > 0 {
			_, err = r.buf.Write(data[nn:])
			if err != nil {
				return idx, err
			}
		}
	}
	return idx, nil
}

func (r *Client) DownloadFromRemote(ctx context.Context, req *external.GetFedDownloadRequest, targetDomain string) (reader io.Reader, code int, header http.Header, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client DownloadFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetDisplayNameFromRemote", req)
	ch := make(chan []byte, 64)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, 0, nil, err
	}
	c := pb.NewFederationClient(conn)
	downloadCli, err := c.Download(ctx, &pb.DownloadReq{
		TargetDomain: targetDomain,
		Id:           req.ID,
		FileType:     req.FileType,
		MediaID:      req.MediaID,
		Width:        req.Width,
		Method:       req.Method,
	})
	if err != nil {
		return nil, 0, nil, err
	}
	reader = &DownloadReader{ch: ch}
	recvCh := make(chan *pb.DownloadRsp, 1)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client DownloadFromRemote panic recovered err %#v", e)
			}
		}()
		// seq := int64(0)
		// windows := []*pb.DownloadRsp{}
		for {
			rsp, err := downloadCli.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Errorf("Recv error:%v", err)
					continue
				}
			}
			if rsp.Seq == 0 {
				recvCh <- rsp
			}
			// if rsp.Seq != seq {
			// 	windows = append(windows, rsp)
			// }
			ch <- rsp.Data
		}
	}()
	seq0 := <-recvCh
	response := external.GetFedDownloadResponse{}
	json.Unmarshal(seq0.Data, &response)
	cl.end(nil)
	return reader, response.StatusCode, response.Header, nil
}

func (r *Client) GetUserInfoFromRemote(ctx context.Context, req *external.GetUserInfoRequest, targetDomain string) (result *external.GetUserInfoResponse, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client GetUserInfoFromRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("GetUserInfoFromRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.GetUserInfo(ctx, &pb.GetUserInfoReq{
		TargetDomain: targetDomain,
		UserID:       req.UserID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &external.GetUserInfoResponse{
		UserName:  rsp.UserName,
		JobNumber: rsp.JobNumber,
		Mobile:    rsp.Mobile,
		Landline:  rsp.Landline,
		Email:     rsp.Email,
		State:     int(rsp.State),
	}, nil
}

func (r *Client) MakeJoinToRemote(ctx context.Context, req *external.GetMakeJoinRequest, targetDomain string) (result *gomatrixserverlib.RespMakeJoin, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client MakeJoinToRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("MakeJoinToRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.MakeJoin(ctx, &pb.MakeJoinReq{
		TargetDomain: targetDomain,
		RoomID:       req.RoomID,
		UserID:       req.UserID,
		Ver:          req.Ver,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &gomatrixserverlib.RespMakeJoin{
		JoinEvent: *helper.ToEventBuilder(rsp.JoinEvent),
	}, nil
}

func (r *Client) SendJoinToRemote(ctx context.Context, req *external.PutSendJoinRequest, targetDomain string) (result *gomatrixserverlib.RespSendJoin, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SendJoinToRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("SendJoinToRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.SendJoin(ctx, &pb.SendJoinReq{
		TargetDomain: targetDomain,
		RoomID:       req.RoomID,
		EventID:      req.EventID,
		Event:        helper.ToPBEvent(&req.Event),
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	states := &gomatrixserverlib.RespSendJoin{}
	for _, v := range rsp.StateEvents {
		states.StateEvents = append(states.StateEvents, *helper.ToEvent(v))
	}
	for _, v := range rsp.AuthEvents {
		states.AuthEvents = append(states.AuthEvents, *helper.ToEvent(v))
	}
	return states, nil
}

func (r *Client) MakeLeaveToRemote(ctx context.Context, req *external.GetMakeLeaveRequest, targetDomain string) (result *gomatrixserverlib.RespMakeLeave, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client MakeLeaveToRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("MakeLeaveToRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.MakeLeave(ctx, &pb.MakeLeaveReq{
		TargetDomain: targetDomain,
		RoomID:       req.RoomID,
		UserID:       req.UserID,
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &gomatrixserverlib.RespMakeLeave{
		Event: *helper.ToEventBuilder(rsp.Event),
	}, nil
}

func (r *Client) SendLeaveToRemote(ctx context.Context, req *external.PutSendLeaveRequest, targetDomain string) (result *gomatrixserverlib.RespSendLeave, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SendLeaveToRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("SendLeaveToRemote", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.SendLeave(ctx, &pb.SendLeaveReq{
		TargetDomain: targetDomain,
		RoomID:       req.RoomID,
		EventID:      req.EventID,
		Event:        helper.ToPBEvent(&req.Event),
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &gomatrixserverlib.RespSendLeave{
		Code: int(rsp.Code),
	}, nil
}

func (r *Client) SendInviteToRemote(ctx context.Context, event *gomatrixserverlib.Event, targetDomain string) (result *gomatrixserverlib.RespInvite, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client SendInviteToRemote panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("SendInviteToRemote", event)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Fed, 0)
	if err != nil {
		return nil, err
	}
	c := pb.NewFederationClient(conn)
	rsp, err := c.SendInvite(ctx, &pb.SendInviteReq{
		TargetDomain: targetDomain,
		Event:        helper.ToPBEvent(event),
	})
	if err != nil {
		return nil, err
	}
	cl.end(nil)
	return &gomatrixserverlib.RespInvite{
		Code:  int(rsp.Code),
		Event: *helper.ToEvent(rsp.Event),
	}, nil
}

func (r *Client) PushData(ctx context.Context, req *pushapitypes.PushPubContents) (err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("grpc client PushData panic recovered err %#v", e)
			err = toErr(e)
		}
	}()
	cl := newCallLog("PushData", req)
	conn, err := r.connGetter.GetConn(r.connMgr, &r.cfg.Rpc.Push, 0)
	if err != nil {
		return err
	}
	contents, _ := json.Marshal(req.Contents)
	c := pb.NewPushClient(conn)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("grpc client PushData panic recovered err %#v", e)
			}
		}()
		_, err = c.PushData(ctx, &pb.PushDataReq{
			Input:             helper.ToClientEvent(req.Input),
			SenderDisplayName: req.SenderDisplayName,
			RoomName:          req.RoomName,
			RoomAlias:         req.RoomAlias,
			Contents:          contents,
			CreateContent:     req.CreateContent,
			Slot:              req.Slot,
			TraceId:           req.TraceId,
		})
		if err != nil {
			log.Errorf("PushData err %s", err)
		}
	}()
	cl.end(nil)
	return nil
}
