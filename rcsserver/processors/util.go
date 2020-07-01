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
	"encoding/json"
	"strings"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/plugins/message/external"
)

func isBot(account string) bool {
	return strings.Contains(account, "-bot:")
}

func remark(account string, ev *gomatrixserverlib.Event) string {
	var content external.MemberContent
	if err := json.Unmarshal(ev.Content(), &content); err != nil {
		log.Errorf("Failed to unmarshal external.MemberContent: %v\n", err)
	}
	var remark string
	if content.DisplayName != "" {
		remark = content.DisplayName
	} else {
		remark = getUsername(account)
	}
	return remark
}

func getUsername(account string) string {
	index1 := strings.Index(account, ":")
	if index1 == -1 {
		return ""
	}

	arr1 := strings.Split(account, ":")
	return arr1[0][1:]
}

func getDomain(account string) string {
	domain, _ := common.DomainFromID(account)
	return domain
}

func getID(fcID, toFcID string) string {
	if fcID < toFcID {
		return fcID + toFcID
	}

	return toFcID + fcID
}
