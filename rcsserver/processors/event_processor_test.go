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
	"fmt"
	"testing"

	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/plugins/message/external"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/storage/model"

	"github.com/finogeeks/ligase/model/types"
)

type MockFriendshipDatabase struct {
	Friendships []types.RCSFriendship
}

func (db *MockFriendshipDatabase) SetIDGenerator(idg *uid.UidGenerator) {

}

func (db *MockFriendshipDatabase) InsertFriendship(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string) error {
	f := types.RCSFriendship{
		ID:               ID,
		RoomID:           roomID,
		FcID:             fcID,
		ToFcID:           toFcID,
		FcIDState:        fcIDState,
		ToFcIDState:      toFcIDState,
		FcIDIsBot:        fcIDIsBot,
		ToFcIDIsBot:      toFcIDIsBot,
		FcIDRemark:       fcIDRemark,
		ToFcIDRemark:     toFcIDRemark,
		FcIDOnceJoined:   fcIDOnceJoined,
		ToFcIDOnceJoined: toFcIDOnceJoined,
		FcIDDomain:       fcIDDomain,
		ToFcIDDomain:     toFcIDDomain,
		EventID:          eventID,
	}
	db.Friendships = append(db.Friendships, f)
	return nil
}

func (db *MockFriendshipDatabase) GetFriendshipByRoomID(ctx context.Context, roomID string) (*types.RCSFriendship, error) {
	for _, v := range db.Friendships {
		if v.RoomID == roomID {
			return &v, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (db *MockFriendshipDatabase) GetFriendshipByFcIDOrToFcID(ctx context.Context, fcID, toFcID string) (*external.GetFriendshipResponse, error) {
	return nil, nil
}

func (db *MockFriendshipDatabase) GetFriendshipByFcIDAndToFcID(ctx context.Context, fcID, toFcID string) (*types.RCSFriendship, error) {
	for _, v := range db.Friendships {
		if v.FcID == fcID && v.ToFcID == toFcID {
			return &v, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (db *MockFriendshipDatabase) GetFriendshipsByFcIDOrToFcID(ctx context.Context, userID string) ([]external.Friendship, error) {
	return nil, nil
}

func (db *MockFriendshipDatabase) GetFriendshipsByFcIDOrToFcIDWithBot(ctx context.Context, userID string) ([]external.Friendship, error) {
	return nil, nil
}

func (db *MockFriendshipDatabase) UpdateFriendshipByRoomID(
	ctx context.Context, ID, roomID, fcID, toFcID, fcIDState, toFcIDState string,
	fcIDIsBot, toFcIDIsBot bool, fcIDRemark, toFcIDRemark string,
	fcIDOnceJoined, toFcIDOnceJoined bool, fcIDDomain, toFcIDDomain, eventID string) error {
	for i, v := range db.Friendships {
		if v.RoomID == roomID {
			db.Friendships[i] = types.RCSFriendship{
				ID:               ID,
				RoomID:           roomID,
				FcID:             fcID,
				ToFcID:           toFcID,
				FcIDState:        fcIDState,
				ToFcIDState:      toFcIDState,
				FcIDIsBot:        fcIDIsBot,
				ToFcIDIsBot:      toFcIDIsBot,
				FcIDRemark:       fcIDRemark,
				ToFcIDRemark:     toFcIDRemark,
				FcIDOnceJoined:   fcIDOnceJoined,
				ToFcIDOnceJoined: toFcIDOnceJoined,
				FcIDDomain:       fcIDDomain,
				ToFcIDDomain:     toFcIDDomain,
				EventID:          eventID,
			}
			return nil
		}
	}
	return sql.ErrNoRows
}

func (db *MockFriendshipDatabase) DeleteFriendshipByRoomID(ctx context.Context, roomID string) error {
	found := false
	i := 0
	for _, v := range db.Friendships {
		if v.RoomID == roomID {
			found = true
		} else {
			db.Friendships[i] = v
			i++
		}
	}
	if !found {
		return sql.ErrNoRows
	}
	db.Friendships = db.Friendships[:i]
	return nil
}

func (db *MockFriendshipDatabase) NotFound(err error) bool {
	return err != nil && err.Error() == sql.ErrNoRows.Error()
}

func buildEvent(
	sender, roomID, stateKey, eventType, membership string, cfg *config.Dendrite, idg *uid.UidGenerator,
) (*gomatrixserverlib.Event, error) {
	builder := gomatrixserverlib.EventBuilder{
		Sender:   sender,
		RoomID:   roomID,
		Type:     eventType,
		StateKey: &stateKey,
	}

	var content interface{}
	if eventType == gomatrixserverlib.MRoomCreate {
		fed := false
		isDir := true
		content = common.CreateContent{
			Creator:  sender,
			Federate: &fed,
			IsDirect: &isDir,
		}
	} else {
		content = external.MemberContent{
			Membership: membership,
			Reason:     "",
		}
	}
	if err := builder.SetContent(content); err != nil {
		log.Errorf("Failed to set content: %v\n", err)
		return nil, err
	}
	domainID, _ := common.DomainFromID(sender)
	e, err := common.BuildEvent(&builder, domainID, *cfg, idg)
	if err != nil {
		log.Errorf("Failed to build event: %v\n", err)
		return nil, err
	}
	return e, nil
}

func TestHandleCreate(t *testing.T) {
	domainID := "dendrite1"
	sender := "@aa:" + domainID
	var cfg config.Dendrite
	cfg.Matrix.ServerName = []string{domainID}
	idg, _ := uid.NewDefaultIdGenerator(0)
	var dbIface interface{}
	var db MockFriendshipDatabase
	dbIface = &db
	p := NewEventProcessor(&cfg, idg, dbIface.(model.RCSServerDatabase))
	ctx := context.Background()
	nid, _ := idg.Next()
	roomID := fmt.Sprintf("!%d:%s", nid, domainID)
	ev, _ := buildEvent(sender, roomID, "", gomatrixserverlib.MRoomCreate, "", &cfg, idg)
	evs, err := p.HandleCreate(ctx, ev)
	if err != nil {
		t.Fatalf("HandleCreate failed")
	}
	if len(evs) != 1 {
		t.Fatalf("Number of events return HandleCreate: %d\n", len(evs))
	}
}

func TestHandleMembership(t *testing.T) {

}

func TestHandleInvite(t *testing.T) {

}

func TestHandleLocalInvite(t *testing.T) {
	domainID := "dendrite"
	fc1 := "fcID1"
	fc2 := "fcID2"
	fcID1 := "@fcID1:" + domainID
	toFcID1 := "@fcID2:" + domainID
	fcID2 := toFcID1
	toFcID2 := fcID1
	var cfg config.Dendrite
	cfg.Matrix.ServerName = []string{domainID}
	idg, _ := uid.NewDefaultIdGenerator(0)
	nid1, _ := idg.Next()
	nid2, _ := idg.Next()
	roomID1 := fmt.Sprintf("!%d:%s", nid1, domainID)
	roomID2 := fmt.Sprintf("!%d:%s", nid2, domainID)
	ctx := context.Background()

	type eventOutput struct {
		roomID     string
		membership string
		sender     string
		stateKey   string
	}

	type testCase struct {
		name string

		roomID1           string
		fcID1             string
		toFcID1           string
		fcIDState1        string
		toFcIDState1      string
		fcIDRemark1       string
		toFcIDRemark1     string
		fcIDOnceJoined1   bool
		toFcIDOnceJoined1 bool
		fcIDDomain1       string
		toFcIDDomain1     string

		roomID2           string
		fcID2             string
		toFcID2           string
		fcIDState2        string
		toFcIDState2      string
		fcIDRemark2       string
		toFcIDRemark2     string
		fcIDOnceJoined2   bool
		toFcIDOnceJoined2 bool
		fcIDDomain2       string
		toFcIDDomain2     string

		membership string

		wantFcID2             string
		wantToFcID2           string
		wantRoomID2           string
		wantFcIDState2        string
		wantToFcIDState2      string
		wantFcIDRemark2       string
		wantToFcIDRemark2     string
		wantFcIDOnceJoined2   bool
		wantToFcIDOnceJoined2 bool
		wantFcIDDomain2       string
		wantToFcIDDomain2     string

		wantEvents []eventOutput
	}

	tc := testCase{
		name: "room2 exist",

		roomID1:           roomID1,
		fcID1:             fcID1,
		toFcID1:           toFcID1,
		fcIDState1:        ST_JOIN,
		toFcIDState1:      ST_INIT,
		fcIDOnceJoined1:   true,
		toFcIDOnceJoined1: false,
		fcIDDomain1:       domainID,
		toFcIDDomain1:     domainID,

		roomID2:           roomID2,
		fcID2:             fcID2,
		toFcID2:           toFcID2,
		fcIDState2:        ST_JOIN,
		toFcIDState2:      ST_INVITE,
		fcIDRemark2:       fc2,
		toFcIDRemark2:     fc1,
		fcIDOnceJoined2:   true,
		toFcIDOnceJoined2: false,
		fcIDDomain2:       domainID,
		toFcIDDomain2:     domainID,

		membership: "invite",

		wantFcID2:             fcID2,
		wantToFcID2:           fcID1,
		wantRoomID2:           roomID2,
		wantFcIDState2:        ST_JOIN,
		wantToFcIDState2:      ST_JOIN,
		wantFcIDRemark2:       fc2,
		wantToFcIDRemark2:     fc1,
		wantFcIDOnceJoined2:   true,
		wantToFcIDOnceJoined2: true,
		wantFcIDDomain2:       domainID,
		wantToFcIDDomain2:     domainID,
		wantEvents: []eventOutput{
			{
				membership: "leave",
				roomID:     roomID1,
				sender:     fcID1,
				stateKey:   fcID1,
			},
			{
				membership: "join",
				roomID:     roomID2,
				sender:     fcID1,
				stateKey:   fcID1,
			},
		},
	}

	setup := func(tc *testCase) (*gomatrixserverlib.Event, *EventProcessor) {
		db := MockFriendshipDatabase{
			Friendships: []types.RCSFriendship{
				{
					RoomID:           tc.roomID2,
					FcID:             tc.fcID2,
					ToFcID:           tc.toFcID2,
					FcIDState:        tc.fcIDState2,
					ToFcIDState:      tc.toFcIDState2,
					FcIDRemark:       tc.fcIDRemark2,
					ToFcIDRemark:     tc.toFcIDRemark2,
					FcIDOnceJoined:   tc.fcIDOnceJoined2,
					ToFcIDOnceJoined: tc.toFcIDOnceJoined2,
					FcIDDomain:       tc.fcIDDomain2,
					ToFcIDDomain:     tc.toFcIDDomain2,
				},
				{
					RoomID:           tc.roomID1,
					FcID:             tc.fcID1,
					ToFcID:           tc.toFcID1,
					FcIDState:        tc.fcIDState1,
					ToFcIDState:      tc.toFcIDState1,
					FcIDRemark:       tc.fcIDRemark1,
					ToFcIDRemark:     tc.toFcIDRemark1,
					FcIDOnceJoined:   tc.fcIDOnceJoined1,
					ToFcIDOnceJoined: tc.toFcIDOnceJoined1,
					FcIDDomain:       tc.fcIDDomain1,
					ToFcIDDomain:     tc.toFcIDDomain1,
				},
			},
		}
		var dbIface interface{}
		dbIface = &db
		p := NewEventProcessor(&cfg, idg, dbIface.(model.RCSServerDatabase))
		ev, _ := buildEvent(tc.fcID1, tc.roomID1, tc.fcID2, gomatrixserverlib.MRoomMember, tc.membership, &cfg, idg)
		return ev, p
	}

	t.Run(tc.name, func(t *testing.T) {
		ev, p := setup(&tc)
		evs, err := p.handleLocalInvite(ctx, ev)
		if err != nil {
			t.Fatalf("handleLocalInvite failed: %v\n", err)
		}
		if len(evs) != len(tc.wantEvents) {
			t.Fatalf("Invalid number of events: %d\n", len(evs))
		}
		for i := 0; i < len(evs); i++ {
			m, _ := evs[i].Membership()
			if evs[i].Sender() != tc.wantEvents[i].sender || *evs[i].StateKey() != tc.wantEvents[i].stateKey || evs[i].RoomID() != tc.wantEvents[i].roomID || m != tc.wantEvents[i].membership {
				t.Fatalf("Invalid event: %v\n", evs[i])
			}
		}
		f, err := p.db.GetFriendshipByRoomID(ctx, roomID1)
		if err == nil {
			t.Fatalf("Room1 should be deleted")
		}
		if !p.db.NotFound(err) {
			t.Fatalf("Room1 should be deleted")
		}
		f, err = p.db.GetFriendshipByRoomID(ctx, roomID2)
		if err != nil {
			t.Fatalf("Query room2 failed: %v\n", err)
		}
		if f.RoomID != tc.wantRoomID2 || f.FcID != tc.wantFcID2 || f.ToFcID != tc.wantToFcID2 || f.FcIDState != tc.wantFcIDState2 || f.ToFcIDState != tc.wantToFcIDState2 ||
			f.FcIDRemark != tc.wantFcIDRemark2 || f.ToFcIDRemark != tc.wantToFcIDRemark2 ||
			f.FcIDOnceJoined != tc.wantFcIDOnceJoined2 || f.ToFcIDOnceJoined != tc.wantToFcIDOnceJoined2 || f.FcIDDomain != tc.wantToFcIDDomain2 || f.ToFcIDDomain != tc.wantToFcIDDomain2 {
			t.Fatalf("Invalid data for room2: %v\n", f)
		}
	})
}
func TestHandleFedInvite(t *testing.T) {
	domainID1 := "dendrite1"
	domainID2 := "dendrite2"
	fc1 := "fcID1"
	fc2 := "fcID2"
	fcID1 := "@fcID1:" + domainID1
	toFcID1 := "@fcID2:" + domainID2
	fcID2 := toFcID1
	toFcID2 := fcID1
	var cfg config.Dendrite
	cfg.Matrix.ServerName = []string{domainID2}
	idg, _ := uid.NewDefaultIdGenerator(0)
	nid1, _ := idg.Next()
	nid2, _ := idg.Next()
	roomID1 := fmt.Sprintf("!%d:%s", nid1, domainID1)
	roomID2 := fmt.Sprintf("!%d:%s", nid2, domainID2)

	nid22, _ := idg.Next()
	nid11, _ := idg.Next()
	roomID22 := fmt.Sprintf("!%d:%s", nid22, domainID2)
	roomID11 := fmt.Sprintf("!%d:%s", nid11, domainID1)
	ctx := context.Background()

	type eventOutput struct {
		roomID     string
		membership string
		sender     string
		stateKey   string
	}

	type testCase struct {
		name string

		roomID1           string
		fcID1             string
		toFcID1           string
		fcIDState1        string
		toFcIDState1      string
		fcIDRemark1       string
		toFcIDRemark1     string
		fcIDOnceJoined1   bool
		toFcIDOnceJoined1 bool
		fcIDDomain1       string
		toFcIDDomain1     string

		roomID2           string
		fcID2             string
		toFcID2           string
		fcIDState2        string
		toFcIDState2      string
		fcIDRemark2       string
		toFcIDRemark2     string
		fcIDOnceJoined2   bool
		toFcIDOnceJoined2 bool
		fcIDDomain2       string
		toFcIDDomain2     string

		membership string

		wantRoom1Deleted      bool
		wantFcID1             string
		wantToFcID1           string
		wantRoomID1           string
		wantFcIDState1        string
		wantToFcIDState1      string
		wantFcIDRemark1       string
		wantToFcIDRemark1     string
		wantFcIDOnceJoined1   bool
		wantToFcIDOnceJoined1 bool
		wantFcIDDomain1       string
		wantToFcIDDomain1     string

		wantRoom2Deleted      bool
		wantFcID2             string
		wantToFcID2           string
		wantRoomID2           string
		wantFcIDState2        string
		wantToFcIDState2      string
		wantFcIDRemark2       string
		wantToFcIDRemark2     string
		wantFcIDOnceJoined2   bool
		wantToFcIDOnceJoined2 bool
		wantFcIDDomain2       string
		wantToFcIDDomain2     string

		wantEvents []eventOutput
	}

	tcs := []testCase{
		{
			name: "room1 less than room2",

			roomID1:           roomID1,
			fcID1:             fcID1,
			toFcID1:           toFcID1,
			fcIDState1:        ST_JOIN,
			toFcIDState1:      ST_INIT,
			fcIDRemark1:       fc1,
			toFcIDRemark1:     "",
			fcIDOnceJoined1:   true,
			toFcIDOnceJoined1: false,
			fcIDDomain1:       domainID1,
			toFcIDDomain1:     domainID2,

			roomID2:           roomID2,
			fcID2:             fcID2,
			toFcID2:           toFcID2,
			fcIDState2:        ST_JOIN,
			toFcIDState2:      ST_INVITE,
			fcIDRemark2:       fc2,
			toFcIDRemark2:     fc1,
			fcIDOnceJoined2:   true,
			toFcIDOnceJoined2: false,
			fcIDDomain2:       domainID2,
			toFcIDDomain2:     domainID1,

			membership: "invite",

			wantRoom1Deleted:      false,
			wantFcID1:             fcID1,
			wantToFcID1:           fcID2,
			wantRoomID1:           roomID1,
			wantFcIDState1:        ST_JOIN,
			wantToFcIDState1:      ST_JOIN,
			wantFcIDRemark1:       fc1,
			wantToFcIDRemark1:     fc2,
			wantFcIDOnceJoined1:   true,
			wantToFcIDOnceJoined1: true,
			wantFcIDDomain1:       domainID1,
			wantToFcIDDomain1:     domainID2,

			wantRoom2Deleted: true,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID2,
					sender:     fcID2,
					stateKey:   fcID2,
				},
				{
					membership: "join",
					roomID:     roomID1,
					sender:     fcID2,
					stateKey:   fcID2,
				},
			},
		},
		{
			name: "room1 greater than room2",

			roomID1:           roomID11,
			fcID1:             fcID1,
			toFcID1:           toFcID1,
			fcIDState1:        ST_JOIN,
			toFcIDState1:      ST_INIT,
			fcIDRemark1:       fc1,
			toFcIDRemark1:     "",
			fcIDOnceJoined1:   true,
			toFcIDOnceJoined1: false,
			fcIDDomain1:       domainID1,
			toFcIDDomain1:     domainID2,

			roomID2:           roomID22,
			fcID2:             fcID2,
			toFcID2:           toFcID2,
			fcIDState2:        ST_JOIN,
			toFcIDState2:      ST_INVITE,
			fcIDRemark2:       fc2,
			toFcIDRemark2:     fc1,
			fcIDOnceJoined2:   true,
			toFcIDOnceJoined2: false,
			fcIDDomain2:       domainID2,
			toFcIDDomain2:     domainID1,

			membership: "invite",

			wantRoom1Deleted: true,

			wantRoom2Deleted:      false,
			wantFcID2:             fcID2,
			wantToFcID2:           fcID1,
			wantRoomID2:           roomID22,
			wantFcIDState2:        ST_JOIN,
			wantToFcIDState2:      ST_INVITE,
			wantFcIDRemark2:       fc2,
			wantToFcIDRemark2:     fc1,
			wantFcIDOnceJoined2:   true,
			wantToFcIDOnceJoined2: false,
			wantFcIDDomain2:       domainID2,
			wantToFcIDDomain2:     domainID1,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID11,
					sender:     fcID2,
					stateKey:   fcID2,
				},
			},
		},
	}

	setup := func(tc *testCase) (*gomatrixserverlib.Event, *EventProcessor) {
		db := MockFriendshipDatabase{
			Friendships: []types.RCSFriendship{
				{
					RoomID:           tc.roomID2,
					FcID:             tc.fcID2,
					ToFcID:           tc.toFcID2,
					FcIDState:        tc.fcIDState2,
					ToFcIDState:      tc.toFcIDState2,
					FcIDRemark:       tc.fcIDRemark2,
					ToFcIDRemark:     tc.toFcIDRemark2,
					FcIDOnceJoined:   tc.fcIDOnceJoined2,
					ToFcIDOnceJoined: tc.toFcIDOnceJoined2,
					FcIDDomain:       tc.fcIDDomain2,
					ToFcIDDomain:     tc.toFcIDDomain2,
				},
				{
					RoomID:           tc.roomID1,
					FcID:             tc.fcID1,
					ToFcID:           tc.toFcID1,
					FcIDState:        tc.fcIDState1,
					ToFcIDState:      tc.toFcIDState1,
					FcIDRemark:       tc.fcIDRemark1,
					ToFcIDRemark:     tc.toFcIDRemark1,
					FcIDOnceJoined:   tc.fcIDOnceJoined1,
					ToFcIDOnceJoined: tc.toFcIDOnceJoined1,
					FcIDDomain:       tc.fcIDDomain1,
					ToFcIDDomain:     tc.toFcIDDomain1,
				},
			},
		}
		var dbIface interface{}
		dbIface = &db
		p := NewEventProcessor(&cfg, idg, dbIface.(model.RCSServerDatabase))
		ev, _ := buildEvent(tc.fcID1, tc.roomID1, tc.fcID2, gomatrixserverlib.MRoomMember, tc.membership, &cfg, idg)
		return ev, p
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ev, p := setup(&tc)
			evs, err := p.handleFedInvite(ctx, ev)
			if err != nil {
				t.Fatalf("handleFedInvite failed: %v\n", err)
			}
			if len(evs) != len(tc.wantEvents) {
				t.Fatalf("Invalid number of events: %d\n", len(evs))
			}
			for i := 0; i < len(evs); i++ {
				m, _ := evs[i].Membership()
				if evs[i].Sender() != tc.wantEvents[i].sender || *evs[i].StateKey() != tc.wantEvents[i].stateKey || evs[i].RoomID() != tc.wantEvents[i].roomID || m != tc.wantEvents[i].membership {
					t.Fatalf("Invalid event: %v\n", evs[i])
				}
			}
			if tc.wantRoom1Deleted {
				f, err := p.db.GetFriendshipByRoomID(ctx, tc.roomID1)
				if err == nil {
					t.Fatalf("Room1 should be deleted")
				}
				if !p.db.NotFound(err) {
					t.Fatalf("Room1 should be deleted")
				}
				f, err = p.db.GetFriendshipByRoomID(ctx, tc.roomID2)
				if err != nil {
					t.Fatalf("Query room2 failed: %v\n", err)
				}
				if f.RoomID != tc.wantRoomID2 || f.FcID != tc.wantFcID2 || f.ToFcID != tc.wantToFcID2 || f.FcIDState != tc.wantFcIDState2 || f.ToFcIDState != tc.wantToFcIDState2 ||
					f.FcIDRemark != tc.wantFcIDRemark2 || f.ToFcIDRemark != tc.wantToFcIDRemark2 ||
					f.FcIDOnceJoined != tc.wantFcIDOnceJoined2 || f.ToFcIDOnceJoined != tc.wantToFcIDOnceJoined2 || f.FcIDDomain != tc.wantFcIDDomain2 || f.ToFcIDDomain != tc.wantToFcIDDomain2 {
					t.Fatalf("Invalid data for room2: %v\n", f)
				}
			}
			if tc.wantRoom2Deleted {
				f, err := p.db.GetFriendshipByRoomID(ctx, tc.roomID2)
				if err == nil {
					t.Fatalf("Room2 should be deleted")
				}
				if !p.db.NotFound(err) {
					t.Fatalf("Room2 should be deleted")
				}
				f, err = p.db.GetFriendshipByRoomID(ctx, tc.roomID1)
				if err != nil {
					t.Fatalf("Query room1 failed: %v\n", err)
				}
				if f.RoomID != tc.wantRoomID1 || f.FcID != tc.wantFcID1 || f.ToFcID != tc.wantToFcID1 || f.FcIDState != tc.wantFcIDState1 || f.ToFcIDState != tc.wantToFcIDState1 ||
					f.FcIDRemark != tc.wantFcIDRemark1 || f.ToFcIDRemark != tc.wantToFcIDRemark1 ||
					f.FcIDOnceJoined != tc.wantFcIDOnceJoined1 || f.ToFcIDOnceJoined != tc.wantToFcIDOnceJoined1 || f.FcIDDomain != tc.wantFcIDDomain1 || f.ToFcIDDomain != tc.wantToFcIDDomain1 {
					t.Fatalf("Invalid data for room1: %v\n", f)
				}
			}
		})
	}
}
func TestNormalMembership(t *testing.T) {
	domainID := "dendrite"
	fc := "fcID1"
	toFc := "fcID2"
	fcID := "@fcID1:" + domainID
	toFcID := "@fcID2:" + domainID
	var cfg config.Dendrite
	cfg.Matrix.ServerName = []string{domainID}
	idg, _ := uid.NewDefaultIdGenerator(0)
	nid, _ := idg.Next()
	roomID := fmt.Sprintf("!%d:%s", nid, domainID)

	domainIDFed := "dendrite_fed"
	fcIDFed := "@fcID1:" + domainIDFed
	toFcIDFed := "@fcID2:" + domainID
	roomIDFed := fmt.Sprintf("!%d:%s", nid, domainIDFed)
	fcFed := "fcID1"
	toFcFed := "fcID2"

	ctx := context.Background()

	type eventOutput struct {
		roomID     string
		membership string
		sender     string
		stateKey   string
	}

	type testCase struct {
		name string

		roomID           string
		fcID             string
		toFcID           string
		fcIDState        string
		toFcIDState      string
		fcIDRemark       string
		toFcIDRemark     string
		fcIDOnceJoined   bool
		toFcIDOnceJoined bool
		fcIDDomain       string
		toFcIDDomain     string

		membership string

		sender   string
		stateKey string

		wantFcID             string
		wantToFcID           string
		wantRoomID           string
		wantFcIDState        string
		wantToFcIDState      string
		wantFcIDRemark       string
		wantToFcIDRemark     string
		wantFcIDOnceJoined   bool
		wantToFcIDOnceJoined bool
		wantFcIDDomain       string
		wantToFcIDDomain     string

		wantEvents []eventOutput
	}

	tcs := []testCase{
		{
			name: "local-1: init_init to join_init",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           "",
			fcIDState:        ST_INIT,
			toFcIDState:      ST_INIT,
			fcIDRemark:       "",
			toFcIDRemark:     "",
			fcIDOnceJoined:   false,
			toFcIDOnceJoined: false,
			fcIDDomain:       "",
			toFcIDDomain:     "",

			sender:   fcID,
			stateKey: fcID,

			membership: "join",

			wantFcID:             fcID,
			wantToFcID:           "",
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INIT,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     "",
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: false,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     "",

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-2: join_init to join_invite",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           "",
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_INIT,
			fcIDRemark:       fc,
			toFcIDRemark:     "",
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: false,
			fcIDDomain:       domainID,
			toFcIDDomain:     "",

			sender:   fcID,
			stateKey: toFcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: false,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-3: join_invite to join_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_INVITE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: false,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: toFcID,

			membership: "join",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-4: join_invite to join_leave",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_INVITE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: false,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: toFcID,

			membership: "leave",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: false,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-5: join_join to join_leave",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: toFcID,

			membership: "leave",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-6: join_join to leave_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: fcID,

			membership: "leave",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-7: join_leave to join_invite",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: toFcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-8: join_leave to join_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: fcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
				{
					membership: "join",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-9: join_leave to leave_leave",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: fcID,

			membership: "leave",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-10: leave_join to invite_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: fcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_INVITE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-11: leave_join to join_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: toFcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
				{
					membership: "join",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-12: leave_join to leave_leave",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: toFcID,

			membership: "leave",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-13: invite_join to join_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_INVITE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: fcID,

			membership: "join",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-14: invite_join to leave_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_INVITE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: fcID,

			membership: "leave",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "local-15: leave_leave to join_invite",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   fcID,
			stateKey: toFcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
				{
					membership: "join",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   fcID,
				},
				{
					membership: "invite",
					roomID:     roomID,
					sender:     fcID,
					stateKey:   toFcID,
				},
			},
		},
		{
			name: "local-16: leave_leave to invite_join",

			roomID:           roomID,
			fcID:             fcID,
			toFcID:           toFcID,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fc,
			toFcIDRemark:     toFc,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainID,
			toFcIDDomain:     domainID,

			sender:   toFcID,
			stateKey: fcID,

			membership: "invite",

			wantFcID:             fcID,
			wantToFcID:           toFcID,
			wantRoomID:           roomID,
			wantFcIDState:        ST_INVITE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fc,
			wantToFcIDRemark:     toFc,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainID,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
				{
					membership: "join",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   toFcID,
				},
				{
					membership: "invite",
					roomID:     roomID,
					sender:     toFcID,
					stateKey:   fcID,
				},
			},
		},
		{
			name: "fed-1: init_init to join_init",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           "",
			fcIDState:        ST_INIT,
			toFcIDState:      ST_INIT,
			fcIDRemark:       "",
			toFcIDRemark:     "",
			fcIDOnceJoined:   false,
			toFcIDOnceJoined: false,
			fcIDDomain:       "",
			toFcIDDomain:     "",

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "join",

			wantFcID:             fcIDFed,
			wantToFcID:           "",
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INIT,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     "",
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: false,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     "",

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-2: join_init to join_invite",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           "",
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_INIT,
			fcIDRemark:       fcFed,
			toFcIDRemark:     "",
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: false,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     "",

			sender:   fcIDFed,
			stateKey: toFcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: false,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-3: join_invite to join_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_INVITE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: false,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "join",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-4: join_invite to join_leave",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_INVITE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: false,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "leave",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: false,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-5: join_join to join_leave",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "leave",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-6: join_join to leave_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "leave",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-7: join_leave to join_invite",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: toFcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-8: join_leave to leave_invite",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "leave",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-9: join_leave to join_invite",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_JOIN,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-10: leave_join to invite_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: fcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_INVITE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-11: leave_join to leave_leave",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "leave",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-12: leave_join to invite_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_INVITE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-13: invite_join to join_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_INVITE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "join",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-14: invite_join to leave_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_INVITE,
			toFcIDState:      ST_JOIN,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "leave",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "leave",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-15: leave_leave to invite_leave",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_INVITE,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-16: leave_leave to leave_invite",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "invite",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_INVITE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "invite",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
		{
			name: "fed-17: invite_leave to join_leave",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_INVITE,
			toFcIDState:      ST_LEAVE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   fcIDFed,
			stateKey: fcIDFed,

			membership: "join",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_JOIN,
			wantToFcIDState:      ST_LEAVE,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomIDFed,
					sender:     fcIDFed,
					stateKey:   fcIDFed,
				},
			},
		},
		{
			name: "fed-18: leave_invite to leave_join",

			roomID:           roomIDFed,
			fcID:             fcIDFed,
			toFcID:           toFcIDFed,
			fcIDState:        ST_LEAVE,
			toFcIDState:      ST_INVITE,
			fcIDRemark:       fcFed,
			toFcIDRemark:     toFcFed,
			fcIDOnceJoined:   true,
			toFcIDOnceJoined: true,
			fcIDDomain:       domainIDFed,
			toFcIDDomain:     domainID,

			sender:   toFcIDFed,
			stateKey: toFcIDFed,

			membership: "join",

			wantFcID:             fcIDFed,
			wantToFcID:           toFcIDFed,
			wantRoomID:           roomIDFed,
			wantFcIDState:        ST_LEAVE,
			wantToFcIDState:      ST_JOIN,
			wantFcIDRemark:       fcFed,
			wantToFcIDRemark:     toFcFed,
			wantFcIDOnceJoined:   true,
			wantToFcIDOnceJoined: true,
			wantFcIDDomain:       domainIDFed,
			wantToFcIDDomain:     domainID,

			wantEvents: []eventOutput{
				{
					membership: "join",
					roomID:     roomIDFed,
					sender:     toFcIDFed,
					stateKey:   toFcIDFed,
				},
			},
		},
	}

	setup := func(tc *testCase) (*gomatrixserverlib.Event, *EventProcessor) {
		db := MockFriendshipDatabase{
			Friendships: []types.RCSFriendship{
				{
					RoomID:           tc.roomID,
					FcID:             tc.fcID,
					ToFcID:           tc.toFcID,
					FcIDState:        tc.fcIDState,
					ToFcIDState:      tc.toFcIDState,
					FcIDRemark:       tc.fcIDRemark,
					ToFcIDRemark:     tc.toFcIDRemark,
					FcIDOnceJoined:   tc.fcIDOnceJoined,
					ToFcIDOnceJoined: tc.toFcIDOnceJoined,
					FcIDDomain:       tc.fcIDDomain,
					ToFcIDDomain:     tc.toFcIDDomain,
				},
			},
		}
		var dbIface interface{}
		dbIface = &db
		p := NewEventProcessor(&cfg, idg, dbIface.(model.RCSServerDatabase))
		ev, _ := buildEvent(tc.sender, tc.roomID, tc.stateKey, gomatrixserverlib.MRoomMember, tc.membership, &cfg, idg)
		return ev, p
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ev, p := setup(&tc)
			evs, err := p.handleNormalMembership(ctx, ev)
			if err != nil {
				t.Fatalf("handleNormalMembership failed: %v\n", err)
			}
			if len(evs) != len(tc.wantEvents) {
				t.Fatalf("Invalid number of events: %d\n", len(evs))
			}
			for i := 0; i < len(evs); i++ {
				m, _ := evs[i].Membership()
				if evs[i].Sender() != tc.wantEvents[i].sender || *evs[i].StateKey() != tc.wantEvents[i].stateKey || evs[i].RoomID() != tc.wantEvents[i].roomID || m != tc.wantEvents[i].membership {
					t.Fatalf("Invalid event: %v\n", evs[i])
				}
			}

			f, err := p.db.GetFriendshipByRoomID(ctx, tc.roomID)
			if err != nil {
				t.Fatalf("Query room failed: %v\n", err)
			}
			if f.RoomID != tc.wantRoomID || f.FcID != tc.wantFcID || f.ToFcID != tc.wantToFcID || f.FcIDState != tc.wantFcIDState || f.ToFcIDState != tc.wantToFcIDState ||
				f.FcIDRemark != tc.wantFcIDRemark || f.ToFcIDRemark != tc.wantToFcIDRemark ||
				f.FcIDOnceJoined != tc.wantFcIDOnceJoined || f.ToFcIDOnceJoined != tc.wantToFcIDOnceJoined || f.FcIDDomain != tc.wantFcIDDomain || f.ToFcIDDomain != tc.wantToFcIDDomain {
				t.Fatalf("Invalid data for room: %v\n", f)
			}
		})
	}
}
