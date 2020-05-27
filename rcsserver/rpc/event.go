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

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/rcsserver/processors"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/go-nats"
)

type EventConsumer struct {
	rpcClient *common.RpcClient
	proc      *processors.EventProcessor
	//db        model.RCSServerDatabase
	//repo      *repos.RCSServerRepo
	cfg      *config.Dendrite
	slot     uint32
	chanSize uint32
	//msgChan  []chan *types.RCSInputEventContent
	msgChan []chan common.ContextMsg
}

func NewEventConsumer(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	proc *processors.EventProcessor,
	//db model.RCSServerDatabase,
	//repo *repos.RCSServerRepo,
) *EventConsumer {
	return &EventConsumer{
		rpcClient: rpcClient,
		proc:      proc,
		//db:        db,
		//repo:      repo,
		cfg:      cfg,
		slot:     64,   // Low frequency.
		chanSize: 1024, // Todo: use linked list.
	}
}

func (s *EventConsumer) GetTopic() string {
	return types.RCSEventTopicDef
}

func (s *EventConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var cont types.RCSInputEventContent
	if err := json.Unmarshal(msg.Data, &cont); err != nil {
		log.Errorf("Failed to unmarshal nats.Msg to gomatrixserverlib.Event: %v\n", err)
		return
	}
	cont.Reply = msg.Reply
	// TODO: glare situation.
	idx := common.CalcStringHashCode(cont.Event.RoomID()) % s.slot
	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &cont}
}

func (s *EventConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.slot)
	for i := uint32(0); i < s.slot; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, s.chanSize)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyGrpWithContext(s.GetTopic(), types.RCSSERVER_RPC_GROUP, s.cb)
	return nil
}

func (s *EventConsumer) startWorker(msgChan chan common.ContextMsg) {
	for cont := range msgChan {
		s.handleEvent(cont.Ctx, cont.Msg.(*types.RCSInputEventContent))
	}
}

func (s *EventConsumer) handleEvent(ctx context.Context, cont *types.RCSInputEventContent) {
	ev, _ := json.Marshal(cont.Event)
	log.Infof("rcsserver=====================EventConsumer.handleEvent, RCS Server receive event: %s\n", string(ev))
	var evs []gomatrixserverlib.Event
	var err error
	if cont.Event.Type() == gomatrixserverlib.MRoomCreate {
		evs, err = s.proc.HandleCreate(ctx, &cont.Event)
	} else if cont.Event.Type() == gomatrixserverlib.MRoomMember {
		evs, err = s.proc.HandleMembership(ctx, &cont.Event)
	} else {
		evs = append(evs, cont.Event)
		err = nil
	}

	resp := types.RCSOutputEventContent{
		Events:  evs,
		Succeed: true,
	}

	if err != nil {
		log.Errorf("Failed to handle event, event=%s, error: %v\n", string(ev), err)
		resp.Succeed = false
	}

	s.rpcClient.PubObj(cont.Reply, resp)
	return
}
