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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var host = flag.String("host", "127.0.0.1", "Name for the server")
var port = flag.Int("port", 8008, "port")
var domain = flag.String("domain", "dendrite", "domain of server")
var loop = flag.Int("loop", 100, "loop")
var cmd = flag.String("cmd", "login", "cmd")
var wg sync.WaitGroup

var sucess uint32
var fail uint32

func main() {
	flag.Parse()
	start := time.Now()
	rand.Seed(time.Now().UnixNano())

	if *cmd == "login" {
		url := buildLoginURL(*host, *port)
		for i := 0; i < *loop; i++ {
			wg.Add(1)
			go doLogin(i, url, false)
		}
	} else if *cmd == "createroom" {
		url := buildLoginURL(*host, *port)
		wg.Add(1)
		token := doLogin(0, url, true)
		url = buildCreateRoomURL(*host, token, *port)
		for i := 0; i < *loop; i++ {
			wg.Add(1)
			go func() {
				res, _ := doCreateRoom(i, url)
				if res != nil && res.StatusCode == 200 {
					atomic.AddUint32(&sucess, 1)
				} else {
					atomic.AddUint32(&fail, 1)
				}
			}()
		}
	} else if *cmd == "invite" {
		invite()
	}

	wg.Wait()
	fmt.Printf("cmd:%s use :%v, sucess: %d, fail: %d\n", *cmd, time.Now().Sub(start), sucess, fail)
}

func invite() {
	url := buildLoginURL(*host, *port)
	wg.Add(1)
	token := doLogin(0, url, true)
	fmt.Println("token: ", token)
	url = buildCreateRoomURL(*host, token, *port)
	randNum := rand.Int31n(10000000)
	wg.Add(1)
	res, err := doCreateRoom(int(randNum), url)
	if res.StatusCode != 200 || err != nil {
		fmt.Println("invite: createRoom error ", res.StatusCode, err)
		os.Exit(-1)
	}
	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		fmt.Println("invite: read createRoom resp error ", err)
		os.Exit(-1)
	}
	fmt.Println(string(data))
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	roomID, ok := m["room_id"].(string)
	if !ok {
		fmt.Println("invite: read createRoom resp error ", err)
		os.Exit(-1)
	}

	_ = roomID
	for i := 0; i < *loop; i++ {
		wg.Add(1)
		url := buildInviteURL(roomID, token, *host, *port)
		go func(i int) {
			defer wg.Done()
			res, err := doInvite(rand.Intn(10000000), url, true)
			if err != nil {
				fmt.Println("invite error", i, err)
				atomic.AddUint32(&fail, 1)
				return
			}
			if res != nil && res.StatusCode == 200 {
				fmt.Println("invite success,", i, res.StatusCode)
				atomic.AddUint32(&sucess, 1)
			} else {
				fmt.Println("invite failed,", i, res.StatusCode)
				atomic.AddUint32(&fail, 1)
			}
		}(i)
	}
}
