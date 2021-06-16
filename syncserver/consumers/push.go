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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/feedstypes"
	push "github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/pushapi/routing"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/tidwall/gjson"
)

type PushConsumer struct {
	rpcClient    *common.RpcClient
	cache        service.Cache
	eventRepo    *repos.EventReadStreamRepo
	countRepo    *repos.ReadCountRepo
	roomCurState *repos.RoomCurStateRepo
	rsTimeline   *repos.RoomStateTimeLineRepo
	roomHistory  *repos.RoomHistoryTimeLineRepo
	pubTopic     string
	complexCache *common.ComplexCache
	msgChan      []chan *PushEvent
	chanSize     uint32
	slotSize     uint32
	pushDataRepo *repos.PushDataRepo
	cfg          *config.Dendrite
}

type PushEvent struct {
	Ev     *gomatrixserverlib.ClientEvent
	Static *push.StaticObj
	result chan types.TimeLineItem
}

func NewPushConsumer(
	cache service.Cache,
	client *common.RpcClient,
	complexCache *common.ComplexCache,
	pushDataRepo *repos.PushDataRepo,
	cfg *config.Dendrite,
) *PushConsumer {
	s := &PushConsumer{
		cache:        cache,
		rpcClient:    client,
		complexCache: complexCache,
		chanSize:     20480,
		slotSize:     64,
		pushDataRepo: pushDataRepo,
		cfg:          cfg,
	}
	s.pubTopic = push.PushTopicDef

	return s
}

func (s *PushConsumer) Start() {
	s.msgChan = make([]chan *PushEvent, s.slotSize)
	for i := uint32(0); i < s.slotSize; i++ {
		s.msgChan[i] = make(chan *PushEvent, s.chanSize)
		go s.startWorker(s.msgChan[i])
	}
}

func (s *PushConsumer) startWorker(msgChan chan *PushEvent) {
	for data := range msgChan {
		s.OnEvent(data.Ev, data.Ev.EventOffset, data.Static, data.result)
	}
}

func (s *PushConsumer) DispthEvent(ev *gomatrixserverlib.ClientEvent, static *push.StaticObj, result chan types.TimeLineItem) {
	idx := common.CalcStringHashCode(ev.RoomID) % s.slotSize
	s.msgChan[idx] <- &PushEvent{
		Ev:     ev,
		Static: static,
		result: result,
	}
}

func (s *PushConsumer) SetRoomHistory(roomHistory *repos.RoomHistoryTimeLineRepo) *PushConsumer {
	s.roomHistory = roomHistory
	return s
}

func (s *PushConsumer) SetRsTimeline(rsTimeline *repos.RoomStateTimeLineRepo) *PushConsumer {
	s.rsTimeline = rsTimeline
	return s
}

func (s *PushConsumer) SetRoomCurState(roomCurState *repos.RoomCurStateRepo) *PushConsumer {
	s.roomCurState = roomCurState
	return s
}

func (s *PushConsumer) SetCountRepo(countRepo *repos.ReadCountRepo) *PushConsumer {
	s.countRepo = countRepo
	return s
}

func (s *PushConsumer) SetEventRepo(eventRepo *repos.EventReadStreamRepo) *PushConsumer {
	s.eventRepo = eventRepo
	return s
}

func (s *PushConsumer) IsRelatesContent(redactEv gomatrixserverlib.ClientEvent) bool {
	var unsigned *types.RedactUnsigned
	err := json.Unmarshal(redactEv.Unsigned, &unsigned)
	if err != nil {
		log.Errorf("json.Unmarshal redactEv.RedactUnsigned err:%v", err)
		return false
	}
	if unsigned.IsRelated != nil {
		return *unsigned.IsRelated
	} else {
		return false
	}
}

