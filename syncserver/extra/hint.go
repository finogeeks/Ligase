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

package extra

import (
	"container/list"
	"fmt"
	"sort"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	jsoniter "github.com/json-iterator/go"
)

type Content struct {
	Name              string         `json:"name,omitempty"`
	MemberShip        string         `json:"membership,omitempty"`
	DisplayName       string         `json:"displayname,omitempty"`
	Reason            string         `json:"reason,omitempty"`
	EventsDefault     *int           `json:"events_default,omitempty"`
	HistoryVisibility string         `json:"history_visibility,omitempty"`
	Invite            *int           `json:"invite,omitempty"`
	Users             map[string]int `json:"users"`
	HideIOChanNotice  *bool          `json:"hide_in_out_channel_notice,omitempty"`
	JoinGroupVerify   *bool          `json:"join_group_verify,omitempty"`
	ShareToWX         *bool          `json:"share_to_WX,omitempty"`
	Archive           bool           `json:"archive,omitempty"`
	Desc              string         `json:"desc,omitempty"`
}

type Unsigned struct {
	PrevContent struct {
		Content
	} `json:"prev_content"`
	PrevSender string `json:"prev_sender,omitempty"`
}

type user struct {
	ID   string
	Name string
}

type memberStates struct {
	membership string
	roomID     string
}

type topicWaterMark struct {
	IsWaterMark bool `json:"isWaterMark"`
}

const (
	MRoomCreate                 = "m.room.create"
	MRoomName                   = "m.room.name"
	MRoomNameCreate             = "m.room.name#create"
	MRoomJoinRules              = "m.room.join_rules"
	MRoomPowerLevels            = "m.room.power_levels"
	MRoomPowerLevelsBan         = "m.room.power_levels#ban"
	MRoomPowerLevelsBanSome     = "m.room.power_levels#ban$some"
	MRoomPowerLevelsUnBanSome   = "m.room.power_levels#unban$some"
	MRoomPowerLevelsBanBySome   = "m.room.power_levels#baned$some"
	MRoomPowerLevelsUnBanBySome = "m.room.power_levels#unbaned$some"
	MRoomPowerLevelsInvite      = "m.room.power_levels#invite"
	MRoomPowerLevelsNoticeIO    = "m.room.power_levels#notice$io"
	MRoomPowerLevelsShareWX     = "m.room.power_levels#share$wx"
	MRoomPowerLevelsVerify      = "m.room.power_levels#verify"
	MRoomPowerLevelsAdminTrans  = "m.room.power_levels#admin$trans"
	MRoomPowerLevelsAdminUp     = "m.room.power_levels#admin$up"
	MRoomPowerLevelsAdminDown   = "m.room.power_levels#admin$down"
	MRoomMember                 = "m.room.member"
	MRoomMemberJoin             = "m.room.member#join"
	MRoomMemberJoinDirect       = "m.room.member#join$direct"
	MRoomMemberInvite           = "m.room.member#invite"
	MRoomMemberLeaveInvite      = "m.room.member#leave$invite"
	MRoomMemberLeaveJoin        = "m.room.member#leave$join"
	MRoomMemberLeaveKickInvite  = "m.room.member#leave$invite+kick"
	MRoomMemberLeaveKickJoin    = "m.room.member#leave$join+kick"
	MRoomHistoryVisibility      = "m.room.history_visibility"
	MRoomRedaction              = "m.room.redaction"
	MRoomEncryption             = "m.room.encryption"
	MRoomArchive                = "m.room.archive"
	MRoomDesc                   = "m.room.desc"
	MRoomDescChange             = "m.room.desc#change"
	MRoomDescClear              = "m.room.desc#new"
	MRoomTopic                  = "m.room.topic"
	MRoomTopicWaterMark         = "m.room.topic#watermark"
	MRoomMessage                = "m.room.message"
	MRoomMessageShake           = "m.room.message#shake"
)

var (
	json       = jsoniter.ConfigCompatibleWithStandardLibrary
	hintFormat map[string]string
)

