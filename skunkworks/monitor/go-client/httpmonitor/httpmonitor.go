package httpmonitor

import (
	"net/http"
	"strconv"
	"time"

	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
)

type responseWrapper struct {
	http.ResponseWriter
	status int
}

func (respW *responseWrapper) WriteHeader(code int) {
	respW.status = code
	respW.ResponseWriter.WriteHeader(code)
}

var monitor mon.Monitor

func init() {
	monitor = mon.GetInstance()
}

func Wrap(f http.HandlerFunc) http.HandlerFunc {

	var summary mon.LabeledSummary = monitor.NewLabeledSummary(
		"http_request_duration_seconds",
		[]string{"method", "path", "code"},
		map[float64]float64{0.5: 0.01, 0.9: 0.01, 0.99: 0.001, 0.999: 0.0001},
	)

	var histogram mon.LabeledHistogram = monitor.NewLabeledHistogram(
		"http_request_buckets_seconds",
		[]string{"method", "path", "code"},
		[]float64{0.05, 0.1, 0.5, 1, 2, 5},
	)

	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		respW := responseWrapper{w, 200}
		f(&respW, req)
		duration := float64(time.Since(start)) / float64(time.Second)

		method := req.Method
		path := req.URL.Path
		code := strconv.Itoa(respW.status)

		summary.WithLabelValues(method, path, code).Observe(duration)
		histogram.WithLabelValues(method, path, code).Observe(duration)
	}
}

func Wrap2(f http.HandlerFunc) http.HandlerFunc {
	/*var histogram mon.LabeledHistogram = monitor.NewLabeledHistogram(
		"http_requests_duration_seconds",
		[]string{"method", "path", "code"},
		[]float64{0.05, 0.1, 0.5, 1, 2, 5},
	)

	var counter mon.LabeledCounter = monitor.NewLabeledCounter(
		"http_requests_total",
		[]string{"method", "path", "code"},
	)*/

	return func(w http.ResponseWriter, req *http.Request) {
		//start := time.Now()

		respW := responseWrapper{w, 200}
		f(&respW, req)

		/*duration := float64(time.Since(start)) / float64(time.Second)

		method := req.Method
		path := req.URL.Path
		code := strconv.Itoa(respW.status)

		if method != "OPTION" {
			histogram.WithLabelValues(method, path, code).Observe(duration)
			counter.WithLabelValues(method, path, code).Inc()
		}*/
	}
}