func (s *PushConsumer) PrintStaticData(static *push.StaticObj) {
	if static.ProfileCount <= 0 {
		static.ProfileCount = 1
	}
	if static.MemCount < 2 {
		static.MemCount = 2
	}
	if static.RuleCount <= 0 {
		static.RuleCount = 1
	}
	if static.TotalSpend <= 0 {
		static.TotalSpend = 1
	}
	if static.PusherCount <= 0 {
		static.PusherCount = 1
	}
	if static.PushRuleCount <= 0 {
		static.PushRuleCount = 1
	}
	if static.EffectedRuleCount <= 0 {
		static.EffectedRuleCount = 1
	}
	if static.GlobalMatchCount <= 0 {
		static.GlobalMatchCount = 1
	}
	static.TotalSpend = time.Now().UnixNano()/1000 - static.Start
	static.Avg.AvgMemSpend = static.MemAllSpend / int64(static.MemCount)
	static.Avg.AvgRuleSpend = static.RuleSpend / static.RuleCount
	static.Avg.AvgCheckConditionSpend = static.CheckConditionSpend / int64(static.RuleCount)
	static.Avg.AvgReadUnreadSpend = static.ReadUnreadSpend / int64(static.EffectedRuleCount)
	static.Avg.AvgUnreadSpend = static.UnreadSpend / int64(static.EffectedRuleCount)
	static.Avg.AvgGlobalMatchSpend = static.GlobalMatchSpend / int64(static.GlobalMatchCount)
	static.Avg.AvgProfileSpend = static.ProfileSpend / static.ProfileCount
	static.Avg.AvgPushCacheSpend = static.PushCacheSpend / (static.PusherCount + static.PushRuleCount)
	static.Percent.Rpc = fmt.Sprintf("%.2f", float64(static.NoneMemSpend)/float64(static.TotalSpend)*100) + "%"
	static.Percent.Chan = fmt.Sprintf("%.2f", float64(static.ChanSpend)/float64(static.TotalSpend)*100) + "%"
	cacheAvg := static.MemCache / int64(static.MemCount-1)
	ruleAvg := static.MemRule / int64(static.MemCount-1)
	static.Percent.RuleCache = fmt.Sprintf("%.2f", float64(cacheAvg)/float64(static.TotalSpend)*100) + "%"
	static.Percent.RuleHandle = fmt.Sprintf("%.2f", float64(ruleAvg)/float64(static.TotalSpend)*100) + "%"
	static.Percent.SenderCache = fmt.Sprintf("%.2f", float64(static.SenderCache)/float64(static.TotalSpend)*100) + "%"
	log.Infof("traceid:%s PushConsumer static:%+v", static.TraceId, static)
}

