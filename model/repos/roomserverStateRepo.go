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
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/storage/model"
)

type DomainTLItem struct {
	tl        *feedstypes.TimeLines
	MemberCnt int   `json:"member_cnt"`
	offset    int64 `json:"offset"`
}

type eventFeed struct {
	ev     *gomatrixserverlib.Event
	offset int64
}

func (ef *eventFeed) GetOffset() int64 {
	return ef.offset
}

type RoomServerState struct {
	//change only state event
	roomId      string
	RoomNid     int64
	SnapId      int64
	IsEncrypted bool `json:"is_encrypt"`
	IsFed       bool `json:"is_fed"`
	//是否是直聊房间
	isDirect bool `json:"is_direct"`

	Creator           *gomatrixserverlib.Event `json:"create_ev"`
	JoinRule          *gomatrixserverlib.Event `json:"join_rule_ev"`
	HistoryVisibility *gomatrixserverlib.Event `json:"history_visibility_ev"`
	Visibility        *gomatrixserverlib.Event `json:"visibility_ev"`
	Name              *gomatrixserverlib.Event `json:"name_ev"`
	Topic             *gomatrixserverlib.Event `json:"topic_ev"`
	Desc              *gomatrixserverlib.Event `json:"desc_ev"`
	CanonicalAlias    *gomatrixserverlib.Event `json:"canonical_alias_ev"`
	Alias             *gomatrixserverlib.Event `json:"alias_ev"`
	Power             *gomatrixserverlib.Event `json:"power_ev"`
	GuestAccess       *gomatrixserverlib.Event `json:"guest_access"`

	Avatar *gomatrixserverlib.Event `json:"avatar_ev"`
	Pin    *gomatrixserverlib.Event `json:"pin_ev"`

	join        sync.Map
	leave       sync.Map
	invite      sync.Map
	thirdInvite sync.Map

	JoinExport        map[string]*gomatrixserverlib.Event `json:"join_map"`
	LeaveExport       map[string]*gomatrixserverlib.Event `json:"leave_map"`
	InviteExport      map[string]*gomatrixserverlib.Event `json:"invite_map"`
	ThirdInviteExport map[string]*gomatrixserverlib.Event `json:"third_invite_map"`
	DomainsExport     map[string]*DomainTLItem            `json:"domains"`

	domainTl sync.Map
	Version  int32 `json:"version"`
	//change often
	ext *types.RoomStateExt
}

//remain
func (rs *RoomServerState) GetAllState() []gomatrixserverlib.Event {
	var res SortedEventArray
	res = append(res, *rs.Creator)
	if rs.JoinRule != nil {
		res = append(res, *rs.JoinRule)
	}
	if rs.HistoryVisibility != nil {
		res = append(res, *rs.HistoryVisibility)
	}
	if rs.Visibility != nil {
		res = append(res, *rs.Visibility)
	}
	if rs.Name != nil {
		res = append(res, *rs.Name)
	}
	if rs.Topic != nil {
		res = append(res, *rs.Topic)
	}
	if rs.CanonicalAlias != nil {
		res = append(res, *rs.CanonicalAlias)
	}
	if rs.Alias != nil {
		res = append(res, *rs.Alias)
	}
	if rs.Power != nil {
		res = append(res, *rs.Power)
	}
	if rs.Avatar != nil {
		res = append(res, *rs.Avatar)
	}
	if rs.Pin != nil {
		res = append(res, *rs.Pin)
	}
	if rs.GuestAccess != nil {
		res = append(res, *rs.GuestAccess)
	}
	rs.join.Range(func(key, value interface{}) bool {
		res = append(res, *value.(*gomatrixserverlib.Event))
		return true
	})
	rs.leave.Range(func(key, value interface{}) bool {
		res = append(res, *value.(*gomatrixserverlib.Event))
		return true
	})
	rs.invite.Range(func(key, value interface{}) bool {
		res = append(res, *value.(*gomatrixserverlib.Event))
		return true
	})
	rs.thirdInvite.Range(func(key, value interface{}) bool {
		res = append(res, *value.(*gomatrixserverlib.Event))
		return true
	})

	sort.Sort(res)
	return res
}

