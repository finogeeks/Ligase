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

package repos

import (
	"sync"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type RangeItem struct {
	Start    int64 `json:"start"`
	End      int64 `json:"end"`
	isRef    bool  // 是否全局RangeItem的索引
	pairMask uint8 // 只用在backfill中，用于判断是否匹配,1表示Start是确定的，2表示End是确定的，3表示Start/End都确定
}

type RoomState struct {
	creator              string
	isDirect             bool
	federate             bool
	isChannel            bool
	isSecret             bool
	joinRule             string
	historyVisibility    string
	historyVisibilityPos int64
	visibility           string
	name                 string
	topic                string
	desc                 string
	canonicalAlias       string
	power                *common.PowerLevelContent
	isEncrypted          bool

	join   sync.Map
	leave  sync.Map
	invite sync.Map

	visiableTl []*RangeItem

	createPos        int64
	userMemberView   sync.Map
	userMemberViewOk *sync.Map

	preState       sync.Map
	preMemberState sync.Map
	preUserAlias   sync.Map

	curState       sync.Map
	curMemberState sync.Map
	curUserAlias   sync.Map
}

func newRoomState() *RoomState {
	rs := new(RoomState)
	rs.userMemberViewOk = new(sync.Map)
	return rs
}

//RoomStateTimeLineRepo 保证state 会重演
func (rs *RoomState) onEvent(ev *gomatrixserverlib.ClientEvent, offset int64, backfill bool) {
	if !backfill {
		switch ev.Type {
		case "m.room.create":
			con := common.CreateContent{}
			json.Unmarshal(ev.Content, &con)
			rs.creator = con.Creator
			if con.Federate != nil {
				rs.federate = *con.Federate
			}
			if con.IsDirect != nil {
				rs.isDirect = *con.IsDirect
			}
			if con.IsChannel != nil {
				rs.isChannel = *con.IsChannel
			}
			if con.IsSecret != nil {
				rs.isSecret = *con.IsSecret
			}
			//rs.creator = con["creator"].(string)
			rs.createPos = int64(ev.OriginServerTS)
		case "m.room.member":
			rs.onMembership(ev, offset)
		case "m.room.power_levels":
			pl := common.PowerLevelContent{}
			json.Unmarshal(ev.Content, &pl)
			rs.power = &pl
		case "m.room.join_rules":
			jr := common.JoinRulesContent{}
			json.Unmarshal(ev.Content, &jr)
			rs.joinRule = jr.JoinRule
		case "m.room.history_visibility": //每个房间都有重演，所以不需要保存历史
			rs.onHistoryVisibility(ev, offset)
		case "m.room.visibility":
			vi := common.VisibilityContent{}
			json.Unmarshal(ev.Content, &vi)
			rs.visibility = vi.Visibility
		case "m.room.name":
			name := common.NameContent{}
			json.Unmarshal(ev.Content, &name)
			rs.name = name.Name
		case "m.room.topic":
			topic := common.TopicContent{}
			json.Unmarshal(ev.Content, &topic)
			rs.topic = topic.Topic
		case "m.room.desc":
			desc := common.DescContent{}
			json.Unmarshal(ev.Content, &desc)
			rs.desc = desc.Desc
		case "m.room.canonical_alias":
			alias := common.CanonicalAliasContent{}
			json.Unmarshal(ev.Content, &alias)
			rs.canonicalAlias = alias.Alias
		case "m.room.encryption":
			rs.isEncrypted = true
		}

		if common.IsStateClientEv(ev) {
			stream := new(feedstypes.StreamEvent)
			stream.Offset = offset
			stream.Ev = ev

			statKey := ""
			if ev.StateKey != nil {
				statKey = *ev.StateKey
			}
			cur := rs.GetState(ev.Type, statKey)
			if ev.Type == "m.room.member" {
				rs.curMemberState.Store(statKey, stream)
				rs.preMemberState.Store(statKey, cur)
			} else if ev.Type == "m.room.aliases" {
				rs.curUserAlias.Store(statKey, stream)
				rs.preUserAlias.Store(statKey, cur)
			} else {
				rs.curState.Store(ev.Type, stream)
				rs.preState.Store(ev.Type, cur)
			}
		}
	} else {
		switch ev.Type {
		case "m.room.member":
			member := external.MemberContent{}
			json.Unmarshal(ev.Content, &member)
			val, _ := rs.curMemberState.Load(*ev.StateKey)
			if val != nil && int64(ev.OriginServerTS) > int64(val.(*feedstypes.StreamEvent).GetEv().OriginServerTS) {
				rs.onMembership(ev, offset)
				stream := new(feedstypes.StreamEvent)
				stream.Offset = offset
				stream.Ev = ev
				statKey := ""
				if ev.StateKey != nil {
					statKey = *ev.StateKey
				}
				rs.curMemberState.Store(statKey, stream)
				rs.preMemberState.Store(statKey, val)
			} else {
				var items []*RangeItem
				val, ok := rs.userMemberView.Load(*ev.StateKey)
				if !ok {
					items = []*RangeItem{}
				} else {
					items = val.([]*RangeItem)
				}
				if len(items) == 0 {
					items = append(items, &RangeItem{Start: rs.createPos, End: rs.createPos})
				}
				openClose := 0
				if member.Membership == "join" {
					openClose = 1
				} else if member.Membership == "leave" || member.Membership == "ban" {
					openClose = 2
				}
				items = rs.insertIntoRanges(items, ev, offset, false, openClose)
				rs.userMemberView.Store(*ev.StateKey, items)
			}
		case "m.room.history_visibility":
			if int64(ev.OriginServerTS) > rs.historyVisibilityPos {
				rs.onHistoryVisibility(ev, offset)
			} else {
				hv := common.HistoryVisibilityContent{}
				json.Unmarshal(ev.Content, &hv)
				openClose := 0
				if hv.HistoryVisibility == "shared" {
					openClose = 1
				} else if hv.HistoryVisibility == "joined" {
					openClose = 2
				}
				rs.visiableTl = rs.insertIntoRanges(rs.visiableTl, ev, offset, true, openClose)
			}
		case "m.room.power_levels":
			pre := rs.GetState(ev.Type, "")
			if pre != nil && ev.OriginServerTS > pre.Ev.OriginServerTS {
				pl := common.PowerLevelContent{}
				json.Unmarshal(ev.Content, &pl)
				rs.power = &pl
			}
		}
	}
}

func (rs *RoomState) onMembership(ev *gomatrixserverlib.ClientEvent, offset int64) {
	member := external.MemberContent{}
	json.Unmarshal(ev.Content, &member)
	pre := ""

	if _, ok := rs.join.Load(*ev.StateKey); ok {
		pre = "join"
	} else if _, ok := rs.leave.Load(*ev.StateKey); ok {
		pre = "leave"
	} else if _, ok := rs.invite.Load(*ev.StateKey); ok {
		pre = "invite"
	}

	// Calculate history visibility first in case error occurs.
	if member.Membership != pre {
		rs.onUserMembershipChange(*ev.StateKey, rs.historyVisibility, pre, member.Membership, int64(ev.OriginServerTS))
	}

	if member.Membership == "join" {
		rs.join.Store(*ev.StateKey, offset)
	} else if member.Membership == "invite" {
		rs.invite.Store(*ev.StateKey, offset)
	} else if member.Membership == "leave" || member.Membership == "ban" {
		rs.leave.Store(*ev.StateKey, offset)
	}

	//log.Errorf("onEvent user:%s pre:%s, mem:%s", *ev.StateKey(), pre, member.Membership)
	if member.Membership != pre {
		if pre == "join" {
			rs.join.Delete(*ev.StateKey)
		} else if pre == "invite" {
			rs.invite.Delete(*ev.StateKey)
		} else if pre == "leave" {
			rs.leave.Delete(*ev.StateKey)
		}
	}
}

func (rs *RoomState) onHistoryVisibility(ev *gomatrixserverlib.ClientEvent, offset int64) {
	hv := common.HistoryVisibilityContent{}
	json.Unmarshal(ev.Content, &hv)

	log.Debugf("onHistoryVisibility, old: %s, new: %s", rs.historyVisibility, hv.HistoryVisibility)
	if rs.historyVisibility != hv.HistoryVisibility {
		if hv.HistoryVisibility == "shared" {
			rs.visiableTl = append(rs.visiableTl, &RangeItem{
				Start:    int64(ev.OriginServerTS),
				End:      -1,
				isRef:    true,
				pairMask: 1,
			})
			log.Debugf("onUserMembershipChange onEvent append visiable start:%d end:%d", ev.OriginServerTS, -1)
		}

		if hv.HistoryVisibility == "joined" && len(rs.visiableTl) > 0 {
			lastItem := rs.visiableTl[len(rs.visiableTl)-1]
			lastItem.End = int64(ev.OriginServerTS) - 1
			log.Debugf("onUserMembershipChange onEvent update visiable start:%d end:%d", lastItem.Start, lastItem.End)
		}
		rs.historyVisibility = hv.HistoryVisibility
		rs.historyVisibilityPos = int64(ev.OriginServerTS)
	}
}

func (rs *RoomState) insertIntoRanges(ranges []*RangeItem, ev *gomatrixserverlib.ClientEvent, offset int64, isRef bool, openClose int) []*RangeItem {
	isLastOne := false
	isAppend := false
	if openClose == 1 {
		index := rs.findRangeIdx(ranges, int64(ev.OriginServerTS), isRef)
		if index >= 0 {
			endTmp := ranges[index].End
			if ranges[index].pairMask == 2 {
				ranges[index].pairMask = 3
				ranges[index].Start = int64(ev.OriginServerTS)
			} else {
				tail := append([]*RangeItem{}, ranges[index+1:]...)
				ranges = append(append(ranges[:index+1], &RangeItem{Start: int64(ev.OriginServerTS), End: endTmp, isRef: isRef}), tail...)
				isAppend = true
				ranges[index].End = int64(ev.OriginServerTS) - 1
				if ranges[index].pairMask == 1 {
					ranges[index+1].pairMask = 1
				} else {
					ranges[index].pairMask = 1
					ranges[index+1].pairMask = 3
				}
				if len(tail) == 0 && ranges[index+1].pairMask == 1 {
					isLastOne = true
				}
			}
		} else {
			idx := -1
			min := int64(-1)
			for i, v := range ranges {
				if int64(ev.OriginServerTS) <= v.Start && (v.Start < min || min == -1) {
					min = v.Start
					idx = i
				}
			}
			if idx >= 0 {
				end := ranges[idx].Start - 1
				tail := append([]*RangeItem{}, ranges[idx:]...)
				ranges = append(append(ranges[:idx], &RangeItem{Start: int64(ev.OriginServerTS), End: end, isRef: isRef, pairMask: 1}), tail...)
			} else {
				if isRef && rs.historyVisibility == "joined" && int64(ev.OriginServerTS) < rs.historyVisibilityPos {
					ranges = append(ranges, &RangeItem{Start: int64(ev.OriginServerTS), End: rs.historyVisibilityPos, isRef: isRef, pairMask: 3})
				} else {
					ranges = append(ranges, &RangeItem{Start: int64(ev.OriginServerTS), End: -1, isRef: isRef, pairMask: 1})
				}
				isLastOne = true
			}
			isAppend = true
		}
	} else if openClose == 2 {
		index := rs.findRangeIdx(ranges, int64(ev.OriginServerTS), isRef)
		if index >= 0 {
			if ranges[index].pairMask == 1 {
				ranges[index].pairMask = 3
				ranges[index].End = int64(ev.OriginServerTS) - 1
				if index == len(ranges) {
					isLastOne = true
				}
			} else {
				start := ranges[index].Start
				tail := append([]*RangeItem{}, ranges[index:]...)
				ranges = append(append(ranges[:index], &RangeItem{Start: start, End: int64(ev.OriginServerTS) - 1, isRef: isRef}), tail...)
				isAppend = true
				ranges[index+1].Start = int64(ev.OriginServerTS)
				if ranges[index+1].pairMask == 2 {
					ranges[index].pairMask = 2
				} else {
					ranges[index].pairMask = 3
					ranges[index+1].pairMask = 2
				}
			}
		} else {
			idx := -1
			max := int64(-1)
			for i, v := range ranges {
				if v.End != -1 && v.End < int64(ev.OriginServerTS) && (max < v.End || max == -1) {
					max = v.End
					idx = i
				}
			}
			start := rs.createPos
			if idx >= 0 {
				start = ranges[idx].End + 1
			}
			tail := append([]*RangeItem{}, ranges[idx+1:]...)
			ranges = append(append(ranges[:idx+1], &RangeItem{Start: start, End: int64(ev.OriginServerTS) - 1, isRef: isRef, pairMask: 2}), tail...)
			isAppend = true
			if len(tail) == 0 {
				isLastOne = true
			}
		}
	}
	if isLastOne {
		stream := new(feedstypes.StreamEvent)
		stream.Offset = offset
		stream.Ev = ev
		if isRef { // 房间visibility
			if openClose == 1 {
				if ranges[len(ranges)-1].pairMask == 1 {
					rs.historyVisibility = "shared"
				} else {
					rs.historyVisibility = "joined"
				}
			} else {
				rs.historyVisibility = "joined"
			}
			rs.historyVisibilityPos = int64(ev.OriginServerTS)
			if val, ok := rs.curState.Load(ev.Type); ok {
				rs.preState.Store(ev.Type, val)
			}
			rs.curState.Store(ev.Type, stream)
		} else { // member
			if val, ok := rs.curMemberState.Load(*ev.StateKey); ok {
				rs.preMemberState.Store(*ev.StateKey, val)
			}
			rs.curMemberState.Store(*ev.StateKey, stream)
		}
	}
	if isAppend && isRef {
		rs.userMemberViewOk = new(sync.Map)
	}
	return ranges
}

func (rs *RoomState) GetPreState(ev *gomatrixserverlib.ClientEvent) *feedstypes.StreamEvent {
	if common.IsStateClientEv(ev) {
		if ev.Type == "m.room.member" {
			if val, ok := rs.preMemberState.Load(*ev.StateKey); ok {
				return val.(*feedstypes.StreamEvent)
			}
			return nil
		} else if ev.Type == "m.room.aliases" {
			if val, ok := rs.preUserAlias.Load(*ev.StateKey); ok {
				return val.(*feedstypes.StreamEvent)
			}
			return nil
		} else {
			if val, ok := rs.preState.Load(ev.Type); ok {
				return val.(*feedstypes.StreamEvent)
			}
			return nil
		}
	}
	return nil
}

func (rs *RoomState) GetState(typ string, key string) *feedstypes.StreamEvent {
	if typ == "m.room.member" {
		if val, ok := rs.curMemberState.Load(key); ok {
			return val.(*feedstypes.StreamEvent)
		}
	} else if typ == "m.room.aliases" {
		if val, ok := rs.curUserAlias.Load(key); ok {
			return val.(*feedstypes.StreamEvent)
		}
	} else {
		if val, ok := rs.curState.Load(typ); ok {
			return val.(*feedstypes.StreamEvent)
		}
	}

	return nil
}

func (rs *RoomState) GetCreator() string {
	return rs.creator
}

func (rs *RoomState) IsDirect() bool {
	return rs.isDirect
}

func (rs *RoomState) GetFederate() bool {
	return rs.federate
}

func (rs *RoomState) IsChannel() bool {
	return rs.isChannel
}

func (rs *RoomState) IsSecret() bool {
	return rs.isSecret
}

func (rs *RoomState) GetJoinRule() string {
	return rs.joinRule
}

func (rs *RoomState) GetPowerLevels() *common.PowerLevelContent {
	return rs.power
}

func (rs *RoomState) GetJoinMap() *sync.Map {
	return &rs.join
}

func (rs *RoomState) GetLeaveMap() *sync.Map {
	return &rs.leave
}

func (rs *RoomState) IsEncrypted() bool {
	return rs.isEncrypted
}

//shared 历史可见，join 历史不可见
func (rs *RoomState) onUserMembershipChange(user string, visibility, preMembership, membership string, offset int64) {
	var items []*RangeItem
	val, ok := rs.userMemberView.Load(user)
	if ok {
		items = val.([]*RangeItem)
	} else {
		items = []*RangeItem{}
	}

	log.Debugf("onUserMembershipChange user:%s visibility:%s, mem:%s", user, visibility, membership)

	if adapter.GetDebugLevel() == adapter.DEBUG_LEVEL_DEBUG {
		delay := utils.GetRandomSleepSecondsForDebug()
		log.Infow("====================================onUserMembershipChange, before sleep",
			log.KeysAndValues{"user", user, "membership", membership, "offset", offset, "delay", delay})
		time.Sleep(time.Duration(delay*1000) * time.Millisecond)
		log.Infow("====================================onUserMembershipChange, after sleep",
			log.KeysAndValues{"user", user, "membership", membership, "offset", offset, "delay", delay})
	}

	if membership == "leave" || membership == "ban" { //离开标记下
		if preMembership == "join" { //may invite->leave
			if len(items) == 0 {
				items = append(items, &RangeItem{
					Start: rs.createPos,
					End:   -1,
				})
			}

			lastItem := items[len(items)-1]
			lastItem.End = offset

			log.Debugf("onUserMembershipChange user:%s update view start:%d end:%d", user, lastItem.Start, lastItem.End)
			rs.userMemberView.Store(user, items)
		}

		return
	}

	if membership == "join" {
		if len(items) == 0 {
			items = append(items, &RangeItem{
				Start: rs.createPos,
				End:   rs.createPos,
			})
			log.Debugf("onUserMembershipChange user:%s add view start:%d end:%d", user, rs.createPos, rs.createPos)
		}

		//add history tl
		for i := 0; i < len(rs.visiableTl); i++ {
			hsItem := rs.visiableTl[i]
			need := true

			for j := 0; j < len(items); j++ {
				cur := items[j]
				if cur.Start <= hsItem.Start && (cur.End == -1 || cur.End >= hsItem.End) {
					need = false
					break
				}
			}

			if need {
				items = append(items, hsItem)
				log.Debugf("onUserMembershipChange user:%s add view start:%d end:%d", user, hsItem.Start, hsItem.End)
			}
		}

		ri := &RangeItem{}
		ri.End = -1
		ri.Start = offset
		if visibility == "shared" {
			ri.Start = rs.historyVisibilityPos
		}

		items = append(items, ri)
		log.Debugf("onUserMembershipChange user:%s add view start:%d end:%d", user, ri.Start, ri.End)

		rs.userMemberView.Store(user, items)
	}
}

func (rs *RoomState) CheckEventVisibility(user string, pos int64) bool {
	val, ok := rs.userMemberView.Load(user) //may sync happen before history-visibility event arrive, no user view build
	if !ok {
		return true
	}

	visible := false
	userMemberView := val.([]*RangeItem)
	for i := 0; i < len(userMemberView); i++ {
		item := userMemberView[i]
		// log.Debugf("------- CheckEventVisibility: start: %d, end: %d, ref: %t, pair: %d", item.Start, item.End, item.isRef, item.pairMask)
		if (pos >= item.Start && item.End == -1) || (pos >= item.Start && pos <= item.End) {
			visible = true
			break
		}
	}

	if !visible {
		log.Infof("RoomState.CheckEventVisibility false user %s pos %d", user, pos)
	}

	return visible
}

func (rs *RoomState) GetEventVisibility(user string) []RangeItem {
	if len(user) == 0 {
		return nil
	}

	val, ok := rs.userMemberView.Load(user) // before history-visibility event arrive, no user view build
	if !ok {
		return []RangeItem{{0, -1, false, 0}}
	}

	if adapter.GetDebugLevel() == adapter.DEBUG_LEVEL_DEBUG {
		delay := utils.GetRandomSleepSecondsForDebug()
		log.Infow("====================================GetEventVisibility, before sleep",
			log.KeysAndValues{"user", user, "delay", delay})
		time.Sleep(time.Duration(delay*1000) * time.Millisecond)
		log.Infow("====================================GetEventVisibility, after sleep",
			log.KeysAndValues{"user", user, "delay", delay})
	}

	userMemberView := val.([]*RangeItem)
	items := []RangeItem{}
	for i := 0; i < len(userMemberView); i++ {
		items = append(items, *userMemberView[i])
	}

	return items
}

func (rs *RoomState) findRangeIdx(ranges []*RangeItem, evTS int64, isRef bool) int {
	index := -1
	for i, v := range ranges {
		if v.isRef == isRef && v.Start <= evTS && (evTS < v.End || v.End == -1) {
			index = i
			break
		}
	}

	return index
}

type RoomCurStateRepo struct {
	roomState sync.Map
	persist   model.SyncAPIDatabase
}

type RoomCurStateLoadedData struct {
	Rooms int
}

func (tl *RoomCurStateRepo) GetLoadedData() *RoomCurStateLoadedData {
	data := RoomCurStateLoadedData{
		Rooms: 0,
	}
	tl.roomState.Range(func(key interface{}, value interface{}) bool {
		data.Rooms++
		return true
	})

	return &data
}

func (tl *RoomCurStateRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (repo *RoomCurStateRepo) GetRoomState(roomID string) *RoomState {
	if val, ok := repo.roomState.Load(roomID); ok {
		return val.(*RoomState)
	}

	return nil
}

//RoomStateTimeLineRepo 保证state 会重演,
//consumer 收到event会触发,或者sync 主动触发
func (repo *RoomCurStateRepo) onEvent(ev *gomatrixserverlib.ClientEvent, offset int64, backfill bool) {
	var rs *RoomState
	val, ok := repo.roomState.Load(ev.RoomID)
	if ok {
		rs = val.(*RoomState)
	} else if !backfill {
		rs = newRoomState()
		log.Debugf("=-------==-=-=-==-= new %s %s", ev.RoomID, ev.Type)
		repo.roomState.Store(ev.RoomID, rs)
	} else {
		return
	}

	rs.onEvent(ev, offset, backfill)
}

func (repo *RoomCurStateRepo) removeRoomState(roomID string) {
	repo.roomState.Delete(roomID)
}
