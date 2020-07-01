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
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/finogeeks/ligase/skunkworks/util/cas"
)

type DomainTLItem struct {
	tl        *feedstypes.TimeLines
	MemberCnt int   `json:"member_cnt"`
	Offset    int64 `json:"offset"`
}

type eventFeed struct {
	ev     *gomatrixserverlib.Event
	offset int64
}

func (ef *eventFeed) GetOffset() int64 {
	return ef.offset
}

type RoomServerState struct {
	serverDomain []string
	lastStateId  []string
	lastStateRef [][]byte
	lastMsgId    []string
	lastMsgRef   [][]byte

	roomId      string
	RoomNid     int64
	SnapId      int64
	Depth       int64
	HasUpdate   bool
	IsEncrypted bool `json:"is_encrypt"`
	IsFed       bool `json:"is_fed"`
	isDirect    bool `json:"is_direct"`

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
	flushed  bool
	dirt     int32
	Version  int32 `json:"version"`
}

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
		log.Debugf("GetRefs type:%s id:%s", ev.Type(), rs.lastStateId[0])
		return rs.lastStateId[0], rs.lastStateRef[0]
	}

	log.Debugf("GetRefs type:%s id:%s", ev.Type(), rs.lastMsgId[0])
	return rs.lastMsgId[0], rs.lastMsgRef[0]
}