type SortedEventArray []gomatrixserverlib.Event

func (list SortedEventArray) Len() int {
	return len(list)
}

func (list SortedEventArray) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list SortedEventArray) Less(i, j int) bool {
	ei := list[i].EventNID()
	ej := list[j].EventNID()

	return ei < ej
}

func (rs *RoomServerState) GetRefs(ev *gomatrixserverlib.Event) (string, []byte) {
	switch ev.Type() {
	case "m.room.create":
		fallthrough
	case "m.room.member":
		fallthrough
	case "m.room.power_levels":
		fallthrough
	case "m.room.third_party_invite":
		fallthrough
	case "m.room.join_rules":
		fallthrough
	case "m.room.history_visibility": //每个房间都有重演，所以不需要保存历史
		fallthrough
	case "m.room.visibility":
		fallthrough
	case "m.room.name":
		fallthrough
	case "m.room.topic":
		fallthrough
	case "m.room.desc":
		fallthrough
	case "m.room.aliases":
		fallthrough
	case "m.room.avatar":
		fallthrough
	case "m.room.pinned_events":
		fallthrough
	case "m.room.canonical_alias":
		log.Debugf("GetRefs type:%s id:%s", ev.Type(), rs.ext.PreStateId)
		return rs.ext.PreStateId, []byte{}
	}

	log.Debugf("GetRefs type:%s id:%s", ev.Type(), rs.ext.PreMsgId)
	return rs.ext.PreMsgId, []byte{}
}

func (rs *RoomServerState) HasUpdate() bool {
	return rs.ext.HasUpdate
}

func (rs *RoomServerState) GetLastMsgID() string {
	return rs.ext.LastMsgId
}

//获取preEv 以及是否state-ev
func (rs *RoomServerState) GetPreEvent(ev *gomatrixserverlib.Event) (*gomatrixserverlib.Event, bool) {
	switch ev.Type() {
	case "m.room.create":
		return rs.Creator, true
	case "m.room.member":
		if val, ok := rs.join.Load(*ev.StateKey()); ok {
			return val.(*gomatrixserverlib.Event), true
		} else if val, ok := rs.leave.Load(*ev.StateKey()); ok {
			return val.(*gomatrixserverlib.Event), true
		} else if val, ok := rs.invite.Load(*ev.StateKey()); ok {
			return val.(*gomatrixserverlib.Event), true
		}
		return nil, true
	case "m.room.power_levels":
		return rs.Power, true
	case "m.room.third_party_invite":
		if val, ok := rs.thirdInvite.Load(*ev.StateKey()); ok {
			return val.(*gomatrixserverlib.Event), true
		}
		return nil, true
	case "m.room.join_rules":
		return rs.JoinRule, true
	case "m.room.history_visibility": //每个房间都有重演，所以不需要保存历史
		return rs.HistoryVisibility, true
	case "m.room.visibility":
		return rs.Visibility, true
	case "m.room.name":
		return rs.Name, true
	case "m.room.topic":
		return rs.Topic, true
	case "m.room.desc":
		return rs.Desc, true
	case "m.room.canonical_alias":
		return rs.CanonicalAlias, true
	case "m.room.aliases":
		return rs.Alias, true
	case "m.room.avatar":
		return rs.Avatar, true
	case "m.room.guest_access":
		return rs.GuestAccess, true
	case "m.room.pinned_events":
		return rs.Pin, true
	case "m.room.encryption":
		return nil, true
	}

	return nil, false
}

func (rs *RoomServerState) init(roomId string) {
	rs.roomId = roomId
	rs.RoomNid = -1
	rs.ext = &types.RoomStateExt{
		Domains: make(map[string]int64),
	}
}

func (rs *RoomServerState) GetLastDomainOffset(ev *gomatrixserverlib.Event, domain string) int64 {
	if v, ok := rs.ext.Domains[domain]; ok {
		if v > ev.DomainOffset() {
			return v
		}
	}
	return ev.DomainOffset()
}

func (rs *RoomServerState) UpdateDomainExt(domains map[string]int64) {
	rs.ext.Domains = domains
}

