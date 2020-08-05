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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/feedstypes"
	push "github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/service"
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
	msgChan      []chan common.ContextMsg
	chanSize     uint32
	slotSize     uint32
}

func NewPushConsumer(
	cache service.Cache,
	client *common.RpcClient,
	complexCache *common.ComplexCache,
) *PushConsumer {
	s := &PushConsumer{
		cache:        cache,
		rpcClient:    client,
		complexCache: complexCache,
		chanSize:    20480,
		slotSize: 	 64,
	}
	s.pubTopic = push.PushTopicDef

	return s
}

func (s *PushConsumer) Start() {
	s.msgChan = make([]chan common.ContextMsg, s.slotSize)
	for i := uint32(0); i < s.slotSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, s.chanSize)
		go s.startWorker(s.msgChan[i])
	}
}

func (s *PushConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		data := msg.Msg.(*gomatrixserverlib.ClientEvent)
		s.OnEvent(msg.Ctx,data,data.EventOffset)
	}
}

func (s *PushConsumer) DispthEvent(ctx context.Context, ev *gomatrixserverlib.ClientEvent){
	idx := common.CalcStringHashCode(ev.RoomID) % s.slotSize
	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: ev}
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

func (s *PushConsumer) OnEvent(ctx context.Context, input *gomatrixserverlib.ClientEvent, eventOffset int64) {
	bs := time.Now().UnixNano() / 1000000
	defer func(bs int64,input *gomatrixserverlib.ClientEvent){
		spend := time.Now().UnixNano() /1000000 - bs
		log.Infof("PushConsumer onevent roomID:%s eventID:%s eventOffset:%d spend:%d", input.RoomID, input.EventID, input.EventOffset, spend)
	}(bs,input)
	eventJson, err := json.Marshal(&input)
	if err != nil {
		log.Errorf("PushConsumer processEvent marshal error %d, message %s", err, input.EventID)
		return
	}

	redactOffset := int64(-1)
	var members []string

	switch input.Type {
	case "m.room.message", "m.room.encrypted":
		members = s.getRoomMembers(input)
	case "m.call.invite":
		members = s.getRoomMembers(input)
	case "m.room._ext.leave", "m.room._ext.enter":
		members = s.getRoomMembers(input)
	case "m.room.redaction", "m.room.update":
		members = s.getRoomMembers(input)
		redactID := input.Redacts
		stream := s.roomHistory.GetStreamEv(ctx, input.RoomID, redactID)
		if stream != nil {
			redactOffset = stream.Offset
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
	senderDisplayName, _ := s.cache.GetDisplayNameByUser(input.Sender)

	//log.Errorf("notify count evaluate sender %s room %s members %d", input.Sender(), input.RoomID(), members)
	pushContents := push.PushPubContents{
		Contents: []*push.PushPubContent{},
	}

	var wg sync.WaitGroup
	for _, member := range members {
		wg.Add(1)
		go func(
			member string,
			input *gomatrixserverlib.ClientEvent,
			senderDisplayName string,
			memCount int,
			eventOffset,
			redactOffset int64,
			eventJson *[]byte,
			pushContents *push.PushPubContents,
		) {
			s.preProcessPush(ctx, &member, input, &senderDisplayName, memCount, eventOffset, redactOffset, eventJson, pushContents)
			wg.Done()
		}(member, input, senderDisplayName, memCount, eventOffset, redactOffset, &eventJson, &pushContents)
	}
	wg.Wait()

	//将需要推送的消息聚合一次推送
	if s.rpcClient != nil && len(pushContents.Contents) > 0 {
		pushContents.Input = input
		pushContents.RoomAlias = ""
		pushContents.SenderDisplayName = senderDisplayName
		go s.pubPushContents(ctx, &pushContents, &eventJson)
	}
}

func (s *PushConsumer) preProcessPush(
	ctx context.Context,
	member *string,
	input *gomatrixserverlib.ClientEvent,
	senderDisplayName *string,
	memCount int,
	eventOffset,
	redactOffset int64,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
) {
	if *member != input.Sender {
		if input.Type == "m.room.redaction" || input.Type == "m.room.update" {
			if s.eventRepo.GetUserLastOffset(ctx, *member, input.RoomID) < redactOffset || redactOffset == -1 {
				//如果一个用户读完消息以后，有新的未读，此时hs重启，其他人撤销之前已读消息，计数会不准确
				//高亮信息撤回，暂时也不好处理计减
				s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *member, "decrease")
			}
		}

		pushers := routing.GetPushersByName(*member, s.cache, false)

		global := routing.GetUserPushRules(*member, s.cache, false)

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

		displayName, _, _ := s.complexCache.GetProfileByUserID(ctx, *member)

		s.processPush(&pushers, &rules, input, &displayName, member, memCount, eventJson, pushContents)
	} else {
		//当前用户在发消息，应该把该用户的未读数置为0
		s.eventRepo.AddUserReceiptOffset(*member, input.RoomID, eventOffset)
		s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *member, "reset")
	}
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

func (s *PushConsumer) getRoomName(ctx context.Context, roomID string) string {
	states := s.rsTimeline.GetStates(ctx, roomID)
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

func (s *PushConsumer) getCreateContent(ctx context.Context, roomID string) interface{} {
	states := s.rsTimeline.GetStates(ctx, roomID)

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

func (s *PushConsumer) pubPushContents(ctx context.Context, pushContents *push.PushPubContents, eventJson *[]byte) {
	//临时处理，rcs去除邀请重试以后可以去掉
	if pushContents.Input.Type == "m.room.member" {
		result := gjson.Get(string(*eventJson), "unsigned.prev_content.membership")
		if result.Str == "invite" {
			return
		}
	}

	pushContents.RoomName = s.getRoomName(ctx, pushContents.Input.RoomID)
	createContent := s.getCreateContent(ctx, pushContents.Input.RoomID)
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
	userDisplayName,
	userID *string,
	memCount int,
	eventJson *[]byte,
	pushContents *push.PushPubContents,
) {
	//这种写法真的很挫，但没找到其他的处理方式
	result := gjson.Get(string(*eventJson), "content.msgtype")
	if result.Str == "m.notice" {
		return
	}

	for _, v := range *rules {
		if !v.Enabled {
			continue
		}
		if s.checkCondition(&v.Conditions, userID, userDisplayName, memCount, eventJson) {
			action := s.getActions(v.Actions)

			if input.Type == "m.room.message" || input.Type == "m.room.encrypted" {
				s.countRepo.UpdateRoomReadCount(input.RoomID, input.EventID, *userID, "increase")
			}

			count, _ := s.countRepo.GetRoomReadCount(input.RoomID, *userID)

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
		}
	}
}

func (s *PushConsumer) checkCondition(
	conditions *[]push.PushCondition,
	userID,
	displayName *string,
	memCount int,
	eventJSON *[]byte,
) bool {
	if len(*conditions) > 0 {
		for _, v := range *conditions {
			match := s.isMatch(&v, userID, displayName, memCount, eventJSON)
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
	userID,
	displayName *string,
	memCount int,
	eventJSON *[]byte,
) bool {
	switch condition.Kind {
	case "event_match":
		return s.eventMatch(condition, userID, eventJSON)
	case "contains_display_name":
		return s.containsDisplayName(displayName, eventJSON)
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

	return s.globalMatch(pattern, &context, wordBoundary)
}

func (s *PushConsumer) containsDisplayName(
	displayName *string,
	eventJSON *[]byte,
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

	return s.globalMatch(displayName, &valueStr, true)
}

func (s *PushConsumer) globalMatch(
	global,
	req *string,
	wordBoundary bool,
) bool {
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
		if v, ok := interface{}(val).(string); ok {
			action.Notify = v
			continue
		}
		if v, ok := interface{}(val).(push.Tweak); ok {
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
		}
	}

	return action
}