func init() {
	hintFormat = make(map[string]string)
	hintFormat[MRoomCreate] = "✔️消息禁止收藏、下载、转发、复制\n✔️默认开启水印背景\n✔️在移动端截屏会在房间内提示"
	hintFormat[MRoomName] = "%s修改%s名称为\"%s\""
	hintFormat[MRoomNameCreate] = "%s创建了频道\"%s\""
	hintFormat[MRoomMemberInvite] = "%s向%s发出了加入邀请"
	hintFormat[MRoomMemberJoinDirect] = "%s已接受%s的好友申请，你们现在可以开始聊天了"
	hintFormat[MRoomMemberJoin] = "%s%s%s加入了%s"
	hintFormat[MRoomMemberLeaveJoin] = "%s退出了%s"
	hintFormat[MRoomMemberLeaveInvite] = "%s拒绝了%s的邀请"
	hintFormat[MRoomMemberLeaveKickJoin] = "%s将%s移出了%s"
	hintFormat[MRoomMemberLeaveKickInvite] = "%s撤回了对%s的邀请"
	hintFormat[MRoomPowerLevels] = "%s修改了%s的权限\""
	hintFormat[MRoomPowerLevelsBan] = "%s已%s\"全员禁言\""
	hintFormat[MRoomPowerLevelsBanSome] = "你已将%s禁言"
	hintFormat[MRoomPowerLevelsUnBanSome] = "你已解除%s的禁言"
	hintFormat[MRoomPowerLevelsBanBySome] = "%s已被%s禁言"
	hintFormat[MRoomPowerLevelsUnBanBySome] = "%s的禁言已被%s解除"
	hintFormat[MRoomPowerLevelsInvite] = "%s已%s\"仅管理员可加人\""
	hintFormat[MRoomPowerLevelsNoticeIO] = "%s已%s\"不显示成员进出频道提示\""
	hintFormat[MRoomPowerLevelsShareWX] = "%s已%s\"允许分享频道到微信\""
	hintFormat[MRoomPowerLevelsVerify] = "%s已%s\"邀请加入需管理员确认\""
	hintFormat[MRoomPowerLevelsAdminTrans] = "%s将管理员转移给了%s"
	hintFormat[MRoomPowerLevelsAdminUp] = "%s将%s设置为管理员"
	hintFormat[MRoomPowerLevelsAdminDown] = "%s将%s撤销了管理员"
	hintFormat[MRoomHistoryVisibility] = "%s已%s\"新成员历史消息可见\""
	hintFormat[MRoomRedaction] = "%s撤回了一条消息"
	hintFormat[MRoomEncryption] = "%s已%s\"端到端加密\""
	hintFormat[MRoomArchive] = "%s%s了此频道"
	hintFormat[MRoomDescChange] = "%s将频道描述为：%s"
	hintFormat[MRoomDescClear] = "%s清空了频道描述"
	hintFormat[MRoomTopicWaterMark] = "%s已%s\"水印背景\""
	hintFormat[MRoomMessageShake] = "%s发送了一个窗口抖动"
}

func getFormat(evType string) string {
	if v, ok := hintFormat[evType]; ok {
		return v
	}
	return ""
}

func GetDisplayName(displayNameRepo *repos.DisplayNameRepo, userID string) string {
	if len(userID) == 0 {
		return "${}"
	}

	// return displayNameRepo.GetDisplayName(userID)
	return fmt.Sprintf("${%s}", userID) // name tag for front end
}

func isAdmin(user string, power *common.PowerLevelContent) bool {
	if power != nil {
		if p, ok := power.Users[user]; ok && p == 100 {
			return true
		}
	}
	return false
}

func getAdmin(users *map[string]int) (map[string]bool, map[string]bool) {
	admins := make(map[string]bool)
	nonAdmins := make(map[string]bool)

	if users != nil {
		for user, p := range *users {
			if p == 100 {
				admins[user] = true
			} else {
				nonAdmins[user] = true
			}
		}
	}
	return admins, nonAdmins
}

func getNonreduAdmins(curUsers, prevUsers map[string]int) (map[string]bool, map[string]bool, map[string]bool, map[string]bool) {
	curAdmins := make(map[string]bool)
	curNonAdmins := make(map[string]bool)
	prevAdmins := make(map[string]bool)
	prevNonAdmins := make(map[string]bool)

	if curUsers != nil {
		for user, curPower := range curUsers {
			if prePower, ok := prevUsers[user]; ok && curPower == prePower {
				continue
			}
			if curPower == 100 {
				curAdmins[user] = true
			} else {
				curNonAdmins[user] = true
			}
		}
	}
	if prevUsers != nil {
		for user, prePower := range prevUsers {
			if curPower, ok := curUsers[user]; ok && prePower == curPower {
				continue
			}
			if prePower == 100 {
				prevAdmins[user] = true
			} else {
				prevNonAdmins[user] = true
			}
		}
	}

	// len(curAdmins) and len(curNonAdmins) should be 0 or 1
	return curAdmins, curNonAdmins, prevAdmins, prevNonAdmins
}