func (rs *RoomServerState) getDomainTL(domain string, create bool) *DomainTLItem {
	if val, ok := rs.domainTl.Load(domain); ok {
		return val.(*DomainTLItem)
	}
	if create {
		item := new(DomainTLItem)
		item.tl = feedstypes.NewEvTimeLines(16, false)
		actual, loaded := rs.domainTl.LoadOrStore(domain, item)
		if loaded {
			item = actual.(*DomainTLItem)
		}
		return item
	}
	return nil
}

func (rs *RoomServerState) GetLastState() []int64 {
	var state []int64
	rs.domainTl.Range(func(key, value interface{}) bool {
		tl := value.(*DomainTLItem).tl
		_, feedUp := tl.GetFeedRange()

		state = append(state, feedUp)
		return true
	})

	return state
}

func (rs *RoomServerState) GetSnapId() int64 {
	return rs.SnapId
}

func (rs *RoomServerState) SetSnapId(snap int64) {
	rs.SnapId = snap
}

func (rs *RoomServerState) GetRoomNID() int64 {
	return rs.RoomNid
}

func (rs *RoomServerState) SetRoomNID(nid int64) {
	rs.RoomNid = nid
}

func (rs *RoomServerState) GetRoomID() string {
	return rs.roomId
}

func (rs *RoomServerState) IsDirect() bool {
	return rs.isDirect
}

func (rs *RoomServerState) calcToken() string {
	var domains []string
	rs.domainTl.Range(func(key, value interface{}) bool {
		domains = append(domains, key.(string))
		return true
	})
	sort.Strings(domains)
	if len(domains) == 1 {
		val, _ := rs.domainTl.Load(domains[0])
		tl := val.(*DomainTLItem).tl
		_, feedUp := tl.GetFeedRange()
		return fmt.Sprintf("%s:%d", domains[0], feedUp)
	} else {
		token := ""
		for k, v := range domains {
			val, _ := rs.domainTl.Load(v)
			tl := val.(*DomainTLItem).tl
			_, feedUp := tl.GetFeedRange()
			if k == 0 {
				token = fmt.Sprintf("%s:%d", v, feedUp)
			} else {
				token = token + fmt.Sprintf("_%s:%d", v, feedUp)
			}
		}
		return token
	}
}

