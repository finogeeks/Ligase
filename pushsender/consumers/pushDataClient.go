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
	"bytes"
	"context"
	"fmt"
	"github.com/finogeeks/ligase/common/filter"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/storage/model"
	"github.com/json-iterator/go"
	"github.com/nats-io/go-nats"

	"github.com/finogeeks/ligase/skunkworks/log"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type PushDataConsumer struct {
	cfg        *config.Dendrite
	pushFilter *filter.Filter
	rpcClient  *common.RpcClient
	pushDB     model.PushAPIDatabase
	pushCount  *sync.Map
	chanSize   uint32
	//msgChan    []chan *pushapitypes.PushPubContents
	msgChan    []chan common.ContextMsg
	httpClient *http.Client
	lock    	*sync.Mutex
}

func NewPushDataConsumer(
	cfg *config.Dendrite,
	pushDB model.PushAPIDatabase,
	client *common.RpcClient,
) *PushDataConsumer {
	s := &PushDataConsumer{
		cfg:       cfg,
		pushDB:    pushDB,
		rpcClient: client,
		chanSize:  16,
		lock: new(sync.Mutex),
	}
	s.pushCount = new(sync.Map)
	s.httpClient = &http.Client{
		Transport: s.createTransport(),
	}
	pushFilter := filter.GetFilterMng().Register("pushSender", nil)
	s.pushFilter = pushFilter
	return s
}

func (s *PushDataConsumer) createTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   200,
	}
}

func (s *PushDataConsumer) GetCB() common.MsgHandlerWithContext {
	return s.cb
}

func (s *PushDataConsumer) GetTopic() string {
	return pushapitypes.PushTopicDef
}

func (s *PushDataConsumer) Clean() {
}

func (s *PushDataConsumer) cb(ctx context.Context, msg *nats.Msg) {
	var result pushapitypes.PushPubContents
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("PushDataConsumer cb Unmarshal err: %v", err)
		return
	}

	idx := common.CalcStringHashCode(result.Input.RoomID) % s.chanSize
	log.Infof("PushDataConsumer cb slot:%d roomID:%s eventID:%s len:%d", idx, result.Input.RoomID, result.Input.EventID, len(s.msgChan[idx]))
	s.msgChan[idx] <- common.ContextMsg{Ctx: ctx, Msg: &result}
}

func (s *PushDataConsumer) startWorker(msgChan chan common.ContextMsg) {
	for msg := range msgChan {
		ctx := msg.Ctx
		data := msg.Msg.(*pushapitypes.PushPubContents)
		if s.pushFilter.Lookup([]byte(data.Input.EventID)) {
			log.Infof("PushDataConsumer startWorker lookup eventID:%s len(msgChan):%d", data.Input.EventID, len(msgChan))
			return
		}
		log.Infof("start push roomID:%s eventID:%s len(msgChan):%d len(contents):%d", data.Input.RoomID, data.Input.EventID, len(msgChan), len(data.Contents))
		s.pushFilter.Insert([]byte(data.Input.EventID))
		for _, content := range data.Contents {
			s.PushData(
				ctx,
				data.Input,
				content.Pushers,
				data.SenderDisplayName,
				data.RoomName,
				content.UserID,
				data.RoomAlias,
				content.Action,
				content.NotifyCount,
				data.CreateContent,
			)
		}
	}
}

func (s *PushDataConsumer) Start() error {
	s.msgChan = make([]chan common.ContextMsg, s.chanSize)
	for i := uint32(0); i < s.chanSize; i++ {
		s.msgChan[i] = make(chan common.ContextMsg, 512)
		go s.startWorker(s.msgChan[i])
	}

	s.rpcClient.ReplyWithContext(s.GetTopic(), s.cb)

	return nil
}

func (s *PushDataConsumer) SetPushFailTimes(pusherKey string, success bool) int {
	s.lock.Lock()
	defer s.lock.Unlock()
	if success {
		s.pushCount.Store(pusherKey, 0)
		return 0
	}

	if val, ok := s.pushCount.Load(pusherKey); ok {
		count := val.(int)
		count++
		s.pushCount.Store(pusherKey, count)
		return count
	} else {
		s.pushCount.Store(pusherKey, 1)
		return 1
	}
}

