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
	"sort"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/federation/config"
	"github.com/finogeeks/ligase/federation/federationapi/rpc"
	"github.com/finogeeks/ligase/federation/fedutil"
	"github.com/finogeeks/ligase/federation/model/repos"
	fedmodel "github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
)

type BackFillJob struct {
	PDUs            []*gomatrixserverlib.Event
	Limit           int
	Depth           int64
	FinishedDomains map[string]*BackfillJobFinishItem
}

type BackfillItem struct {
	domain string
	pdus   []gomatrixserverlib.Event
	end    bool
	err    bool
}

type BackfillProcessor struct {
	cfg          *config.Fed
	db           fedmodel.FederationDatabase
	fedClient    *client.FedClientWrap
	fedRpcCli    *rpc.FederationRpcClient
	feddomains   *common.FedDomains
	clientMap    sync.Map
	backfillRepo *repos.BackfillRepo
}

func (p *BackfillProcessor) Process(job *BackFillJob) {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("Process Backfill panic: %#v", e)
		}
	}()
	roomID := job.PDUs[0].RoomID()
	log.Infof("Backfill start roomID: %s", roomID)
	idMap := map[string]*BackfillJobFinishItem{}
	states := map[string]*gomatrixserverlib.Event{}
	for _, ev := range job.PDUs {
		states[ev.EventID()] = ev
		domain, _ := utils.DomainFromID(ev.Sender())
		if _, ok := idMap[domain]; ok {
			continue
		}
		if v, ok := job.FinishedDomains[domain]; ok {
			idMap[domain] = v
		} else {
			if common.CheckValidDomain(domain, p.cfg.GetServerName()) {
				idMap[domain] = &BackfillJobFinishItem{Finished: true}
			} else {
				idMap[domain] = &BackfillJobFinishItem{}
			}
		}
	}

	const kBatchSize = 50
	respCh := make(chan BackfillItem, 1000)
	for domain, item := range idMap {
		if item.Finished {
			continue
		}
		if common.CheckValidDomain(domain, p.cfg.GetServerName()) {
			continue
		}
		go func(domain string, eventID string) {
			val, ok := p.clientMap.Load(domain)
			if !ok {
				_, ok = p.feddomains.GetDomainInfo(domain)
				if !ok {
					log.Errorf("Process Backfill domain %s not set", domain)
					respCh <- BackfillItem{domain, nil, true, false}
					return
				}

				sitem := new(senderItem)
				sitem.domain = domain
				sitem.fedClient, _ = client.GetFedClient(p.cfg.GetServerName()[0])
				val, _ = p.clientMap.LoadOrStore(domain, sitem)
			}
			item := val.(*senderItem)
			log.Infof("Backfill try to back from domain %s roomid: %s eventID: %s", domain, roomID, eventID)
			failedTimes := 0
			for {
				id := []string{eventID}
				host, _ := p.feddomains.GetDomainHost(domain)
				resp, err := item.fedClient.Backfill(context.TODO(), gomatrixserverlib.ServerName(host), domain, roomID, kBatchSize, id, "b")
				if err != nil || resp.Error != "" {
					errStr := ""
					if err != nil {
						errStr = err.Error()
					} else {
						errStr = resp.Error
					}
					log.Errorf("Backfill try to back from domain %s roomid:%s eventid: %s err %s", domain, roomID, eventID, errStr)
					if failedTimes == 0 {
						respCh <- BackfillItem{domain, nil, false, true}
					}
					failedTimes++
					sleepTime := time.Millisecond * 100 * time.Duration(failedTimes)
					if sleepTime > time.Second*10 {
						sleepTime = time.Second * 10
					}
					time.Sleep(sleepTime)
					continue
				} else {
					bytes, _ := json.Marshal(resp)
					log.Infof("fed-backfill from domain %s roomid:%s eventid: %s resp: %s", domain, roomID, eventID, bytes)
				}
				if len(resp.PDUs) == 0 {
					respCh <- BackfillItem{domain, nil, true, false}
					break
				}

				sort.Slice(resp.PDUs, func(i, j int) bool { return resp.PDUs[i].EventNID() > resp.PDUs[j].EventNID() })
				newEventID := resp.PDUs[len(resp.PDUs)-1].EventID()
				if newEventID == "" || newEventID == eventID {
					log.Errorf("Backfill from domain %s deadloop, roomID: %s, oldEventID: %s, newEventID: %s", domain, roomID, eventID, newEventID)
					break
				}
				eventID = newEventID

				for i := len(resp.PDUs) - 1; i >= 0; i-- {
					resp.PDUs[i].SetDepth(-1)
					if _, ok := states[resp.PDUs[i].EventID()]; ok {
						if i == len(resp.PDUs)-1 {
							resp.PDUs = resp.PDUs[:i]
						} else {
							resp.PDUs = append(resp.PDUs[:i], resp.PDUs[i+1:]...)
						}
					}
				}
				go p.downloadMedia(resp.PDUs)
				p.inputRoomEvents(roomID, resp.PDUs, roomserverapi.KindBackfill)

				failedTimes = 0
			}
		}(domain, item.EventID)
	}

	finishedDomains, _ := json.Marshal(idMap)
	stateEvents := []*gomatrixserverlib.Event{}
	for _, v := range states {
		stateEvents = append(stateEvents, v)
	}
	statesJSON, _ := json.Marshal(stateEvents)
	p.db.UpdateBackfillRecordDomainsInfo(context.TODO(), roomID, 0, true, string(finishedDomains), string(statesJSON))
	p.backfillRepo.SetFinishedDomains(roomID, string(finishedDomains))
	p.backfillRepo.SetState(roomID, string(statesJSON))
	p.backfillRepo.SetBackfillFinished(roomID)
	log.Infof("Backfill count: %d, roomID: %s", 0, roomID)
}

func (c *BackfillProcessor) inputRoomEvents(roomID string, pdus []gomatrixserverlib.Event, kind int) (int, error) {
	if len(pdus) == 0 {
		return 0, nil
	}
	return c.fedRpcCli.InputRoomEvents(context.TODO(), &roomserverapi.RawEvent{
		RoomID: roomID,
		Kind:   kind,
		Trust:  true,
		BulkEvents: roomserverapi.BulkEvent{
			Events: pdus,
		},
	})
}

func (c *BackfillProcessor) downloadMedia(pdus []gomatrixserverlib.Event) {
	for i := 0; i < len(pdus); i++ {
		ev := &pdus[i]
		if ev.Type() != "m.room.message" {
			continue
		}
		domain, _ := utils.DomainFromID(ev.Sender())
		if common.CheckValidDomain(domain, c.cfg.GetServerName()) {
			continue
		}
		destination, ok := c.feddomains.GetDomainHost(domain)
		if !ok {
			log.Errorf("BackfillProcessor.downloadMedia domain: %s", domain)
			continue
		}
		var content map[string]interface{}
		err := json.Unmarshal(ev.Content(), &content)
		if err != nil {
			continue
		}
		if fedutil.IsMediaEv(content) {
			fedutil.DownloadFromNetdisk(domain, destination, ev, content, c.fedClient)
		}
	}
}
