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

package processors

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/finogeeks/ligase/adapter"

	"github.com/finogeeks/ligase/common/uid"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	fed "github.com/finogeeks/ligase/federation/fedreq"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"

	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

type InputResult struct {
	Num   int
	Error error
}

type InputContext struct {
	ctx    context.Context
	input  *roomserverapi.RawEvent
	result chan InputResult
}

type EventsProcessor struct {
	DB           model.RoomServerDatabase
	Repo         *repos.RoomServerCurStateRepo
	UmsRepo      *repos.RoomServerUserMembershipRepo
	Fed          *FedProcessor
	Cfg          *config.Dendrite
	Idg          *uid.UidGenerator
	RpcClient    *common.RpcClient
	Federation   *fed.Federation
	evtProcGauge mon.LabeledGauge
	slot         uint32
	chanSize     int
	inputChan    []chan *InputContext
}

func (r *EventsProcessor) SetFed(fed *FedProcessor) {
	r.Fed = fed
}

func (r *EventsProcessor) NewMonitor() {
	monitor := mon.GetInstance()
	r.evtProcGauge = monitor.NewLabeledGauge("room_event_process_duration_millisecond", []string{"query", "addition", "room_id"})
}

func (r *EventsProcessor) Start() {
	r.chanSize = 1024
	r.slot = 128
	r.inputChan = make([]chan *InputContext, r.slot)
	for i := uint32(0); i < r.slot; i++ {
		r.inputChan[i] = make(chan *InputContext, r.chanSize)
		go r.startWorker(r.inputChan[i])
	}
}

func (r *EventsProcessor) startWorker(channel chan *InputContext) {
	for input := range channel {
		r.handleInput(input)
	}
}

func (r *EventsProcessor) handleInput(input *InputContext) {
	n, err := r.processInput(input.ctx, input.input)
	input.result <- InputResult{
		Num:   n,
		Error: err,
	}
}

func (r *EventsProcessor) dispthInput(ctx context.Context, input *roomserverapi.RawEvent, result chan InputResult) {
	hash := common.CalcStringHashCode(input.RoomID)
	slot := hash%r.slot
	log.Infof("dispth input roomID:%s slot:%d", input.RoomID, slot)
	r.inputChan[slot] <- &InputContext{
		ctx:    ctx,
		input:  input,
		result: result,
	}
}

func (r *EventsProcessor) WriteOutputEvents(ctx context.Context, roomID string, updates []roomserverapi.OutputEvent) error {
	updateEvents := []string{}
	for _, event := range updates {
		updateEvents = append(updateEvents, event.NewRoomEvent.Event.EventID)
	}
	log.Infof("before WriteOutputEvents roomID:%s updates:%+v", roomID, updateEvents)
	bs := time.Now().UnixNano()/1000000
	roomIDData := []byte(roomID)
	for i := range updates {
		err := func() error {
			span, _ := common.StartSpanFromContext(ctx, r.Cfg.Kafka.Producer.OutputRoomEvent.Name)
			defer span.Finish()
			common.ExportMetricsBeforeSending(span, r.Cfg.Kafka.Producer.OutputRoomEvent.Name,
				r.Cfg.Kafka.Producer.OutputRoomEvent.Underlying)
			return common.GetTransportMultiplexer().SendAndRecvWithRetry(
				r.Cfg.Kafka.Producer.OutputRoomEvent.Underlying,
				r.Cfg.Kafka.Producer.OutputRoomEvent.Name,
				&core.TransportPubMsg{
					Keys:    roomIDData,
					Obj:     updates[i],
					Inst:    r.Cfg.Kafka.Producer.OutputRoomEvent.Inst,
					Headers: common.InjectSpanToHeaderForSending(span),
				})
		}()
		if err != nil {
			log.Errorf("WriteOutputEvents roomID:%s event:%s err:%v", roomID, updates[i].NewRoomEvent.Event.EventID, err)
			return err
		}
	}
	spend := time.Now().UnixNano()/ 1000000 - bs
	log.Infof("after WriteOutputEvents spend:%d roomID:%s updates:%+v", spend, roomID, updateEvents)
	return nil
}

