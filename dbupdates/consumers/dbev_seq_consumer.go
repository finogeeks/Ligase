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
	"encoding/json"
	"github.com/finogeeks/ligase/common"
	"time"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

type IKafkaChannelSub interface {
	SubscribeTopic(topic string) error
}

type DBEventSeqConsumer struct {
	name      string
	Cfg       *config.Dendrite
	processor dbupdatetypes.DBEventSeqProcessor
	commiter  dbupdatetypes.KafkaCommiter
	//msgChan   chan dbupdatetypes.DBEventDataInput
	msgChan   chan common.ContextMsg
	batchKeys map[int64]bool
}

func newDBEventSeqConsumer(
	name string,
	cfg *config.Dendrite,
	processor dbupdatetypes.DBEventSeqProcessor,
	commiter dbupdatetypes.KafkaCommiter,
) *DBEventSeqConsumer {
	c := new(DBEventSeqConsumer)
	c.name = name
	c.Cfg = cfg
	c.processor = processor
	if b, ok := processor.(dbupdatetypes.DBEventSeqBatch); ok {
		c.batchKeys = b.BatchKeys()
	}
	c.commiter = commiter
	return c
}

func (c *DBEventSeqConsumer) Start() error {
	c.msgChan = make(chan common.ContextMsg, 4096)
	go c.startWorker(c.msgChan)
	c.processor.Start()

	return nil
}

func (c *DBEventSeqConsumer) startWorker(msgChan chan common.ContextMsg) {
	if c.Cfg.Database.EnableBatch {
		const duration = time.Millisecond * 50
		timer := time.NewTimer(duration)
		inputSeq := []dbupdatetypes.DBEventDataInput{}
		span, ctx := common.StartSomSpan(context.Background(), "DBEventSeqConsumer.startWorker")
		defer span.Finish()
		lastKey := int64(0)
		for {
			select {
			case <-timer.C:
				if len(inputSeq) > 0 {
					c.processInputs(ctx, inputSeq)
					inputSeq = inputSeq[:0]
					lastKey = 0
				}
				timer.Reset(duration)
			case msg := <-msgChan:
				ctx = msg.Ctx
				input := msg.Msg.(dbupdatetypes.DBEventDataInput)
				if input.Event.Key != lastKey && lastKey != 0 {
					c.processInputs(ctx, inputSeq)
					inputSeq = inputSeq[:0]
					timer.Reset(duration)
					// lastKey = 0
				}
				inputSeq = append(inputSeq, input)
				lastKey = input.Event.Key
				if len(inputSeq) >= 30 || c.batchKeys == nil || !c.batchKeys[input.Event.Key] {
					c.processInputs(ctx, inputSeq)
					inputSeq = inputSeq[:0]
					timer.Reset(duration)
					lastKey = 0
				}
			}
		}
	} else {
		inputSeq := [1]dbupdatetypes.DBEventDataInput{}
		for {
			msg := <-msgChan
			inputSeq[0] = msg.Msg.(dbupdatetypes.DBEventDataInput)
			c.processInputs(msg.Ctx, inputSeq[:])
		}
	}
}

func (c *DBEventSeqConsumer) processInputs(ctx context.Context, inputs []dbupdatetypes.DBEventDataInput) {
	if len(inputs) == 0 {
		return
	}
	defer func() {
		msgs := []interface{}{}
		for _, v := range inputs {
			msgs = append(msgs, v.RawMsg)
		}
		c.CommitMessage(msgs)
	}()
	c.processor.Process(ctx, inputs)
}

func (c *DBEventSeqConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var output dbtypes.DBEvent
	if err := json.Unmarshal(data, &output); err != nil {
		log.Errorw("dbevent: message parse failure", log.KeysAndValues{"error", err})
		return
	}

	if output.IsRecovery {
		return //recover from db, just need to write cache
	}
	c.msgChan <- common.ContextMsg{
		Ctx: ctx,
		Msg: dbupdatetypes.DBEventDataInput{
			Data:   data,
			RawMsg: rawMsg,
			Event:  &output,
		},
	}
}

func (c *DBEventSeqConsumer) CommitMessage(rawMsg []interface{}) {
	err := c.commiter.Commit(rawMsg)
	if err != nil {
		log.Errorf("DBEVentDataConsumer commit error %v", err)
	}
}
