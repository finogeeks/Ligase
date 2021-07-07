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

package api

import (
	"context"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/model/repos"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type InternalMsgConsumer struct {
	apiconsumer.APIConsumer
	idg *uid.UidGenerator
	db  model.RCSServerDatabase
	repo *repos.RCSRepo
}

func NewInternalMsgConsumer(
	cfg config.Dendrite,
	rpcCli *common.RpcClient,
	idg *uid.UidGenerator,
	db model.RCSServerDatabase,
	repo *repos.RCSRepo,
) *InternalMsgConsumer {
	c := new(InternalMsgConsumer)
	c.Cfg = cfg
	c.RpcCli = rpcCli
	c.idg = idg
	c.db = db
	c.repo = repo
	return c
}

func (c *InternalMsgConsumer) Start() {
	c.APIConsumer.Init("rcsserverapi", c, c.Cfg.Rpc.ProxyRCSServerApiTopic)
	c.APIConsumer.Start()
}

func (c *InternalMsgConsumer) getFriendship(req *external.GetFriendshipRequest) (*external.GetFriendshipResponse, error) {
	resp, err := c.repo.GetFriendshipByFcIDOrToFcID(context.Background(), req.FcID, req.ToFcID)
	if err != nil {
		if c.repo.NotFound(err) {
			return resp, nil
		}
		log.Errorf("Failed to get roomId: %v\n", err)
		return resp, err
	}
	return resp, nil
}

func (c *InternalMsgConsumer) getFriendships(req *external.GetFriendshipsRequest, userID string) (*external.GetFriendshipsResponse, error) {
	var resp external.GetFriendshipsResponse
	var err error
	if req.Type == external.RCSFriendshipTypeBot {
		resp.Friendships, err = c.db.GetFriendshipsByFcIDOrToFcIDWithBot(context.Background(), userID)
	} else {
		resp.Friendships, err = c.db.GetFriendshipsByFcIDOrToFcID(context.Background(), userID)
	}
	if err != nil {
		log.Errorf("Failed to get friendships: %v\n", err)
		return &resp, err
	}
	return &resp, nil
}

func getProxyRpcTopic(cfg *config.Dendrite) string {
	return cfg.Rpc.ProxyRCSServerApiTopic
}