func (r *EventsProcessor) WriteFedEvents(ctx context.Context, roomID string, update *gomatrixserverlib.Event) error {
	roomIDData := []byte(roomID)

	//bytes, _ := update.MarshalJSON()
	//log.Infof("processRoomEvent WriteFedEvents room:%s topic:%s event:%s", roomID, r.Cfg.Kafka.Producer.OutputRoomFedEvent.Topic, string(bytes))
	span, _ := common.StartSpanFromContext(ctx, r.Cfg.Kafka.Producer.OutputRoomFedEvent.Name)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, r.Cfg.Kafka.Producer.OutputRoomFedEvent.Name,
		r.Cfg.Kafka.Producer.OutputRoomFedEvent.Underlying)
	return common.GetTransportMultiplexer().SendAndRecvWithRetry(
		r.Cfg.Kafka.Producer.OutputRoomFedEvent.Underlying,
		r.Cfg.Kafka.Producer.OutputRoomFedEvent.Name,
		&core.TransportPubMsg{
			Keys:    roomIDData,
			Obj:     update,
			Inst:    r.Cfg.Kafka.Producer.OutputRoomFedEvent.Inst,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}

func (r *EventsProcessor) InputRoomEvents(
	ctx context.Context,
	input *roomserverapi.RawEvent,
) (int, error) {
	start := time.Now()
	//n, err := r.processInput(ctx, input)
	result := make(chan InputResult)
	if input.TxnID != nil {
		log.Infof("InputRoomEvents dispatch input txnId:%s", input.TxnID.TransactionID)
	}else{
		log.Infof("InputRoomEvents dispatch input")
	}
	for _, event := range input.BulkEvents.Events {
		log.Infof("begin dispatch input room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID(), event.EventID(), event.DomainOffset(), event.OriginServerTS(), event.Depth())
	}
	r.dispthInput(ctx, input, result)
	inputResult := <-result
	for _, event := range input.BulkEvents.Events {
		log.Infof("after dispatch input room_id:%s event_id:%s domain_offset:%d origin_server_ts:%d depth:%d",
			event.RoomID(), event.EventID(), event.DomainOffset(), event.OriginServerTS(), event.Depth())
	}
	if input.TxnID != nil {
		log.Infof("InputRoomEvents dispatch input txnId:%s response", input.TxnID.TransactionID)
	}else{
		log.Infof("InputRoomEvents dispatch input response")
	}
	// monitor report
	duration := float64(time.Since(start)) / float64(time.Microsecond)
	if len(input.Query) >= 2 {
		r.evtProcGauge.WithLabelValues(input.Query[0], input.Query[1], input.RoomID).Set(float64(duration))
	}
	//return n, err
	return inputResult.Num, inputResult.Error
}

func (r *EventsProcessor) processInput(
	ctx context.Context,
	input *roomserverapi.RawEvent,
) (int, error) {
	n := 0
	for _, ev := range input.BulkEvents.Events {
		bytes, _ := ev.MarshalJSON()
		now := time.Now()
		log.Infof("processRoomEvent recv %s type %s kind %d use %v content:%s", ev.EventID(), ev.Type(), input.Kind, time.Now().Sub(now), bytes)
		if err := r.processRoomEvent(ctx, ev, input.Kind, input.Trust, input.TxnID, input.BulkEvents.SvrName); err != nil {
			log.Errorf("EventsProcessor.InputRoomEvents processRoomEvent process event %s err %v", string(bytes), err)
			return n, err
		}
		n++
		//log.Infof("processRoomEvent process %s type %s use %v content:%s", ev.EventID(), ev.Type(), time.Now().Sub(now), bytes)
	}
	return n, nil
}

func (r *EventsProcessor) processRoomEvent(
	ctx context.Context,
	event gomatrixserverlib.Event,
	kind int,
	trustedJSON bool,
	txnID *roomservertypes.TransactionID,
	svrName string,
) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("processRoomEvent roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, event)
	if kind == roomserverapi.KindNew {
		return r.processRoomNewEvent(ctx, event, trustedJSON, txnID, svrName)
	} else if kind == roomserverapi.KindBackfill {
		return r.processBackfill(ctx, event, txnID, svrName)
	} else {
		log.Infof("processRoomEvent unknown type: %d", kind)
	}
	return nil
}

func (r *EventsProcessor) processRoomNewEvent(
	ctx context.Context,
	event gomatrixserverlib.Event,
	trustedJSON bool,
	txnID *roomservertypes.TransactionID,
	svrName string,
) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("processRoomNewEvent roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, event)
	lockProcess := common.IsStateEv(&event)
	if lockProcess {
		lockKey := types.LOCK_ROOMSTATE_PREFIX + event.RoomID()
		start := time.Now().UnixNano() / 1000
		token, err := r.Repo.GetCache().Lock(lockKey, adapter.GetDistLockCfg().LockRoomState.Timeout, adapter.GetDistLockCfg().LockRoomState.Wait)
		if err != nil {
			log.Errorf("dist lock key:%s token:%s err:%v", lockKey, token, err)
		} else {
			log.Infof("dist lock key:%s spend:%d us", lockKey, time.Now().UnixNano()/1000-start)
		}
		defer func(start int64) {
			err := r.Repo.GetCache().UnLock(lockKey, token, adapter.GetDistLockCfg().LockRoomState.Force)
			if err != nil {
				log.Errorf("dist unlock key:%s token:%s err:%v", lockKey, token, err)
			}
			log.Infof("dist unlock key:%s token:%s spend:%d us", lockKey, token, time.Now().UnixNano()/1000-start)
		}(start)
	}
	var rs *repos.RoomServerState
	var isDirect bool
	var err error
	if event.Type() == gomatrixserverlib.MRoomCreate {
		isDirect, err = common.IsCreatingDirectRoomEv(&event)
		if err != nil {
			log.Errorf("Failed to check direct room: %v\n", err)
			return err
		}
	} else {
		bs := time.Now().UnixNano() / 1000000
		rs = r.Repo.GetRoomState(ctx, event.RoomID())
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("processRoomNewEvent GetRoomState roomid:%s spend:%d ms", event.RoomID(), spend)
		if rs == nil {
			return errors.New("Room not found")
		}
		isDirect = rs.IsDirect()
	}

	if isDirect &&
		(event.Type() == gomatrixserverlib.MRoomCreate || event.Type() == gomatrixserverlib.MRoomMember) {
		bs := time.Now().UnixNano() / 1000000
		evs, succeed, err := r.processDirectRoomCreateOrMemberEvent(ctx, event)
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("processRoomNewEvent processDirectRoomCreateOrMemberEvent roomid:%s spend:%d ms", event.RoomID(), spend)
		if err != nil {
			log.Errorf("Failed to process direct room state event: %v", err)
			return err
		}
		if !succeed {
			log.Errorln("RCS Server failed to process")
			return errors.New("RCS Server error")
		}
		for _, ev := range evs {
			bs := time.Now().UnixNano() / 1000000
			ev, err = r.processFedInvite(ctx, ev, rs)
			spend := time.Now().UnixNano()/1000000 - bs
			log.Infof("processRoomNewEvent processFedInvite roomid:%s spend:%d ms", event.RoomID(), spend)
			if err != nil {
				return err
			}
			bs = time.Now().UnixNano() / 1000000
			err := r.processNew(ctx, ev, txnID, svrName, trustedJSON, rs)
			spend = time.Now().UnixNano()/1000000 - bs
			log.Infof("processRoomNewEvent processNew roomid:%s spend:%d ms", event.RoomID(), spend)
			if err != nil {
				log.Errorw("Failed to process event", log.KeysAndValues{"error", err, "event", ev})
				return err
			}
		}
	} else {
		bs := time.Now().UnixNano() / 1000000
		event, err = r.processFedInvite(ctx, event, rs)
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("processRoomNewEvent processFedInvite roomid:%s spend:%d ms", event.RoomID(), spend)
		if err != nil {
			return err
		}
		bs = time.Now().UnixNano() / 1000000
		err = r.processNew(ctx, event, txnID, svrName, trustedJSON, rs)
		spend = time.Now().UnixNano()/1000000 - bs
		log.Infof("processRoomNewEvent processNew roomid:%s spend:%d ms", event.RoomID(), spend)
		return err
	}
	return nil
}

func (r *EventsProcessor) processDirectRoomCreateOrMemberEvent(
	ctx context.Context,
	event gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, bool, error) {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("processDirectRoomCreateOrMemberEvent roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, event)
	inCont := types.RCSInputEventContent{
		Event: event,
	}
	bytes, err := json.Marshal(inCont)
	if err != nil {
		log.Errorf("Failed to marshal RCSInputEventContent: %v\n", err)
		return nil, false, err
	}
	data, err := r.RpcClient.Request(types.RCSEventTopicDef, bytes, 35000)
	if err != nil {
		log.Errorf("Failed to call rcs server: %v\n", err)
		return nil, false, err
	}
	var cont types.RCSOutputEventContent
	err = json.Unmarshal(data, &cont)
	if err != nil {
		log.Errorf("Failed to unmarshal RCSOutputEventContent: %v\n", err)
		return nil, false, err
	}
	if !cont.Succeed {
		log.Errorln("RCS server failed to process")
		return nil, false, nil
	}
	return cont.Events, true, nil
}

func (r *EventsProcessor) processFedInvite(ctx context.Context, event gomatrixserverlib.Event, rs *repos.RoomServerState) (gomatrixserverlib.Event, error) {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("processFedInvite roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, event)
	if event.Type() != gomatrixserverlib.MRoomMember || event.StateKey() == nil {
		return event, nil
	}
	sender := event.Sender()
	senderDomain, _ := common.DomainFromID(sender)
	if !common.CheckValidDomain(senderDomain, r.Cfg.Matrix.ServerName) {
		return event, nil
	}
	content := map[string]interface{}{}
	err := json.Unmarshal(event.Content(), &content)
	if err != nil {
		return event, err
	}
	val, ok := content["membership"]
	if !ok {
		return event, nil
	}
	membership, ok := val.(string)
	if !ok {
		return event, nil
	}
	if membership != "invite" {
		return event, nil
	}
	if rs == nil {
		return event, errors.New("Room not found")
	}
	userID := *event.StateKey()
	inviteeDomain, err := common.DomainFromID(userID)
	if err != nil {
		return event, errors.New("invitee Id must be in the form '@localpart:domain'")
	}

	if !common.CheckValidDomain(inviteeDomain, r.Cfg.Matrix.ServerName) {
		//TODO federation auto join
		type UnsingedInviteStates struct {
			States []gomatrixserverlib.Event `json:"invite_room_state"`
		}
		states := rs.GetAllState()
		inviteStates := UnsingedInviteStates{States: states}
		inviteEv, err := event.SetUnsigned(inviteStates, false)
		if err != nil {
			return event, err
		}
		resp, err := r.Federation.SendInvite(inviteeDomain, inviteEv)
		if resp.Code != 200 {
			log.Errorf("SendInvite error: %v", err)
			data, _ := resp.Encode()
			return event, errors.New("SendInvite error: " + string(data))
		}
		signedEvent := resp.Event
		if err != nil {
			return event, err
		}

		event = signedEvent
	}

	return event, nil
}

func (r *EventsProcessor) processNew(
	ctx context.Context,
	event gomatrixserverlib.Event,
	txnID *roomservertypes.TransactionID,
	svrName string,
	trustedJSON bool,
	rs *repos.RoomServerState,
) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("processNew roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, event)
	log.Debugf("------------------------processNew start")
	begin := time.Now()
	last := begin
	eventNID := event.EventNID()
	var roomNID int64
	var err error
	var preEv *gomatrixserverlib.Event
	if event.Type() == "m.room.create" {
		if rs != nil { // 防止重复create
			return nil
		}
		bs := time.Now().UnixNano() / 1000000
		roomNID, err = r.DB.AssignRoomNID(ctx, event.RoomID())
		spend := time.Now().UnixNano()/1000000 - bs
		log.Infof("processNew AssignRoomNID roomid:%s spend:%d ms", event.RoomID(), spend)
		if err != nil {
			return err
		}
	} else {
		if rs == nil {
			return errors.New("can't find room")
		}
		roomNID = rs.GetRoomNID()
		preEv, _ = rs.GetPreEvent(&event)
	}
	if trustedJSON == false {
		if err := gomatrixserverlib.Allowed(event, rs); err != nil {
			if err != nil {
				log.Infof("------------------------processNew auth fail,  err %v", err)
			}
			return err
		}
	}

	log.Debugf("------------------------processNew auth %v", time.Now().Sub(last))
	last = time.Now()
	//event.SetDepth(depth)
	bs = time.Now().UnixNano() / 1000000
	rs = r.Repo.OnEvent(ctx, &event, eventNID, rs)
	spend := time.Now().UnixNano()/1000000 - bs
	log.Infof("processNew r.Repo.OnEvent roomid:%s spend:%d ms", event.RoomID(), spend)
	bs = time.Now().UnixNano() / 1000000
	r.UmsRepo.OnEvent(ctx, &event, preEv)
	spend = time.Now().UnixNano()/1000000 - bs
	log.Infof("processNew r.UmsRepo.OnEvent roomid:%s spend:%d ms", event.RoomID(), spend)
	bs = time.Now().UnixNano() / 1000000
	err = r.updateRoomStateExt(ctx, &event, rs, true)
	spend = time.Now().UnixNano()/1000000 - bs
	log.Infof("processNew r.updateRoomStateExt roomid:%s spend:%d ms", event.RoomID(), spend)
	if err != nil {
		return err
	}
	//rs.AllocDomainOffset(&event)
	rs.SetRoomNID(roomNID)

	log.Debugf("------------------------processNew OnEvent %v", time.Now().Sub(last))
	last = time.Now()

	//有状态更新
	curSnap := rs.GetSnapId()
	if common.IsStateEv(&event) {
		state := rs.GetLastState()
		log.Infof("snap len state:%d, room_id:%s state:%v", len(state), event.RoomID(), state)
		bs = time.Now().UnixNano() / 1000000
		curSnap, err = r.DB.AddState(ctx, roomNID, state)
		spend = time.Now().UnixNano()/1000000 - bs
		log.Infof("processNew AddState roomid:%s spend:%d ms", event.RoomID(), spend)
		if err != nil { //落地当前token
			log.Warnf("EventProcessor.processNew db AddState error:%v", err)
			return err
		}

		rs.SetSnapId(curSnap)
	}

	log.Debugf("------------------------processNew set-state %v", time.Now().Sub(last))
	last = time.Now()

	err = r.postProcessNew(ctx, &event, preEv, eventNID, roomNID, rs, svrName, txnID, curSnap)
	log.Infof("------------------------processRoomEvent roomid:%s ev-graph-update %v ", event.RoomID(), time.Now().Sub(last))
	return err
}

func (r *EventsProcessor) processBackfill(
	ctx context.Context,
	event gomatrixserverlib.Event,
	txnID *roomservertypes.TransactionID,
	svrName string,
) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("processBackfill roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, event)
	log.Debugf("------------------------processRoomEvent start")
	eventNID := event.EventNID()
	var err error
	rs := r.Repo.GetRoomState(ctx, event.RoomID())
	if rs == nil {
		return errors.New("can't find room")
	}
	preEv, isState := rs.GetPreEvent(&event)
	if preEv != nil && isState == true && preEv.EventID() == event.EventID() {
		return nil
	}
	roomNID := rs.GetRoomNID()
	curSnap := rs.GetSnapId()
	// Update the extremities of the event graph for the room
	err = r.postProcessBackfill(ctx, &event, preEv, eventNID, roomNID, rs, svrName, txnID, curSnap)

	return err
}

func (r *EventsProcessor) updateRoomStateExt(ctx context.Context, event *gomatrixserverlib.Event, rs *repos.RoomServerState, all bool) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("updateRoomStateExt roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, *event)
	domain, res := common.DomainFromID(event.Sender())
	if res != nil {
		return errors.New("event send invalid doamin")
	}
	lockKey := types.LOCK_ROOMSTATE_EXT_PREFIX + event.RoomID()
	bs = time.Now().UnixNano() / 1000
	token, err := r.Repo.GetCache().Lock(lockKey, adapter.GetDistLockCfg().LockRoomStateExt.Timeout, adapter.GetDistLockCfg().LockRoomStateExt.Wait)
	if err != nil {
		log.Errorf("dist lock key:%s token:%s err:%v", lockKey, token, err)
		return err
	} else {
		spend := time.Now().UnixNano()/1000 - bs
		log.Infof("dist lock key:%s token:%s spend:%d us", lockKey, token, spend)
	}
	defer func(start int64) {
		err := r.Repo.GetCache().UnLock(lockKey, token, adapter.GetDistLockCfg().LockRoomStateExt.Force)
		if err != nil {
			log.Errorf("dist unlock key:%s token:%s err:%v", lockKey, token, err)
		}
		log.Infof("dist unlock key:%s token:%s spend:%d us", lockKey, token, time.Now().UnixNano()/1000-start)
	}(time.Now().UnixNano() / 1000)

	updateExt := make(map[string]interface{})

	roomStateExt, err := r.Repo.GetRoomStateExt(ctx, event.RoomID())
	if err != nil {
		if err == sql.ErrNoRows && event.Type() == "m.room.create" {
			roomStateExt = &types.RoomStateExt{
				Domains: make(map[string]int64),
			}
		} else {
			return err
		}
	}
	depth := roomStateExt.Depth
	if event.Depth() == 0 {
		event.SetDepth(depth + 1)
		updateExt["depth"] = depth + 1
		roomStateExt.Depth = depth + 1
	} else {
		if depth < event.Depth() {
			updateExt["depth"] = event.Depth()
			roomStateExt.Depth = event.Depth()
		}
	}
	domainOffset := int64(0)
	if v, ok := roomStateExt.Domains[domain]; ok {
		domainOffset = v
	}
	if event.DomainOffset() == 0 {
		event.SetDomainOffset(domainOffset + 1)
		updateExt["domain:"+domain] = domainOffset + 1
		roomStateExt.Domains[domain] = domainOffset + 1
	} else {
		if domainOffset < event.DomainOffset() {
			updateExt["domain:"+domain] = event.DomainOffset()
			roomStateExt.Domains[domain] = event.DomainOffset()
		}
	}
	if all {
		if common.IsStateEv(event) {
			updateExt["pre_state_id"] = roomStateExt.LastStateId
			updateExt["last_state_id"] = event.EventID()
			updateExt["last_msg_id"] = event.EventID()
			roomStateExt.PreStateId = roomStateExt.LastStateId
			roomStateExt.LastStateId = event.EventID()
			roomStateExt.LastMsgId = event.EventID()
		} else {
			updateExt["pre_msg_id"] = roomStateExt.LastMsgId
			updateExt["last_msg_id"] = event.EventID()
			roomStateExt.PreMsgId = roomStateExt.LastMsgId
			roomStateExt.LastMsgId = event.EventID()
		}
		rs.UpdateMsgExt(roomStateExt.PreStateId, roomStateExt.LastStateId, roomStateExt.PreMsgId, roomStateExt.LastMsgId)
	}
	updateExt["has_update"] = rs.HasUpdate()
	outEventOffset := r.getOutEventOffset(roomStateExt.OutEventOffset)
	updateExt["out_event_offset"] = outEventOffset
	rs.UpdateOutEventOffset(outEventOffset)
	rs.UpdateDepth(roomStateExt.Depth)
	rs.UpdateDomainExt(roomStateExt.Domains)
	if err = r.Repo.UpdateRoomStateExt(event.RoomID(), updateExt); err != nil {
		log.Warnf("update room:%s roomstate ext err:%v", event.RoomID(), err.Error())
		return err
	}
	return nil
}

func (r *EventsProcessor) getOutEventOffset(last int64) (out int64) {
	for {
		out, _ = r.Idg.Next()
		if out > last {
			return
		}
	}
	return
}

func (r *EventsProcessor) postProcessNew(
	ctx context.Context,
	event, pre *gomatrixserverlib.Event,
	eventNID int64,
	roomNID int64,
	rs *repos.RoomServerState,
	sendServer string,
	transactionID *roomservertypes.TransactionID,
	curSnap int64,
) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("postProcessNew roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, *event)
	last := time.Now()
	updates, err := r.updateMemberShip(ctx, roomNID, eventNID, *event, pre)
	if err != nil {
		log.Errorf("EventProcessor.postProcessNew update membership error, eventID: %s, err: %v", event.EventID(), err)
		return err
	}
	log.Infof("postProcessNew update membership roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	last = time.Now()
	sendDomain, _ := common.DomainFromID(event.Sender())
	// rs set over, flush cache
	if common.IsStateEv(event) {
		r.Repo.FlushRoomState(rs)
		log.Infof("postProcessNew FlushRoomState roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	}
	if common.CheckValidDomain(sendDomain, r.Cfg.Matrix.ServerName) && event.OriginServerTS() == 0 {
		event.SetOriginServerTS(gomatrixserverlib.AsTimestamp(time.Now()))
	}
	ore := r.buildOutputRoomEvent(transactionID, sendServer, rs, *event, pre)

	updates = append(updates, roomserverapi.OutputEvent{
		Type:         roomserverapi.OutputTypeNewRoomEvent,
		NewRoomEvent: &ore,
	})
	last = time.Now()
	err = r.DB.SetLatestEvents(ctx, roomNID, eventNID, curSnap, rs.GetDepth())
	log.Infof("postProcessNew SetLatestEvents roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}

	refId, refHash := rs.GetRefs(event)
	last = time.Now()
	err = r.DB.StoreEvent(ctx, event, roomNID, curSnap, refId, refHash)
	log.Infof("postProcessNew StoreEvent roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}

	last = time.Now()
	//if (rs.HasUpdate && common.IsStateEv(event) == true) || common.IsStateEv(event) == false {
	if err := r.WriteOutputEvents(ctx, event.RoomID(), updates); err != nil {
		return err
	}
	//}

	log.Infof("postProcessNew roomid:%s WriteOutputEvents kafka spend %v", event.RoomID(), time.Now().Sub(last))
	last = time.Now()
	err = r.DB.SaveRoomDomainsOffset(ctx, roomNID, sendDomain, rs.GetLastDomainOffset(event, sendDomain))
	log.Infof("postProcessNew  roomid:%s SaveRoomDomainsOffset spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}
	last = time.Now()
	if common.CheckValidDomain(sendDomain, r.Cfg.Matrix.ServerName) == true {
		domains := rs.GetDomainTlMap()
		hasFed := false
		domains.Range(func(key, value interface{}) bool {
			domain := key.(string)
			if domain != sendServer {
				hasFed = true
				return false
			}
			return true
		})

		if hasFed {
			r.WriteFedEvents(ctx, event.RoomID(), event)
		}
	} else {
		r.Fed.OnRoomEvent(ctx, event)
	}

	log.Infof("postProcessNew, roomid: %s,  setlast & flush %v", event.RoomID(), time.Now().Sub(last))

	return nil
}

func (r *EventsProcessor) postProcessBackfill(
	ctx context.Context,
	event, pre *gomatrixserverlib.Event,
	evnid int64,
	roomNID int64,
	rs *repos.RoomServerState,
	sendServer string,
	transactionID *roomservertypes.TransactionID,
	curSnap int64,
) error {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64, ev gomatrixserverlib.Event){
		spend := time.Now().UnixNano() / 1000000 - bs
		log.Infof("postProcessBackfill roomID:%s eventId:%s type:%s spend:%d", ev.RoomID(), ev.EventID(), ev.Type(), spend)
	}(bs, *event)
	last := time.Now()

	refId, refHash := rs.GetRefs(event)

	log.Debugf("============postProcessBackfill store previous %v", time.Now().Sub(last))
	last = time.Now()

	var updates []roomserverapi.OutputEvent

	err := r.updateRoomStateExt(ctx, event, rs, false)
	log.Infof("postProcessBackfill updateRoomStateExt roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}

	ore := roomserverapi.OutputNewRoomEvent{
		TransactionID: transactionID,
		SendAsServer:  sendServer,
	}

	ore.Event.InitFromEvent(event)
	outEventOffset := rs.GetOutEventOffset()
	if outEventOffset <= 0 {
		outEventOffset, _ = r.Idg.Next()
	}
	ore.Event.EventOffset = outEventOffset
	output := roomserverapi.OutputEvent{
		Type:         roomserverapi.OutputBackfillRoomEvent,
		NewRoomEvent: &ore,
	}
	updates = append(updates, output)

	//if err := r.DB.SetLatestEvents(roomNID, evnid, curSnap, event.Depth()); err != nil {
	//	return err
	//}
	last = time.Now()
	err = r.DB.StoreEvent(ctx, event, roomNID, curSnap, refId, refHash)
	log.Infof("postProcessBackfill StoreEvent roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}

	//if (rs.HasUpdate && common.IsStateEv(event) == true) || common.IsStateEv(event) == false {
	last = time.Now()
	err = r.WriteOutputEvents(ctx, event.RoomID(), updates)
	log.Infof("postProcessBackfill WriteOutputEvents roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}
	//}

	sendDomain, _ := common.DomainFromID(event.Sender())
	last = time.Now()
	err = r.DB.SaveRoomDomainsOffset(ctx, roomNID, sendDomain, rs.GetLastDomainOffset(event, sendDomain))
	log.Infof("============postProcessBackfill  SaveRoomDomainsOffset roomid:%s spend %v", event.RoomID(), time.Now().Sub(last))
	if err != nil {
		return err
	}

	return nil
}

func (r *EventsProcessor) buildOutputRoomEvent(
	transactionID *roomservertypes.TransactionID,
	sendServer string,
	rs *repos.RoomServerState,
	event gomatrixserverlib.Event,
	pre *gomatrixserverlib.Event,
) roomserverapi.OutputNewRoomEvent {
	ore := roomserverapi.OutputNewRoomEvent{
		TransactionID: transactionID,
		SendAsServer:  sendServer,
	}

	ore.Event.InitFromEvent(&event)

	if common.IsStateEv(&event) {
		ore.AddsStateEventIDs = []string{event.EventID()}
	}

	if pre != nil {
		ore.RemovesStateEventIDs = []string{pre.EventID()}
	}

	rs.GetJoinMap().Range(func(key, value interface{}) bool {
		ore.Joined = append(ore.Joined, key.(string))
		return true
	})
	outEventOffset := rs.GetOutEventOffset()
	if outEventOffset <= 0 {
		outEventOffset, _ = r.Idg.Next()
	}
	ore.Event.EventOffset = outEventOffset
	log.Infof("buildout eventId:%s eventOffset:%s room_id:%s", ore.Event.EventID, ore.Event.EventOffset, ore.Event.RoomID)
	return ore
}

func (r *EventsProcessor) updateMemberShip(
	ctx context.Context,
	roomNID, eventNID int64,
	event gomatrixserverlib.Event,
	pre *gomatrixserverlib.Event,
) ([]roomserverapi.OutputEvent, error) {
	var updates []roomserverapi.OutputEvent
	if event.Type() == "m.room.member" {
		old := ""
		if pre != nil {
			old, _ = pre.Membership()
		}
		new, _ := event.Membership()

		var err error
		log.Debugf("============postProcessNew pre:%s new:%s nid:%d", old, new, eventNID)
		if old != new || new == "join" { //donothing
			switch new {
			case "invite":
				updates, err = r.updateToInviteMembership(ctx, roomNID, &event, eventNID, updates, old, event.RoomID())
			case "join":
				updates, err = r.updateToJoinMembership(ctx, roomNID, &event, eventNID, updates, old, event.RoomID())
			case "leave", "ban", "kick":
				updates, err = r.updateToLeaveMembership(ctx, roomNID, &event, eventNID, updates, old, event.RoomID())
			case "forget":
				updates, err = r.updateToForgetMembership(ctx, roomNID, &event, eventNID, updates, old, event.RoomID())
			}
		}

		if err != nil {
			return nil, err
		}
	}
	return updates, nil
}

func (r *EventsProcessor) updateToInviteMembership(
	ctx context.Context, roomNID int64,
	addEvent *gomatrixserverlib.Event, evnid int64,
	outputEvents []roomserverapi.OutputEvent, pre, roomID string,
) ([]roomserverapi.OutputEvent, error) {
	err := r.DB.SetToInvite(ctx, roomNID, *addEvent.StateKey(),
		addEvent.Sender(), addEvent.EventID(), addEvent.JSON(),
		int64(evnid), pre, roomID)
	if err != nil {
		return nil, err
	}

	return outputEvents, nil
}

func (r *EventsProcessor) updateToJoinMembership(
	ctx context.Context, roomNID int64,
	addEvent *gomatrixserverlib.Event, evnid int64,
	updates []roomserverapi.OutputEvent, pre, roomID string,
) ([]roomserverapi.OutputEvent, error) {
	err := r.DB.SetToJoin(ctx, roomNID, *addEvent.StateKey(),
		addEvent.Sender(), int64(evnid), pre, roomID)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

func (r *EventsProcessor) updateToLeaveMembership(
	ctx context.Context, roomNID int64,
	addEvent *gomatrixserverlib.Event, evnid int64,
	updates []roomserverapi.OutputEvent, pre, roomID string,
) ([]roomserverapi.OutputEvent, error) {
	err := r.DB.SetToLeave(ctx, roomNID, *addEvent.StateKey(),
		addEvent.Sender(), int64(evnid), pre, roomID)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

func (r *EventsProcessor) updateToForgetMembership(
	ctx context.Context, roomNID int64,
	addEvent *gomatrixserverlib.Event, evnid int64,
	updates []roomserverapi.OutputEvent, pre, roomID string,
) ([]roomserverapi.OutputEvent, error) {
	err := r.DB.SetToLeave(ctx, roomNID, *addEvent.StateKey(),
		addEvent.Sender(), int64(evnid), pre, roomID)
	if err != nil {
		return nil, err
	}
	err = r.DB.SetToForget(ctx, roomNID, *addEvent.StateKey(), int64(evnid), pre, roomID)
	if err != nil {
		return nil, err
	}

	return updates, nil
}
