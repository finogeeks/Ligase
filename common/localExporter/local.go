package localExporter

import (
	"github.com/finogeeks/ligase/common/config"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"strconv"
)

var (
	monitor  = mon.GetInstance()
	httpRequest  mon.LabeledCounter
	httpDuration mon.LabeledHistogram
	handleDuration mon.LabeledHistogram
	grpcRequest mon.LabeledCounter
	grpcDuration mon.LabeledHistogram
	socketCount mon.LabeledGauge
	syncNumberSameTime mon.LabeledGauge
	Instance string
)

func init(){
	// http请求量统计
	httpRequest = monitor.NewLabeledCounter("chat_http_requests_count", []string{"server_name","srv_inst","method","path","code"})
	// http整个请求耗时统计
	httpDuration = monitor.NewLabeledHistogram("chat_http_requests_duration_milliseconds",
		[]string{"server_name","srv_inst", "method", "path", "code"},
		[]float64{10.0, 50.0, 100.0, 500.0, 1000.0, 3000.0},)
	// 主要服务操作步骤耗时统计
	handleDuration = monitor.NewLabeledHistogram("chat_handle_requests_duration_milliseconds",
		[]string{"server_name","srv_inst", "method", "path", "code"},
		[]float64{10.0, 50.0, 100.0, 500.0, 1000.0, 3000.0},)
	// grpc时延统计
	grpcDuration = monitor.NewLabeledHistogram("chat_grpc_requests_duration_milliseconds",
		[]string{"from","to", "srv_inst", "method","code"},
		[]float64{10.0, 50.0, 100.0, 500.0, 1000.0, 3000.0},)
	// 服务socket使用情况
	socketCount = monitor.NewLabeledGauge("chat_socket_count",[]string{"server_name", "srv_inst", "proto", "socket_state"})
	// 同时sync的用户数量
	syncNumberSameTime = monitor.NewLabeledGauge("sync_number_same_time", []string{"server_name", "srv_inst"})
}

// must after config load over
func InitMon(){
	Instance = strconv.Itoa(config.GetConfig().Matrix.InstanceId)
}

const (
	// 服务名
	CHAT_PROXY = "proxy"
	CHAT_SYNC_AGG = "sync_aggregate"
	CHAT_SYNC_SERVER = "sync_server"
	CHAT_FRONT = "front"

	// 请求方法
	// rpc请求
	METHOD_RPC = "rpc"
	// 一般请求
	METHOD_GENERAL = "general"
)

func exportHandleDurationRequest(serverName, method, path, code string, dur float64){
	handleDuration.WithLabelValues(serverName, Instance, method, path, code).Observe(dur)
}

func ExportGrpcRequestDuration(from, to, method, code string, dur float64){
	grpcDuration.WithLabelValues(from, to, Instance, method, code).Observe(dur)
}