func getNonreduAdmin(curUsers, prevUsers map[string]int) (string, string, string, string) {
	curAdmin, curNonAdmin, prevAdmin, prevNonAdmin := "", "", "", ""

	if curUsers != nil {
		for user, curPower := range curUsers {
			prePower, ok := prevUsers[user]
			if ok && curPower == prePower {
				continue
			}
			if curPower == 100 {
				curAdmin = user
			} else if prePower > 0 && curPower <= 0 {
				curNonAdmin = user
			}
		}
	}
	if prevUsers != nil {
		for user, prePower := range prevUsers {
			if curPower, ok := curUsers[user]; ok && prePower == curPower {
				continue
			}
			if prePower == 100 {
				prevAdmin = user
			} else {
				prevNonAdmin = user
			}
		}
	}
	return curAdmin, curNonAdmin, prevAdmin, prevNonAdmin
}

func getUserInfo(displayNameRepo *repos.DisplayNameRepo, userID string) *user {
	return &user{
		ID: userID,
		// Name: GetDisplayName(displayNameRepo, userID),
		Name: fmt.Sprintf("${%s}", userID), // name tag for front end
	}
}

func fromFederation(user1, user2 *user) bool {
	if user1 != nil && user2 != nil {
		domain1, _ := common.DomainFromID(user1.ID)
		domain2, _ := common.DomainFromID(user2.ID)
		if domain1 == domain2 {
			return false
		}
	}
	return true
}

func mRoomCreateHandler(repo *repos.RoomCurStateRepo, roomID string, e *gomatrixserverlib.ClientEvent) {
	state := repo.GetRoomState(roomID)
	if state == nil {
		return
	}
	if state.IsSecret() {
		e.Hint = getFormat(e.Type)
	}
}

func mRoomNameHandler(repo *repos.RoomCurStateRepo, userID, roomID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	state := repo.GetRoomState(roomID)
	if state == nil {
		return
	}
	if state.IsDirect() {
		return
	}

	content := Content{}
	unsigned := Unsigned{}
	json.Unmarshal(e.Content, &content)
	json.Unmarshal(e.Unsigned, &unsigned)

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}

	if state.IsChannel() {
		if unsigned.PrevContent.Name == "" {
			e.Hint = fmt.Sprintf(getFormat(e.Type+"#create"), operator, content.Name)
		} else if unsigned.PrevContent.Name != content.Name {
			e.Hint = fmt.Sprintf(getFormat(e.Type), operator, "频道", content.Name)
		}
	} else {
		if unsigned.PrevContent.Name != content.Name {
			if state.IsSecret() {
				e.Hint = fmt.Sprintf(getFormat(e.Type), operator, "保密房间", content.Name)
			} else {
				e.Hint = fmt.Sprintf(getFormat(e.Type), operator, "群聊房间", content.Name)
			}
		}
	}
}

func inviteHandler(state *repos.RoomState, evType string, receiver, sender, stateKey *user, e *gomatrixserverlib.ClientEvent) {
	if (state.IsChannel() && !state.GetFederate()) || (!state.IsChannel() && state.IsDirect()) {
		return
	}

	if receiver.ID == sender.ID {
		e.Hint = fmt.Sprintf(getFormat(evType), "你", stateKey.Name)
	} else {
		if receiver.ID == stateKey.ID {
			e.Hint = fmt.Sprintf(getFormat(evType), sender.Name, "你")
		} else {
			if isAdmin(receiver.ID, state.GetPowerLevels()) {
				e.Hint = fmt.Sprintf(getFormat(evType), sender.Name, stateKey.Name)
			}
		}
	}
}

func joinChannelHandler(state *repos.RoomState, evType string, receiver, sender, prevSender, stateKey *user, e *gomatrixserverlib.ClientEvent) {
	if state.GetJoinRule() == "public" {
		if receiver.ID == sender.ID {
			if stateKey.ID != "" && stateKey.ID != receiver.ID {
				e.Hint = fmt.Sprintf(getFormat(evType), "", "", stateKey.Name, "频道")
			} else {
				e.Hint = fmt.Sprintf(getFormat(evType), "", "", "你", "频道")
			}
		} else {
			if stateKey.ID != "" {
				if stateKey.ID != receiver.ID {
					e.Hint = fmt.Sprintf(getFormat(evType), "", "", stateKey.Name, "频道")
				} else {
					e.Hint = fmt.Sprintf(getFormat(evType), "", "", "你", "频道")
				}
			} else {
				e.Hint = fmt.Sprintf(getFormat(evType), "", "", sender.Name, "频道")
			}
		}
	} else {
		if receiver.ID == prevSender.ID {
			e.Hint = fmt.Sprintf(getFormat(evType), "你", "邀请", stateKey.Name, "频道")
		} else if receiver.ID == stateKey.ID {
			e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "邀请", "你", "频道")
		} else {
			e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "邀请", stateKey.Name, "频道")
		}
	}
}

