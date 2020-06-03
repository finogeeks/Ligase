package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

type gauge struct {
	prometheus.Gauge
	enable bool
}

func (g *gauge) Set(v float64) {
	if g.enable {
		g.Gauge.Set(v)
	}
}

func (g *gauge) Inc() {
	if g.enable {
		g.Gauge.Inc()
	}
}

func (g *gauge) Dec() {
	if g.enable {
		g.Gauge.Dec()
	}
}

func (g *gauge) Add(v float64) {
	if g.enable {
		g.Gauge.Add(v)
	}
}

func (g *gauge) Sub(v float64) {
	if g.enable {
		g.Gauge.Sub(v)
	}
}

func (g *gauge) SetToCurrentTime() {
	if g.enable {
		g.Gauge.SetToCurrentTime()
	}
}

type labeledGauge struct {
	*prometheus.GaugeVec
	enable bool
}

func (g *labeledGauge) WithLabelValues(lvs ...string) Gauge {
	if g.enable {
		return &gauge{g.GaugeVec.WithLabelValues(lvs...), true}
	} else {
		return &gauge{enable: false}
	}
}

func (g *labeledGauge) With(labels Labels) Gauge {
	if g.enable {
		return &gauge{g.GaugeVec.With(prometheus.Labels(labels)), true}
	} else {
		return &gauge{enable: false}
	}
}