func (s *PushConsumer) OnEvent(input *gomatrixserverlib.ClientEvent, eventOffset int64, static *push.StaticObj, result chan types.TimeLineItem) {
	log.Debugw("Trace timestamp, PushConsumer.OnEvent, begin", log.KeysAndValues{
		"timestamp", time.Now().UnixNano()/1000000, "roomID", input.RoomID, "eventID", input.EventID, "eventOffset", input.EventOffset})
	static.ChanStart = time.Now().UnixNano() / 1000
	static.ChanSpend = static.ChanStart - static.Start
	defer func() {
		log.Debugw("Trace timestamp, PushConsumer.OnEvent, end", log.KeysAndValues{
			"timestamp", time.Now().UnixNano()/1000000, "roomID", input.RoomID, "eventID", input.EventID, "eventOffset", input.EventOffset})
		result <- types.TimeLineItem{
			TraceId: static.TraceId,
			Start:   static.Start,
			Ev:      input,
		}
		s.PrintStaticData(static)
	}()
	eventJson, err := json.Marshal(&input)
	if err != nil {
		log.Errorf("PushConsumer processEvent marshal error %d, message %s", err, input.EventID)
		return
	}
	redactOffset := int64(-1)
	isRelatesContent := false
	var members []string

	switch input.Type {
	case "m.room.message", "m.room.encrypted":
		members = s.getRoomMembers(input)
	case "m.call.invite":
		members = s.getRoomMembers(input)
	case "m.room._ext.leave", "m.room._ext.enter":
		members = s.getRoomMembers(input)
	case "m.room.redaction":
		members = s.getRoomMembers(input)
		redactID := input.Redacts
		stream := s.roomHistory.GetStreamEv(input.RoomID, redactID)
		if stream != nil {
			redactOffset = stream.Offset
			isRelatesContent = s.IsRelatesContent(*stream.GetEv())
		}
	case "m.room.member":
		members = s.getRoomMembers(input)
		members = append(members, *input.StateKey)
	case "m.modular.video":
		var users push.PushContentUsers
		if err := json.Unmarshal(input.Content, &users); err != nil {
			log.Errorf("PushConsumer processEvent marshal PushContentUsers error %d, message %s", err, input.EventID)
			return
		}
		for _, user := range users.Data.Members {
			members = append(members, user)
		}
	default:
		return
	}

	memCount := len(members)

	pushContents := push.PushPubContents{
		Contents: []*push.PushPubContent{},
	}
	//get all user push data
	rs := time.Now().UnixNano() / 1000
	pushData := s.getUserPushData(input.Sender, members)
	bs := time.Now().UnixNano() / 1000
	static.NoneMemSpend = bs - rs
	var wg sync.WaitGroup
	for _, member := range members {
		wg.Add(1)
		go func(
			member string,
			input *gomatrixserverlib.ClientEvent,
			memCount int,
			eventOffset,
			redactOffset int64,
			eventJson *[]byte,
			pushContents *push.PushPubContents,
			isRelatesContent bool,
			static *push.StaticObj,
			pushData map[string]push.RespPushData,
		) {
			s.preProcessPush(&member, input, memCount, eventOffset, redactOffset, eventJson, pushContents, isRelatesContent, static, pushData)
			wg.Done()
		}(member, input, memCount, eventOffset, redactOffset, &eventJson, &pushContents, isRelatesContent, static, pushData)
	}
	wg.Wait()
	static.MemCount = len(members)
	static.MemSpend = time.Now().UnixNano()/1000 - bs
	//将需要推送的消息聚合一次推送
	if s.rpcClient != nil && len(pushContents.Contents) > 0 {
		pushContents.Input = input
		pushContents.RoomAlias = ""
		go s.pubPushContents(&pushContents, &eventJson)
	}
}

func (s *PushConsumer) getUserPushData(sender string, members []string) map[string]push.RespPushData {
	slotMembers := make(map[uint32]*push.ReqPushUsers)
	for _, member := range members {
		if member == sender {
			continue
		}
		slot := common.CalcStringHashCode(member) % s.cfg.MultiInstance.Total
		if _, ok := slotMembers[slot]; ok {
			slotMembers[slot].Users = append(slotMembers[slot].Users, member)
		} else {
			slotMembers[slot] = &push.ReqPushUsers{
				Users: []string{member},
				Slot:  slot,
			}
		}
	}
	collectionResults := make(chan *push.RespPushUsersData, s.cfg.MultiInstance.Total)
	var wg sync.WaitGroup
	for _, req := range slotMembers {
		if len(req.Users) <= 0 {
			continue
		}
		wg.Add(1)
		go s.rpcGetUserPushData(&wg, req, collectionResults)
	}
	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(collectionResults)
	}(&wg)
	result := make(map[string]push.RespPushData)
	for r := range collectionResults {
		for k, v := range r.Data {
			result[k] = v
		}
	}
	return result
}

func (s *PushConsumer) rpcGetUserPushData(wg *sync.WaitGroup, req *push.ReqPushUsers, result chan *push.RespPushUsersData) {
	defer wg.Done()
	if req.Slot == s.cfg.MultiInstance.Instance {
		result <- s.getUserPushDataFromLocal(req)
	} else {
		result <- s.getUserPushDataFromRemote(req)
	}
	//for test rpc
	//result <- s.getUserPushDataFromRemote(req)
}

func (s *PushConsumer) getUserPushDataFromLocal(req *push.ReqPushUsers) *push.RespPushUsersData {
	data := &push.RespPushUsersData{
		Data: make(map[string]push.RespPushData),
	}
	for _, user := range req.Users {
		resp := push.RespPushData{
			Pushers: routing.GetPushersByName(user, s.pushDataRepo, false, nil),
			Rules:   routing.GetUserPushRules(user, s.pushDataRepo, false, nil),
		}
		data.Data[user] = resp
	}
	return data
}

