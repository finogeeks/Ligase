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

package fedbackfill

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/federationapi/rpc"
	"github.com/finogeeks/ligase/federation/model/repos"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model"
	rpcService "github.com/finogeeks/ligase/rpc"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type senderItem struct {
	domain    string
	fedClient *client.FedClientWrap
}

type BackfillJobFinishItem struct {
	EventID      string
	StartEventID string
	StartOffset  int64
	Finished     bool
}

type BackFillRecord struct {
	RoomID string

	InitState []gomatrixserverlib.Event
}

type FederationBackFill struct {
	processor    BackfillProcessor
	cfg          *config.Dendrite
	fedRpcCli    *rpc.FederationRpcClient
	msgChan      chan *BackFillJob
	db           fedmodel.FederationDatabase
	fedClient    *client.FedClientWrap
	backfillRepo *repos.BackfillRepo
}

func NewFederationBackFill(
	cfg *config.Dendrite,
	db fedmodel.FederationDatabase,
	fedClient *client.FedClientWrap,
	feddomains *common.FedDomains,
	repo *repos.BackfillRepo,
) *FederationBackFill {
	rpcCli, err := rpcService.NewRpcClient(cfg.Rpc.Driver, cfg)
	if err != nil {
		log.Panicf("failed to create rpc client, driver %s err:%v", cfg.Rpc.Driver, err)
	}
	fedRpcCli := rpc.NewFederationRpcClient(cfg, rpcCli, nil, nil, nil)

	sender := &FederationBackFill{
		cfg:          cfg,
		fedRpcCli:    fedRpcCli,
		fedClient:    fedClient,
		db:           db,
		backfillRepo: repo,
	}
	sender.processor = BackfillProcessor{
		cfg:          cfg,
		db:           db,
		fedClient:    fedClient,
		fedRpcCli:    fedRpcCli,
		feddomains:   feddomains,
		backfillRepo: repo,
	}

	sender.msgChan = make(chan *BackFillJob, 4096)

	if err := sender.loadRecords(); err != nil {
		panic(err)
	}
	if err := sender.retryBackfill(); err != nil {
		panic(err)
	}

	sender.start()
	return sender
}

func (c *FederationBackFill) loadRecords() error {
	records, err := c.db.SelectAllBackfillRecord(context.TODO())
	if err != nil {
		return err
	}
	for i := 0; i < len(records); i++ {
		record := &records[i]
		if record.Finished {
			// 节约内存
			record.States = ""
		}
		c.backfillRepo.InitBackfillRec(record.RoomID, record)
	}
	return nil
}

func (c *FederationBackFill) retryBackfill() error {
	unfinished := c.backfillRepo.GetUnfinishedRecs()
	for _, v := range unfinished {
		roomID := v.RoomID
		record := v
		job := new(BackFillJob)
		job.Depth = record.Depth
		job.Limit = record.Limit
		if err := json.Unmarshal([]byte(record.States), &job.PDUs); err != nil {
			log.Errorf("retryBackfill unmarshal states roomID: %s, err: %v state: %s", roomID, err, record.States)
			continue
		}
		if err := json.Unmarshal([]byte(record.FinishedDomains), &job.FinishedDomains); err != nil {
			log.Errorf("retryBackfill unmarshal finishedDomains roomID: %s, err: %v", roomID, err)
			continue
		}
		c.processBackfill(job)
	}
	return nil
}

func (c *FederationBackFill) start() {
	c.fedRpcCli.Start()
}

func (c *FederationBackFill) processBackfill(job *BackFillJob) {
	go c.processor.Process(job)
}

func (c *FederationBackFill) AddRequest(evs []gomatrixserverlib.Event, limit bool) error {
	if len(evs) == 0 || evs[0].Type() != "m.room.create" {
		return errors.New("backfill request state invalid")
	}
	roomID := evs[0].RoomID()
	rec := fedmodel.BackfillRecord{RoomID: roomID, FinishedDomains: "{}"}

	if ok := c.backfillRepo.InitBackfillRec(roomID, &rec); !ok {
		isFinished, _, _ := c.backfillRepo.IsBackfillFinished(roomID)
		if isFinished {
			return errors.New("backfill finished")
		} else {
			log.Infof("Backfill roomID %s already in progress", roomID)
			return errors.New("backfill already in progress")
		}
	}
	pdus := make([]*gomatrixserverlib.Event, 0, len(evs))
	for i := 0; i < len(evs); i++ {
		evs[i].SetDepth(0)
		pdus = append(pdus, &evs[i])
	}

	const kLimitSize = 1000
	states, _ := json.Marshal(evs)
	rec.States = string(states)
	if limit {
		rec.Limit = kLimitSize
	}

	err := c.db.InsertBackfillRecord(context.TODO(), rec)
	if err != nil {
		c.backfillRepo.DeleteBackfillRec(roomID)
		log.Warnf("Backfill insert record err: %v", err)
		return errors.New("backfill insert record err " + err.Error())
	}

	job := new(BackFillJob)
	job.PDUs = pdus
	if limit {
		job.Limit = kLimitSize
	}
	c.processBackfill(job)
	return nil
}

func (c *FederationBackFill) OnMessage(ctx context.Context, subject string, partition int32, data []byte, rawMsg interface{}) {
	msg := &model.GobMessage{}
	err := json.Unmarshal(data, msg)
	if err != nil {
		log.Errorf("decode error: %v", err)
		return
	}
	log.Infof("fed-backfill recv topic:%s", subject)

	if msg.Cmd == model.CMD_FED_SEND {
		t := gomatrixserverlib.Transaction{}
		_ = json.Unmarshal(msg.Body, &t)
		if len(t.PDUs) > 0 {
			c.AddRequest(t.PDUs, false) // backfill 第一次都要全部event吧
		}
	}
}
