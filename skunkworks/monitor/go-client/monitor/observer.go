package monitor

// Observe calls f(value). It implements Observer.
func (f ObserverFunc) Observe(v float64) {
	f(v)
}