func joinRoomHandler(state *repos.RoomState, isHuman bool, evType string, receiver, sender, prevSender, stateKey *user, e *gomatrixserverlib.ClientEvent, prevStates *list.List) {
	if state.IsDirect() {
		if isHuman && state.GetFederate() && fromFederation(sender, prevSender) {
			evType += "$direct"

			// FIXME: ugly code
			isReverseHint := false
			if prevStates != nil && prevStates.Len() >= 2 {
				ele := prevStates.Back()
				ele2 := prevStates.Back().Prev()
				prevRoom := (ele.Value).(memberStates).roomID
				prevRoom2 := (ele2.Value).(memberStates).roomID
				if prevRoom == prevRoom2 {
					prevMemberShip := (ele.Value).(memberStates).membership
					prevMemberShip2 := (ele2.Value).(memberStates).membership
					if prevMemberShip == "invite" && prevMemberShip2 == "leave" {
						isReverseHint = true
					}
				}
			}
			if isReverseHint == true {
				if receiver.ID == sender.ID {
					e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "你")
				} else {
					e.Hint = fmt.Sprintf(getFormat(evType), "你", prevSender.Name)
				}
			} else {
				if receiver.ID == prevSender.ID {
					e.Hint = fmt.Sprintf(getFormat(evType), stateKey.Name, "你")
				} else {
					e.Hint = fmt.Sprintf(getFormat(evType), "你", prevSender.Name)
				}
			}
		}
	} else {
		if receiver.ID == prevSender.ID {
			if state.IsSecret() {
				e.Hint = fmt.Sprintf(getFormat(evType), "你", "邀请", stateKey.Name, "保密房间")
			} else {
				e.Hint = fmt.Sprintf(getFormat(evType), "你", "邀请", stateKey.Name, "群聊")
			}
		} else {
			if receiver.ID == stateKey.ID {
				if state.IsSecret() {
					e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "邀请", "你", "保密房间")
				} else {
					e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "邀请", "你", "群聊")
				}
			} else {
				if state.IsSecret() {
					e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "邀请", stateKey.Name, "保密房间")
				} else {
					e.Hint = fmt.Sprintf(getFormat(evType), prevSender.Name, "邀请", stateKey.Name, "群聊")
				}
			}
		}
	}
}

func leaveJoinHandler(state *repos.RoomState, evType string, receiver, sender *user, e *gomatrixserverlib.ClientEvent) {
	leaver := "你"
	roomType := "群聊"

	if receiver.ID == sender.ID {
		if state.IsChannel() {
			roomType = "频道"
		} else if state.IsSecret() {
			roomType = "保密房间"
		}
	} else {
		leaver = sender.Name

		if !isAdmin(receiver.ID, state.GetPowerLevels()) {
			return
		}
		if state.IsChannel() {
			if state.GetJoinRule() == "invite" && !state.GetFederate() {
				roomType = "频道"
			}
		} else if state.IsSecret() {
			roomType = "保密房间"
		}
	}
	e.Hint = fmt.Sprintf(getFormat(evType), leaver, roomType)
}

func leaveJoinKickHandler(state *repos.RoomState, evType string, receiver, sender, stateKey *user, e *gomatrixserverlib.ClientEvent) {
	evType += "+kick"
	roomType := "群聊"

	if receiver.ID == sender.ID {
		if state.IsChannel() {
			roomType = "频道"
		} else if state.IsSecret() {
			roomType = "保密房间"
		}
		e.Hint = fmt.Sprintf(getFormat(evType), "你", stateKey.Name, roomType)
	} else if receiver.ID == stateKey.ID {
		e.Hint = fmt.Sprintf(getFormat(evType), sender.Name, "你", roomType)
	}
}

func leaveInviteHandler(state *repos.RoomState, evType string, receiver, sender, prevSender *user, e *gomatrixserverlib.ClientEvent) {
	if receiver.ID == prevSender.ID {
		e.Hint = fmt.Sprintf(getFormat(evType), sender.Name, "你")
	} else if receiver.ID == sender.ID {
		e.Hint = fmt.Sprintf(getFormat(evType), "你", prevSender.Name)
	} else {
		if isAdmin(receiver.ID, state.GetPowerLevels()) {
			e.Hint = fmt.Sprintf(getFormat(evType), sender.Name, prevSender.Name)
		}
	}
}