func (rs *RoomServerState) serialize() ([]byte, error) {
	rs.Version = rs.Version + 1
	rs.JoinExport = make(map[string]*gomatrixserverlib.Event)
	rs.LeaveExport = make(map[string]*gomatrixserverlib.Event)
	rs.InviteExport = make(map[string]*gomatrixserverlib.Event)
	rs.ThirdInviteExport = make(map[string]*gomatrixserverlib.Event)
	rs.DomainsExport = make(map[string]*DomainTLItem)

	rs.join.Range(func(key, value interface{}) bool {
		rs.JoinExport[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})
	rs.leave.Range(func(key, value interface{}) bool {
		rs.LeaveExport[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})
	rs.invite.Range(func(key, value interface{}) bool {
		rs.InviteExport[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})
	rs.thirdInvite.Range(func(key, value interface{}) bool {
		rs.ThirdInviteExport[key.(string)] = value.(*gomatrixserverlib.Event)
		return true
	})

	var item *DomainTLItem
	rs.domainTl.Range(func(key, value interface{}) bool {
		item = value.(*DomainTLItem)
		rs.DomainsExport[key.(string)] = item
		return true
	})

	return json.Marshal(rs)
}

func (rs *RoomServerState) GetDepth() int64 {
	return rs.ext.Depth
}

func (rs *RoomServerState) UpdateDepth(depth int64) {
	rs.ext.Depth = depth
}

func (rs *RoomServerState) GetOutEventOffset() int64 {
	return rs.ext.OutEventOffset
}

func (rs *RoomServerState) UpdateOutEventOffset(outEventOffset int64) {
	rs.ext.OutEventOffset = outEventOffset
}

func (rs *RoomServerState) UpdateMsgExt(preStateId, lastStateId, preMsgId, lastMsgId string) {
	rs.ext.PreStateId = preStateId
	rs.ext.LastStateId = lastStateId
	rs.ext.PreMsgId = preMsgId
	rs.ext.LastMsgId = lastMsgId
}

func (rs *RoomServerState) onCreateEv(ev *gomatrixserverlib.Event) {
	rs.Creator = ev
	con := common.CreateContent{}
	json.Unmarshal(ev.Content(), &con)
	if con.Federate != nil {
		rs.IsFed = *con.Federate
	} else {
		rs.IsFed = false
	}
	if con.IsDirect != nil {
		rs.isDirect = *con.IsDirect
	} else {
		rs.isDirect = false
	}
}

func (rs *RoomServerState) onMemberEv(ev *gomatrixserverlib.Event, offset int64) {
	domain, res := common.DomainFromID(*ev.StateKey())
	if res != nil {
		return //unknown domain
	}
	tgtItem := rs.getDomainTL(domain, true)
	member := external.MemberContent{}
	json.Unmarshal(ev.Content(), &member)
	pre := ""
	if _, ok := rs.join.Load(*ev.StateKey()); ok {
		pre = "join"
	} else if _, ok := rs.leave.Load(*ev.StateKey()); ok {
		pre = "leave"
	} else if _, ok := rs.invite.Load(*ev.StateKey()); ok {
		pre = "invite"
	}
	if member.Membership == "join" {
		rs.join.Store(*ev.StateKey(), ev)
		log.Debugf("onEvent type:%s offset:%d add member:%s", ev.Type(), offset, *ev.StateKey())
		rs.thirdInvite.Delete(*ev.StateKey())
	} else if member.Membership == "invite" {
		rs.invite.Store(*ev.StateKey(), ev)
	} else if member.Membership == "leave" || member.Membership == "ban" {
		rs.leave.Store(*ev.StateKey(), ev)
		rs.thirdInvite.Delete(*ev.StateKey())
	}
	if member.Membership != pre {
		if pre == "join" {
			rs.join.Delete(*ev.StateKey())
			tgtItem.MemberCnt = tgtItem.MemberCnt - 1
			log.Debugf("onEvent type:%s offset:%d del member:%s num:%d", ev.Type(), offset, *ev.StateKey(), tgtItem.MemberCnt)
		} else if pre == "invite" {
			rs.invite.Delete(*ev.StateKey())
		} else if pre == "leave" {
			rs.leave.Delete(*ev.StateKey())
		}
		if member.Membership == "join" {
			tgtItem.MemberCnt = tgtItem.MemberCnt + 1
			log.Debugf("onEvent type:%s offset:%d add member:%s num:%d", ev.Type(), offset, *ev.StateKey(), tgtItem.MemberCnt)
		}
	}
}

func (rs *RoomServerState) onEventUpdate(ev *gomatrixserverlib.Event, offset int64) {
	switch ev.Type() {
	case "m.room.create":
		rs.onCreateEv(ev)
	case "m.room.member":
		rs.onMemberEv(ev, offset)
	case "m.room.power_levels":
		rs.Power = ev
	case "m.room.third_party_invite":
		rs.thirdInvite.Store(*ev.StateKey(), ev)
	case "m.room.join_rules":
		rs.JoinRule = ev
	case "m.room.history_visibility": //每个房间都有重演，所以不需要保存历史
		rs.HistoryVisibility = ev
	case "m.room.visibility":
		rs.Visibility = ev
	case "m.room.name":
		rs.Name = ev
	case "m.room.topic":
		rs.Topic = ev
	case "m.room.desc":
		rs.Desc = ev
	case "m.room.canonical_alias":
		rs.CanonicalAlias = ev
	case "m.room.aliases":
		rs.Alias = ev
	case "m.room.avatar":
		rs.Avatar = ev
	case "m.room.pinned_events":
		rs.Pin = ev
	case "m.room.encryption":
		rs.IsEncrypted = true
	case "m.room.guest_access":
		rs.GuestAccess = ev
	}
}

func (rs *RoomServerState) onEventCheckUpdate(ev, pre *gomatrixserverlib.Event, domain string, isState, recover bool) bool {
	// lock update msgid, depth, offset
	if common.CheckValidDomain(domain, adapter.GetDomainCfg()) { //本域
		rs.ext.HasUpdate = isState
		if recover {
			if isState {
				rs.ext.PreStateId = rs.ext.LastStateId
				rs.ext.LastStateId = ev.EventID()
				rs.ext.LastMsgId = ev.EventID()

			} else {
				rs.ext.PreMsgId = rs.ext.LastMsgId
				rs.ext.LastMsgId = ev.EventID()
			}
		}
		return isState

	} else { //不是本域，只处理状态
		if isState == true && (pre == nil || (pre != nil && (pre.OriginServerTS() <= ev.OriginServerTS()) && (pre.EventID() != ev.EventID()))) {
			return true
		} else {
			return false
		}
	}
}

func (rs *RoomServerState) onEvent(ev *gomatrixserverlib.Event, offset int64, recover bool) {
	log.Debugf("onEvent type:%s offset:%d", ev.Type(), offset)
	pre, isState := rs.GetPreEvent(ev)
	domain, res := common.DomainFromID(ev.Sender())
	if res != nil {
		return //unknown domain
	}
	rs.ext.HasUpdate = rs.onEventCheckUpdate(ev, pre, domain, isState, recover)
	if rs.ext.HasUpdate == false {
		return
	}
	rs.onEventUpdate(ev, offset)
	item := rs.getDomainTL(domain, true)
	ef := new(eventFeed)
	ef.ev = ev
	ef.offset = offset
	item.tl.Add(ef)
}

func (rs *RoomServerState) GetJoinMap() *sync.Map {
	return &rs.join
}

func (rs *RoomServerState) GetLeaveMap() *sync.Map {
	return &rs.leave
}

func (rs *RoomServerState) GetInviteMap() *sync.Map {
	return &rs.invite
}

func (rs *RoomServerState) GetThirdInviteMap() *sync.Map {
	return &rs.thirdInvite
}

func (rs *RoomServerState) GetDomainTlMap() *sync.Map {
	return &rs.domainTl
}

func (rs *RoomServerState) Create() (*gomatrixserverlib.Event, error) {
	if rs == nil {
		return nil, nil
	}
	return rs.Creator, nil
}

func (rs *RoomServerState) JoinRules() (*gomatrixserverlib.Event, error) {
	return rs.JoinRule, nil
}

func (rs *RoomServerState) PowerLevels() (*gomatrixserverlib.Event, error) {
	return rs.Power, nil
}

func (rs *RoomServerState) Member(stateKey string) (*gomatrixserverlib.Event, error) {
	if val, ok := rs.join.Load(stateKey); ok {
		if ok {
			return val.(*gomatrixserverlib.Event), nil
		}
	}

	if val, ok := rs.leave.Load(stateKey); ok {
		if ok {
			return val.(*gomatrixserverlib.Event), nil
		}
	}

	if val, ok := rs.invite.Load(stateKey); ok {
		if ok {
			return val.(*gomatrixserverlib.Event), nil
		}
	}

	return nil, nil
}

func (rs *RoomServerState) ThirdPartyInvite(stateKey string) (*gomatrixserverlib.Event, error) {
	if val, ok := rs.thirdInvite.Load(stateKey); ok {
		if ok {
			return val.(*gomatrixserverlib.Event), nil
		}
	}

	return nil, nil
}

func (rs *RoomServerState) flush(cache service.Cache) {
	bs := time.Now().UnixNano() / 1000
	bytes, err := rs.serialize()
	token := rs.calcToken()
	if err != nil {
		log.Errorf("FlushRoomState bytes: %s, token: %s err:%s", string(bytes), token, err.Error())
		return
	}
	if err := cache.SetRoomState(rs.GetRoomID(), bytes, token); err != nil {
		log.Warnf("flush room:%s roomstate to cache err:%v", rs.GetRoomID(), err)
	}
	log.Info("flush roomstate to cache spend:%d us", time.Now().UnixNano()/1000-bs)
}

type RoomServerCurStateRepo struct {
	persist model.RoomServerDatabase
	cache   service.Cache

	queryHitCounter mon.LabeledCounter
}

func NewRoomServerCurStateRepo(
	persist model.RoomServerDatabase,
	cache service.Cache,
	queryHitCounter mon.LabeledCounter,
) *RoomServerCurStateRepo {
	return &RoomServerCurStateRepo{
		persist:         persist,
		cache:           cache,
		queryHitCounter: queryHitCounter,
	}
}

func (repo *RoomServerCurStateRepo) GetCache() service.Cache {
	return repo.cache
}

func (repo *RoomServerCurStateRepo) parseCacheRoomState(rs *RoomServerState) {
	var states []*gomatrixserverlib.Event
	if rs.Creator != nil {
		states = append(states, rs.Creator)
		rs.Creator = nil
	}
	if rs.JoinRule != nil {
		states = append(states, rs.JoinRule)
		rs.JoinRule = nil
	}
	if rs.HistoryVisibility != nil {
		states = append(states, rs.HistoryVisibility)
		rs.HistoryVisibility = nil
	}
	if rs.Visibility != nil {
		states = append(states, rs.Visibility)
		rs.Visibility = nil
	}
	if rs.Name != nil {
		states = append(states, rs.Name)
		rs.Name = nil
	}
	if rs.Power != nil {
		states = append(states, rs.Power)
		rs.Power = nil
	}
	if rs.Topic != nil {
		states = append(states, rs.Topic)
		rs.Topic = nil
	}
	if rs.CanonicalAlias != nil {
		states = append(states, rs.CanonicalAlias)
		rs.CanonicalAlias = nil
	}
	if rs.Alias != nil {
		states = append(states, rs.Alias)
		rs.Alias = nil
	}
	if rs.Avatar != nil {
		states = append(states, rs.Avatar)
		rs.Avatar = nil
	}
	if rs.Pin != nil {
		states = append(states, rs.Pin)
		rs.Pin = nil
	}
	if rs.GuestAccess != nil {
		states = append(states, rs.GuestAccess)
		rs.GuestAccess = nil
	}
	if rs.JoinExport != nil {
		for _, v := range rs.JoinExport {
			states = append(states, v)
		}
	}
	if rs.LeaveExport != nil {
		for _, v := range rs.LeaveExport {
			states = append(states, v)
		}
	}
	if rs.InviteExport != nil {
		for _, v := range rs.InviteExport {
			states = append(states, v)
		}
	}
	if rs.ThirdInviteExport != nil {
		for _, v := range rs.ThirdInviteExport {
			states = append(states, v)
		}
	}
	sort.Slice(states, func(i, j int) bool { return states[i].EventNID() < states[j].EventNID() })
	for _, v := range states {
		rs.onEvent(v, v.EventNID(), true)
	}
}

func (repo *RoomServerCurStateRepo) getRoomStateFromCache(roomid string) (*RoomServerState, error) {
	log.Infof("RoomServerCurStateRepo GetRoomState %s in cache", roomid)
	repo.queryHitCounter.WithLabelValues("cache", "RoomServerCurStateRepo", "GetRoomState").Add(1)
	bytes, _, err := repo.cache.GetRoomState(roomid)
	if err == nil {
		if bytes == nil {
			return nil, nil
		} else {
			var rs RoomServerState
			rs.init(roomid)
			if err := json.Unmarshal(bytes, &rs); err != nil {
				return nil, err
			} else {
				repo.parseCacheRoomState(&rs)
				return &rs, nil
			}
		}
	} else {
		return nil, err
	}
}

func (repo *RoomServerCurStateRepo) getRoomStateFromDb(ctx context.Context, roomid string) (*RoomServerState, error) {
	log.Infof("RoomServerCurStateRepo GetRoomState %s in db", roomid)
	repo.queryHitCounter.WithLabelValues("db", "RoomServerCurStateRepo", "GetRoomState").Add(1)
	bs := time.Now().UnixNano() / 1000000
	events, ids, err := repo.persist.GetRoomStates(ctx, roomid)
	spend := time.Now().UnixNano()/1000000 - bs
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomServerCurStateRepo getRoomStateFromDb GetRoomStates roomid:%s spend:%d ms", types.DB_EXCEED_TIME, roomid, spend)
	} else {
		log.Infof("load db RoomServerCurStateRepo getRoomStateFromDb GetRoomStates roomid:%s spend:%d ms", roomid, spend)
	}
	if err != nil {
		log.Errorf("RoomServerCurStateRepo GetRoomState %s in db err:%v", roomid, err)
		return nil, err
	}
	rs := new(RoomServerState)
	rs.init(roomid)
	for idx, ev := range events {
		rs.onEvent(ev, ids[idx], true)
	}
	bs = time.Now().UnixNano() / 1000000
	roomNid, snapId, depth, err := repo.persist.RoomInfo(ctx, roomid)
	spend = time.Now().UnixNano()/1000000 - bs
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomServerCurStateRepo getRoomStateFromDb RoomInfo roomid:%s spend:%d ms", types.DB_EXCEED_TIME, roomid, spend)
	} else {
		log.Infof("load db RoomServerCurStateRepo getRoomStateFromDb RoomInfo roomid:%s spend:%d ms", roomid, spend)
	}
	if err != nil {
		log.Errorf("getRoomStateFromDb RoomInfo snap room:%s in db err:%v depth:%d", roomid, err, depth)
		return nil, err
	}
	rs.SetSnapId(snapId)
	rs.SetRoomNID(roomNid)
	return rs, nil
}

func (repo *RoomServerCurStateRepo) getRoomStateExtFromDb(ctx context.Context, roomid string) (*types.RoomStateExt, error) {
	log.Infof("RoomServerCurStateRepo GetRoomStateExt %s in db", roomid)
	repo.queryHitCounter.WithLabelValues("db", "RoomServerCurStateRepo", "GetRoomStateExt").Add(1)
	ext := &types.RoomStateExt{
		Domains: make(map[string]int64),
	}
	bs := time.Now().UnixNano() / 1000000
	roomNid, _, depth, err := repo.persist.RoomInfo(ctx, roomid)
	spend := time.Now().UnixNano()/1000000 - bs
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms RoomServerCurStateRepo getRoomStateExtFromDb RoomInfo roomid:%s spend:%d ms", types.DB_EXCEED_TIME, roomid, spend)
	} else {
		log.Infof("load db RoomServerCurStateRepo getRoomStateExtFromDb RoomInfo roomid:%s spend:%d ms", roomid, spend)
	}
	if err != nil {
		if err != sql.ErrNoRows {
			log.Errorf("getRoomStateExtFromDb RoomInfo snap room:%s in db err:%v", roomid, err)
			return nil, err
		} else {
			log.Warnf("getRoomStateExtFromDb RoomInfo snap room:%s in db err:%v", roomid, err)
		}
	}
	ext.Depth = depth
	bs = time.Now().UnixNano() / 1000000
	domains, offsets, err := repo.persist.GetRoomDomainsOffset(ctx, roomNid)
	spend = time.Now().UnixNano()/1000000 - bs
	if spend > types.DB_EXCEED_TIME {
		log.Warnf("load db exceed %d ms getRoomStateExtFromDb GetRoomDomainsOffset RoomInfo roomid:%s spend:%d ms", types.DB_EXCEED_TIME, roomid, spend)
	} else {
		log.Infof("load db RoomServerCurStateRepo getRoomStateExtFromDb GetRoomDomainsOffset roomid:%s spend:%d ms", roomid, spend)
	}
	if err != nil {
		if err != sql.ErrNoRows {
			log.Errorf("getRoomStateExtFromDb GetRoomDomainsOffset snap room:%d in db err:%v", roomNid, err)
			return nil, err
		} else {
			log.Warnf("getRoomStateExtFromDb GetRoomDomainsOffset snap room:%d in db err:%v", roomNid, err)
			return ext, nil
		}
	}
	for idx, domain := range domains {
		ext.Domains[domain] = offsets[idx]
	}
	return ext, nil
}

