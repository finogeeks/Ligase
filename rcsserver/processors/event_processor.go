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
	"errors"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	fsm "github.com/smallnest/gofsm"
)

const (
	ST_INIT   = "init"
	ST_INVITE = "invite"
	ST_JOIN   = "join"
	ST_LEAVE  = "leave"
)

const (
	OP_JOIN   = "join"
	OP_INVITE = "invite"
	OP_LEAVE  = "leave"
)

const (
	ACT_DEFAULT = "update database"
	ACT_IGNORE  = "ignore"
	ACT_JOIN    = "join"
	ACT_INVITE  = "invite"
)

type RCSDelegate struct {
	db model.RCSServerDatabase
}

func (d *RCSDelegate) HandleEvent(action, fromState, toState string, args []interface{}) error {
	ctx := args[0].(context.Context)
	sender := args[1].(string)
	stateKey := args[2].(string)
	stateKeyIsCreator := args[3].(bool)
	f := args[4].(*types.RCSFriendship)
	ev := args[5].(*gomatrixserverlib.Event)
	log.Infow("rcsserver=====================RCSDelegate.HandleEvent", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	switch action {
	case ACT_IGNORE:
		return nil
	case ACT_JOIN:
		return d.handleJoinOperation(ctx, toState, stateKey, stateKeyIsCreator, f, ev)
	case ACT_INVITE:
		return d.handleInviteOperation(ctx, toState, sender, stateKey, stateKeyIsCreator, f, ev)
	case ACT_DEFAULT:
		return d.handleNormalOperation(ctx, toState, stateKey, stateKeyIsCreator, f, ev)
	}
	return nil
}

func (d *RCSDelegate) handleJoinOperation(
	ctx context.Context, toState, stateKey string, isCreator bool, f *types.RCSFriendship, ev *gomatrixserverlib.Event,
) error {
	log.Infow("rcsserver=====================RCSDelegate.handleJoinOperation", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	if isCreator {
		return d.db.UpdateFriendshipByRoomID(
			ctx, f.ID, f.RoomID, stateKey, f.ToFcID, toState, f.ToFcIDState,
			isBot(stateKey), isBot(f.ToFcID), remark(stateKey, ev), f.ToFcIDRemark,
			true, f.ToFcIDOnceJoined, getDomain(stateKey), getDomain(f.ToFcID), ev.EventID(),
		)
	} else {
		return d.db.UpdateFriendshipByRoomID(
			ctx, f.ID, f.RoomID, f.FcID, stateKey, f.FcIDState, toState,
			isBot(f.FcID), isBot(stateKey), f.FcIDRemark, remark(stateKey, ev),
			f.FcIDOnceJoined, true, getDomain(f.FcID), getDomain(stateKey), ev.EventID())
	}
}

func (d *RCSDelegate) handleInviteOperation(
	ctx context.Context, toState, sender, stateKey string, isCreator bool, f *types.RCSFriendship, ev *gomatrixserverlib.Event,
) error {
	log.Infow("rcsserver=====================RCSDelegate.handleInviteOperation", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	var ID string
	if sender != stateKey {
		ID = getID(sender, stateKey)
	} else {
		ID = f.ID
	}

	if isCreator {
		return d.db.UpdateFriendshipByRoomID(
			ctx, ID, f.RoomID, stateKey, f.ToFcID, toState, f.ToFcIDState,
			isBot(stateKey), isBot(f.ToFcID), remark(stateKey, ev), f.ToFcIDRemark,
			f.FcIDOnceJoined, f.ToFcIDOnceJoined, getDomain(stateKey), getDomain(f.ToFcID), ev.EventID(),
		)
	} else {
		return d.db.UpdateFriendshipByRoomID(
			ctx, ID, f.RoomID, f.FcID, stateKey, f.FcIDState, toState,
			isBot(f.FcID), isBot(stateKey), f.FcIDRemark, remark(stateKey, ev),
			f.FcIDOnceJoined, f.ToFcIDOnceJoined, getDomain(f.FcID), getDomain(stateKey), ev.EventID())
	}
}

func (d *RCSDelegate) handleNormalOperation(
	ctx context.Context, toState, stateKey string, isCreator bool, f *types.RCSFriendship, ev *gomatrixserverlib.Event,
) error {
	log.Infow("rcsserver=====================RCSDelegate.handleNormalOperation", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	if isCreator {
		return d.db.UpdateFriendshipByRoomID(
			ctx, f.ID, f.RoomID, stateKey, f.ToFcID, toState, f.ToFcIDState,
			isBot(stateKey), isBot(f.ToFcID), remark(stateKey, ev), f.ToFcIDRemark,
			f.FcIDOnceJoined, f.ToFcIDOnceJoined, getDomain(stateKey), getDomain(f.ToFcID), ev.EventID(),
		)
	} else {
		return d.db.UpdateFriendshipByRoomID(
			ctx, f.ID, f.RoomID, f.FcID, stateKey, f.FcIDState, toState,
			isBot(f.FcID), isBot(stateKey), f.FcIDRemark, remark(stateKey, ev),
			f.FcIDOnceJoined, f.ToFcIDOnceJoined, getDomain(f.FcID), getDomain(stateKey), ev.EventID())
	}
}

func NewFSM(db model.RCSServerDatabase) *fsm.StateMachine {
	delegate := &RCSDelegate{
		db: db,
	}
	transitions := []fsm.Transition{
		{
			From:   ST_INIT,
			Event:  OP_JOIN,
			To:     ST_JOIN,
			Action: ACT_JOIN,
		},
		{
			From:   ST_INIT,
			Event:  OP_INVITE,
			To:     ST_INVITE,
			Action: ACT_INVITE,
		},
		{
			From:   ST_JOIN,
			Event:  OP_LEAVE,
			To:     ST_LEAVE,
			Action: ACT_DEFAULT,
		},
		{
			From:   ST_INVITE,
			Event:  OP_JOIN,
			To:     ST_JOIN,
			Action: ACT_JOIN,
		},
		{
			From:   ST_INVITE,
			Event:  OP_LEAVE,
			To:     ST_LEAVE,
			Action: ACT_DEFAULT,
		},
		{
			From:   ST_LEAVE,
			Event:  OP_INVITE,
			To:     ST_INVITE,
			Action: ACT_INVITE,
		},
		{
			From:   ST_JOIN,
			Event:  OP_JOIN,
			To:     ST_JOIN,
			Action: ACT_IGNORE,
		},
		{
			From:   ST_INVITE,
			Event:  OP_INVITE,
			To:     ST_INVITE,
			Action: ACT_IGNORE,
		},
		{
			From:   ST_LEAVE,
			Event:  OP_LEAVE,
			To:     ST_LEAVE,
			Action: ACT_IGNORE,
		},
	}
	return fsm.NewStateMachine(delegate, transitions...)
}

type EventProcessor struct {
	cfg *config.Dendrite
	idg *uid.UidGenerator
	db  model.RCSServerDatabase
	sm  *fsm.StateMachine
}

func NewEventProcessor(cfg *config.Dendrite, idg *uid.UidGenerator, db model.RCSServerDatabase) *EventProcessor {
	return &EventProcessor{
		cfg: cfg,
		idg: idg,
		db:  db,
		sm:  NewFSM(db),
	}
}

func (p *EventProcessor) HandleCreate(
	ctx context.Context, ev *gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, error) {
	log.Infow("rcsserver=====================EventProcessor.HandleCreate", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	// Update is_bot, remark, domain in membership event.
	// Set primary key value with eventID temporary.
	err := p.db.InsertFriendship(
		ctx, ev.EventID(), ev.RoomID(), ev.Sender(), "", ST_INIT, ST_INIT, false, false, "", "", false, false, "", "", ev.EventID())
	if err != nil {
		log.Errorf("Failed to insert friendship: %v\n", err)
		return nil, err
	}
	return []gomatrixserverlib.Event{*ev}, nil
}

func (p *EventProcessor) HandleMembership(
	ctx context.Context, ev *gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, error) {
	log.Infow("rcsserver=====================EventProcessor.HandleMembership", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	m, err := ev.Membership()
	if err != nil {
		log.Errorf("Failed to check membership: %v\n", err)
		return nil, err
	}
	if m == "invite" {
		return p.handleInvite(ctx, ev)
	}
	return p.handleNormalMembership(ctx, ev)
}

func (p *EventProcessor) handleInvite(
	ctx context.Context, ev *gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, error) {
	log.Infow("rcsserver=====================EventProcessor.handleInvite", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	if ev.StateKey() == nil {
		log.Errorf("State key is nil")
		return nil, errors.New("State key is nil")
	}
	domain, _ := common.DomainFromID(ev.Sender())
	if common.CheckValidDomain(domain, p.cfg.Matrix.ServerName) {
		return p.handleLocalInvite(ctx, ev)
	}
	return p.handleFedInvite(ctx, ev)
}

func (p *EventProcessor) handleFedInvite(
	ctx context.Context, ev *gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, error) {
	log.Infow("rcsserver=====================EventProcessor.handleFedInvite", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	fcID1 := ev.Sender()
	toFcID1 := *ev.StateKey()
	room1 := ev.RoomID()
	fcID2 := toFcID1
	toFcID2 := fcID1
	existRoom2 := true
	f2, err := p.db.GetFriendshipByFcIDAndToFcID(ctx, fcID2, toFcID2)
	if err != nil {
		if p.db.NotFound(err) {
			existRoom2 = false
		} else {
			log.Errorf("Failed to get friendship: %v\n", err)
			return nil, err
		}
	}
	if existRoom2 && f2.RoomID != room1 {
		var evs []gomatrixserverlib.Event
		room2 := f2.RoomID
		if room1 < room2 {
			log.Infow("rcsserver=====================EventProcessor.handleFedInvite, room1 < room2",
				log.KeysAndValues{"room1", room1, "room2", room2})
			// Select room1 from other domain.
			if err := p.db.DeleteFriendshipByRoomID(ctx, room2); err != nil {
				log.Errorf("Failed to delete friendship: %v\n", err)
				return nil, err
			}

			f1, err := p.db.GetFriendshipByRoomID(ctx, room1)
			if err != nil {
				log.Errorf("Failed to get friendship: %v\n", err)
				return nil, err
			}
			if err := p.db.UpdateFriendshipByRoomID(
				ctx, getID(fcID1, toFcID1), room1, fcID1, toFcID1, f1.FcIDState, ST_JOIN,
				isBot(fcID1), isBot(toFcID1), f1.FcIDRemark, remark(toFcID1, ev),
				true, true, getDomain(fcID1), getDomain(toFcID1), ev.EventID()); err != nil {
				log.Errorf("Failed to update friendship: %v\n", err)
				return nil, err
			}
			e, err := p.buildEvent(fcID2, room2, fcID2, "leave")
			if err != nil {
				log.Errorf("Failed to build event: %v\n", err)
				return nil, err
			}
			evs = append(evs, *e)
			e, err = p.buildEvent(fcID2, room1, fcID2, "join")
			if err != nil {
				log.Errorf("Failed to build event: %v\n", err)
				return nil, err
			}
			evs = append(evs, *e)
		} else {
			log.Infow("rcsserver=====================EventProcessor.handleFedInvite, room1 > room2",
				log.KeysAndValues{"room1", room1, "room2", room2})
			// Select room2 from local domain.
			if err := p.db.DeleteFriendshipByRoomID(ctx, room1); err != nil {
				log.Errorf("Failed to delete friendship: %v\n", err)
				return nil, err
			}
			e, err := p.buildEvent(fcID2, room1, fcID2, "leave")
			if err != nil {
				log.Errorf("Failed to build event: %v\n", err)
				return nil, err
			}
			evs = append(evs, *e)
		}
		return evs, nil
	}

	return p.handleNormalMembership(ctx, ev)
}

func (p *EventProcessor) buildEvent(
	sender, roomID, stateKey, membership string,
) (*gomatrixserverlib.Event, error) {
	builder := gomatrixserverlib.EventBuilder{
		Sender:   sender,
		RoomID:   roomID,
		Type:     gomatrixserverlib.MRoomMember,
		StateKey: &stateKey,
	}
	content := external.MemberContent{
		Membership: membership,
		Reason:     "",
	}
	if err := builder.SetContent(content); err != nil {
		log.Errorf("Failed to set content: %v\n", err)
		return nil, err
	}
	domainID, _ := common.DomainFromID(sender)
	e, err := common.BuildEvent(&builder, domainID, *p.cfg, p.idg)
	if err != nil {
		log.Errorf("Failed to build event: %v\n", err)
		return nil, err
	}
	log.Infow("rcsserver=====================EventProcessor.buildEvent", log.KeysAndValues{"e.EventID()", e.EventID()})
	return e, nil
}

func (p *EventProcessor) handleLocalInvite(
	ctx context.Context, ev *gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, error) {
	log.Infow("rcsserver=====================EventProcessor.handleLocalInvite", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	fcID1 := ev.Sender()
	toFcID1 := *ev.StateKey()
	room1 := ev.RoomID()
	fcID2 := toFcID1
	toFcID2 := fcID1
	existRoom2 := true
	f2, err := p.db.GetFriendshipByFcIDAndToFcID(ctx, fcID2, toFcID2)
	if err != nil {
		if p.db.NotFound(err) {
			existRoom2 = false
		} else {
			log.Errorf("Failed to get friendship: %v\n", err)
			return nil, err
		}
	}

	// Only apply glare when two rooms are different.
	if existRoom2 && f2.RoomID != room1 {
		var evs []gomatrixserverlib.Event
		room2 := f2.RoomID
		log.Infow("rcsserver=====================EventProcessor.handleLocalInvite, existRoom2",
			log.KeysAndValues{"room1", room1, "room2", room2})
		if err := p.db.DeleteFriendshipByRoomID(ctx, room1); err != nil {
			log.Errorf("Failed to delete friendship: %v\n", err)
			return nil, err
		}
		e, err := p.buildEvent(fcID1, room1, fcID1, "leave")
		if err != nil {
			log.Errorf("Failed to build event: %v\n", err)
			return nil, err
		}
		evs = append(evs, *e)
		if err := p.db.UpdateFriendshipByRoomID(
			ctx, f2.ID, room2, f2.FcID, f2.ToFcID, f2.FcIDState, ST_JOIN,
			isBot(f2.FcID), isBot(f2.ToFcID), f2.FcIDRemark, getUsername(f2.ToFcID),
			f2.FcIDOnceJoined, true, getDomain(f2.FcID), getDomain(f2.ToFcID), ev.EventID()); err != nil {
			log.Errorf("Failed to update friendship: %v\n", err)
			return nil, err
		}
		e, err = p.buildEvent(fcID1, room2, fcID1, "join")
		if err != nil {
			log.Errorf("Failed to build event: %v\n", err)
			return nil, err
		}
		evs = append(evs, *e)
		return evs, nil
	} else {
		// Check if sender has been invited, if so, then join room instead of sending invite.
		var evs []gomatrixserverlib.Event
		f1, err := p.db.GetFriendshipByRoomID(ctx, room1)
		if err != nil {
			log.Errorf("Failed to get friendship: %v\n", err)
			return nil, err
		}
		if fcID1 == f1.FcID {
			if f1.FcIDState == ST_INVITE {
				if err := p.db.UpdateFriendshipByRoomID(
					ctx, f1.ID, f1.RoomID, f1.FcID, f1.ToFcID, ST_JOIN, f1.ToFcIDState,
					isBot(f1.FcID), isBot(f1.ToFcID), f1.FcIDRemark, f1.ToFcIDRemark,
					true, f1.ToFcIDOnceJoined, getDomain(f1.FcID), getDomain(f1.ToFcID), ev.EventID()); err != nil {
					log.Errorf("Failed to update friendship: %v\n", err)
					return nil, err
				}
				e, err := p.buildEvent(fcID1, room1, fcID1, "join")
				if err != nil {
					log.Errorf("Failed to build event: %v\n", err)
					return nil, err
				}
				evs = append(evs, *e)
			}
		} else {
			if f1.ToFcIDState == ST_INVITE {
				if err := p.db.UpdateFriendshipByRoomID(
					ctx, f1.ID, f1.RoomID, f1.FcID, f1.ToFcID, f1.FcIDState, ST_JOIN,
					isBot(f1.FcID), isBot(f1.ToFcID), f1.FcIDRemark, f1.ToFcIDRemark,
					f1.FcIDOnceJoined, true, getDomain(f1.FcID), getDomain(f1.ToFcID), ev.EventID()); err != nil {
					log.Errorf("Failed to update friendship: %v\n", err)
					return nil, err
				}
				e, err := p.buildEvent(fcID1, room1, fcID1, "join")
				if err != nil {
					log.Errorf("Failed to build event: %v\n", err)
					return nil, err
				}
				evs = append(evs, *e)
			}
		}

		if len(evs) > 0 {
			log.Infow("rcsserver=====================EventProcessor.handleLocalInvite, change membership invite to join",
				log.KeysAndValues{"sender", fcID1, "state_key", toFcID1})
			return evs, nil
		}
	}

	return p.handleNormalMembership(ctx, ev)
}

func (p *EventProcessor) handleNormalMembership(
	ctx context.Context, ev *gomatrixserverlib.Event,
) ([]gomatrixserverlib.Event, error) {
	log.Infow("rcsserver=====================EventProcessor.handleNormalMembership", log.KeysAndValues{"ev.EventID()", ev.EventID()})
	sender := ev.Sender()
	if ev.StateKey() == nil {
		msg := "StateKey is nil"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	stateKey := *ev.StateKey()
	roomID := ev.RoomID()
	domain, _ := common.DomainFromID(sender)

	m, err := ev.Membership()
	if err != nil {
		log.Errorf("Failed to check membership: %v\n", err)
		return nil, err
	}
	op := m

	f, err := p.db.GetFriendshipByRoomID(ctx, roomID)
	if err != nil {
		// Special case: leave deleted room, only for remote domain.
		if p.db.NotFound(err) && op == OP_LEAVE && !common.CheckValidDomain(domain, p.cfg.Matrix.ServerName) {
			return []gomatrixserverlib.Event{*ev}, nil
		}
		log.Errorf("Failed to get friendship: %v\n", err)
		return nil, err
	}

	var senderSt string
	var stateKeySt string
	var senderIsCreator bool
	var stateKeyIsCreator bool
	if sender == f.FcID {
		senderSt = f.FcIDState
		senderIsCreator = true
	} else {
		senderSt = f.ToFcIDState
		senderIsCreator = false
	}

	if stateKey == f.FcID {
		stateKeySt = f.FcIDState
		stateKeyIsCreator = true
	} else {
		stateKeySt = f.ToFcIDState
		stateKeyIsCreator = false
	}

	var evs []gomatrixserverlib.Event
	// Special case: leaved member invite another member, only for local domain.
	if common.CheckValidDomain(domain, p.cfg.Matrix.ServerName) &&
		senderSt == ST_LEAVE && op == OP_INVITE && sender != stateKey {
		log.Infoln("rcsserver=====================EventProcessor.handleNormalMembership, leaved member invite another member")
		// events: invite myself, join, invite another(if he leaved).
		e, err := p.buildEvent(sender, roomID, sender, "invite")
		if err != nil {
			log.Errorf("Failed to build event: %v\n", err)
			return nil, err
		}
		evs = append(evs, *e)
		e, err = p.buildEvent(sender, roomID, sender, "join")
		if err != nil {
			log.Errorf("Failed to build event: %v\n", err)
			return nil, err
		}
		evs = append(evs, *e)
		var newStateKeySt string
		if stateKeySt == ST_LEAVE {
			newStateKeySt = ST_INVITE
			e, err := p.buildEvent(sender, roomID, stateKey, "invite")
			if err != nil {
				log.Errorf("Failed to build event: %v\n", err)
				return nil, err
			}
			evs = append(evs, *e)
		} else {
			newStateKeySt = stateKeySt
		}

		if senderIsCreator {
			err = p.db.UpdateFriendshipByRoomID(
				ctx, f.ID, f.RoomID, f.FcID, f.ToFcID, ST_JOIN, newStateKeySt,
				isBot(f.FcID), isBot(f.ToFcID), f.FcIDRemark, f.ToFcIDRemark,
				f.FcIDOnceJoined, f.ToFcIDOnceJoined, getDomain(f.FcID), getDomain(f.ToFcID), ev.EventID())
		} else {
			err = p.db.UpdateFriendshipByRoomID(
				ctx, f.ID, f.RoomID, f.FcID, f.ToFcID, newStateKeySt, ST_JOIN,
				isBot(f.FcID), isBot(f.ToFcID), f.FcIDRemark, f.ToFcIDRemark,
				f.FcIDOnceJoined, f.ToFcIDOnceJoined, getDomain(f.FcID), getDomain(f.ToFcID), ev.EventID())
		}
		if err != nil {
			log.Errorf("Failed to update friendship: %v\n", err)
			return nil, err
		}
	} else {
		if err := p.sm.Trigger(stateKeySt, op, ctx, sender, stateKey, stateKeyIsCreator, f, ev); err != nil {
			log.Errorf("Failed to trigger state machine: %v\n", err)
			return nil, err
		}
		evs = append(evs, *ev)
	}

	return evs, nil
}
