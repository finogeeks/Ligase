package common

import (
	"strconv"
	"time"

	"github.com/finogeeks/ligase/skunkworks/log"
)

type TimeSpend struct {
	start int64
	end   int64
}

func NewTimeSpend() *TimeSpend {
	return &TimeSpend{
		start: time.Now().UnixNano(),
	}
}

func (t *TimeSpend) Reset() *TimeSpend {
	t.start = time.Now().UnixNano()
	t.end = 0
	return t
}

func (t *TimeSpend) setEnd() *TimeSpend {
	t.end = time.Now().UnixNano()
	return t
}

func (t *TimeSpend) MS() int64 {
	t.setEnd()
	return (t.end - t.start) / 1000000
}

func (t *TimeSpend) Logf(exceed int64, format string, p ...interface{}) {
	spend := t.MS()
	if spend > exceed {
		log.Warnf(format+" spend "+strconv.FormatInt(spend, 10)+"ms exceed", p...)
	} else {
		log.Infof(format+" spend "+strconv.FormatInt(spend, 10)+"ms", p...)
	}
}

func (t *TimeSpend) WarnIfExceedf(exceed int64, format string, p ...interface{}) {
	spend := t.MS()
	if spend > exceed {
		log.Warnf(format+" spend "+strconv.FormatInt(spend, 10)+"ms exceed", p...)
	}
}
