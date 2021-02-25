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

package consumers

import (
	"context"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/clientapi/routing"
	"github.com/finogeeks/ligase/clientapi/threepid"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type DismissRoomConsumer struct {
	rpcCli       roomserverapi.RoomserverRPCAPI
	cache        service.Cache
	accountDB    model.AccountsDatabase
	cfg          *config.Dendrite
	federation   *fed.Federation
	complexCache *common.ComplexCache
	idg          *uid.UidGenerator
}

func NewDismissRoomConsumer(underlying, name string,
	rpcCli roomserverapi.RoomserverRPCAPI,
	cache service.Cache,
	accountDB model.AccountsDatabase,
	cfg *config.Dendrite,
	federation *fed.Federation,
	complexCache *common.ComplexCache,
	idg *uid.UidGenerator) *DismissRoomConsumer {
	val, ok := common.GetTransportMultiplexer().GetChannel(underlying, name)
	if ok {
		channel := val.(core.IChannel)
		c := &DismissRoomConsumer{}
		c.accountDB = accountDB
		c.rpcCli = rpcCli
		c.cache = cache
		c.federation = federation
		c.complexCache = complexCache
		c.idg = idg
		c.cfg = cfg
		channel.SetHandler(c)
		return c
	}

	return nil
}

func (c *DismissRoomConsumer) Start() error {
	return nil
}

// if leave fail , continue
func (c *DismissRoomConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var req external.DismissRoomRequest
	err := json.Unmarshal(data, &req)
	if err != nil {
		log.Errorf("SettingsConsumer unmarshal error %v", err)
		return
	}
	log.Infof("DismissRoomConsumer OnMessage topic: %s, partition: %d, data: %s", topic, partition, string(data))
	roomID := req.RoomID
	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	err = c.rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		log.Errorf("SettingsConsumer QueryRoomState error %v", err)
		return
	}
	log.Infof("DismissRoomConsumer, roomid: %s, join: %d, invites: %d", roomID, len(queryRes.Join), len(queryRes.Invite))

	msg := external.PostRoomsMembershipRequest{}
	msg.Membership = "dismiss"
	msg.RoomID = roomID
	var body threepid.MembershipRequest
	for _, ev := range queryRes.Join {
		userID := *ev.StateKey()
		body.UserID = userID
		content, _ := json.Marshal(body)
		msg.Content = content
		// sender must be the people dismiss room
		// deviceId cannot 100% get by userID, use leave member's userID instead
		status, _ := routing.SendMembership(ctx, &msg, c.accountDB, req.UserID, userID, roomID, "dismiss", *c.cfg, c.rpcCli, c.federation, c.cache, c.idg, c.complexCache)
		if status != http.StatusOK {
			log.Errorf("DismissRoomConsumer leave fail! skip user:%s, roomID:%s", userID, roomID)
		}
		time.Sleep(200 * time.Millisecond)
	}
	for _, ev := range queryRes.Invite {
		userID := *ev.StateKey()
		body.UserID = userID
		content, _ := json.Marshal(body)
		msg.Content = content
		// sender must be the people dismiss room
		// deviceId cannot 100% get by userID, use leave member's userID instead
		status, _ := routing.SendMembership(ctx, &msg, c.accountDB, req.UserID, userID, roomID, "dismiss", *c.cfg, c.rpcCli, c.federation, c.cache, c.idg, c.complexCache)
		if status != http.StatusOK {
			log.Errorf("DismissRoomConsumer leave fail! skip user:%s, roomID:%s", userID, roomID)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
