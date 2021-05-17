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

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
)

type RpcClient interface {
	// to syncserver
	SyncLoad(ctx context.Context, req *syncapitypes.SyncServerRequest) (*syncapitypes.SyncServerResponse, error)
	SyncProcess(ctx context.Context, req *syncapitypes.SyncServerRequest) (*syncapitypes.SyncServerResponse, error)
	GetPusherByDevice(ctx context.Context, req *pushapitypes.ReqPushUser) (*pushapitypes.Pushers, error)
	GetPushRuleByUser(ctx context.Context, req *pushapitypes.ReqPushUser) (*pushapitypes.Rules, error)
	GetPushDataBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (*pushapitypes.RespPushUsersData, error)
	GetPusherBatch(ctx context.Context, req *pushapitypes.ReqPushUsers) (*pushapitypes.RespUsersPusher, error)
	OnReceipt(ctx context.Context, req *types.ReceiptContent) error
	OnTyping(ctx context.Context, req *types.TypingContent) error
	OnUnRead(ctx context.Context, req *syncapitypes.SyncUnreadRequest) (*syncapitypes.SyncUnreadResponse, error)

	// to syncaggregate
	UpdateOneTimeKey(ctx context.Context, req *types.KeyUpdateContent) error
	UpdateDeviceKey(ctx context.Context, req *types.KeyUpdateContent) error
	GetOnlinePresence(ctx context.Context, userID string) (*types.OnlinePresence, error)
	SetReceiptLatest(ctx context.Context, req *syncapitypes.ReceiptUpdate) error
	AddTyping(ctx context.Context, req *syncapitypes.TypingUpdate) error
	RemoveTyping(ctx context.Context, req *syncapitypes.TypingUpdate) error

	// to clientapi
	UpdateProfile(ctx context.Context, req *types.ProfileContent) error

	// to proxy
	AddFilterToken(ctx context.Context, req *types.FilterTokenContent) error
	DelFilterToken(ctx context.Context, req *types.FilterTokenContent) error
	VerifyToken(ctx context.Context, req *types.VerifyTokenRequest) (*types.VerifyTokenResponse, error)

	// to rcsServer
	HandleEventByRcs(ctx context.Context, req *gomatrixserverlib.Event) (*types.RCSOutputEventContent, error)

	// to tokenwriter
	UpdateToken(ctx context.Context, req *types.LoginInfoContent) error

	// to publicroom
	QueryPublicRoomState(ctx context.Context, req *publicroomsapi.QueryPublicRoomsRequest) (*publicroomsapi.QueryPublicRoomsResponse, error)

	// to roomserver
	QueryEventsByID(ctx context.Context, req *roomserverapi.QueryEventsByIDRequest) (*roomserverapi.QueryEventsByIDResponse, error)
	QueryRoomEventByID(ctx context.Context, req *roomserverapi.QueryRoomEventByIDRequest) (*roomserverapi.QueryRoomEventByIDResponse, error)
	QueryJoinRooms(ctx context.Context, req *roomserverapi.QueryJoinRoomsRequest) (*roomserverapi.QueryJoinRoomsResponse, error)
	QueryRoomState(ctx context.Context, req *roomserverapi.QueryRoomStateRequest) (*roomserverapi.QueryRoomStateResponse, error)
	QueryBackFillEvents(ctx context.Context, req *roomserverapi.QueryBackFillEventsRequest) (*roomserverapi.QueryBackFillEventsResponse, error)
	QueryEventAuth(ctx context.Context, req *roomserverapi.QueryEventAuthRequest) (*roomserverapi.QueryEventAuthResponse, error)
	SetRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (*roomserverapi.SetRoomAliasResponse, error)
	GetAliasRoomID(ctx context.Context, req *roomserverapi.GetAliasRoomIDRequest) (*roomserverapi.GetAliasRoomIDResponse, error)
	RemoveRoomAlias(ctx context.Context, req *roomserverapi.RemoveRoomAliasRequest) (*roomserverapi.RemoveRoomAliasResponse, error)
	AllocRoomAlias(ctx context.Context, req *roomserverapi.SetRoomAliasRequest) (*roomserverapi.SetRoomAliasResponse, error)
}

var (
	factors = map[string]func(*config.Dendrite) RpcClient{}
)

func Register(diver string, factor func(*config.Dendrite) RpcClient) {
	factors[diver] = factor
}

func NewRpcClient(driver string, cfg *config.Dendrite) (RpcClient, error) {
	factor, ok := factors[driver]
	if !ok {
		return nil, errors.New("invalid rpc driver " + driver)
	}
	return factor(cfg), nil
}