func (repo *RoomServerCurStateRepo) GetRoomState(ctx context.Context, roomid string) *RoomServerState {
	bs := time.Now().UnixNano() / 1000
	rs, err := repo.getRoomStateFromCache(roomid)
	log.Infof("load roomstate roomid:%s from cache spend:%d us", roomid, time.Now().UnixNano()/1000-bs)
	if err != nil || rs == nil {
		bs = time.Now().UnixNano() / 1000000
		rs, err = repo.getRoomStateFromDb(ctx, roomid)
		log.Infof("load roomstate roomid:%s from db spend:%d ms", roomid, time.Now().UnixNano()/1000000-bs)
		if err != nil {
			return nil
		} else {
			repo.FlushRoomState(rs)
		}
	}
	bs = time.Now().UnixNano() / 1000
	ext, err := repo.getRoomStateExtFromCache(roomid)
	log.Info("load roomstateext from cache spend:%d us", time.Now().UnixNano()/1000-bs)
	if err != nil || ext == nil {
		bs = time.Now().UnixNano() / 1000000
		ext, err = repo.getRoomStateExtFromDb(ctx, roomid)
		log.Info("load roomstateext roomid:%s from db spend:%d ms", roomid, time.Now().UnixNano()/1000000-bs)
		if err != nil {
			return nil
		} else {
			ext.PreMsgId = rs.ext.PreMsgId
			ext.LastMsgId = rs.ext.LastMsgId
			ext.PreStateId = rs.ext.PreStateId
			ext.LastStateId = rs.ext.LastStateId
			ext.HasUpdate = false
			if e := repo.cache.SetRoomStateExt(roomid, ext); e != nil {
				log.Warnf("RoomServerCurStateRepo GetRoomState roomid:%s set db to cache err:%s", roomid, e.Error())
			}
		}
	}
	rs.ext = ext
	return rs
}