func (s *PushConsumer) getUserPushDataFromRemote(req *push.ReqPushUsers) *push.RespPushUsersData {
	data := &push.RespPushUsersData{
		Data: make(map[string]push.RespPushData),
	}
	payload, err := json.Marshal(req)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Marshal payload:%+v err:%v", req, err)
		return data
	}
	request := push.PushDataRequest{
		Payload: payload,
		ReqType: types.GET_PUSHDATA_BATCH,
		Slot:    req.Slot,
	}
	bt, err := json.Marshal(request)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Marshal request:%+v err:%v", req, err)
		return data
	}
	r, err := s.rpcClient.Request(types.PushDataTopicDef, bt, 15000)
	if err != nil {
		log.Error("getUserPushDataFromRemote rpc req:%+v err:%v", req, err)
		return data
	}
	resp := push.RpcResponse{}
	err = json.Unmarshal(r, &resp)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Unmarshal RpcResponse err:%v", err)
		return data
	}
	err = json.Unmarshal(resp.Payload, &data)
	if err != nil {
		log.Error("getUserPushDataFromRemote json.Unmarshal Rpc payload err:%v", err)
		return data
	}
	return data
}

func (s *PushConsumer) getUserPusher(user string, pushData map[string]push.RespPushData) push.Pushers {
	if v, ok := pushData[user]; ok {
		return v.Pushers
	} else {
		return push.Pushers{Pushers: []push.Pusher{}}
	}
}

func (s *PushConsumer) getUserPushRule(user string, pushData map[string]push.RespPushData) push.Rules {
	if v, ok := pushData[user]; ok {
		return v.Rules
	} else {
		return push.Rules{}
	}
}

func (s *PushConsumer) preProcessPush(
	member *string,
	input *gomatrixserverlib.ClientEvent,
	memCount int,
	eventOffset,
	redactOffset int64,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	isRelatesContent bool,
	static *push.StaticObj,
	pushData map[string]push.RespPushData,
) {
	bs := time.Now().UnixNano() / 1000
	if *member != input.Sender {
		if input.Type == "m.room.redaction" {
			if s.eventRepo.GetUserLastOffset(*member, input.RoomID) < redactOffset || redactOffset == -1 {
				//如果一个用户读完消息以后，有新的未读，此时hs重启，其他人撤销之前已读消息，计数会不准确
				//高亮信息撤回，暂时也不好处理计减
				if !isRelatesContent {
					s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *member, "decrease")
				}
			}
		}
		ms := time.Now().UnixNano() / 1000
		pushers := s.getUserPusher(*member, pushData)
		global := s.getUserPushRule(*member, pushData)
		sp := time.Now().UnixNano()/1000 - ms
		atomic.AddInt64(&static.PushCacheSpend, sp)
		var rules []push.PushRule
		for _, v := range global.Override {
			rules = append(rules, v)
		}
		for _, v := range global.Content {
			rules = append(rules, v)
		}
		for _, v := range global.Room {
			rules = append(rules, v)
		}
		for _, v := range global.Sender {
			rules = append(rules, v)
		}
		for _, v := range global.UnderRide {
			rules = append(rules, v)
		}
		//log.Infof("roomID:%s eventID:%s userID:%s rules:%+v", input.RoomID, input.EventID, *member, rules)
		rs := time.Now().UnixNano() / 1000
		s.processPush(&pushers, &rules, input, member, memCount, eventJson, pushContents, static)
		rsp := time.Now().UnixNano()/1000 - rs
		atomic.AddInt64(&static.MemRule, rsp)
	} else {
		//当前用户在发消息，应该把该用户的未读数置为0
		s.eventRepo.AddUserReceiptOffset(*member, input.RoomID, eventOffset)
		ss := time.Now().UnixNano() / 1000
		s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *member, "reset")
		sp := time.Now().UnixNano()/1000 - ss
		atomic.AddInt64(&static.UnreadSpend, sp)
	}
	spend := time.Now().UnixNano()/1000 - bs
	atomic.AddInt64(&static.MemAllSpend, spend)
}

