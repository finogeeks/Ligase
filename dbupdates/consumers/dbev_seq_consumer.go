package consumers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/dbupdatetypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type IKafkaChannelSub interface {
	SubscribeTopic(topic string) error
}

type DBEventSeqConsumer struct {
	name      string
	Cfg       *config.Dendrite
	processor dbupdatetypes.DBEventSeqProcessor
	commiter  dbupdatetypes.KafkaCommiter
	msgChan   chan dbupdatetypes.DBEventDataInput
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
	c.msgChan = make(chan dbupdatetypes.DBEventDataInput, 4096)
	go c.startWorker(c.msgChan)
	c.processor.Start()

	return nil
}

func (c *DBEventSeqConsumer) startWorker(msgChan chan dbupdatetypes.DBEventDataInput) {
	if c.Cfg.Database.EnableBatch {
		const duration = time.Millisecond * 50
		timer := time.NewTimer(duration)
		inputSeq := []dbupdatetypes.DBEventDataInput{}
		ctx := context.Background()
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
			case input := <-msgChan:
				if input.Event.Key != lastKey && lastKey != 0 {
					c.processInputs(ctx, inputSeq)
					inputSeq = inputSeq[:0]
					timer.Reset(duration)
					// lastKey = 0
				}
				inputSeq = append(inputSeq, input)
				lastKey = input.Event.Key
				if len(inputSeq) >= 300 || c.batchKeys == nil || !c.batchKeys[input.Event.Key] {
					c.processInputs(ctx, inputSeq)
					inputSeq = inputSeq[:0]
					timer.Reset(duration)
					lastKey = 0
				}
			}
		}
	} else {
		ctx := context.Background()
		inputSeq := [1]dbupdatetypes.DBEventDataInput{}
		for {
			inputSeq[0] = <-msgChan
			c.processInputs(ctx, inputSeq[:])
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
	c.msgChan <- dbupdatetypes.DBEventDataInput{
		Data:   data,
		RawMsg: rawMsg,
		Event:  &output,
	}
}

func (c *DBEventSeqConsumer) CommitMessage(rawMsg []interface{}) {
	err := c.commiter.Commit(rawMsg)
	if err != nil {
		log.Errorf("DBEVentDataConsumer commit error %v", err)
	}
}
