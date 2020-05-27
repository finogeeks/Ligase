package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

type timer struct {
	*prometheus.Timer
	enable bool
}

func (t *timer) ObserveDuration() {
	if t.enable {
		t.Timer.ObserveDuration()
	}
}
