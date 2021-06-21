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

var (
	globalRegex          = regexp.MustCompile(`\\\[(\\\!|)(.*)\\\]`)
	isGlobal             = regexp.MustCompile(`[\?\*\[\]]`)
	mRoomMemberRegex     = regexp.MustCompile(`^m\.room\.member$`)
	inviteRegex          = regexp.MustCompile(`^invite$`)
	mRoomMessageRegex    = regexp.MustCompile(`^m\.room\.message$`)
	mRoomEncryptedRegex  = regexp.MustCompile(`^m\.room\.encrypted$`)
	mModularVideoRegex   = regexp.MustCompile(`^m\.modular\.video$`)
	mNoticeRegex         = regexp.MustCompile(`^m\.notice$`)
	mAlertRegex          = regexp.MustCompile(`^m\.alert$`)
	mCallInviteRegex     = regexp.MustCompile(`^m\.call\.invite$`)
	roomMemberCountRegex = regexp.MustCompile("^([=<>]*)([0-9]*)$")
)

type HelperSignalContent struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Type  string `json:"type"`
	Val   string `json:"val"`
}

type HelperContent struct {
	Body    string              `json:"body"`
	MsgType string              `json:"msgtype"`
	Signals []HelperSignalContent `json:"signals"`
}

type HelperEvent struct {
	Content HelperContent `json:"content,omitempty"`
	EventID string        `json:"event_id,omitempty"`
	RoomID  string        `json:"room_id,omitempty"`
	Type    string        `json:"type,omitempty"`
	Sender  string        `json:"sender,omitempty"`
}

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
	static.ChanStart = time.Now().UnixNano() / 1000
	static.ChanSpend = static.ChanStart - static.Start
	defer func() {
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

	var helperEvent HelperEvent
	if err = json.Unmarshal(eventJson, &helperEvent); err != nil {
		log.Errorf("PushConsumer processEvent unmarshal error %d, message %s", err, input.EventID)
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
			helperEvent *HelperEvent,
			memCount int,
			eventOffset,
			redactOffset int64,
			eventJson *[]byte,
			pushContents *push.PushPubContents,
			isRelatesContent bool,
			static *push.StaticObj,
			pushData map[string]push.RespPushData,
		) {
			s.preProcessPush(&member, helperEvent, memCount, eventOffset, redactOffset, eventJson, pushContents, isRelatesContent, static, pushData)
			wg.Done()
		}(member, &helperEvent, memCount, eventOffset, redactOffset, &eventJson, &pushContents, isRelatesContent, static, pushData)
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
	helperEvent *HelperEvent,
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
	if *member != helperEvent.Sender {
		if helperEvent.Type == "m.room.redaction" {
			if s.eventRepo.GetUserLastOffset(*member, helperEvent.RoomID) < redactOffset || redactOffset == -1 {
				//如果一个用户读完消息以后，有新的未读，此时hs重启，其他人撤销之前已读消息，计数会不准确
				//高亮信息撤回，暂时也不好处理计减
				if !isRelatesContent {
					s.countRepo.UpdateRoomReadCount(helperEvent.RoomID, helperEvent.EventID, *member, "decrease")
				}
			}
		}
		ms := time.Now().UnixNano() / 1000
		pushers := s.getUserPusher(*member, pushData)
		global := s.getUserPushRule(*member, pushData)
		sp := time.Now().UnixNano()/1000 - ms
		atomic.AddInt64(&static.PushCacheSpend, sp)
		rs := time.Now().UnixNano() / 1000
		s.processPush(&pushers, &global, helperEvent, member, memCount, eventJson, pushContents, static)
		rsp := time.Now().UnixNano()/1000 - rs
		atomic.AddInt64(&static.MemRule, rsp)
	} else {
		//当前用户在发消息，应该把该用户的未读数置为0
		s.eventRepo.AddUserReceiptOffset(*member, helperEvent.RoomID, eventOffset)
		ss := time.Now().UnixNano() / 1000
		s.countRepo.UpdateRoomReadCount(helperEvent.RoomID, helperEvent.EventID, *member, "reset")
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

func (s *PushConsumer) processMessageEvent(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) {
	if global.Default {
		highlight := s.isSignalRule(userID, helperEvent)
		if highlight {
			s.countRepo.UpdateRoomReadCount(helperEvent.RoomID, helperEvent.EventID, *userID, "increase_hl")
		}

		action := s.getDefaultAction(highlight)
		s.updateReadCountAndNotify(pushers, helperEvent, userID, pushContents, action, true, true, static)
		return
	}

	if global.OverrideDefault {
		highlight := s.isSignalRule(userID, helperEvent)
		if highlight {
			s.countRepo.UpdateRoomReadCount(helperEvent.RoomID, helperEvent.EventID, *userID, "increase_hl")
			action := s.getDefaultAction(highlight)
			s.updateReadCountAndNotify(pushers, helperEvent, userID, pushContents, action, true, true, static)
			return
		}
	}

	if s.processOverrideRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) {
		return
	}

	if !global.ContentDefault && s.processContentRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) {
		return
	}

	if !global.RoomDefault && s.processRoomRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) {
		return
	}

	if !global.SenderDefault && s.processSenderRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) {
		return
	}

	if global.UnderRideDefault {
		action := s.getDefaultAction(false)
		s.updateReadCountAndNotify(pushers, helperEvent, userID, pushContents, action, true, true, static)
		return
	}

	s.processUnderRideRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static)
}

