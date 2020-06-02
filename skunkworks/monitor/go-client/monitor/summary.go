package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

type summary struct {
	prometheus.Observer
	enable bool
}

func (s *summary) Observe(v float64) {
	if s.enable {
		s.Observer.Observe(v)
	}
}

type labeledSummary struct {
	*prometheus.SummaryVec
	enable bool
}

func (s *labeledSummary) WithLabelValues(lvs ...string) Summary {
	if s.enable {
		return &summary{s.SummaryVec.WithLabelValues(lvs...), true}
	} else {
		return &summary{enable: false}
	}
}

func (s *labeledSummary) With(labels Labels) Summary {
	if s.enable {
		return &summary{s.SummaryVec.With(prometheus.Labels(labels)), true}
	} else {
		return &summary{enable: false}
	}
}