func (s *PushConsumer) getRoomMembers(
	input *gomatrixserverlib.ClientEvent,
) []string {
	var result []string
	result = []string{}

	rs := s.roomCurState.GetRoomState(input.RoomID)
	if rs != nil {
		joined := rs.GetJoinMap()
		joined.Range(func(key, _ interface{}) bool {
			result = append(result, key.(string))

			return true
		})
	}

	return result
}

func (s *PushConsumer) getRoomName(roomID string) string {
	states := s.rsTimeline.GetStates(roomID)
	name := ""

	if states != nil {
		var feeds []feedstypes.Feed
		states.ForRange(func(offset int, feed feedstypes.Feed) bool {
			if feed == nil {
				log.Errorf("PushConsumer.getRoomName get feed nil offset %d", offset)
				states.Console()
			} else {
				feeds = append(feeds, feed)
			}
			return true
		})
		for _, feed := range feeds {
			if feed == nil {
				continue
			}

			stream := feed.(*feedstypes.StreamEvent)
			if stream.IsDeleted {
				continue
			}

			ev := stream.GetEv()
			if ev.Type == "m.room.name" {
				var content common.NameContent
				err := json.Unmarshal(ev.Content, &content)
				if err != nil {
					log.Errorf("PushConsumer.getRoomName Unmarshal, roomID %s error %v", roomID, err)
				} else {
					name = content.Name
				}
				break
			}
		}
	}

	return name
}

func (s *PushConsumer) getCreateContent(roomID string) interface{} {
	states := s.rsTimeline.GetStates(roomID)

	if states != nil {
		var feeds []feedstypes.Feed
		states.ForRange(func(offset int, feed feedstypes.Feed) bool {
			if feed == nil {
				log.Errorf("PushConsumer.getCreateContent get feed nil offset %d", offset)
				states.Console()
			} else {
				feeds = append(feeds, feed)
			}
			return true
		})
		for _, feed := range feeds {
			if feed == nil {
				continue
			}

			stream := feed.(*feedstypes.StreamEvent)
			ev := stream.GetEv()

			if ev.Type == "m.room.create" {
				return ev.Content
			}
		}
	}

	return nil
}

func (s *PushConsumer) pubPushContents(pushContents *push.PushPubContents, eventJson *[]byte) {
	senderDisplayName, _ := s.cache.GetDisplayNameByUser(pushContents.Input.Sender)
	pushContents.SenderDisplayName = senderDisplayName
	//临时处理，rcs去除邀请重试以后可以去掉
	if pushContents.Input.Type == "m.room.member" {
		result := gjson.Get(string(*eventJson), "unsigned.prev_content.membership")
		if result.Str == "invite" {
			return
		}
	}

	pushContents.RoomName = s.getRoomName(pushContents.Input.RoomID)
	createContent := s.getCreateContent(pushContents.Input.RoomID)
	if createContent != nil {
		pushContents.CreateContent = &createContent
	}

	bytes, err := json.Marshal(pushContents)
	if err == nil {
		log.Infof("EventDataConsumer.pubPushContents %s", string(bytes))
		s.rpcClient.Pub(s.pubTopic, bytes)
	} else {
		log.Errorf("EventDataConsumer.pubPushContents marsh err %v", err)
	}
}

