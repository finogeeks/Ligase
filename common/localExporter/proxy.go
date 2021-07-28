package localExporter

func ExportProxyHttpRequest(method, path, code string){
	httpRequest.WithLabelValues(CHAT_PROXY, Instance, method, path, code).Add(1)
}

func ExportProxyHttpDurationRequest(method, path, code string, dur float64){
	if path == "initial_sync" || path == "sync" {
		return
	}
	httpDuration.WithLabelValues(CHAT_PROXY, Instance, method, path, code).Observe(dur)
}

func ExportProxyHandleDurationRequest(method, path, code string, dur float64){
	exportHandleDurationRequest(CHAT_PROXY, method, path, code, dur)
}
