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
	"github.com/finogeeks/ligase/common"
	"sync"

	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/tidwall/gjson"
)

type DisplayNameRepo struct {
	persist     model.SyncAPIDatabase
	displayName *sync.Map
	avatarUrl   *sync.Map

	// ext user info
	userName  *sync.Map
	jobNumber *sync.Map
	mobile    *sync.Map
	landline  *sync.Map
	email     *sync.Map
	state     *sync.Map
}

func NewDisplayNameRepo() *DisplayNameRepo {
	tls := new(DisplayNameRepo)
	tls.displayName = new(sync.Map)
	tls.avatarUrl = new(sync.Map)
	tls.userName = new(sync.Map)
	tls.jobNumber = new(sync.Map)
	tls.mobile = new(sync.Map)
	tls.landline = new(sync.Map)
	tls.email = new(sync.Map)
	tls.state = new(sync.Map)
	return tls
}

func (tl *DisplayNameRepo) SetPersist(db model.SyncAPIDatabase) {
	tl.persist = db
}

func (tl *DisplayNameRepo) AddPresenceDataStream(dataStream *types.PresenceStream) {
	value := gjson.Get(string(dataStream.Content), "content.displayname")
	if value.String() != "" {
		tl.displayName.Store(dataStream.UserID, value.String())
	}
	value = gjson.Get(string(dataStream.Content), "content.avatar_url")
	if value.String() != "" {
		tl.avatarUrl.Store(dataStream.UserID, value.String())
	}

	value = gjson.Get(string(dataStream.Content), "content.user_name")
	if value.String() != "" {
		tl.userName.Store(dataStream.UserID, value.String())
	}
	value = gjson.Get(string(dataStream.Content), "content.job_number")
	if value.String() != "" {
		tl.jobNumber.Store(dataStream.UserID, value.String())
	}
	value = gjson.Get(string(dataStream.Content), "content.mobile")
	if value.String() != "" {
		tl.mobile.Store(dataStream.UserID, value.String())
	}
	value = gjson.Get(string(dataStream.Content), "content.landline")
	if value.String() != "" {
		tl.landline.Store(dataStream.UserID, value.String())
	}
	value = gjson.Get(string(dataStream.Content), "content.email")
	if value.String() != "" {
		tl.email.Store(dataStream.UserID, value.String())
	}
	value = gjson.Get(string(dataStream.Content), "content.state")
	if value.Int() != 0 {
		tl.state.Store(dataStream.UserID, value.Int())
	}
}

func (tl *DisplayNameRepo) LoadHistory() {
	limit := 1000
	offset := 0
	exists := true

	span, ctx := common.StartSobSomSpan(context.Background(), "DisplayNameRepo.LoadHistory")
	defer span.Finish()
	for exists {
		exists = false
		streams, _, err := tl.persist.GetHistoryPresenceDataStream(ctx, limit, offset)
		if err != nil {
			log.Panicf("PresenceDataStreamRepo load history err: %v", err)
			return
		}

		for idx := range streams {
			exists = true
			offset = offset + 1
			tl.AddPresenceDataStream(&streams[idx])
		}
	}

}

func (tl *DisplayNameRepo) GetDisplayName(userID string) string {
	if value, ok := tl.displayName.Load(userID); ok {
		return value.(string)
	}

	name, _ := utils.NickNameFromID(userID)
	return name
}

func (tl *DisplayNameRepo) GetOriginDisplayName(userID string) string {
	if value, ok := tl.displayName.Load(userID); ok {
		return value.(string)
	}
	return ""
}

func (tl *DisplayNameRepo) GetAvatarUrl(userID string) string {
	if value, ok := tl.avatarUrl.Load(userID); ok {
		return value.(string)
	}
	return ""
}
func (tl *DisplayNameRepo) GetUserName(userID string) string {
	if value, ok := tl.userName.Load(userID); ok {
		return value.(string)
	}
	return ""
}
func (tl *DisplayNameRepo) GetJobNumber(userID string) string {
	if value, ok := tl.jobNumber.Load(userID); ok {
		return value.(string)
	}
	return ""
}
func (tl *DisplayNameRepo) GetMobile(userID string) string {
	if value, ok := tl.mobile.Load(userID); ok {
		return value.(string)
	}
	return ""
}
func (tl *DisplayNameRepo) GetLandline(userID string) string {
	if value, ok := tl.landline.Load(userID); ok {
		return value.(string)
	}
	return ""
}
func (tl *DisplayNameRepo) GetEmail(userID string) string {
	if value, ok := tl.email.Load(userID); ok {
		return value.(string)
	}
	return ""
}
func (tl *DisplayNameRepo) GetState(userID string) int {
	if value, ok := tl.state.Load(userID); ok {
		return int(value.(int64))
	}
	return 0
}