func (s *PushConsumer) processPush(
	pushers *push.Pushers,
	rules *[]push.PushRule,
	input *gomatrixserverlib.ClientEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) {
	bs := time.Now().UnixNano() / 1000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000 - bs
		atomic.AddInt64(&static.RuleSpend, spend)
	}(bs)
	//这种写法真的很挫，但没找到其他的处理方式
	result := gjson.Get(string(*eventJson), "content.msgtype")
	if result.Str == "m.notice" {
		return
	}
	for _, v := range *rules {
		if !v.Enabled {
			log.Infof("roomID:%s eventID:%s userID:%s rule:%s is not enable", input.RoomID, input.EventID, *userID, v.RuleId)
			continue
		}
		atomic.AddInt64(&static.RuleCount, 1)
		if s.checkCondition(&v.Conditions, userID, memCount, eventJson, static) {
			atomic.AddInt64(&static.EffectedRuleCount, 1)
			log.Infof("roomID:%s eventID:%s userID:%s match rule:%s", input.RoomID, input.EventID, *userID, v.RuleId)
			action := s.getActions(v.Actions)

			increase := false
			if input.Type == "m.room.encrypted" {
				increase = true
			}
			if input.Type == "m.room.message" {
				var content struct {
					MsgType string `json:"msgtype"`
				}
				json.Unmarshal(input.Content, &content)
				if content.MsgType != "m.shake" {
					increase = true
				}
			}
			if increase {
				ss := time.Now().UnixNano() / 1000
				s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *userID, "increase")
				sp := time.Now().UnixNano()/1000 - ss
				atomic.AddInt64(&static.UnreadSpend, sp)
			}

			rcss := time.Now().UnixNano() / 1000
			count, _ := s.countRepo.GetRoomReadCount(input.RoomID, *userID)
			atomic.AddInt64(&static.ReadUnreadSpend, time.Now().UnixNano()/1000 - rcss)

			if action.HighLight {
				if input.Type == "m.room.message" || input.Type == "m.room.encrypted" {
					s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *userID, "increase_hl")
				}
			}

			if action.Notify == "notify" {
				if s.rpcClient != nil && len(pushers.Pushers) > 0 {
					var pubContent push.PushPubContent
					pubContent.UserID = *userID
					pubContent.Pushers = pushers
					pubContent.Action = &action
					pubContent.NotifyCount = count

					pushContents.Contents = append(pushContents.Contents, &pubContent)
				}
			}
			break
		} else {
			log.Infof("roomID:%s eventID:%s userID:%s not match rule:%s", input.RoomID, input.EventID, *userID, v.RuleId)
		}
	}
}

func (s *PushConsumer) checkCondition(
	conditions *[]push.PushCondition,
	userID *string,
	memCount int,
	eventJSON *[]byte,
	static *push.StaticObj,
) bool {
	bs := time.Now().UnixNano() / 1000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000 - bs
		atomic.AddInt64(&static.CheckConditionSpend, spend)
	}(bs)
	if len(*conditions) > 0 {
		for _, v := range *conditions {
			match := s.isMatch(&v, userID, memCount, eventJSON, static)
			if !match {
				return false
			}
		}
		return true
	}
	return true
}

func (s *PushConsumer) isMatch(
	condition *push.PushCondition,
	userID *string,
	memCount int,
	eventJSON *[]byte,
	static *push.StaticObj,
) bool {
	switch condition.Kind {
	case "event_match":
		return s.eventMatch(condition, userID, eventJSON, static)
	case "room_member_count":
		return s.roomMemberCount(condition, memCount)
	case "signal":
		return s.signal(userID, eventJSON)
	}
	return true
}

func (s *PushConsumer) signal(
	userID *string,
	eventJSON *[]byte,
) bool {
	if userID == nil {
		return false
	}

	value := gjson.Get(string(*eventJSON), "content.signals")
	if value.Raw == "" {
		return false
	}

	if strings.Contains(value.Raw, "@all") {
		return true
	}

	return strings.Contains(value.Raw, *userID)
}

func (s *PushConsumer) eventMatch(
	condition *push.PushCondition,
	userID *string,
	eventJSON *[]byte,
	static *push.StaticObj,
) bool {
	var pattern *string
	var context string
	var wordBoundary bool

	switch condition.Pattern {
	case "":
		return false
	case "user_id":
		pattern = userID
	case "user_localpart":
		localPart, _, err := gomatrixserverlib.SplitID('@', *userID)
		if err != nil {
			return false
		}
		pattern = &localPart
	default:
		pattern = &condition.Pattern
	}

	if condition.Key == "content.body" {
		value := gjson.Get(string(*eventJSON), "content.body")
		if value.String() == "" {
			return false
		}
		wordBoundary = true
		context = value.String()
	} else {
		value := gjson.Get(string(*eventJSON), condition.Key)
		if value.String() == "" {
			return false
		}
		wordBoundary = false
		context = value.String()
	}

	return s.globalMatch(pattern, &context, wordBoundary, static)
}