func (s *PushDataConsumer) PushData(
	ctx context.Context,
	input *gomatrixserverlib.ClientEvent,
	pushers *pushapitypes.Pushers,
	senderDisplayName,
	roomName,
	userID,
	roomAlias string,
	action *pushapitypes.TweakAction,
	notifyCount int64,
	createContent interface{},
) {
	defer func() {
		if e := recover(); e != nil {
			stack := common.PanicTrace(4)
			log.Panicf("%v\n%s\n", e, stack)
		}
	}()
	for _, pusher := range pushers.Pushers {
		var content pushapitypes.PushContent
		err := json.Unmarshal(input.Content, &content.Content)
		if err != nil {
			log.Errorf("PushData Unmarshal err: %d  Event: %s", err, string(input.EventID))
			continue
		}

		userIsTarget := false
		if (input.StateKey != nil) && (userID == *input.StateKey) {
			userIsTarget = true
		}

		var url string
		pushChannel := "ios"
		var pusherData interface{}
		if v, ok := interface{}(pusher.Data).(map[string]interface{}); ok {
			var data map[string]interface{}
			data = v
			if v, ok := data["url"]; ok {
				url = v.(string)
			}
			delete(data, "url")
			if v, ok := data["push_channel"]; ok {
				pushChannel = v.(string)
			}
			pusherData = data
		} else {
			continue
		}

		if roomName == "" {
			roomName = senderDisplayName
		}

		notify := pushapitypes.Notify{
			Notify: pushapitypes.Notification{
				EventId:           input.EventID,
				RoomId:            input.RoomID,
				Type:              input.Type,
				Sender:            input.Sender,
				SenderDisplayName: senderDisplayName,
				RoomName:          roomName,
				RoomAlias:         roomAlias,
				UserIsTarget:      userIsTarget,
				Priority:          "high",
				Content:           content.Content,
				Counts: pushapitypes.Counts{
					UnRead: notifyCount,
				},
				Devices: []pushapitypes.Device{
					{
						DeviceID:  pusher.DeviceID,
						UserName:  pusher.UserName,
						AppId:     pusher.AppId,
						PushKey:   pusher.PushKey,
						PushKeyTs: pusher.PushKeyTs,
						Data:      pusherData,
						Tweak: pushapitypes.Tweaks{
							Sound:     action.Sound,
							HighLight: action.HighLight,
						},
					},
				},
				CreateEvent: createContent,
			},
		}

		request, err := json.Marshal(notify)
		if err != nil {
			log.Errorw("process marshal error", log.KeysAndValues{"err", err})
			continue
		}

		if pushChannel == "ios" || pushChannel == "" {
			if s.cfg.PushService.PushServerUrl != "" {
				url = s.cfg.PushService.PushServerUrl
			}
		} else {
			if s.cfg.PushService.AndroidPushServerUrl != "" {
				url = s.cfg.PushService.AndroidPushServerUrl
			}
		}
		go s.doPush(url, request, pusher)
	}
}

func (s *PushDataConsumer) doPush(url string, request []byte, pusher pushapitypes.Pusher){
	code, body, err := s.HttpRequest(url, request)
	if err != nil {
		log.Errorw("http request error", log.KeysAndValues{"content", string(request), "error", err})
		return
	}

	pusherKey := fmt.Sprintf("%s:%s", pusher.AppId, pusher.PushKey)

	if code != http.StatusOK {
		log.Errorw("http request error", log.KeysAndValues{"status_code", code, "response", string(body), "appId", pusher.AppId, "pushkey", pusher.PushKey, "content", string(request)})

		failCount := s.SetPushFailTimes(pusherKey, false)
		if failCount > s.cfg.PushService.RemoveFailTimes {
			log.Warnf("for failed too many del appId:%s, pushKey:%s, display:%s", pusher.AppId, pusher.PushKey, pusher.DeviceDisplayName)
			if err := s.pushDB.DeletePushersByKey(context.TODO(), pusher.AppId, pusher.PushKey); err != nil {
				log.Errorw("delete pusher error", log.KeysAndValues{"err", err, "AppId", pusher.AppId, "PushKey", pusher.PushKey})
			}
		}
	} else {
		//用以追踪IOS重复推送问题
		log.Infof("push content success, appid:%s, pushkey:%s , content:%s", pusher.AppId, pusher.PushKey, string(request))
		s.SetPushFailTimes(pusherKey, true)

		var ack pushapitypes.PushAck
		err = json.Unmarshal(body, &ack)
		if len(ack.Rejected) > 0 {
			for _, v := range ack.Rejected {
				log.Warnf("for reject del pushKey:%s", v)
				if err := s.pushDB.DeletePushersByKeyOnly(context.TODO(), v); err != nil {
					log.Errorw("delete pusher error", log.KeysAndValues{"err", err})
				}
			}
		}
	}
}

func (s *PushDataConsumer) HttpRequest(
	reqUrl string,
	content []byte,
) (int, []byte, error) {
	bs := time.Now().UnixNano() / 1000000
	req, err := http.NewRequest(http.MethodPost, reqUrl, bytes.NewReader(content))
	if err != nil {
		return -1, nil, err
	}

	req.Close = true
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()
	spend := time.Now().UnixNano()/1000000 - bs
	log.Infof("post http to push-service spend:%d", spend)
	if err != nil {
		return -1, nil, err
	}
	b, _ := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}
