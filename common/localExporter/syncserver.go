package localExporter

func ExportSyncServerHandleDurationRequest(method, path, code string, dur float64){
	exportHandleDurationRequest(CHAT_SYNC_SERVER, method, path, code, dur)
}