func leaveInviteKickHandler(evType string, receiver, sender, stateKey *user, e *gomatrixserverlib.ClientEvent) {
	evType += "+kick"
	if receiver.ID == sender.ID {
		e.Hint = fmt.Sprintf(getFormat(evType), "你", stateKey.Name)
	} else if receiver.ID == stateKey.ID {
		e.Hint = fmt.Sprintf(getFormat(evType), sender.Name, "你")
	}
}

func mRoomMemberHandler(repo *repos.RoomCurStateRepo, device *authtypes.Device, roomID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent, prevStates *list.List) {
	state := repo.GetRoomState(roomID)
	if state == nil {
		//log.Infof("hint mRoomMemberHandler get room state failed by roomID: %s", roomID)
		return
	}

	content := Content{}
	unsigned := Unsigned{}
	json.Unmarshal(e.Content, &content)
	json.Unmarshal(e.Unsigned, &unsigned)

	evType := e.Type + "#" + content.MemberShip
	receiver := getUserInfo(displayNameRepo, device.UserID)
	sender := getUserInfo(displayNameRepo, e.Sender)
	stateKey := getUserInfo(displayNameRepo, *e.StateKey)
	prevSender := getUserInfo(displayNameRepo, unsigned.PrevSender)

	if unsigned.PrevContent.MemberShip != "" {
		prevStates.PushBack(memberStates{unsigned.PrevContent.MemberShip, roomID})
	}

	if content.MemberShip == "invite" {
		// we don't need "invite msg" any more
		// inviteHandler(state, evType, receiver, sender, stateKey, e)
	} else if content.MemberShip == "join" {
		// TODO: for "auto_join", sender is stateKey, but is not for normal join, so we should distinguish them
		if state.GetCreator() == sender.ID && unsigned.PrevContent.MemberShip == "" {
			return
		}
		// modify profile handler, just ignore
		if unsigned.PrevContent.MemberShip == "join" {
			return
		}

		if state.IsChannel() {
			joinChannelHandler(state, evType, receiver, sender, prevSender, stateKey, e)
		} else {
			joinRoomHandler(state, device.IsHuman, evType, receiver, sender, prevSender, stateKey, e, prevStates)
		}
	} else if content.MemberShip == "leave" {
		if state.IsDirect() {
			return
		}

		evType += "$" + unsigned.PrevContent.MemberShip
		if unsigned.PrevContent.MemberShip == "join" {
			if stateKey.ID != sender.ID {
				// joined, kicked by someone
				leaveJoinKickHandler(state, evType, receiver, sender, stateKey, e)
			} else {
				// joined, leave by yourself
				leaveJoinHandler(state, evType, receiver, sender, e)
			}
		} else if unsigned.PrevContent.MemberShip == "invite" {
			if stateKey.ID != sender.ID {
				// not joined, kicked by someone
				leaveInviteKickHandler(evType, receiver, sender, stateKey, e)
			} else {
				// not joined, refused
				leaveInviteHandler(state, evType, receiver, sender, prevSender, e)
			}
		}
	}
}