func (s *PushConsumer) containsDisplayName(
	displayName *string,
	eventJSON *[]byte,
	static *push.StaticObj,
) bool {
	if displayName == nil {
		return false
	}

	emptyName := ""
	if *displayName == emptyName {
		return false
	}

	value := gjson.Get(string(*eventJSON), "content.body")
	if value.String() == "" {
		return false
	}
	valueStr := value.String()

	return s.globalMatch(displayName, &valueStr, true, static)
}

func (s *PushConsumer) globalMatch(
	global,
	req *string,
	wordBoundary bool,
	static *push.StaticObj,
) bool {
	atomic.AddInt64(&static.GlobalMatchCount, 1)
	bs := time.Now().UnixNano() / 1000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000 - bs
		atomic.AddInt64(&static.GlobalMatchSpend, spend)
	}(bs)
	globalRegex := regexp.MustCompile(`\\\[(\\\!|)(.*)\\\]`)
	isGlobal := regexp.MustCompile(`[\?\*\[\]]`)

	if isGlobal.Match([]byte(*global)) {
		*global = regexp.QuoteMeta(*global)
		*global = strings.Replace(*global, `\*`, `.*?`, -1)
		*global = strings.Replace(*global, `\?`, `.`, -1)

		if globalRegex.Match([]byte(*global)) {
			s := globalRegex.FindStringSubmatch(*global)
			if s[1] != "" {
				s[1] = "^"
			}
			s[2] = strings.Replace(s[2], `\\\-`, "-", -1)
			*global = fmt.Sprintf("[%s%s]", s[1], s[2])
		}

		if wordBoundary {
			*global = fmt.Sprintf(`(^|\W)%s(\W|$)`, *global)
		} else {
			*global = "^" + *global + "$"
		}
	} else if wordBoundary {
		*global = regexp.QuoteMeta(*global)
		*global = fmt.Sprintf(`(^|\W)%s(\W|$)`, *global)
	} else {
		*global = "^" + regexp.QuoteMeta(*global) + "$"
	}

	reg := regexp.MustCompile(*global)
	return reg.Match([]byte(*req))
}

func (s *PushConsumer) roomMemberCount(
	condition *push.PushCondition,
	memCount int,
) bool {
	if condition.Is == "" {
		return false
	}

	reg := regexp.MustCompile("^([=<>]*)([0-9]*)$")
	if reg.Match([]byte(condition.Is)) {
		s := reg.FindStringSubmatch(condition.Is)
		num, _ := strconv.Atoi(s[2])

		switch s[1] {
		case "":
			return memCount == num
		case "==":
			return memCount == num
		case "<":
			return memCount < num
		case ">":
			return memCount > num
		case "<=":
			return memCount <= num
		case ">=":
			return memCount >= num
		default:
			return false
		}
	}

	return false
}

func (s *PushConsumer) getActions(actions []interface{}) push.TweakAction {
	action := push.TweakAction{}

	for _, val := range actions {
		if v, ok := val.(string); ok {
			action.Notify = v
			continue
		} else if v, ok := val.(push.Tweak); ok {
			setTweak := v.SetTweak
			value := v.Value

			switch setTweak {
			case "sound":
				action.Sound = value.(string)
			case "highlight":
				if value == nil {
					action.HighLight = true
				} else {
					action.HighLight = value.(bool)
				}
			}
		} else if v, ok := val.(map[string]interface{}); ok {
			key := ""
			if val, ok := v["set_tweak"]; ok {
				key = val.(string)
			} else {
				continue
			}
			if val, ok := v["value"]; ok {
				if key == "sound" {
					action.Sound = val.(string)
				} else if key == "highlight" {
					action.HighLight = val.(bool)
				}
			} else {
				if key == "sound" {
					action.Sound = "default"
				} else if key == "highlight" {
					action.HighLight = true
				}
			}
		}
	}

	return action
}
