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

package entry

import (
	"context"
	"encoding/json"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

func StartProfileRecover(base *basecomponent.BaseDendrite, cmd *serverCmdPar) {
	transportMultiplexer := common.GetTransportMultiplexer()
	kafka := base.Cfg.Kafka

	addProducer(transportMultiplexer, kafka.Producer.DBUpdates)

	for _, v := range dbUpdateProducerName {
		dbUpdates := kafka.Producer.DBUpdates
		dbUpdates.Topic = dbUpdates.Topic + "_" + v
		dbUpdates.Name = dbUpdates.Name + "_" + v
		addProducer(transportMultiplexer, dbUpdates)
	}

	transportMultiplexer.PreStart()
	transportMultiplexer.Start()

	syncDB := base.CreateSyncDB()
	accountsDB := base.CreateAccountsDB()

	profiles, err := accountsDB.GetAllProfile()
	if err != nil {
		panic(err)
	}

	profileMap := make(map[string]*authtypes.Profile)
	recoverUsers := []string{}
	for i, profile := range profiles {
		if profile.AvatarURL == "" || profile.DisplayName == "" {
			recoverUsers = append(recoverUsers, profile.UserID)
			profileMap[profile.UserID] = &profiles[i]
			log.Infof("recover profile user: %s, DisplayName: %s, AvatarURL: %s", profile.UserID, profile.DisplayName, profile.AvatarURL)
		}
	}

	if len(recoverUsers) > 0 {
		streams, _, err := syncDB.GetUserPresenceDataStream(context.TODO(), recoverUsers)
		if err != nil {
			panic(err)
		}
		for idx := range streams {
			stream := streams[idx]
			var presenceEvent gomatrixserverlib.ClientEvent
			err := json.Unmarshal(stream.Content, &presenceEvent)
			if err != nil {
				continue
			}
			content := types.PresenceShowJSON{}
			err = json.Unmarshal(presenceEvent.Content, &content)
			if err != nil {
				continue
			}

			profile, ok := profileMap[stream.UserID]
			if !ok {
				continue
			}

			if (content.AvatarURL != profile.AvatarURL && content.AvatarURL != "") ||
				(content.DisplayName != profile.DisplayName && content.DisplayName != "") {
				accountsDB.UpsertProfile(context.TODO(), stream.UserID, content.DisplayName, content.AvatarURL)
				log.Infof("start recover profile user: %s, DisplayName: %s, AvatarURL: %s", profile.UserID, content.DisplayName, content.AvatarURL)
			}
		}
	}

	log.Infof("profile recover finished.")

}

func loadPresence(syncDB model.SyncAPIDatabase) {
	limit := 1000
	offset := 0
	exists := true

	for exists {
		exists = false
		streams, _, err := syncDB.GetHistoryPresenceDataStream(context.TODO(), limit, offset)
		if err != nil {
			log.Panicf("PresenceDataStreamRepo load history err: %v", err)
			return
		}

		for idx := range streams {
			exists = true
			offset = offset + 1
			stream := streams[idx]
			var presenceEvent gomatrixserverlib.ClientEvent
			json.Unmarshal(stream.Content, &presenceEvent)
			content := types.PresenceShowJSON{}
			json.Unmarshal(presenceEvent.Content, &content)
			if content.AvatarURL != "" || content.DisplayName != "" {

			}
		}
	}
}