func powerLevelsAtCreateHandler(operator string, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) bool {
	operation := "开启"
	var evType string

	if unsigned.PrevContent.EventsDefault == nil && content.EventsDefault != nil && *content.EventsDefault == 100 {
		evType = e.Type + "#ban"
		e.Hint = fmt.Sprintf(getFormat(evType), operator, operation)
	}
	if unsigned.PrevContent.Invite == nil && content.Invite != nil && *content.Invite == 100 {
		evType = e.Type + "#invite"
		if len(e.Hint) > 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
	if unsigned.PrevContent.HideIOChanNotice == nil && content.HideIOChanNotice != nil && *content.HideIOChanNotice {
		evType = e.Type + "#notice$io"
		if len(e.Hint) > 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
	if unsigned.PrevContent.JoinGroupVerify == nil && content.JoinGroupVerify != nil && *content.JoinGroupVerify {
		evType = e.Type + "#verify"
		if len(e.Hint) > 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
	if unsigned.PrevContent.ShareToWX == nil && content.ShareToWX != nil && *content.ShareToWX {
		evType = e.Type + "#share$wx"
		if len(e.Hint) > 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}

	if len(e.Hint) > 0 {
		return true
	}
	return false
}

func plModifyEventsDefaultHandler(operator string, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	operation := "开启"
	evType := e.Type + "#ban"

	if unsigned.PrevContent.EventsDefault != nil && content.EventsDefault != nil &&
		*content.EventsDefault != *unsigned.PrevContent.EventsDefault {
		if *content.EventsDefault != 100 && *unsigned.PrevContent.EventsDefault != 100 {
			return
		}
		if *unsigned.PrevContent.EventsDefault == 100 {
			operation = "关闭"
		}
		if len(e.Hint) != 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
}

func plModifyInviteHandler(operator string, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	operation := "开启"
	evType := e.Type + "#invite"

	if unsigned.PrevContent.Invite != nil && content.Invite != nil &&
		*content.Invite != *unsigned.PrevContent.Invite {
		if *content.Invite != 100 && *unsigned.PrevContent.Invite != 100 {
			return
		}
		if *unsigned.PrevContent.Invite == 100 {
			operation = "关闭"
		}
		if len(e.Hint) != 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
}

func plModifyNoticeHandler(operator string, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	operation := "开启"
	evType := e.Type + "#notice$io"

	if unsigned.PrevContent.HideIOChanNotice != nil && content.HideIOChanNotice != nil &&
		*content.HideIOChanNotice != *unsigned.PrevContent.HideIOChanNotice {
		if *unsigned.PrevContent.HideIOChanNotice {
			operation = "关闭"
		}
		if len(e.Hint) != 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
}

func plModifyJoinGroupVerifyHandler(operator string, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	operation := "开启"
	evType := e.Type + "#verify"

	if unsigned.PrevContent.JoinGroupVerify != nil && content.JoinGroupVerify != nil &&
		*content.JoinGroupVerify != *unsigned.PrevContent.JoinGroupVerify {
		if *unsigned.PrevContent.JoinGroupVerify {
			operation = "关闭"
		}
		if len(e.Hint) != 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
}

func plModifyShareWXHandler(operator string, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	operation := "开启"
	evType := e.Type + "#share$wx"

	if unsigned.PrevContent.ShareToWX != nil && content.ShareToWX != nil &&
		*content.ShareToWX != *unsigned.PrevContent.ShareToWX {
		if *unsigned.PrevContent.ShareToWX {
			operation = "关闭"
		}
		if len(e.Hint) != 0 {
			e.Hint += "\n"
		}
		e.Hint += fmt.Sprintf(getFormat(evType), operator, operation)
	}
}

func plModifyAdminHandler(userID, operator string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	if unsigned.PrevContent.Users == nil {
		return
	}

	curAdmin, curNonAdmin, _, _ := getNonreduAdmin(content.Users, unsigned.PrevContent.Users)
	if len(curAdmin) == 0 && len(curNonAdmin) == 0 {
		return
	}
	var evType, receiver string
	if len(curAdmin) > 0 {
		if len(curNonAdmin) > 0 {
			evType += e.Type + "#admin$trans"
		} else {
			evType += e.Type + "#admin$up"
		}
		receiver = GetDisplayName(displayNameRepo, curAdmin)
		if userID == curAdmin {
			receiver = "你"
		}
	} else {
		evType += e.Type + "#admin$down"
		receiver = GetDisplayName(displayNameRepo, curNonAdmin)
		if userID == curNonAdmin {
			receiver = "你"
		}
	}
	if len(e.Hint) != 0 {
		e.Hint += "\n"
	}
	e.Hint += fmt.Sprintf(getFormat(evType), operator, receiver)
}

func plModifyBanSomeHandler(userID, operator string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent, content *Content, unsigned *Unsigned) {
	add := []string{}
	remove := []string{}
	for k, v := range content.Users {
		if vv, ok := unsigned.PrevContent.Users[k]; ok {
			if v == common.BanSendMessagePowLevel && vv > common.BanSendMessagePowLevel {
				add = append(add, k)
			}
		} else {
			if v == common.BanSendMessagePowLevel {
				add = append(add, k)
			}
		}
	}
	for k, v := range unsigned.PrevContent.Users {
		if vv, ok := content.Users[k]; ok {
			if v == common.BanSendMessagePowLevel && vv > common.BanSendMessagePowLevel {
				remove = append(remove, k)
			}
		} else {
			if v == common.BanSendMessagePowLevel {
				remove = append(remove, k)
			}
		}
	}
	sort.Strings(add)
	sort.Strings(remove)

	addUsers := ""
	for i, v := range add {
		if i < len(add)-1 {
			if userID == v {
				addUsers += operator + "，"
			} else {
				addUsers += GetDisplayName(displayNameRepo, v) + "，"
			}
		} else {
			if userID == v {
				addUsers += operator
			} else {
				addUsers += GetDisplayName(displayNameRepo, v)
			}
		}
	}

	removeUsers := ""
	for i, v := range remove {
		if v == e.Sender {
			continue
		}
		if i < len(remove)-1 {
			if userID == v {
				removeUsers += operator + "，"
			} else {
				removeUsers += GetDisplayName(displayNameRepo, v) + "，"
			}
		} else {
			if userID == v {
				removeUsers += operator
			} else {
				removeUsers += GetDisplayName(displayNameRepo, v)
			}
		}
	}
	if addUsers != "" {
		if userID != e.Sender {
			e.Hint += fmt.Sprintf(getFormat(MRoomPowerLevelsBanBySome), addUsers, GetDisplayName(displayNameRepo, e.Sender))
		} else {
			e.Hint += fmt.Sprintf(getFormat(MRoomPowerLevelsBanSome), addUsers)
		}
	}
	if removeUsers != "" {
		e.Hint += "\n"
		if userID != e.Sender {
			e.Hint += fmt.Sprintf(getFormat(MRoomPowerLevelsUnBanBySome), removeUsers, GetDisplayName(displayNameRepo, e.Sender))
		} else {
			e.Hint += fmt.Sprintf(getFormat(MRoomPowerLevelsUnBanSome), removeUsers)
		}
	}
}

func mRoomPowerLevelsHandler(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	content := Content{}
	unsigned := Unsigned{}
	json.Unmarshal(e.Content, &content)
	json.Unmarshal(e.Unsigned, &unsigned)

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}

	if powerLevelsAtCreateHandler(operator, e, &content, &unsigned) {
		return
	}
	plModifyEventsDefaultHandler(operator, e, &content, &unsigned)
	plModifyInviteHandler(operator, e, &content, &unsigned)
	plModifyNoticeHandler(operator, e, &content, &unsigned)
	plModifyJoinGroupVerifyHandler(operator, e, &content, &unsigned)
	plModifyShareWXHandler(operator, e, &content, &unsigned)
	plModifyAdminHandler(userID, operator, displayNameRepo, e, &content, &unsigned)
	plModifyBanSomeHandler(userID, operator, displayNameRepo, e, &content, &unsigned)
}

func mRoomHistoryVisibilityHandler(repo *repos.RoomCurStateRepo, userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	content := Content{}
	unsigned := Unsigned{}
	json.Unmarshal(e.Content, &content)
	json.Unmarshal(e.Unsigned, &unsigned)

	// ignore when the room is created
	if unsigned.PrevContent.HistoryVisibility == "" {
		return
	}

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}
	if content.HistoryVisibility == "shared" && unsigned.PrevContent.HistoryVisibility != "shared" {
		e.Hint = fmt.Sprintf(getFormat(e.Type), operator, "开启")
	} else if content.HistoryVisibility == "joined" && unsigned.PrevContent.HistoryVisibility == "shared" {
		e.Hint = fmt.Sprintf(getFormat(e.Type), operator, "关闭")
	}
}

func mRoomRedactionHandler(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	content := Content{}
	json.Unmarshal(e.Content, &content)

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}
	e.Hint = fmt.Sprintf(getFormat(e.Type), operator)
}

func mRoomEncryptionHandler(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	content := Content{}
	json.Unmarshal(e.Content, &content)

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}

	// TODO: check on or off
	e.Hint = fmt.Sprintf(getFormat(e.Type), operator, "开启")
}

func mRoomArchiveHandler(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	content := Content{}
	json.Unmarshal(e.Content, &content)

	operator := "你"
	operation := "恢复"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}
	if content.Archive {
		operation = "归档"
	}
	e.Hint = fmt.Sprintf(getFormat(e.Type), operator, operation)
}

func mRoomDescHandler(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	content := Content{}
	json.Unmarshal(e.Content, &content)

	unsigned := Unsigned{}
	json.Unmarshal(e.Unsigned, &unsigned)

	if unsigned.PrevContent.Desc == "" && content.Desc == "" {
		return
	}

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}
	if content.Desc == "" {
		e.Hint = fmt.Sprintf(getFormat(MRoomDescClear), operator)
	} else {
		e.Hint = fmt.Sprintf(getFormat(MRoomDescChange), operator, content.Desc)
	}
}

func mRoomTopicHandller(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	var content struct {
		Topic string `json:"topic"`
	}
	json.Unmarshal(e.Content, &content)

	waterMark := topicWaterMark{}
	json.Unmarshal([]byte(content.Topic), &waterMark)

	var unsigned struct {
		PrevContent struct {
			Topic string `json:"topic"`
		} `json:"prev_content"`
	}
	json.Unmarshal(e.Unsigned, &unsigned)
	preWaterMark := topicWaterMark{}
	json.Unmarshal([]byte(unsigned.PrevContent.Topic), &preWaterMark)
	if waterMark.IsWaterMark == preWaterMark.IsWaterMark {
		return
	}

	operator := "你"
	if userID != e.Sender {
		operator = GetDisplayName(displayNameRepo, e.Sender)
	}
	if waterMark.IsWaterMark {
		e.Hint = fmt.Sprintf(getFormat(MRoomTopicWaterMark), operator, "开启")
	} else {
		e.Hint = fmt.Sprintf(getFormat(MRoomTopicWaterMark), operator, "关闭")
	}
}

func mRoomMessageHandler(userID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent) {
	var content struct {
		MsgType string `json:"msgtype"`
	}
	json.Unmarshal(e.Content, &content)
	if content.MsgType == "m.shake" {
		operator := "你"
		if userID != e.Sender {
			operator = GetDisplayName(displayNameRepo, e.Sender)
		}
		e.Hint = fmt.Sprintf(getFormat(MRoomMessageShake), operator, "开启")
	}
}

func doExtra(repo *repos.RoomCurStateRepo, device *authtypes.Device, roomID string, displayNameRepo *repos.DisplayNameRepo, e *gomatrixserverlib.ClientEvent, prvStates *list.List) {
	if e.Type == MRoomCreate {
		mRoomCreateHandler(repo, roomID, e)
	} else if e.Type == MRoomName {
		mRoomNameHandler(repo, device.UserID, roomID, displayNameRepo, e)
	} else if e.Type == MRoomMember {
		mRoomMemberHandler(repo, device, roomID, displayNameRepo, e, prvStates)
	} else if e.Type == MRoomPowerLevels {
		mRoomPowerLevelsHandler(device.UserID, displayNameRepo, e)
	} else if e.Type == MRoomHistoryVisibility {
		mRoomHistoryVisibilityHandler(repo, device.UserID, displayNameRepo, e)
	} else if e.Type == MRoomEncryption {
		mRoomEncryptionHandler(device.UserID, displayNameRepo, e)
	} else if e.Type == MRoomArchive {
		mRoomArchiveHandler(device.UserID, displayNameRepo, e)
	} else if e.Type == MRoomDesc {
		mRoomDescHandler(device.UserID, displayNameRepo, e)
	} else if e.Type == MRoomTopic {
		mRoomTopicHandller(device.UserID, displayNameRepo, e)
	}
}

func ExpandEventHint(event *gomatrixserverlib.ClientEvent, device *authtypes.Device, repo *repos.RoomCurStateRepo, displayNameRepo *repos.DisplayNameRepo) {
	if event == nil {
		return
	}

	prevStates := list.New()
	doExtra(repo, device, event.RoomID, displayNameRepo, event, prevStates)
}

func ExpandHints(repo *repos.RoomCurStateRepo, device *authtypes.Device, displayNameRepo *repos.DisplayNameRepo, res *syncapitypes.SyncServerResponse) {
	if res == nil {
		return
	}

	prevStatesJoin := list.New()
	for roomID, j := range res.Rooms.Join {
		for i, e := range j.Timeline.Events {
			// if len(j.Timeline.Events[i].Hint) > 0 {
			// 	return
			// }
			doExtra(repo, device, roomID, displayNameRepo, &e, prevStatesJoin)
			j.Timeline.Events[i].Hint = e.Hint
		}
	}

	prevStatesLeave := list.New()
	for roomID, ivt := range res.Rooms.Invite {
		for i, e := range ivt.InviteState.Events {
			doExtra(repo, device, roomID, displayNameRepo, &e, prevStatesLeave)
			ivt.InviteState.Events[i].Hint = e.Hint
		}
	}

	prevStatesInvite := list.New()
	for roomID, l := range res.Rooms.Leave {
		for i, e := range l.Timeline.Events {
			doExtra(repo, device, roomID, displayNameRepo, &e, prevStatesInvite)
			l.Timeline.Events[i].Hint = e.Hint
		}
	}
	/*for i, e := range res.AccountData.Events {
		doExtra(repo, device, cache, &e)
		res.AccountData.Events[i].Hint = e.Hint
	}
	for i, e := range res.Presence.Events {
		doExtra(repo, device, cache, &e)
		res.Presence.Events[i].Hint = e.Hint
	}
	*/
}