func (s *PushConsumer) updateReadCountAndNotify(
	pushers *push.Pushers,
	helperEvent *HelperEvent,
	userID *string,
	pushContents *push.PushPubContents,
	action *push.TweakAction,
	updateReadCount bool,
	notify bool,
	static *push.StaticObj,
) {
	if updateReadCount {
		bs := time.Now().UnixNano() / 1000
		s.countRepo.UpdateRoomReadCount(helperEvent.RoomID, helperEvent.EventID, *userID, "increase")
		atomic.AddInt64(&static.UnreadSpend, time.Now().UnixNano()/1000-bs)
	}

	bs := time.Now().UnixNano() / 1000
	count, _ := s.countRepo.GetRoomReadCount(helperEvent.RoomID, *userID)
	atomic.AddInt64(&static.ReadUnreadSpend, time.Now().UnixNano()/1000-bs)
	if notify && s.rpcClient != nil && len(pushers.Pushers) > 0 {
		var pubContent push.PushPubContent
		pubContent.UserID = *userID
		pubContent.Pushers = pushers
		pubContent.Action = action
		pubContent.NotifyCount = count

		pushContents.Contents = append(pushContents.Contents, &pubContent)
	}
}

func (s *PushConsumer) isSignalRule(userID *string, helperEvent *HelperEvent) bool {
	return helperEvent.Content.MsgType == "m.alert" && s.signal(userID, helperEvent)
}

func (s *PushConsumer) processOverrideRules(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) bool {
	for _, v := range global.Override {
		if s.processCommonRule(pushers, &v, helperEvent, userID, memCount, eventJson, pushContents, static) {
			return true
		}
	}

	return false
}

func (s *PushConsumer) processContentRules(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) bool {
	for _, v := range global.Content {
		if s.processCommonRule(pushers, &v, helperEvent, userID, memCount, eventJson, pushContents, static) {
			return true
		}
	}

	return false
}

func (s *PushConsumer) processSenderRules(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) bool {
	for _, v := range global.Sender {
		if s.processCommonRule(pushers, &v, helperEvent, userID, memCount, eventJson, pushContents, static) {
			return true
		}
	}

	return false
}

func (s *PushConsumer) processUnderRideRules(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) bool {
	for _, v := range global.UnderRide {
		if s.processCommonRule(pushers, &v, helperEvent, userID, memCount, eventJson, pushContents, static) {
			return true
		}
	}

	return false
}

func (s *PushConsumer) processRoomRules(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) bool {
	for _, v := range global.Room {
		if v.RuleId == helperEvent.RoomID && s.processCommonRule(pushers, &v, helperEvent, userID, memCount, eventJson, pushContents, static) {
			return true
		}
	}

	return false
}

