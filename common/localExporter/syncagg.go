package localExporter

const (

)

func ExportSyncAggHttpDurationRequest(method, path, code string, dur float64){
	httpDuration.WithLabelValues(CHAT_SYNC_AGG, Instance, method, path, code).Observe(dur)
}

func ExportSyncAggHandleDurationRequest(method, path, code string, dur float64){
	exportHandleDurationRequest(CHAT_SYNC_AGG, method, path, code, dur)
}