func (rs *RoomServerState) GetLastMsgID() string {
	return rs.lastMsgId[1]
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

func (rs *RoomServerState) init(roomId string, domain []string) {
	rs.roomId = roomId
	rs.serverDomain = domain
	rs.lastStateId = []string{"", ""}
	rs.lastStateRef = [][]byte{{}, {}}
	rs.lastMsgId = []string{"", ""}
	rs.lastMsgRef = [][]byte{{}, {}}
	rs.dirt = 0
	rs.flushed = false
	rs.RoomNid = -1
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

func (rs *RoomServerState) setDepth(depth int64) {
	rs.Depth = depth
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

func (rs *RoomServerState) AllocDepth() int64 {
	depth := atomic.AddInt64(&rs.Depth, 1)
	return depth
}

func (rs *RoomServerState) UpdateDepth(val int64) {
	cas.LargeAndSetInt64(val, &rs.Depth)
}

func (rs *RoomServerState) AllocDomainOffset(ev *gomatrixserverlib.Event) {
	domain, res := common.DomainFromID(ev.Sender())
	if res != nil {
		return //unknown domain
	}

	//val, _ := rs.domainTl.Load(domain)
	//item := val.(*DomainTLItem)
	item := rs.getDomainTL(domain, true)

	if ev.DomainOffset() == 0 {
		offset := atomic.AddInt64(&item.Offset, 1)
		ev.SetDomainOffset(offset)
	}
}

func (rs *RoomServerState) onEvent(ev *gomatrixserverlib.Event, offset int64) {
	log.Debugf("onEvent type:%s offset:%d", ev.Type(), offset)
	pre, isState := rs.GetPreEvent(ev)
	rs.HasUpdate = false

	domain, res := common.DomainFromID(ev.Sender())
	if res != nil {
		return //unknown domain
	}

	if common.CheckValidDomain(domain, rs.serverDomain) { //本域
		rs.HasUpdate = isState
		if ev.Depth() > rs.Depth {
			rs.Depth = ev.Depth()
		}

		if isState == true {
			rs.lastStateId[0] = rs.lastStateId[1]
			rs.lastStateRef[0] = rs.lastStateRef[1]

			rs.lastStateId[1] = ev.EventID()
			rs.lastStateRef[1] = ev.EventReference().EventSHA256
			rs.lastMsgId[1] = ev.EventID()
			rs.lastMsgRef[1] = ev.EventReference().EventSHA256

			log.Debugf("onEvent type:%s offset:%d laststateid:%v lastmsgid:%v", ev.Type(), offset, rs.lastStateId, rs.lastMsgId)
		} else {
			rs.lastMsgId[0] = rs.lastMsgId[1]
			rs.lastMsgRef[0] = rs.lastMsgRef[1]

			rs.lastMsgId[1] = ev.EventID()
			rs.lastMsgRef[1] = ev.EventReference().EventSHA256

			log.Debugf("onEvent type:%s offset:%d laststateid:%v lastmsgid:%v", ev.Type(), offset, rs.lastStateId, rs.lastMsgId)
		}
	} else { //不是本域，只处理状态
		if isState == true && (pre == nil || (pre != nil && (pre.OriginServerTS() <= ev.OriginServerTS()) && (pre.EventID() != ev.EventID()))) {
			rs.HasUpdate = true
		}

		if ev.Depth() > rs.Depth {
			rs.Depth = ev.Depth()
		}
	}

	if rs.HasUpdate == false {
		return
	}

	item := rs.getDomainTL(domain, true)

	switch ev.Type() {
	case "m.room.create":
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
	case "m.room.member":
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

	ef := new(eventFeed)
	ef.ev = ev
	ef.offset = offset
	item.tl.Add(ef)

	atomic.CompareAndSwapInt32(&rs.dirt, 0, 1)
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

func (rs *RoomServerState) IsFlushed() bool {
	return rs.flushed
}

func (rs *RoomServerState) IsDirt() bool {
	dirt := atomic.LoadInt32(&rs.dirt)
	return dirt == 1
}

func (rs *RoomServerState) flush(cache service.Cache) {
	bytes, err := rs.serialize()
	token := rs.calcToken()
	if err == nil {
		atomic.CompareAndSwapInt32(&rs.dirt, 1, 0)
		log.Infof("FlushRoomState bytes: %s, token: %s dirt: %d", string(bytes), token, rs.dirt)
	}

	// update redis
	cache.SetRoomState(rs.GetRoomID(), bytes, token)
	rs.flushed = true
}

type RoomServerCurStateRepo struct {
	roomState sync.Map
	persist   model.RoomServerDatabase
	domain    []string
	cache     service.Cache

	queryHitCounter mon.LabeledCounter
}

func (repo *RoomServerCurStateRepo) SetPersist(db model.RoomServerDatabase) {
	repo.persist = db
}

func (repo *RoomServerCurStateRepo) SetDomain(domain []string) {
	repo.domain = domain
}

func (repo *RoomServerCurStateRepo) SetCache(cache service.Cache) {
	repo.cache = cache
}

func (repo *RoomServerCurStateRepo) SetMonitor(queryHitCounter mon.LabeledCounter) {
	repo.queryHitCounter = queryHitCounter
}

func (repo *RoomServerCurStateRepo) Start() {
	ticker := time.NewTimer(0)
	go func() {
		for {
			select {
			case <-ticker.C:
				ticker.Reset(time.Second * 15)
				repo.flushRoomState()
			}
		}
	}()
}

func (repo *RoomServerCurStateRepo) GetRoomState(roomid string) *RoomServerState {
	return repo.getRoomState(roomid, false)
}

func (repo *RoomServerCurStateRepo) GetRoomStateNoCache(roomid string) *RoomServerState {
	return repo.getRoomState(roomid, true)
}

func (repo *RoomServerCurStateRepo) getRoomState(roomid string, nocache bool) *RoomServerState {
	log.Infof("RoomServerCurStateRepo GetRoomState %s", roomid)
	if !nocache {
		if val, ok := repo.roomState.Load(roomid); ok {
			//log.Infof("RoomServerCurStateRepo GetRoomState %s in repo, val :%v", roomid, *(val.(*RoomServerState)))
			repo.queryHitCounter.WithLabelValues("cache", "RoomServerCurStateRepo", "GetRoomState").Add(1)
			return val.(*RoomServerState)
		}
	}

	bytes, _, err := repo.cache.GetRoomState(roomid)
	if err == nil && bytes != nil {
		log.Infof("RoomServerCurStateRepo GetRoomState %s in cache, input:%s", roomid, string(bytes))
		var rs RoomServerState
		rs.init(roomid, repo.domain)
		if json.Unmarshal(bytes, &rs) == nil {
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
				rs.onEvent(v, v.EventNID())
			}

			res, err := repo.cache.GetRoomOffsets(rs.RoomNid)
			if err == nil {
				for key, val := range res {
					if key == "depth" {
						rs.Depth = val
					} else {
						if v, ok := rs.domainTl.Load(key); ok {
							item := v.(*DomainTLItem)
							item.Offset = val
						}
					}
				}
			}

			if nocache {
				repo.roomState.Store(roomid, &rs)
				return &rs
			} else {
				val, _ := repo.roomState.LoadOrStore(roomid, &rs)
				return val.(*RoomServerState)
			}
		}
	}

	repo.queryHitCounter.WithLabelValues("db", "RoomServerCurStateRepo", "GetRoomState").Add(1)

	log.Infof("RoomServerCurStateRepo GetRoomState %s in persist", roomid)
	events, ids, err := repo.persist.GetRoomStates(context.Background(), roomid)
	if err != nil {
		log.Errorf("RoomServerCurStateRepo GetRoomState %s in persist err:%v", roomid, err)
		return nil
	}

	rs := new(RoomServerState)
	rs.init(roomid, repo.domain)

	for idx, ev := range events {
		rs.onEvent(ev, ids[idx])
	}

	roomNid, snapId, depth, err := repo.persist.RoomInfo(context.Background(), roomid)
	if err != nil {
		log.Errorf("RoomServerCurStateRepo RoomInfo snap room:%s in persist err:%v", roomid, err)
		return nil
	}

	domains, offsets, err := repo.persist.GetRoomDomainsOffset(context.Background(), roomNid)
	if err != nil {
		log.Errorf("RoomServerCurStateRepo GetRoomDomainsOffset snap room:%d in persist err:%v", roomNid, err)
		return nil
	}

	var item *DomainTLItem
	for idx, domain := range domains {
		item = rs.getDomainTL(domain, true)
		item.Offset = offsets[idx]
	}

	rs.SetSnapId(snapId)
	rs.SetRoomNID(roomNid)
	rs.setDepth(depth)
	val, _ := repo.roomState.LoadOrStore(roomid, rs)
	return val.(*RoomServerState)
}

func (repo *RoomServerCurStateRepo) flushRoomState() {
	repo.roomState.Range(func(key, value interface{}) bool {
		rs := value.(*RoomServerState)
		if rs.IsDirt() == true {
			rs.flush(repo.cache)
		}

		return true
	})
}

func (repo *RoomServerCurStateRepo) FlushRoomStateByID(roomid string) {
	if val, ok := repo.roomState.Load(roomid); ok {
		rs := val.(*RoomServerState)
		rs.flush(repo.cache)
	}
}

func (repo *RoomServerCurStateRepo) OnEvent(ev *gomatrixserverlib.Event, offset int64) *RoomServerState {
	var rs *RoomServerState
	val, ok := repo.roomState.Load(ev.RoomID())
	if ok {
		rs = val.(*RoomServerState)
	} else {
		if ev.Type() != "m.room.create" { //missed in cache
			rs = repo.GetRoomState(ev.RoomID())
		} else {
			rs = new(RoomServerState)
			rs.init(ev.RoomID(), repo.domain)
			repo.roomState.Store(ev.RoomID(), rs)
		}
	}

	rs.onEvent(ev, offset)

	return rs
}

func (repo *RoomServerCurStateRepo) OnEventNoCache(ev *gomatrixserverlib.Event, offset int64) *RoomServerState {
	rs := repo.GetRoomStateNoCache(ev.RoomID())
	if rs != nil {
		rs.onEvent(ev, offset)
	}

	return rs
}
