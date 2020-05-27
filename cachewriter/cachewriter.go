// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package cachewriter

import (
	"github.com/finogeeks/ligase/cachewriter/consumers"
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func SetupCacheWriterComponent(
	base *basecomponent.BaseDendrite,
) {
	dbEvConsumer := consumers.NewDBEventCacheConsumer(base.Cfg)

	if err := dbEvConsumer.Start(); err != nil {
		log.Panicw("failed to start cache data consumer", log.KeysAndValues{"error", err})
	}
}
