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
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type TypingConsumer struct {
	typing         *sync.Map
	roomTimeLine   *sync.Map
	eventTimeLine  *sync.Map
	eventTimer     *sync.Map
	eventRead      *sync.Map
	ntyUserMap     *sync.Map
	joined         *sync.Map
	joinedTimer    *sync.Map
	calculateDelay int64
	typingTimeOut  int64
	delay          int64

	listener *common.Listener
}

func NewTypingConsumer(
	calculateDelay int64,
	typingTimeOut int64,
	delay int64,
) *TypingConsumer {
	typingConsumer := new(TypingConsumer)
	typingConsumer.typing = new(sync.Map)
	typingConsumer.roomTimeLine = new(sync.Map)
	typingConsumer.eventTimeLine = new(sync.Map)
	typingConsumer.eventRead = new(sync.Map)
	typingConsumer.eventTimer = new(sync.Map)
	typingConsumer.ntyUserMap = new(sync.Map)
	typingConsumer.joined = new(sync.Map)
	typingConsumer.joinedTimer = new(sync.Map)
	typingConsumer.calculateDelay = calculateDelay
	typingConsumer.typingTimeOut = typingTimeOut
	typingConsumer.delay = delay
	typingConsumer.listener = common.NewListener()
	return typingConsumer
}

func (s *TypingConsumer) StartCalculate() {
	s.startCalculate()
	s.startProduce()
}

func (s *TypingConsumer) startProduce() error {
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("TypingConsumer startProduce panic recovered err %#v", e)
				go s.startProduce()
			}
		}()
		t := time.NewTimer(time.Millisecond * time.Duration(s.delay))
		for {
			select {
			case <-t.C:
				s.roomTimeLine.Range(func(key, val interface{}) bool {
					var users []string
					users = []string{}
					roomID := key.(string)

					item, ok := s.typing.Load(roomID)
					if ok {
						typingMap := item.(*sync.Map)
						typingMap.Range(func(user, _ interface{}) bool {
							users = append(users, user.(string))
							return true
						})
					}

					if len(users) == 0 {
						//已经无人typing
						s.typing.Delete(roomID)
					}

					event := gomatrixserverlib.ClientEvent{}
					event.Type = "m.typing"
					event.RoomID = roomID
					event.Content, _ = json.Marshal(
						struct {
							UserIds []string `json:"user_ids"`
						}{UserIds: users},
					)

					s.eventTimeLine.Store(roomID, event)
					s.eventRead.Store(roomID, new(sync.Map))
					s.broadcastTypingEvent(roomID)
					s.roomTimeLine.Delete(roomID)
					s.eventTimer.Store(roomID, time.Now().Unix())

					if join, ok := s.joined.Load(roomID); ok {
						for _, user := range join.([]string) {
							if roomMap, ok := s.ntyUserMap.Load(user); ok {
								typingMap := roomMap.(*sync.Map)
								typingMap.Store(roomID, true)
							} else {
								typingMap := new(sync.Map)
								typingMap.Store(roomID, true)
								s.ntyUserMap.Store(user, typingMap)
							}
						}
					}

					return true
				})

				t.Reset(time.Millisecond * time.Duration(s.delay))
			}
		}
	}()
	return nil
}

func (s *TypingConsumer) startCalculate() error {
	var poc1 func()
	var poc2 func()
	var poc3 func()
	poc1 = func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("TypingConsumer startCalculate poc1 panic recovered err %#v", e)
				go poc1()
			}
		}()
		t := time.NewTimer(time.Millisecond * time.Duration(s.calculateDelay))
		for {
			select {
			case <-t.C:
				now := time.Now().Unix()

				s.typing.Range(func(key, val interface{}) bool {
					rid := key.(string)
					users := val.(*sync.Map)
					users.Range(func(key1, val1 interface{}) bool {
						uid := key1.(string)
						ts := val1.(int64)
						if now-ts > s.typingTimeOut {
							users.Delete(uid)
							s.roomTimeLine.Store(rid, true)
						}
						return true
					})
					s.typing.Store(rid, users)
					return true
				})
				t.Reset(time.Millisecond * time.Duration(s.calculateDelay))
			}
		}
	}

	poc2 = func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("TypingConsumer startCalculate poc2 panic recovered err %#v", e)
				go poc1()
			}
		}()
		t := time.NewTimer(time.Millisecond * time.Duration(s.calculateDelay))
		for {
			select {
			case <-t.C:
				now := time.Now().Unix()

				s.eventTimer.Range(func(key, val interface{}) bool {
					rid := key.(string)
					ts := val.(int64)
					if now-ts > s.typingTimeOut {
						s.eventTimer.Delete(rid)
						s.eventRead.Delete(rid)
						s.eventTimeLine.Delete(rid)

						if join, ok := s.joined.Load(rid); ok {
							for _, user := range join.([]string) {
								if roomMap, ok := s.ntyUserMap.Load(user); ok {
									typingMap := roomMap.(*sync.Map)
									typingMap.Delete(rid)

									empty := true
									typingMap.Range(func(key, value interface{}) bool {
										empty = false
										return false
									})
									if empty {
										s.ntyUserMap.Delete(user)
									}
								}
							}
						}
					}
					return true
				})
				t.Reset(time.Millisecond * time.Duration(s.calculateDelay))
			}
		}
	}

	poc3 = func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("TypingConsumer startCalculate poc3 panic recovered err %#v", e)
				go poc1()
			}
		}()
		t := time.NewTimer(time.Millisecond * time.Duration(s.calculateDelay))
		for {
			select {
			case <-t.C:
				now := time.Now().Unix()

				s.joinedTimer.Range(func(key, val interface{}) bool {
					ts := val.(int64)
					if now-ts > s.typingTimeOut*10 {
						s.joined.Delete(key)
						s.joinedTimer.Delete(key)
					}
					return true
				})
				t.Reset(time.Millisecond * time.Duration(s.calculateDelay))
			}
		}
	}

	go poc1()
	go poc2()
	go poc3()

	return nil
}

