package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

type counter struct {
	prometheus.Counter
	enable bool
}

func (c *counter) Inc() {
	if c.enable {
		c.Counter.Inc()
	}
}

func (c *counter) Add(v float64) {
	if c.enable {
		c.Counter.Add(v)
	}
}

type labeledCounter struct {
	*prometheus.CounterVec
	enable bool
}

func (c *labeledCounter) WithLabelValues(lvs ...string) Counter {
	if c.enable {
		return &counter{c.CounterVec.WithLabelValues(lvs...), true}
	} else {
		return &counter{enable: false}
	}
}

func (c *labeledCounter) With(labels Labels) Counter {
	if c.enable {
		return &counter{c.CounterVec.With(prometheus.Labels(labels)), true}
	} else {
		return &counter{enable: false}
	}
}