func (s *PushConsumer) processCommonRule(
	pushers *push.Pushers,
	rule *push.PushRule,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) bool {
	if !rule.Enabled {
		log.Debugf("roomID:%s eventID:%s userID:%s rule:%s is not enable", helperEvent.RoomID, helperEvent.EventID, *userID, rule.RuleId)
		return false
	}
	atomic.AddInt64(&static.RuleCount, 1)
	if s.checkCondition(&rule.Conditions, userID, memCount, eventJson, helperEvent, static) {
		atomic.AddInt64(&static.EffectedRuleCount, 1)
		log.Debugf("roomID:%s eventID:%s userID:%s match rule:%s", helperEvent.RoomID, helperEvent.EventID, *userID, rule.RuleId)
		action := s.getActions(rule.Actions)
		if action.HighLight && (helperEvent.Type == "m.room.message" || helperEvent.Type == "m.room.encrypted") {
			s.countRepo.UpdateRoomReadCount(helperEvent.RoomID, helperEvent.EventID, *userID, "increase_hl")
		}

		increase := helperEvent.Type == "m.room.encrypted" || (helperEvent.Type == "m.room.message" && helperEvent.Content.MsgType != "m.shake")
		s.updateReadCountAndNotify(pushers, helperEvent, userID, pushContents, &action, increase, action.Notify == "notify", static)
		return true
	}

	return false
}

func (s *PushConsumer) processNonMessageEvent(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
	static *push.StaticObj,
) {
	if s.processOverrideRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) ||
		s.processContentRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) ||
		s.processRoomRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) ||
		s.processSenderRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) ||
		s.processUnderRideRules(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static) {
		return
	}
}

func (s *PushConsumer) processPush(
	pushers *push.Pushers,
	global *push.Rules,
	helperEvent *HelperEvent,
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
	if helperEvent.Content.MsgType == "m.notice" {
		return
	}

	if helperEvent.Type == "m.room.message" || helperEvent.Type == "m.room.encrypted" {
		s.processMessageEvent(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static)
	} else {
		s.processNonMessageEvent(pushers, global, helperEvent, userID, memCount, eventJson, pushContents, static)
	}
}

func (s *PushConsumer) checkCondition(
	conditions *[]push.PushCondition,
	userID *string,
	memCount int,
	eventJSON *[]byte,
	helperEvent *HelperEvent,
	static *push.StaticObj,
) bool {
	bs := time.Now().UnixNano() / 1000
	defer func(bs int64) {
		spend := time.Now().UnixNano()/1000 - bs
		atomic.AddInt64(&static.CheckConditionSpend, spend)
	}(bs)
	if len(*conditions) > 0 {
		for _, v := range *conditions {
			match := s.isMatch(&v, userID, memCount, eventJSON, helperEvent, static)
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
	helperEvent *HelperEvent,
	static *push.StaticObj,
) bool {
	switch condition.Kind {
	case "event_match":
		return s.eventMatch(condition, userID, eventJSON, helperEvent, static)
	case "room_member_count":
		return s.roomMemberCount(condition, memCount)
	case "signal":
		return s.signal(userID, helperEvent)
	}
	return true
}

func (s *PushConsumer) signal(
	userID *string,
	helperEvent *HelperEvent,
) bool {
	if userID == nil {
		return false
	}

	for _, v := range helperEvent.Content.Signals {
		if v.Val == *userID || strings.Contains(v.Val, "@all") {
			return true
		}
	}

	return false
}

func (s *PushConsumer) eventMatch(
	condition *push.PushCondition,
	userID *string,
	eventJSON *[]byte,
	helperEvent *HelperEvent,
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
		if helperEvent.Content.Body == "" {
			return false
		}

		context = helperEvent.Content.Body
		wordBoundary = true
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
	switch *global {
	case "m.room.member":
		return mRoomMemberRegex.Match([]byte(*req))
	case "invite":
		return inviteRegex.Match([]byte(*req))
	case "m.room.message":
		return mRoomMessageRegex.Match([]byte(*req))
	case "m.room.encrypted":
		return mRoomEncryptedRegex.Match([]byte(*req))
	case "m.modular.video":
		return mModularVideoRegex.Match([]byte(*req))
	case "m.notice":
		return mNoticeRegex.Match([]byte(*req))
	case "m.alert":
		return mAlertRegex.Match([]byte(*req))
	case "m.call.invite":
		return mCallInviteRegex.Match([]byte(*req))
	}

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

	if roomMemberCountRegex.Match([]byte(condition.Is)) {
		s := roomMemberCountRegex.FindStringSubmatch(condition.Is)
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

func (s *PushConsumer) getDefaultAction(highlight bool) *push.TweakAction {
	return &push.TweakAction{
		Notify:    "notify",
		Sound:     "default",
		HighLight: highlight,
	}
}
