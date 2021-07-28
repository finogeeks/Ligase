package localExporter

func ExportFrontHandleDurationRequest(method, path, code string, dur float64){
	exportHandleDurationRequest(CHAT_FRONT, method, path, code, dur)
}
