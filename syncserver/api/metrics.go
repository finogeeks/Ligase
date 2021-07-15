package api

import (
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/exporter"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"net/http"
	"strconv"
)

func init() {
	apiconsumer.SetServices("sync_server_api")
	apiconsumer.SetAPIProcessor(ReqGetMetrics{})
}

type ReqGetMetrics struct{}

func (ReqGetMetrics) GetRoute() string       { return "/syncserver/{inst}/metrics" }
func (ReqGetMetrics) GetMetricsName() string { return "syncserver_metrics" }
func (ReqGetMetrics) GetMsgType() int32      { return internals.MSG_GET_SYNC_SERVER_METRICS }
func (ReqGetMetrics) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqGetMetrics) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetMetrics) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetMetrics) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetMetrics) NewRequest() core.Coder {
	return new(external.GetSyncServerMetricsRequest)
}
func (ReqGetMetrics) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetSyncServerMetricsRequest)
	if vars != nil {
		msg.Inst = vars["inst"]
	}
	return nil
}
func (ReqGetMetrics) NewResponse(code int) core.Coder {
	return new(external.SyncServerMetrics)
}
func (ReqGetMetrics) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetSyncServerMetricsRequest)
	inst, _ := strconv.Atoi(req.Inst)
	return []uint32{uint32(inst)}
}
func (ReqGetMetrics) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetSyncServerMetricsRequest)
	inst, _ := strconv.Atoi(req.Inst)
	if inst != c.Cfg.Matrix.InstanceId {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	resp := exporter.GetSyncServerMetrics()
	return http.StatusOK, resp
}
