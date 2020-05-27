package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

type histogram struct {
	prometheus.Observer
	enable bool
}

func (h *histogram) Observe(v float64) {
	if h.enable {
		h.Observer.Observe(v)
	}
}

type labeledHistogram struct {
	*prometheus.HistogramVec
	enable bool
}

func (h *labeledHistogram) WithLabelValues(lvs ...string) Histogram {
	if h.enable {
		return &histogram{h.HistogramVec.WithLabelValues(lvs...), true}
	} else {
		return &histogram{enable: false}
	}
}

func (h *labeledHistogram) With(labels Labels) Histogram {
	if h.enable {
		return &histogram{h.HistogramVec.With(prometheus.Labels(labels)), true}
	} else {
		return &histogram{enable: false}
	}
}
