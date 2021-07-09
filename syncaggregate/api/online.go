package api

import (
	"net/http"
	"strconv"

	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetServices("sync_aggregate_api")
	apiconsumer.SetAPIProcessor(ReqGetOnlineCount{})
	apiconsumer.SetAPIProcessor(ReqGetOnlineUsers{})
}

type ReqGetOnlineCount struct{}

func (ReqGetOnlineCount) GetRoute() string       { return "/online/count/{inst}" }
func (ReqGetOnlineCount) GetMetricsName() string { return "get online count" }
func (ReqGetOnlineCount) GetMsgType() int32      { return internals.MSG_GET_ONLINE_COUNT }
func (ReqGetOnlineCount) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetOnlineCount) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetOnlineCount) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetOnlineCount) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqGetOnlineCount) NewRequest() core.Coder {
	return new(external.GetOnlineCountRequest)
}
func (ReqGetOnlineCount) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetOnlineCountRequest)
	if vars != nil {
		msg.Inst = vars["inst"]
	}
	return nil
}
func (ReqGetOnlineCount) NewResponse(code int) core.Coder {
	return new(external.GetOnlineCountResponse)
}
func (ReqGetOnlineCount) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetOnlineCountRequest)
	inst, _ := strconv.Atoi(req.Inst)
	return []uint32{uint32(inst)}
}
func (ReqGetOnlineCount) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetOnlineCountRequest)
	inst, _ := strconv.Atoi(req.Inst)
	if uint32(inst) != c.Cfg.MultiInstance.Instance {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	resp := new(external.GetOnlineCountResponse)
	resp.Count = c.sm.GetOnlineRepo().GetOnlineUserCount()
	return http.StatusOK, resp
}

type ReqGetOnlineUsers struct{}

func (ReqGetOnlineUsers) GetRoute() string       { return "/online/users/{inst}" }
func (ReqGetOnlineUsers) GetMetricsName() string { return "get online users" }
func (ReqGetOnlineUsers) GetMsgType() int32      { return internals.MSG_GET_ONLINE_USERS }
func (ReqGetOnlineUsers) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetOnlineUsers) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetOnlineUsers) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetOnlineUsers) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqGetOnlineUsers) NewRequest() core.Coder {
	return new(external.GetOnlineUsersRequest)
}
func (ReqGetOnlineUsers) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetOnlineUsersRequest)
	if vars != nil {
		msg.Inst = vars["inst"]
	}
	return nil
}
func (ReqGetOnlineUsers) NewResponse(code int) core.Coder {
	return new(external.GetOnlineUsersResponse)
}
func (ReqGetOnlineUsers) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetOnlineUsersRequest)
	inst, _ := strconv.Atoi(req.Inst)
	return []uint32{uint32(inst)}
}
func (ReqGetOnlineUsers) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetOnlineUsersRequest)
	inst, _ := strconv.Atoi(req.Inst)
	if uint32(inst) != c.Cfg.MultiInstance.Instance {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	resp := new(external.GetOnlineUsersResponse)
	resp.Users = c.sm.GetOnlineRepo().GetOnlineUsers()
	return http.StatusOK, resp
}