func (s *TypingConsumer) ExistsTyping(userID, deviceID, curRoomID string) (exists bool) {
	exists = false
	readKey := fmt.Sprintf("%s:%s", userID, deviceID)

	if roomMap, ok := s.ntyUserMap.Load(userID); ok {
		if curRoomID == "" {
			typingMap := roomMap.(*sync.Map)
			typingMap.Range(func(key, value interface{}) bool {
				roomID := key.(string)
				if readMap, ok := s.eventRead.Load(roomID); ok {
					keyMap := readMap.(*sync.Map)
					if _, ok := keyMap.Load(readKey); !ok {
						exists = true
						return false
					}
				}
				return true
			})
		} else {
			if readMap, ok := s.eventRead.Load(curRoomID); ok {
				keyMap := readMap.(*sync.Map)
				if _, ok := keyMap.Load(readKey); !ok {
					exists = true
				}
			}
		}
	}

	return exists
}

func (s *TypingConsumer) AddRoomJoined(roomID string, joined []string) {
	s.joined.Store(roomID, joined)
	s.joinedTimer.Store(roomID, time.Now().Unix())
}

func (s *TypingConsumer) AddTyping(roomID, userID string) {
	item, ok := s.typing.Load(roomID)
	if !ok {
		typingMap := new(sync.Map)
		typingMap.Store(userID, time.Now().Unix())
		s.typing.Store(roomID, typingMap)
		s.roomTimeLine.Store(roomID, true)
	} else {
		typingMap := item.(*sync.Map)
		typingMap.Store(userID, time.Now().Unix())
		s.roomTimeLine.Store(roomID, true)
	}
}

func (s *TypingConsumer) RemoveTyping(roomID, userID string) {
	item, ok := s.typing.Load(roomID)
	if ok {
		typingMap := item.(*sync.Map)
		_, ok := typingMap.Load(userID)
		if ok {
			typingMap.Delete(userID)
			s.roomTimeLine.Store(roomID, true)
		}
	}
}

func (s *TypingConsumer) GetTyping(userID, deviceID, curRoomID string) []gomatrixserverlib.ClientEvent {
	var events []gomatrixserverlib.ClientEvent
	events = []gomatrixserverlib.ClientEvent{}
	readKey := fmt.Sprintf("%s:%s", userID, deviceID)

	if roomMap, ok := s.ntyUserMap.Load(userID); ok {
		if curRoomID == "" {
			typingMap := roomMap.(*sync.Map)
			typingMap.Range(func(key, value interface{}) bool {
				roomID := key.(string)
				if readMap, ok := s.eventRead.Load(roomID); ok {
					keyMap := readMap.(*sync.Map)
					if _, ok := keyMap.Load(readKey); !ok {
						if event, ok := s.eventTimeLine.Load(roomID); ok {
							events = append(events, event.(gomatrixserverlib.ClientEvent))
							keyMap.Store(readKey, true)
						}
					}
				}
				return true
			})
		} else {
			if readMap, ok := s.eventRead.Load(curRoomID); ok {
				keyMap := readMap.(*sync.Map)
				if _, ok := keyMap.Load(readKey); !ok {
					if event, ok := s.eventTimeLine.Load(curRoomID); ok {
						events = append(events, event.(gomatrixserverlib.ClientEvent))
						keyMap.Store(readKey, true)
					}
				}
			}
		}
	}

	return events
}

func (s *TypingConsumer) RegisterTypingEvent(userID, curRoomID string, cb common.ListenerCallback) {
	s.listener.Register(userID+"_"+curRoomID, cb)
}

func (s *TypingConsumer) UnregisterTypingEvent(userID, curRoomID string, cb common.ListenerCallback) {
	s.listener.Unregister(userID+"_"+curRoomID, cb)
}

func (s *TypingConsumer) broadcastTypingEvent(roomID string) {
	if join, ok := s.joined.Load(roomID); ok {
		for _, user := range join.([]string) {
			s.listener.Broadcast(user+"_"+roomID, nil)
			s.listener.Broadcast(user+"_", nil)
		}
	}
}
