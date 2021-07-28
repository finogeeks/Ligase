package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"sync"
)

type promMonitor struct {
	enable bool
}

// Singleton
var once sync.Once
var instance Monitor

func GetInstance() Monitor {
	once.Do(func() {
		instance = newPrometheus()
	})
	return instance
}

// filter old mon data
func filter(metric string) bool {
	if metric == "dendrite_dbop_counter" ||
		metric == "syncaggreate_query_hit" ||
		metric == "syncserver_query_hit" ||
		metric == "roomserver_query_hit" ||
		metric == "room_event_process_duration_millisecond" ||
		metric == "dendrite_kafka_counter" ||
		metric == "syncwriter_query_hit" ||
		metric == "storage_query_duration_millisecond" {
		return false
	}
	return true
}

func newPrometheus() *promMonitor {
	e := os.Getenv("ENABLE_MONITOR")
	if "true" == e {
		log.Println("enabled monitor")
		port := ":9092"
		path := "/metrics"

		p := os.Getenv("MONITOR_PORT")
		if "" != p {
			port = ":" + p
		}

		pa := os.Getenv("MONITOR_PATH")
		if "" != pa {
			path = pa
		}

		go func() {
			http.Handle(path, promhttp.Handler())
			log.Fatal(http.ListenAndServe(port, nil))
		}()

		return &promMonitor{
			enable: true,
		}
	} else {
		return &promMonitor{
			enable: false,
		}
	}
}

func (prom *promMonitor) NewCounter(metirc string) Counter {
	if prom.enable && filter(metirc) {
		cnt := prometheus.NewCounter(prometheus.CounterOpts{
			Name: metirc,
			Help: "nothing",
		})

		prometheus.MustRegister(cnt)
		return &counter{cnt, true}
	} else {
		return &counter{enable: false}
	}
}

func (prom *promMonitor) NewLabeledCounter(metirc string, labelNames []string) LabeledCounter {
	if prom.enable && filter(metirc) {
		cnt := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metirc,
				Help: "nothing",
			},
			labelNames,
		)

		prometheus.MustRegister(cnt)
		return &labeledCounter{cnt, true}
	} else {
		return &labeledCounter{enable: false}
	}
}

func (prom *promMonitor) NewGauge(metirc string) Gauge {
	if prom.enable && filter(metirc){
		gau := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: metirc,
			Help: "nothing",
		})

		prometheus.MustRegister(gau)
		return &gauge{gau, true}
	} else {
		return &gauge{enable: false}
	}
}

func (prom *promMonitor) NewLabeledGauge(metirc string, labelNames []string) LabeledGauge {
	if prom.enable && filter(metirc) {
		gau := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metirc,
				Help: "nothing",
			},
			labelNames,
		)

		prometheus.MustRegister(gau)
		return &labeledGauge{gau, true}
	} else {
		return &labeledGauge{enable: false}
	}
}

func (prom *promMonitor) NewSummary(metirc string, quantile map[float64]float64) Summary {
	if prom.enable && filter(metirc) {
		sum := prometheus.NewSummary(prometheus.SummaryOpts{
			Name:       metirc,
			Help:       "nothing",
			Objectives: quantile,
		})

		prometheus.MustRegister(sum)
		return &summary{sum, true}
	} else {
		return &summary{enable: false}
	}
}

func (prom *promMonitor) NewLabeledSummary(metirc string, labelNames []string, quantile map[float64]float64) LabeledSummary {
	if prom.enable && filter(metirc) {
		sum := prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       metirc,
				Help:       "nothing",
				Objectives: quantile,
			},
			labelNames,
		)

		prometheus.MustRegister(sum)
		return &labeledSummary{sum, true}
	} else {
		return &labeledSummary{enable: false}
	}
}

func (prom *promMonitor) NewHistogram(metirc string, buckets []float64) Histogram {
	if prom.enable && filter(metirc) {
		his := prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    metirc,
			Help:    "nothing",
			Buckets: buckets,
		})

		prometheus.MustRegister(his)
		return &histogram{his, true}
	} else {
		return &histogram{enable: false}
	}
}

func (prom *promMonitor) NewLabeledHistogram(metirc string, labelNames []string, buckets []float64) LabeledHistogram {
	if prom.enable && filter(metirc) {
		his := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    metirc,
				Help:    "nothing",
				Buckets: buckets,
			},
			labelNames,
		)

		prometheus.MustRegister(his)
		return &labeledHistogram{his, true}
	} else {
		return &labeledHistogram{enable: false}
	}
}

func (prom *promMonitor) NewTimer(o Observer) Timer {
	if prom.enable {
		tim := prometheus.NewTimer(o)
		return &timer{tim, true}
	} else {
		return &timer{enable: false}
	}
}
