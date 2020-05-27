package monitor

type Labels map[string]string

type Monitor interface {
	// NewCounter creates a new Counter based on the provided metric name.
	NewCounter(metirc string) Counter
	// NewLabeledCounter creates a new LabeledCounter based on the provided metric name and
	// partitioned by the given label names. At least one label name must be
	// provided.
	NewLabeledCounter(metirc string, labelNames []string) LabeledCounter

	// NewGauge creates a new Gauge based on the provided metric name.
	NewGauge(metirc string) Gauge
	// NewLabeledGauge creates a new LabeledGauge based on the provided metric name and
	// partitioned by the given label names. At least one label name must be
	// provided.
	NewLabeledGauge(metirc string, labelNames []string) LabeledGauge

	// NewSummary creates a new Summary based on the provided metric name.
	//
	// Quantile defines the quantile rank estimates with their respective
	// absolute error. If Objectives[q] = e, then the value reported for q
	// will be the φ-quantile value for some φ between q-e and q+e.  The
	// default value is DefObjectives. It is used if Objectives is left at
	// its zero value (i.e. nil). To create a Summary without Objectives,
	// set it to an empty map (i.e. map[float64]float64{}). The default value is
	// {0.5: 0.05, 0.9: 0.01, 0.99: 0.001} if pass nil.
	NewSummary(metirc string, quantile map[float64]float64) Summary
	// NewLabeledSummary creates a new LabeledSummary based on the provided metric name and
	// partitioned by the given label names. At least one label name must be
	// provided.
	NewLabeledSummary(metirc string, labelNames []string, quantile map[float64]float64) LabeledSummary

	// NewHistogram creates a new Histogram based on the provided metric name.
	//
	// Buckets defines the buckets into which observations are counted. Each
	// element in the slice is the upper inclusive bound of a bucket. The
	// values must be sorted in strictly increasing order. There is no need
	// to add a highest bucket with +Inf bound, it will be added
	// implicitly. The default value is {.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	// if pass nil.
	NewHistogram(metirc string, buckets []float64) Histogram
	// NewLabeledHistogram creates a new LabeledHistogram based on the provided metric name and
	// partitioned by the given label names. At least one label name must be
	// provided.
	NewLabeledHistogram(metirc string, labelNames []string, buckets []float64) LabeledHistogram

	// NewTimer creates a new Timer. The provided Observer is used to observe a
	// duration in seconds. Timer is usually used to time a function call in the
	// following way:
	//    func TimeMe() {
	//        timer := NewTimer(myHistogram)
	//        defer timer.ObserveDuration()
	//        // Do actual work.
	//    }
	NewTimer(o Observer) Timer
}

type Timer interface {
	// ObserveDuration records the duration passed since the Timer was created with
	// NewTimer. It calls the Observe method of the Observer provided during
	// construction with the duration in seconds as an argument. ObserveDuration is
	// usually called with a defer statement.
	ObserveDuration()
}

// Observer is the interface that wraps the Observe method, which is used by
// Histogram and Summary to add observations.
type Observer interface {
	Observe(float64)
}

// The ObserverFunc type is an adapter to allow the use of ordinary
// functions as Observers. If f is a function with the appropriate
// signature, ObserverFunc(f) is an Observer that calls f.
//
// This adapter is usually used in connection with the Timer type, and there are
// two general use cases:
//
// The most common one is to use a Gauge as the Observer for a Timer.
// See the "Gauge" Timer example.
//
// The more advanced use case is to create a function that dynamically decides
// which Observer to use for observing the duration. See the "TestTimerComplex"
// Timer example.
type ObserverFunc func(float64)

type Counter interface {
	// Inc increments the counter by 1. Use Add to increment it by arbitrary
	// non-negative values.
	Inc()
	// Add adds the given value to the counter. It panics if the value is <
	// 0.
	Add(float64)
}

type LabeledCounter interface {
	// WithLabelValues allows shortcuts like
	// counter.WithLabelValues("404", "GET").Add(42)
	WithLabelValues(lvs ...string) Counter
	// With allows shortcuts like
	// counter.With(Labels{"code": "404", "method": "GET"}).Add(42)
	With(labels Labels) Counter
}

type Gauge interface {
	// Set sets the Gauge to an arbitrary value.
	Set(float64)
	// Inc increments the Gauge by 1. Use Add to increment it by arbitrary
	// values.
	Inc()
	// Dec decrements the Gauge by 1. Use Sub to decrement it by arbitrary
	// values.
	Dec()
	// Add adds the given value to the Gauge. (The value can be negative,
	// resulting in a decrease of the Gauge.)
	Add(float64)
	// Sub subtracts the given value from the Gauge. (The value can be
	// negative, resulting in an increase of the Gauge.)
	Sub(float64)

	// SetToCurrentTime sets the Gauge to the current Unix time in seconds.
	SetToCurrentTime()
}

type LabeledGauge interface {
	// WithLabelValues allows shortcuts like
	// gauge.WithLabelValues("404", "GET").Add(42)
	WithLabelValues(lvs ...string) Gauge
	// With allows shortcuts like
	// gauge.With(Labels{"code": "404", "method": "GET"}).Add(42)
	With(labels Labels) Gauge
}

type Summary interface {
	// Observe adds a single observation to the summary.
	Observe(float64)
}

type LabeledSummary interface {
	// WithLabelValues allows shortcuts like
	// summary.WithLabelValues("404", "GET").Observe(42.21)
	WithLabelValues(lvs ...string) Summary
	// With allows shortcuts like
	// summary.With(Labels{"code": "404", "method": "GET"}).Observe(42.21)
	With(labels Labels) Summary
}

type Histogram interface {
	// Observe adds a single observation to the histogram.
	Observe(float64)
}

type LabeledHistogram interface {
	// WithLabelValues allows shortcuts like
	// histogram.WithLabelValues("404", "GET").Observe(42.21)
	WithLabelValues(lvs ...string) Histogram
	// With allows shortcuts like
	// histogram.With(Labels{"code": "404", "method": "GET"}).Observe(42.21)
	With(labels Labels) Histogram
}
