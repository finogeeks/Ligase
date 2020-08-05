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

package federationapi

import (
	"context"
	"fmt"

	"github.com/finogeeks/ligase/cache"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/client/cert"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/federation/federationapi/entry"
	"github.com/finogeeks/ligase/federation/federationapi/rpc"
	"github.com/finogeeks/ligase/federation/model/backfilltypes"
	fedrepos "github.com/finogeeks/ligase/federation/model/repos"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/publicroomsapi"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/skunkworks/util/id"
	dbmodel "github.com/finogeeks/ligase/storage/model"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type FederationAPIComponent struct {
	cfg       *config.Fed
	cache     service.Cache
	fedRpcCli *rpc.FederationRpcClient
	Repo      *repos.RoomServerCurStateRepo
	fedClient *client.FedClientWrap
	db        fedmodel.FederationDatabase
}

func NewFederationAPIComponent(
	cfg *config.Fed,
	_cache service.Cache,
	fedClient *client.FedClientWrap,
	fedDB fedmodel.FederationDatabase,
	keyDB dbmodel.KeyDatabase,
	feddomains *common.FedDomains,
	fedRpcCli *rpc.FederationRpcClient,
	backfillRepo *fedrepos.BackfillRepo,
	joinRoomsRepo *fedrepos.JoinRoomsRepo,
	backfillProc backfilltypes.BackFillProcessor,
	publicroomsAPI publicroomsapi.PublicRoomsQueryAPI,
	rpcClient *common.RpcClient,
	encryptionDB dbmodel.EncryptorAPIDatabase,
	c *cert.Cert,
	idg *uid.UidGenerator,
	complexCache *common.ComplexCache,
) *FederationAPIComponent {
	entry.SetFedDomains(feddomains)
	entry.SetCfg(cfg)
	entry.SetKeyDB(keyDB)
	entry.SetCert(c)
	entry.SetBackfillRepo(backfillRepo)
	entry.SetBackFillProcessor(backfillProc)
	entry.SetJoinRoomsRepo(joinRoomsRepo)
	entry.SetPublicRoomsAPI(publicroomsAPI)
	entry.SetRpcClient(rpcClient)
	entry.SetEncryptionDB(encryptionDB)
	entry.SetComplexCache(complexCache)
	lc := new(cache.LocalCacheRepo)
	lc.Start(1, cfg.Cache.DurationDefault)
	entry.SetLocalCache(lc)
	entry.SetIDG(idg)

	fed := &FederationAPIComponent{
		cfg:       cfg,
		cache:     _cache,
		fedRpcCli: fedRpcCli,
		fedClient: fedClient,
		db:        fedDB,
	}
	return fed
}

func (fed *FederationAPIComponent) SetRepo(repo *repos.RoomServerCurStateRepo) {
	fed.Repo = repo
	entry.SetRepo(repo)
}

func (fed *FederationAPIComponent) Setup() {
	fed.fedRpcCli.Start()
}

func (fed *FederationAPIComponent) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	//dec := gob.NewDecoder(bytes.NewReader(data))
	msg := &model.GobMessage{}
	//err := dec.Decode(msg)
	err := json.Unmarshal(data, msg)
	if err != nil {
		log.Errorf("decode error: %v", err)
		return
	}
	if msg.Key == nil {
		msg.Key = []byte{}
	}
	log.Infof("fed-api recv topic:%s cmd:%d key:%s", topic, msg.Cmd, string(msg.Key))

	// call federation api by commandID
	var retMsg *model.GobMessage

	if _, ok := entry.FedApiFunc[msg.Cmd]; !ok {
		retMsg = &model.GobMessage{}
		retMsg.ErrStr = fmt.Sprintf("cannot find api entry, msg: %v", msg)
	} else {
		retMsg, err = entry.FedApiFunc[msg.Cmd](ctx, msg, fed.cache, fed.fedRpcCli, fed.fedClient, fed.db)
		if err == nil {
			retMsg.ErrStr = ""
		} else {
			retMsg.ErrStr = err.Error()
		}
	}
	retMsg.MsgType = model.REPLY
	retMsg.MsgSeq = msg.MsgSeq
	retMsg.NodeId = id.GetNodeId()
	retMsg.Key = msg.Key
	//nodeID := strings.TrimPrefix(subject, fmt.Sprintf("%s.", fed.cfg.GetMsgBusReqTopic()))
	//resSubject := fmt.Sprintf("%s.%s", fed.cfg.GetMsgBusResTopic(), nodeID)
	log.Infof("resSubject: %s, cmd: %d, key:%s retMsg: %s", fed.cfg.Kafka.Producer.FedAPIOutput.Topic, msg.Cmd, retMsg.Key, retMsg.Body)

	span, _ := common.StartSpanFromContext(ctx, fed.cfg.Kafka.Producer.FedAPIOutput.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, fed.cfg.Kafka.Producer.FedAPIOutput.Name,
		fed.cfg.Kafka.Producer.FedAPIOutput.Underlying)
	common.GetTransportMultiplexer().SendAndRecvWithRetry(
		fed.cfg.Kafka.Producer.FedAPIOutput.Underlying,
		fed.cfg.Kafka.Producer.FedAPIOutput.Name,
		&core.TransportPubMsg{
			//Format: core.FORMAT_GOB,
			Keys:    retMsg.Key,
			Obj:     retMsg,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}