func (repo *RoomServerCurStateRepo) FlushRoomState(rs *RoomServerState) {
	if rs == nil {
		return
	}
	rs.flush(repo.cache)
}

func (repo *RoomServerCurStateRepo) OnEvent(ctx context.Context, ev *gomatrixserverlib.Event, offset int64, rs *RoomServerState) *RoomServerState {
	if rs == nil {
		if ev.Type() != "m.room.create" {
			rs = repo.GetRoomState(ctx, ev.RoomID())
		} else {
			rs = new(RoomServerState)
			rs.init(ev.RoomID())
		}
	}
	rs.onEvent(ev, offset, false)
	return rs
}

func (repo *RoomServerCurStateRepo) OnEventRecover(ctx context.Context, ev *gomatrixserverlib.Event, offset int64) *RoomServerState {
	rs := repo.GetRoomState(ctx, ev.RoomID())
	if rs != nil {
		rs.onEvent(ev, offset, true)
	}
	return rs
}

func (repo *RoomServerCurStateRepo) getRoomStateExtFromCache(roomid string) (*types.RoomStateExt, error) {
	log.Infof("RoomServerCurStateRepo GetRoomStateExt %s in cache", roomid)
	repo.queryHitCounter.WithLabelValues("cache", "RoomServerCurStateRepo", "GetRoomStateExt").Add(1)
	return repo.cache.GetRoomStateExt(roomid)
}

func (repo *RoomServerCurStateRepo) GetRoomStateExt(ctx context.Context, roomid string) (stateExt *types.RoomStateExt, err error) {
	ext, err := repo.getRoomStateExtFromCache(roomid)
	if err != nil || ext == nil {
		log.Infof("RoomServerCurStateRepo GetRoomStateExt miss roomid:%s", roomid)
		ext, err = repo.getRoomStateExtFromDb(ctx, roomid)
		if err == nil && ext != nil {
			if e := repo.cache.SetRoomStateExt(roomid, ext); e != nil {
				log.Warnf("RoomServerCurStateRepo GetRoomStateExt roomid:%s set db to cache err:%s", roomid, e.Error())
			}
		}
	}
	return ext, err
}

func (repo *RoomServerCurStateRepo) UpdateRoomStateExt(roomid string, ext map[string]interface{}) (err error) {
	return repo.cache.UpdateRoomStateExt(roomid, ext)
}
