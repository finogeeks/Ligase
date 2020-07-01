// Copyright 2017 New Vector Ltd
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

package basecomponent

import (
	"flag"

	"github.com/finogeeks/ligase/common/config"

	"log"
)

var configPath = flag.String("config", "dendrite.yaml", "The path to the config file. For more information, see the config file in this repository.")

// ParseFlags parses the commandline flags and uses them to create a config.
// If running as a monolith use `ParseMonolithFlags` instead.
func ParseFlags() {
	flag.Parse()

	if *configPath == "" {
		log.Fatal("--config must be supplied")
	}

	err := config.Load(*configPath)

	if err != nil {
		log.Fatalf("Invalid config file: %s", err)
	}
}

// ParseMonolithFlags parses the commandline flags and uses them to create a
// config. Should only be used if running a monolith. See `ParseFlags`.
func ParseMonolithFlags() {
	flag.Parse()

	if *configPath == "" {
		log.Fatal("--config must be supplied")
	}

	err := config.LoadMonolithic(*configPath)

	if err != nil {
		log.Fatalf("Invalid config file: %s", err)
	}
}
