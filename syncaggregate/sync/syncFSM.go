package sync

import (
	"sync/atomic"
	"time"

	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type SyncStateType byte

const (
	// StateWaitClient 等待客户端 sync. 目前没用
	StateWaitClient SyncStateType = iota + 1

	// StateWaitReady 等待 SyncLoad 返回
	StateWaitReady

	// StateCheckUpdate
	StateCheckUpdate

	// StateSyncBuild
	StateSyncBuild

	SyncStateNewReq SyncStateType = iota + 1
	SyncStateLoadDone
	SyncStateProcessDone

	SyncDataUserEvent
	SyncDataAccData
	SyncDataTyping
	SyncDataUserReceipt
	SyncDataKeyChange
	SyncDataSTDEvent
	SyncDataPresence
	SyncDataUnknown
)

type SyncStateMsg struct {
	Typ  SyncStateType
	Data interface{}
}

type SyncDataMsg struct {
	Typ  SyncStateType
	Data interface{}
}

type SyncFSM struct {
	sm *SyncMng

	request *request

	// state 状态机
	state SyncStateType

	// hasUpdate 有待下发消息
	hasUpdate bool

	timer *time.Timer

	timerChan <-chan time.Time

	stateChan chan SyncStateMsg

	dataChan chan SyncDataMsg

	statusCode int
	response   *syncapitypes.Response

	onDatas map[SyncStateType][]*Callback
	hasData int32
}

func newSyncFSM(request *request, sm *SyncMng) *SyncFSM {
	return &SyncFSM{
		sm:        sm,
		request:   request,
		stateChan: make(chan SyncStateMsg, 1),
		dataChan:  make(chan SyncDataMsg, 64),
		hasData:   0,
	}
}

func (fsm *SyncFSM) Run() (int, *syncapitypes.Response) {
	// 查询内存，初始化相关存储，初始化 request
	// 若有待下发内容，s.hasUpdate = true

	// 向各 kafka/grpc 消息的 consumer 注册 dataChan

	// 启动一定是接收到了新 request
	fsm.StateChange(StateWaitReady)

	for {
		select {
		case <-fsm.request.ctx.Done(): // request done
			fsm.StateChange(StateWaitClient)
			return fsm.statusCode, fsm.response
		case v := <-fsm.stateChan:
			log.Infof("traceid:%s SyncFSM recive state %d, data: %v", fsm.request.traceId, v.Typ, v.Data)
			switch v.Typ {
			case SyncStateNewReq: // 客户端 sync
				fsm.StateChange(StateWaitReady)

			case SyncStateLoadDone: // SyncLoad 返回
				loadReadyInfo := v.Data.(OnLoadReadyInfo)
				if !loadReadyInfo.allReady {
					fsm.StateChange(StateWaitClient)
					return fsm.responseLoadNotReady()
				} else {
					if fsm.sm.isFullSync(fsm.request) {
						fsm.StateChange(StateSyncBuild)
					} else {
						fsm.StateChange(StateCheckUpdate)
					}
				}

			case SyncStateProcessDone: // SyncProcess 返回
				fsm.StateChange(StateWaitReady)
			}
		case v := <-fsm.dataChan:
			switch v.Typ {
			case SyncDataUserEvent: // 接收到事件更新, 更新 SyncMng 相关存储，更新 request
				log.Infof("traceid:%s ExistsUserEventUpdate: %v", fsm.request.traceId, v.Data)
			case SyncDataAccData: // account data 变更，更新 SyncMng 相关存储
				log.Infof("traceid:%s ExistsAccountDataUpdate: %v", fsm.request.traceId, v.Data)
			case SyncDataTyping:
				log.Infof("traceid:%s SyncDataTyping: %v", fsm.request.traceId, v.Data)
			case SyncDataUserReceipt:
				log.Infof("traceid:%s SyncDataUserReceipt: %v", fsm.request.traceId, v.Data)
			case SyncDataKeyChange:
				log.Infof("traceid:%s SyncDataKeyChange: %v", fsm.request.traceId, v.Data)
			case SyncDataSTDEvent:
				log.Infof("traceid:%s SyncDataSTDEvent: %v", fsm.request.traceId, v.Data)
			case SyncDataPresence:
				log.Infof("traceid:%s SyncDataPresence: %v", fsm.request.traceId, v.Data)
			default:
				log.Infof("traceid:%s SyncDataUnknown: %v", fsm.request.traceId, v)
			}
			if fsm.state != StateSyncBuild {
				fsm.hasUpdate = true
				time.Sleep(time.Millisecond * 100)
				fsm.StateChange(StateSyncBuild)
			}

		case <-fsm.timer.C:
			switch fsm.state {
			case StateWaitClient:
				log.Warnf("traceid:%s timer StateWaitClient: no client, exit", fsm.request.traceId)
				fsm.StateChange(StateWaitClient)
				return fsm.statusCode, fsm.response

			case StateWaitReady:
				log.Errorf("traceid:%s timer StateWaitReady: sync load timeout, send resp", fsm.request.traceId)
				fsm.StateChange(StateWaitClient)
				return fsm.responseLoadNotReady()

			case StateCheckUpdate:
				log.Infof("traceid:%s timer StateCheckUpdate2: new update", fsm.request.traceId)
				fsm.hasUpdate = false
				fsm.StateChange(StateSyncBuild)

			case StateSyncBuild:
				log.Errorf("traceid:%s timer StateSyncBuild: build sync timeout, send resp", fsm.request.traceId)
				fsm.StateChange(StateWaitClient)
			}
		}
		if fsm.response != nil {
			return fsm.statusCode, fsm.response
		}
	}
}

func (fsm *SyncFSM) ResetTimer(d time.Duration) {
	if fsm.timerChan != nil {
		fsm.timer.Stop()
	}

	fsm.timer = time.NewTimer(d)
	fsm.timerChan = fsm.timer.C
}

func (fsm *SyncFSM) isFullSync(req *request) bool {
	return req.isFullSync || req.marks.utlRecv == 0
}

func (fsm *SyncFSM) StateChange(new SyncStateType) {
	log.Infof("traceid:%s state change: %d -> %d", fsm.request.traceId, fsm.state, new)
	fsm.state = new
	switch new {
	case StateWaitClient:
		fsm.unregisterDataChan()
		fsm.statusCode, fsm.response = 0, nil
	case StateWaitReady:
		fsm.ResetTimer(time.Millisecond * time.Duration(fsm.request.latest-time.Now().UnixNano()/1000000))
	case StateCheckUpdate:
		fsm.ResetTimer(time.Millisecond * time.Duration(fsm.request.latest-time.Now().UnixNano()/1000000))
		fsm.waitEvent()
	case StateSyncBuild:
		fsm.ResetTimer(time.Second * 10)
		fsm.unregisterDataChan()
		fsm.statusCode, fsm.response = fsm.buildResponse()
	}
}

type OnLoadReadyInfo struct {
	allReady bool
}

func (fsm *SyncFSM) OnLoadReady() {
	allReady := fsm.request.ready && fsm.request.remoteFinished && fsm.request.remoteReady
	fsm.stateChan <- SyncStateMsg{
		Typ: SyncStateLoadDone,
		Data: OnLoadReadyInfo{
			allReady: allReady,
		},
	}
}

func (fsm *SyncFSM) OnSyncDataEvent(typ SyncStateType, data interface{}) {
	if atomic.CompareAndSwapInt32(&fsm.hasData, 0, 1) {
		fsm.dataChan <- SyncDataMsg{Typ: typ, Data: data}
	}
}

func (fsm *SyncFSM) responseLoadNotReady() (int, *syncapitypes.Response) {
	return fsm.sm.BuildNotReadyResponse(fsm.request, time.Now().UnixNano()/1000000)
}

func (fsm *SyncFSM) waitEvent() {
	fsm.onDatas = make(map[SyncStateType][]*Callback)
	device := fsm.request.device

	curUtl, token, err := fsm.sm.userTimeLine.LoadToken(device.UserID, device.ID, fsm.request.marks.utlRecv)
	if err != nil {
		log.Errorf("traceId:%s user:%s device:%s utl:%d load token err:%v", fsm.request.traceId, device.UserID, device.ID, fsm.request.marks.utlRecv, err)
		curUtl = fsm.request.marks.utlRecv
	}
	log.Infof("traceid:%s SyncFSM wait event rooms %d", fsm.request.traceId, len(token))

	fsm.registerDataChan(token)

	go func() {
		if fsm.sm.CheckNewEvent(fsm.request, token, curUtl, fsm.request.start, 0) {
			fsm.unregisterDataChan()
			fsm.OnSyncDataEvent(SyncDataUnknown, nil)
		}
	}()
}

func (fsm *SyncFSM) buildResponse() (int, *syncapitypes.Response) {
	return fsm.sm.BuildResponse(fsm.request)
}

func (fsm *SyncFSM) registerDataChan(rooms map[string]int64) {
	onData := func(typ SyncStateType, id string) *Callback {
		cb := &Callback{
			id: id,
			cb: func(key string, val interface{}) {
				fsm.OnSyncDataEvent(typ, val)
			},
		}
		fsm.onDatas[typ] = append(fsm.onDatas[typ], cb)
		return cb
	}
	sm := fsm.sm
	device := fsm.request.device
	for k := range rooms {
		sm.userTimeLine.RegisterRoomOffsetUpdate(k, onData(SyncDataUserEvent, device.UserID))
	}
	sm.userTimeLine.RegisterNewJoinRoomEvent(device.UserID, onData(SyncDataUserEvent, ""))
	sm.clientDataStreamRepo.RegisterAccountDataUpdate(device.UserID, onData(SyncDataAccData, ""))
	curRoomID := fsm.sm.userTimeLine.GetUserCurRoom(device.UserID, device.ID)
	sm.typingConsumer.RegisterTypingEvent(device.UserID, curRoomID, onData(SyncDataTyping, curRoomID))
	sm.userTimeLine.RegisterReceiptUpdate(device.UserID, onData(SyncDataUserReceipt, ""))
	sm.keyChangeRepo.RegisterKeyChangeUpdate(device.UserID, onData(SyncDataKeyChange, ""))
	sm.stdEventStreamRepo.RegisterSTDEventUpdate(device.UserID, device.ID, onData(SyncDataSTDEvent, ""))
	sm.presenceStreamRepo.RegisterPresenceEvent(device.UserID, onData(SyncDataPresence, ""))
}

func (fsm *SyncFSM) unregisterDataChan() {
	for _, v := range fsm.onDatas[SyncDataUserEvent] {
		fsm.sm.userTimeLine.UnregisterRoomOffsetUpdate(v.id, v)
	}
	device := fsm.request.device
	if v, ok := fsm.onDatas[SyncDataUserEvent]; ok {
		fsm.sm.userTimeLine.UnregisterNewJoinRoomEvent(device.UserID, v[0])
	}
	if v, ok := fsm.onDatas[SyncDataAccData]; ok {
		fsm.sm.clientDataStreamRepo.UnregisterAccountDataUpdate(device.UserID, v[0])
	}
	if v, ok := fsm.onDatas[SyncDataTyping]; ok {
		fsm.sm.typingConsumer.UnregisterTypingEvent(device.UserID, v[0].id, v[0])
	}

	if v, ok := fsm.onDatas[SyncDataUserReceipt]; ok {
		fsm.sm.userTimeLine.UnregisterReceiptUpdate(device.UserID, v[0])
	}
	if v, ok := fsm.onDatas[SyncDataKeyChange]; ok {
		fsm.sm.keyChangeRepo.UnregisterKeyChangeUpdate(device.UserID, v[0])
	}
	if v, ok := fsm.onDatas[SyncDataSTDEvent]; ok {
		fsm.sm.stdEventStreamRepo.UnregisterSTDEventUpdate(device.UserID, device.ID, v[0])
	}
	if v, ok := fsm.onDatas[SyncDataPresence]; ok {
		fsm.sm.presenceStreamRepo.UnregisterPresenceEvent(device.UserID, v[0])
	}
}

type Callback struct {
	id string
	cb func(string, interface{})
}

func (c *Callback) ID() string {
	return c.id
}

func (c *Callback) Cb(key string, val interface{}) {
	c.cb(key, val)
}
