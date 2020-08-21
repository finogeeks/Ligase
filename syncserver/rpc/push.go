package rpc

import (
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/pushapi/routing"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/nats-io/nats.go"
	"strconv"
	"time"
)

type PushDataCB struct {
	rpcClient   *common.RpcClient
	chanSize    uint32
	msgChan     []chan *pushapitypes.PushDataRequest
	repos       *repos.PushDataRepo
	cfg 		*config.Dendrite
}

func (pd *PushDataCB) GetTopic() string {
	return types.PushDataTopicDef
}

func (pd *PushDataCB) GetCB() nats.MsgHandler {
	return pd.cb
}

func (pd *PushDataCB) Clean() {
	//do nothing
}

func NewPushDataConsumer(
	client *common.RpcClient,
	repos *repos.PushDataRepo,
	cfg *config.Dendrite,
) *PushDataCB {
	pd := &PushDataCB{
		rpcClient:   client,
		chanSize:    16,
		repos: 		 repos,
		cfg: 		 cfg,
	}
	return pd
}

func (pd *PushDataCB) cb(msg *nats.Msg) {
	var result pushapitypes.PushDataRequest
	if err := json.Unmarshal(msg.Data, &result); err != nil {
		log.Errorf("rpc push data cb error %v", err)
		return
	}
	result.Reply = msg.Reply
	if result.Slot == pd.cfg.MultiInstance.Instance {
		idx := common.CalcStringHashCode(strconv.FormatInt(time.Now().UnixNano(),10)) % pd.chanSize
		pd.msgChan[idx] <- &result
	}
}

func (pd *PushDataCB) startWorker(msgChan chan *pushapitypes.PushDataRequest) {
	for req := range msgChan {
		pd.process(req)
	}
}

func (pd *PushDataCB) Start() error {
	pd.msgChan = make([]chan *pushapitypes.PushDataRequest, pd.chanSize)
	for i := uint32(0); i < pd.chanSize; i++ {
		pd.msgChan[i] = make(chan *pushapitypes.PushDataRequest, 512)
		go pd.startWorker(pd.msgChan[i])
	}
	pd.rpcClient.Reply(pd.GetTopic(), pd.cb)
	return nil
}

func (pd *PushDataCB) process(req *pushapitypes.PushDataRequest) {
	switch req.ReqType {
	case types.GET_PUSHER_BY_DEVICE:
		pd.getPusherByDevice(req)
	case types.GET_PUSHRULE_BY_USER:
		pd.getPushRuleByUser(req)
	case types.Get_PUSHDATA_BATCH:
		pd.getPushDataBatch(req)
	default:
		log.Infof("unknown pushdata reqtype:%s", req.ReqType)
	}
}

func (pd *PushDataCB) getPusherByDevice(req *pushapitypes.PushDataRequest){
	var data pushapitypes.ReqPushUser
	if err := json.Unmarshal(req.Payload, &data); err != nil {
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Error: fmt.Sprintf("json.Unmarshal payload error %v", err),
		})
		return
	}
	pusher := routing.GetPushersByName(data.UserID, pd.repos, false, nil)
	byte, err := json.Marshal(pusher)
	if err != nil {
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Error: fmt.Sprintf("json.Marshal result error %v", err),
		})
	}else{
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Payload: byte,
		})
	}
}

func (pd *PushDataCB) getPushRuleByUser(req *pushapitypes.PushDataRequest){
	var data pushapitypes.ReqPushUser
	if err := json.Unmarshal(req.Payload, &data); err != nil {
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Error: fmt.Sprintf("json.Unmarshal payload error %v", err),
		})
		return
	}
	rules := routing.GetUserPushRules(data.UserID, pd.repos, true, nil)
	byte, err := json.Marshal(rules)
	if err != nil {
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Error: fmt.Sprintf("json.Marshal result error %v", err),
		})
	}else{
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Payload: byte,
		})
	}
}

func (pd *PushDataCB) getPushDataBatch(req *pushapitypes.PushDataRequest){
	var data pushapitypes.ReqPushUsers
	if err := json.Unmarshal(req.Payload, &data); err != nil {
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Error: fmt.Sprintf("json.Unmarshal payload error %v", err),
		})
		return
	}
	resp := pushapitypes.RespPushUsersData{
		Data: make(map[string]pushapitypes.RespPushData),
	}
	for _, user := range data.Users {
		r := pushapitypes.RespPushData{
			Pushers: routing.GetPushersByName(user, pd.repos, false, nil),
			Rules: routing.GetUserPushRules(user, pd.repos, false, nil),
		}
		resp.Data[user] = r
	}

	byte, err := json.Marshal(resp)
	if err != nil {
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Error: fmt.Sprintf("json.Marshal result error %v", err),
		})
	}else{
		pd.rpcClient.PubObj(req.Reply, pushapitypes.RpcResponse{
			Payload: byte,
		})
	}
}
